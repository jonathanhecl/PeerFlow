package network

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/grandcat/zeroconf"
)

// P2PNode represents the physical node running the PeerFlow application
type P2PNode struct {
	PeerID    string
	Port      int
	server    *zeroconf.Server
	listeners []net.Listener
}

// NewP2PNode initializes a new peer node ready to connect
func NewP2PNode(peerID string) *P2PNode {
	return &P2PNode{
		PeerID: peerID,
	}
}

// Start begins advertising the peer on the LAN and listening for TCP connections
func (n *P2PNode) Start(ctx context.Context) error {
	// 1. Start TCP Listener on a dynamic available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	n.listeners = append(n.listeners, listener)

	// Extract the port that was assigned
	n.Port = listener.Addr().(*net.TCPAddr).Port
	log.Printf("[P2P] Listening for direct TCP connections on port: %d\n", n.Port)

	// 2. Start mDNS Advertising
	// We advertise the "_peerflow._tcp" service so other peers can find us
	n.server, err = zeroconf.Register(
		n.PeerID,           // Instance name (unique ID)
		"_peerflow._tcp",   // Service type
		"local.",           // Domain
		n.Port,             // Port
		[]string{"txtv=1"}, // TXT records for future metadata
		nil,                // Interfaces (nil means all valid network interfaces)
	)
	if err != nil {
		return fmt.Errorf("failed to register zeroconf mDNS service: %w", err)
	}
	log.Printf("[P2P] Announcing presence via mDNS (ID: %s) on LAN.\n", n.PeerID)

	// 3. Start mDNS Discovery (Looking for other peers)
	go n.discoverPeers(ctx)

	// 4. Handle Incoming TCP Connections
	go n.acceptConnections(ctx, listener)

	// Wait for context cancellation to clean up resources gracefully
	go func() {
		<-ctx.Done()
		log.Println("[P2P] Shutting down P2P node...")
		n.Stop()
	}()

	return nil
}

// Stop cleanly terminates the mDNS server and TCP listeners
func (n *P2PNode) Stop() {
	if n.server != nil {
		n.server.Shutdown()
	}
	for _, l := range n.listeners {
		l.Close()
	}
}

// acceptConnections loops infinitely (until closed) to accept peer handshakes
func (n *P2PNode) acceptConnections(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return // Normal shutdown
			default:
				log.Printf("[P2P] Error accepting connection: %v\n", err)
				continue
			}
		}

		log.Printf("[P2P] New incoming connection from %s\n", conn.RemoteAddr().String())
		// TODO: Implement reading the "Envelope" Protobuf message from 'conn'
		// and performing the Handshake/NoteSync process.
		// When a new note is parsed, use Wails' runtime.EventsEmit(ctx, "onNoteReceived", note)
		go n.handleConnection(ctx, conn)
	}
}

func (n *P2PNode) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	// Future implementation: Read from stream, decode Protobuf (Envelope), dispatch to appropriate handler
}

// discoverPeers actively seeks out other "_peerflow._tcp" services on the LAN
func (n *P2PNode) discoverPeers(ctx context.Context) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("[P2P] Failed to initialize mDNS resolver: %v\n", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)

	// Goroutine to process discovered peers
	go func() {
		for entry := range entries {
			// Ignore our own advertisement
			if entry.Instance == n.PeerID {
				continue
			}

			log.Printf("[P2P] Discovered new peer! %s at %s:%d\n", entry.Instance, entry.AddrIPv4[0], entry.Port)
			// TODO: Here we would trigger an outbound TCP dial to this peer and send a HANDSHAKE_REQ
			// e.g. net.Dial("tcp", entry.AddrIPv4[0].String() + ":" + strconv.Itoa(entry.Port))
		}
	}()

	err = resolver.Browse(ctx, "_peerflow._tcp", "local.", entries)
	if err != nil {
		log.Printf("[P2P] Failed to browse mDNS: %v\n", err)
	}
}
