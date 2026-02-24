package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	pb "PeerFlow/pkg/proto"

	"github.com/grandcat/zeroconf"
	"google.golang.org/protobuf/proto"
)

// PeerInfo holds metadata about a discovered peer
type PeerInfo struct {
	ID   string
	Addr string
	Port int
}

// P2PNode represents the local node in the P2P network
type P2PNode struct {
	PeerID string
	Port   int

	// Callbacks — set by app.go before calling Start()
	OnNoteReceived func(noteID string, content string, timestamp int64)
	OnPeerUpdate   func(peers []string)

	// Internal state
	server   *zeroconf.Server
	listener net.Listener
	peers    sync.Map // PeerID -> *peerConn
	note     string   // Current shared note content
	noteMu   sync.RWMutex
	noteTS   int64 // Last-write-wins timestamp
	ctx      context.Context
	cancel   context.CancelFunc
}

// peerConn wraps a TCP connection to a remote peer
type peerConn struct {
	info PeerInfo
	conn net.Conn
	mu   sync.Mutex // Serialize writes
}

// NewP2PNode initializes a new peer node
func NewP2PNode(peerID string) *P2PNode {
	return &P2PNode{
		PeerID: peerID,
	}
}

// Start begins the TCP listener, mDNS advertisement, and peer discovery
func (n *P2PNode) Start(parentCtx context.Context) error {
	n.ctx, n.cancel = context.WithCancel(parentCtx)

	// 1. Start TCP Listener
	var err error
	n.listener, err = net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	n.Port = n.listener.Addr().(*net.TCPAddr).Port
	log.Printf("[P2P] TCP listening on port %d\n", n.Port)

	// 2. Advertise via mDNS
	n.server, err = zeroconf.Register(
		n.PeerID,
		"_peerflow._tcp",
		"local.",
		n.Port,
		[]string{"v=1"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register mDNS: %w", err)
	}
	log.Printf("[P2P] mDNS advertising as %s\n", n.PeerID)

	// 3. Accept incoming connections
	go n.acceptLoop()

	// 4. Discover peers periodically
	go n.discoveryLoop()

	return nil
}

// Stop shuts down the node cleanly
func (n *P2PNode) Stop() {
	if n.cancel != nil {
		n.cancel()
	}
	if n.server != nil {
		n.server.Shutdown()
	}
	if n.listener != nil {
		n.listener.Close()
	}
	// Close all peer connections
	n.peers.Range(func(key, value any) bool {
		if pc, ok := value.(*peerConn); ok {
			pc.conn.Close()
		}
		n.peers.Delete(key)
		return true
	})
}

// SetNote updates the local note state (called from app.go on local edits)
func (n *P2PNode) SetNote(content string, ts int64) {
	n.noteMu.Lock()
	n.note = content
	n.noteTS = ts
	n.noteMu.Unlock()
}

// GetNote returns the current note state
func (n *P2PNode) GetNote() (string, int64) {
	n.noteMu.RLock()
	defer n.noteMu.RUnlock()
	return n.note, n.noteTS
}

// GetPeerIDs returns a snapshot of connected peer IDs
func (n *P2PNode) GetPeerIDs() []string {
	var ids []string
	n.peers.Range(func(key, value any) bool {
		ids = append(ids, key.(string))
		return true
	})
	return ids
}

// BroadcastNote sends a NoteSync message to all connected peers
func (n *P2PNode) BroadcastNote(content string, ts int64) {
	noteSync := &pb.NoteSync{
		NoteId:    "shared",
		Timestamp: ts,
		Content:   content,
	}
	payload, err := proto.Marshal(noteSync)
	if err != nil {
		log.Printf("[P2P] Failed to marshal NoteSync: %v\n", err)
		return
	}
	env := &pb.Envelope{
		Type:    pb.Envelope_NOTE_SYNC,
		Payload: payload,
	}
	n.broadcastEnvelope(env)
}

// --- Internal methods ---

func (n *P2PNode) acceptLoop() {
	for {
		conn, err := n.listener.Accept()
		if err != nil {
			select {
			case <-n.ctx.Done():
				return
			default:
				log.Printf("[P2P] Accept error: %v\n", err)
				continue
			}
		}
		go n.handleInbound(conn)
	}
}

func (n *P2PNode) discoveryLoop() {
	// Initial discovery
	n.browsePeers()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.browsePeers()
		}
	}
}

func (n *P2PNode) browsePeers() {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("[P2P] Resolver error: %v\n", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)
	browseCtx, browseCancel := context.WithTimeout(n.ctx, 3*time.Second)
	defer browseCancel()

	go func() {
		for entry := range entries {
			if entry.Instance == n.PeerID {
				continue
			}
			// Already connected?
			if _, loaded := n.peers.Load(entry.Instance); loaded {
				continue
			}

			var addr string
			if len(entry.AddrIPv4) > 0 {
				addr = entry.AddrIPv4[0].String()
			} else if len(entry.AddrIPv6) > 0 {
				addr = entry.AddrIPv6[0].String()
			} else {
				continue
			}

			log.Printf("[P2P] Discovered peer %s at %s:%d\n", entry.Instance, addr, entry.Port)
			go n.dialPeer(entry.Instance, addr, entry.Port)
		}
	}()

	_ = resolver.Browse(browseCtx, "_peerflow._tcp", "local.", entries)
	<-browseCtx.Done()
}

func (n *P2PNode) dialPeer(peerID, addr string, port int) {
	// Avoid duplicate connections
	if _, loaded := n.peers.Load(peerID); loaded {
		return
	}

	target := addr + ":" + strconv.Itoa(port)
	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		log.Printf("[P2P] Failed to dial %s: %v\n", peerID, err)
		return
	}

	pc := &peerConn{
		info: PeerInfo{ID: peerID, Addr: addr, Port: port},
		conn: conn,
	}
	n.peers.Store(peerID, pc)
	n.emitPeerUpdate()

	log.Printf("[P2P] Connected to peer %s\n", peerID)

	// Send handshake
	n.sendHandshake(pc)

	// Read messages from this peer
	n.readLoop(pc)
}

func (n *P2PNode) handleInbound(conn net.Conn) {
	// Read the first message — should be a handshake
	env, err := readEnvelope(conn)
	if err != nil {
		conn.Close()
		return
	}

	if env.Type != pb.Envelope_HANDSHAKE_REQ {
		conn.Close()
		return
	}

	hs := &pb.Handshake{}
	if err := proto.Unmarshal(env.Payload, hs); err != nil {
		conn.Close()
		return
	}

	peerID := hs.PeerId
	log.Printf("[P2P] Inbound handshake from %s\n", peerID)

	// If we already have a connection, skip
	if _, loaded := n.peers.Load(peerID); loaded {
		conn.Close()
		return
	}

	pc := &peerConn{
		info: PeerInfo{ID: peerID, Addr: conn.RemoteAddr().String()},
		conn: conn,
	}
	n.peers.Store(peerID, pc)
	n.emitPeerUpdate()

	// Process the handshake note timestamp and send our state back
	n.processHandshake(pc, hs)

	// Continue reading
	n.readLoop(pc)
}

func (n *P2PNode) sendHandshake(pc *peerConn) {
	content, ts := n.GetNote()
	noteTS := map[string]int64{}
	if ts > 0 {
		noteTS["shared"] = ts
	}
	hs := &pb.Handshake{
		PeerId:         n.PeerID,
		NoteTimestamps: noteTS,
	}
	payload, _ := proto.Marshal(hs)
	env := &pb.Envelope{
		Type:    pb.Envelope_HANDSHAKE_REQ,
		Payload: payload,
	}
	writeEnvelope(pc, env)

	// Also send our current note if we have content
	if content != "" {
		noteSync := &pb.NoteSync{
			NoteId:    "shared",
			Timestamp: ts,
			Content:   content,
		}
		nsPayload, _ := proto.Marshal(noteSync)
		nsEnv := &pb.Envelope{
			Type:    pb.Envelope_NOTE_SYNC,
			Payload: nsPayload,
		}
		writeEnvelope(pc, nsEnv)
	}
}

func (n *P2PNode) processHandshake(pc *peerConn, hs *pb.Handshake) {
	// Send our response with our note
	content, ts := n.GetNote()

	respHS := &pb.Handshake{
		PeerId:         n.PeerID,
		NoteTimestamps: map[string]int64{},
	}
	if ts > 0 {
		respHS.NoteTimestamps["shared"] = ts
	}
	payload, _ := proto.Marshal(respHS)
	respEnv := &pb.Envelope{
		Type:    pb.Envelope_HANDSHAKE_RES,
		Payload: payload,
	}
	writeEnvelope(pc, respEnv)

	// Send our note if we have newer content
	remoteTS := hs.NoteTimestamps["shared"]
	if content != "" && ts > remoteTS {
		noteSync := &pb.NoteSync{
			NoteId:    "shared",
			Timestamp: ts,
			Content:   content,
		}
		nsPayload, _ := proto.Marshal(noteSync)
		nsEnv := &pb.Envelope{
			Type:    pb.Envelope_NOTE_SYNC,
			Payload: nsPayload,
		}
		writeEnvelope(pc, nsEnv)
	}
}

func (n *P2PNode) readLoop(pc *peerConn) {
	defer func() {
		pc.conn.Close()
		n.peers.Delete(pc.info.ID)
		n.emitPeerUpdate()
		log.Printf("[P2P] Peer %s disconnected\n", pc.info.ID)
	}()

	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		env, err := readEnvelope(pc.conn)
		if err != nil {
			return
		}

		switch env.Type {
		case pb.Envelope_NOTE_SYNC:
			ns := &pb.NoteSync{}
			if err := proto.Unmarshal(env.Payload, ns); err != nil {
				continue
			}
			n.handleNoteSync(ns)

		case pb.Envelope_HANDSHAKE_RES:
			// Handshake response — just log it
			log.Printf("[P2P] Handshake response from %s\n", pc.info.ID)
		}
	}
}

func (n *P2PNode) handleNoteSync(ns *pb.NoteSync) {
	n.noteMu.Lock()
	// Last-Write-Wins: only accept if timestamp is newer
	if ns.Timestamp > n.noteTS {
		n.note = ns.Content
		n.noteTS = ns.Timestamp
		n.noteMu.Unlock()

		log.Printf("[P2P] Note updated from remote (ts=%d)\n", ns.Timestamp)
		if n.OnNoteReceived != nil {
			n.OnNoteReceived(ns.NoteId, ns.Content, ns.Timestamp)
		}
	} else {
		n.noteMu.Unlock()
	}
}

func (n *P2PNode) broadcastEnvelope(env *pb.Envelope) {
	n.peers.Range(func(key, value any) bool {
		pc := value.(*peerConn)
		if err := writeEnvelope(pc, env); err != nil {
			log.Printf("[P2P] Write error to %s: %v\n", pc.info.ID, err)
			pc.conn.Close()
			n.peers.Delete(key)
		}
		return true
	})
}

func (n *P2PNode) emitPeerUpdate() {
	if n.OnPeerUpdate != nil {
		n.OnPeerUpdate(n.GetPeerIDs())
	}
}

// --- Wire protocol: [4 bytes big-endian length][protobuf envelope] ---

func writeEnvelope(pc *peerConn, env *pb.Envelope) error {
	data, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))

	if _, err := pc.conn.Write(lenBuf); err != nil {
		return err
	}
	_, err = pc.conn.Write(data)
	return err
}

func readEnvelope(conn net.Conn) (*pb.Envelope, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen > 10*1024*1024 { // 10MB safety limit
		return nil, fmt.Errorf("message too large: %d bytes", msgLen)
	}

	data := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	env := &pb.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, err
	}
	return env, nil
}
