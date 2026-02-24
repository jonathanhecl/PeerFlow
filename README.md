# PeerFlow

A decentralized, real-time shared notepad that works entirely over LAN. No servers, no cloud, no external dependencies — pure Peer-to-Peer.

Every instance is both a server and a client. When you open PeerFlow on multiple machines in the same network, they automatically discover each other via mDNS and sync notes in real-time over TCP using Protocol Buffers.

## Features

- **Zero Configuration** — Launch and go. Peers are discovered automatically via mDNS/ZeroConf.
- **Real-Time Sync** — Notes update across all connected peers with ~300ms debounce.
- **Last-Write-Wins** — Simple conflict resolution using millisecond timestamps.
- **Peer Visibility** — See your own UUID and all connected peers in the sidebar.
- **Dark UI** — Glassmorphism design with JetBrains Mono editor font.
- **No CGO** — Pure Go SQLite (`modernc.org/sqlite`) planned for persistence, ensuring easy cross-compilation.

## Tech Stack

| Layer | Technology |
|---|---|
| Desktop Framework | [Wails v2](https://wails.io) |
| Backend | Go |
| Frontend | Vanilla JS + Vite |
| Serialization | [Protocol Buffers](https://protobuf.dev) |
| Discovery | mDNS via [`grandcat/zeroconf`](https://github.com/grandcat/zeroconf) |
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
│   ├── network/        # P2P engine (mDNS, TCP, handshake, sync)
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
┌──────────────┐         mDNS          ┌──────────────┐
│   PeerFlow   │◄──── Discovery ──────►│   PeerFlow   │
│   Node A     │                       │   Node B     │
│              │◄──── TCP + Proto ────►│              │
│  :random_port│      (Envelope)       │ :random_port │
└──────────────┘                       └──────────────┘
```

1. Each instance binds a TCP listener on a random port.
2. Announces itself via `_peerflow._tcp` mDNS service.
3. Discovers peers every 5 seconds and dials new ones.
4. Handshake exchanges note timestamps; newer content wins.
5. Edits are broadcast as `NoteSync` messages to all connected peers.

## Roadmap

- [ ] **SQLite Persistence** — Save notes locally using `modernc.org/sqlite` (pure Go, no CGO).
- [ ] **Multiple Notes** — Support for creating, listing, and switching between notes.
- [ ] **Clipboard Sharing** — Share text and images from the system clipboard across peers.
- [ ] **File Transfer** — Send large files between peers using chunked `FileTransfer` messages (already defined in the Protobuf schema).
- [ ] **Peer Nicknames** — Allow peers to set a display name instead of showing raw UUIDs.
- [ ] **Encryption** — TLS over the TCP connections for secure LAN communication.
- [ ] **CI/CD** — GitHub Actions workflow for automated Windows + macOS builds.

## License

GPL-3.0 — See [LICENSE](LICENSE) for details.