# PeerFlow

A cross-platform, decentralized, real-time shared notepad that works entirely over your local network. No servers, no cloud, no external dependencies — pure Peer-to-Peer. Runs on **Windows**, **macOS**, and **Linux**.

Every instance is both a server and a client. When you open PeerFlow on multiple machines, they automatically discover each other and sync notes in real-time over TCP using Protocol Buffers.

## Features

- **Cross-Platform** — Native desktop app for Windows, macOS (ARM64/Intel), and Linux.
- **Zero Configuration** — Launch and go. Peers are discovered automatically.
- **Cross-Subnet Discovery** — Works across different subnets in your home network (WiFi ↔ LAN through multiple routers).
- **Real-Time Sync** — Notes update across all connected peers with ~300ms debounce.
- **Last-Write-Wins** — Simple conflict resolution using millisecond timestamps.
- **Peer Visibility** — See your own UUID and all connected peers in the sidebar.
- **Disconnect Detection** — Peers are removed from the list when they go offline (TCP keepalive + ping mechanism).
- **Manual Connect** — Connect to peers on any reachable IP by entering their address manually.
- **Dark UI** — Glassmorphism design with JetBrains Mono editor font.

## Tech Stack

| Layer | Technology |
|---|---|
| Desktop Framework | [Wails v2](https://wails.io) |
| Backend | Go |
| Frontend | Vanilla JS + Vite |
| Serialization | [Protocol Buffers](https://protobuf.dev) |
| Discovery (same subnet) | mDNS via [`grandcat/zeroconf`](https://github.com/grandcat/zeroconf) |
| Discovery (cross-subnet) | UDP Multicast (`239.255.77.77:49154`, TTL=4) |
| Transport | TCP with 4-byte length-prefixed framing |

## Project Structure

```
PeerFlow/
├── build/              # Wails build assets (icons, installer config)
├── frontend/           # Vanilla JS + Vite frontend
│   └── src/
│       ├── main.js     # UI logic, Wails event listeners
│       └── style.css   # Dark theme with glassmorphism
├── pkg/
│   ├── network/        # P2P engine (mDNS, multicast, TCP, handshake, sync)
│   └── proto/          # Compiled Protobuf Go files
├── proto/
│   └── peerflow.proto  # Protobuf schema (Envelope, Handshake, NoteSync)
├── app.go              # Wails app struct, bound methods, event emission
├── main.go             # Entry point, window configuration
├── build.sh            # Cross-platform build script
├── buf.yaml            # Buf configuration
└── buf.gen.yaml        # Buf codegen configuration
```

## Getting Started

### Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- [Node.js 18+](https://nodejs.org)

### Development

```bash
wails dev
```

### Build

```bash
# Windows
wails build -platform windows/amd64 -trimpath

# macOS Apple Silicon (must be run on a Mac)
wails build -platform darwin/arm64 -trimpath
```

Or use the included script:

```bash
bash build.sh
```

## Architecture

```
┌──────────────┐    mDNS (same subnet)    ┌──────────────┐
│   PeerFlow   │◄────── Discovery ───────►│   PeerFlow   │
│   Node A     │                          │   Node B     │
│              │   UDP Multicast (cross)  │              │
│              │◄────── Discovery ───────►│              │
│              │                          │              │
│              │◄───── TCP + Proto ──────►│              │
│  :random_port│       (Envelope)         │ :random_port │
└──────────────┘                          └──────────────┘
```

1. Each instance binds a TCP listener on a random port.
2. Announces itself via `_peerflow._tcp` mDNS service (same subnet).
3. Sends UDP multicast beacons every 5s to `239.255.77.77:49154` with TTL=4 (cross-subnet).
4. Discovers peers from both mDNS and multicast, dials new ones via TCP.
5. Handshake exchanges note timestamps; newer content wins.
6. Edits are broadcast as `NoteSync` messages to all connected peers.
7. TCP keepalive (15s) and read timeouts (30s) with ping probes detect disconnected peers.

## Roadmap

- [ ] **Private Sessions** — Support for network isolation via passwords or group IDs, allowing multiple distinct PeerFlow networks to coexist on the same LAN.
- [ ] **Clipboard Sharing** — Share text and images from the system clipboard across peers.
- [ ] **Encryption** — TLS over the TCP connections for secure LAN communication.
- [ ] **Peer Nicknames** — Allow peers to set a display name instead of showing raw UUIDs.
- [ ] **File Transfer** — Send large files between peers using chunked `FileTransfer` messages (already defined in the Protobuf schema).
- [ ] **SQLite Persistence** — Save notes locally using `modernc.org/sqlite` (pure Go, no CGO).
- [ ] **Multiple Notes** — Support for creating, listing, and switching between notes.

## License

GPL-3.0 — See [LICENSE](LICENSE) for details.