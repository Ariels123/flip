# FLIP2 - Multi-Agent Coordination System

FLIP2 is a complete rewrite of FLIP with PocketBase backend, proper daemonization, and peer-to-peer synchronization.

## Features

- **PocketBase Backend**: Built-in REST API, realtime subscriptions, admin UI
- **HTTPS/TLS Support**: Native TLS with self-signed or CA certificates
- **Peer-to-Peer Sync**: Distributed coordination across multiple nodes using vector clocks
- **Proper Daemon**: systemd/launchd support, PID management, auto-restart
- **MySQL Support**: Production-ready database option (SQLite default)
- **Migration Tool**: Import data from FLIP v1

## Quick Start

```bash
# Build
go build -o flip2 ./cmd/flip2
go build -o flip2d ./cmd/flip2d

# Start daemon
./flip2 start

# Check status
./flip2 status

# Open admin UI
./flip2 admin
```

## Architecture

```
                          ┌─────────────────────────────────────┐
                          │          Peer Network               │
                          │  (Vector Clock Synchronization)     │
                          └──────────────┬──────────────────────┘
                                         │
┌──────────────┐     ┌──────────────┐    │    ┌──────────────┐
│   flip2      │────>│   flip2d     │<───┼───>│   flip2d     │
│   (CLI)      │     │  (Daemon)    │    │    │  (Peer Node) │
└──────────────┘     └──────┬───────┘    │    └──────────────┘
                            │            │
                     ┌──────┴───────┐    │
                     │  PocketBase  │<───┘
                     │  (REST API)  │
                     │  HTTPS/TLS   │
                     └──────┬───────┘
                            │
                     ┌──────┴───────┐
                     │   Database   │
                     │ SQLite/MySQL │
                     └──────────────┘
```

## Commands

### Daemon Control
```bash
flip2 start     # Start daemon
flip2 stop      # Stop daemon
flip2 restart   # Restart daemon
flip2 status    # Show status
```

### Task Management
```bash
flip2 task list
flip2 task add "Title" --assignee claude
flip2 task start <id>
flip2 task done <id>
```

### Agent Management
```bash
flip2 agent list
flip2 agent spawn <id> <backend> <prompt>
```

### Migration
```bash
flip2 migrate --from /path/to/flip.db
```

## Configuration

Copy `config/config.yaml.example` to `config/config.yaml` and modify:

```yaml
flip2:
  pocketbase:
    host: 0.0.0.0
    port: 8091
    data_dir: ./pb_data
    tls:
      enabled: true
      cert_file: ./certs/flip2.crt
      key_file: ./certs/flip2.key

  sync:
    enabled: true
    node_id: my-node
    sync_interval: 15s
    peers:
      - id: peer-node
        url: https://192.168.1.x:8091
        api_key: your_peer_api_key
        enabled: true
```

## HTTPS/TLS Setup

See [docs/HTTPS_IMPLEMENTATION.md](docs/HTTPS_IMPLEMENTATION.md) for detailed TLS setup instructions.

### Quick Certificate Generation

```bash
# Create certs directory
mkdir -p certs

# Generate self-signed ECDSA certificate (valid 1 year)
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-384 \
  -days 365 -nodes \
  -keyout certs/flip2.key \
  -out certs/flip2.crt \
  -subj "/C=US/O=FLIP2/CN=your-hostname" \
  -addext "subjectAltName=DNS:localhost,DNS:your-hostname,IP:127.0.0.1,IP:YOUR_IP"
```

## PocketBase Admin

Access the admin UI at:
- **HTTPS**: `https://localhost:8091/_/`
- **HTTP**: `http://localhost:8091/_/`

## Service Installation

### macOS (launchd)
```bash
sudo cp config/com.flip.flip2d.plist /Library/LaunchDaemons/
sudo launchctl load /Library/LaunchDaemons/com.flip.flip2d.plist
```

### Linux (systemd)
```bash
sudo cp config/flip2.service /etc/systemd/system/
sudo systemctl enable flip2
sudo systemctl start flip2
```

## Migration from FLIP v1

```bash
# Stop old FLIP
./flip ws stop

# Migrate data
flip2 migrate --from /path/to/ProjectDocs/LLMcomms/flip.db

# Start FLIP2
flip2 start

# Verify
flip2 task list
flip2 agent list
```

## Peer-to-Peer Synchronization

FLIP2 supports distributed coordination across multiple nodes using vector clocks for conflict-free replication.

### How It Works

1. **Vector Clocks**: Each node maintains a vector clock for causality tracking
2. **Bidirectional Sync**: Nodes push and pull changes at configurable intervals
3. **JWT Authentication**: Peer communication uses API key authentication
4. **Self-Signed Certs**: Peers can connect over HTTPS with `InsecureSkipVerify` for self-signed certificates

### Current Deployment

| Node | Address | Protocol | Status |
|------|---------|----------|--------|
| Mac | 192.168.1.53:8091 | HTTPS | Active |
| Windows | 192.168.1.220:8091 | HTTP | Active |

Sync interval: 15 seconds

## Development

```bash
# Run daemon in foreground
FLIP2_FOREGROUND=1 ./flip2d serve --config config/config.yaml

# Run with live reload
go run ./cmd/flip2d serve --config config/config.yaml

# Run tests
go test ./...

# Test HTTPS connectivity
curl -sk https://localhost:8091/api/health
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/metrics` | GET | System metrics |
| `/_/` | GET | PocketBase Admin UI |

## License

MIT
