package main

import (
	"context"
	"log"
	"time"

	"PeerFlow/pkg/network"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx  context.Context
	node *network.P2PNode
}

// NewApp creates a new App application struct
func NewApp() *App {
	peerID := "peer-" + uuid.New().String()[:8]
	return &App{
		node: network.NewP2PNode(peerID),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Wire callbacks before starting the node
	a.node.OnNoteReceived = func(noteID, content string, timestamp int64) {
		runtime.EventsEmit(a.ctx, "onNoteReceived", map[string]interface{}{
			"content":   content,
			"timestamp": timestamp,
		})
	}
	a.node.OnPeerUpdate = func(peers []string) {
		runtime.EventsEmit(a.ctx, "onPeerUpdate", peers)
	}

	log.Printf("Starting P2P Node %s...\n", a.node.PeerID)
	if err := a.node.Start(ctx); err != nil {
		log.Printf("Failed to start P2P node: %v\n", err)
	}
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	log.Println("Shutting down P2P Node...")
	a.node.Stop()
}

// GetPeerID returns our unique Peer ID (bound to frontend)
func (a *App) GetPeerID() string {
	return a.node.PeerID
}

// GetPeers returns the list of currently connected peer IDs (bound to frontend)
func (a *App) GetPeers() []string {
	peers := a.node.GetPeerIDs()
	if peers == nil {
		return []string{}
	}
	return peers
}

// GetNoteContent returns the current shared note content (bound to frontend)
func (a *App) GetNoteContent() string {
	content, _ := a.node.GetNote()
	return content
}

// UpdateNote is called by the frontend when the user edits the note
func (a *App) UpdateNote(content string) {
	ts := time.Now().UnixMilli()
	a.node.SetNote(content, ts)
	a.node.BroadcastNote(content, ts)
}
