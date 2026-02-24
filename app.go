package main

import (
	"context"
	"fmt"
	"log"

	"PeerFlow/pkg/network"

	"github.com/google/uuid"
)

// App struct
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

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
