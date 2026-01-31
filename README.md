# Airgapper

**Consensus-based encrypted backups with ransomware protection**

Airgapper is a control plane for peer-to-peer NAS backups where no single party can read, delete, or restore the data alone. It wraps [restic](https://restic.net) for encrypted backups and uses Shamir's Secret Sharing to split the decryption key between parties.

## Project Structure

```
airgapper/
├── .github/           # GitHub Actions workflows
├── frontend/          # React + Vite web UI
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── backend/           # Go backend
│   ├── cmd/airgapper/ # CLI entrypoint
│   ├── internal/      # Internal packages
│   ├── pkg/           # Public packages
│   └── go.mod
├── docker/            # Docker configuration
├── docs/              # Documentation
├── examples/          # Example configurations
├── docker-compose.yml # Full stack compose
├── Makefile           # Build commands
└── README.md
```

## The Problem

Traditional backups have a critical flaw: if your machine is compromised (ransomware, theft, malicious admin), the attacker can often:
- **Read** your backups (they have the encryption key)
- **Delete** your backups (they have access credentials)
- **Restore** malicious versions over your data

## The Solution

Airgapper provides **defense in depth**:

| Attack | Defense |
|--------|---------|
| Read backup data | Client-side encryption (AES-256) |
| Delete/corrupt backups | Append-only storage (restic-rest-server) |
| Restore without authorization | Consensus-based key release (2-of-2 SSS) |

**Key innovation:** Your backup password is split using Shamir's Secret Sharing. You keep one share, your trusted peer keeps another. Neither can decrypt alone. To restore, both must agree.

## Quick Start

### Prerequisites

- [restic](https://restic.net/installation/) installed
- [Go 1.21+](https://golang.org/dl/) (for building from source)
- [Node.js 20+](https://nodejs.org/) (for frontend development)
- A peer (friend, family member, colleague) willing to hold your key share
- (Optional) [restic-rest-server](https://github.com/restic/rest-server) for append-only storage

### Installation

```bash
# Clone the repository
git clone https://github.com/lcrostarosa/airgapper.git
cd airgapper

# Build everything
make build

# Or build just the backend
make backend-build

# The binary is at ./bin/airgapper
./bin/airgapper --help
```

### Development

```bash
# Install frontend dependencies
make frontend-install

# Run frontend dev server (http://localhost:5173)
make frontend-dev

# Run backend in dev mode (http://localhost:8080)
cd backend && go run ./cmd/airgapper serve --addr :8080

# Run both together
make dev

# Run all tests
make test
```

### Docker

```bash
# Build the Docker image
make docker

# Run full stack (storage + backend + frontend)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

## Workflow Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     AIRGAPPER FLOW                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  SETUP (once):                                              │
│    Alice                              Bob                   │
│    ┌──────────────────┐              ┌──────────────────┐   │
│    │ airgapper init   │  ──share──▶  │ airgapper join   │   │
│    │ (creates repo,   │              │ (stores share)   │   │
│    │  splits key)     │              │                  │   │
│    └──────────────────┘              └──────────────────┘   │
│                                                             │
│  DAILY BACKUP (no approval needed):                         │
│    Alice                                                    │
│    ┌──────────────────┐                                     │
│    │ airgapper backup │  ──encrypted──▶  [Append-only      │
│    │                  │                   Storage]          │
│    └──────────────────┘                                     │
│                                                             │
│  RESTORE (requires consensus):                              │
│    Alice                              Bob                   │
│    ┌──────────────────┐              ┌──────────────────┐   │
│    │ airgapper request│  ──notify──▶ │ airgapper approve│   │
│    └──────────────────┘              └──────────────────┘   │
│           │                                   │             │
│           └───────── combine shares ──────────┘             │
│                          │                                  │
│                          ▼                                  │
│                   Restore data                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Step by Step

**1. Alice initializes (data owner)**

```bash
# Start a restic-rest-server on Bob's machine (append-only mode)
docker run -d -p 8000:8000 restic/rest-server --append-only --no-auth

# Alice initializes her backup
./bin/airgapper init --name alice --repo rest:http://bob-nas:8000/alice-backup

# Output includes a share to give to Bob:
#   Share:   a1b2c3d4e5f6...
#   Index:   2
```

**2. Alice configures scheduled backups**

```bash
# Set up daily backups at 2 AM
./bin/airgapper schedule --set daily ~/Documents ~/Pictures

# Or use cron syntax
./bin/airgapper schedule --set "0 3 * * *" ~/Documents  # 3 AM daily
```

**3. Bob joins (backup host)**

```bash
# Bob receives Alice's share and joins
./bin/airgapper join --name bob \
  --repo rest:http://localhost:8000/alice-backup \
  --share a1b2c3d4e5f6... \
  --index 2
```

**4. Start the server (runs scheduled backups + API)**

```bash
# Alice runs the server for scheduled backups
./bin/airgapper serve --addr :8080
```

**5. Alice requests restore (requires Bob's approval)**

```bash
./bin/airgapper request --snapshot latest --reason "laptop crashed"
```

**6. Bob approves**

```bash
./bin/airgapper pending
./bin/airgapper approve abc123
```

**7. Alice restores**

```bash
./bin/airgapper restore --request abc123 --target ~/restore/
```

## Make Targets

```bash
make help               # Show all targets

# Combined
make build              # Build frontend + backend
make dev                # Run both in dev mode
make test               # Run all tests
make clean              # Clean build artifacts

# Frontend
make frontend-install   # Install npm dependencies
make frontend-dev       # Run Vite dev server
make frontend-build     # Build for production
make frontend-test      # Run frontend tests
make frontend-lint      # Lint frontend code

# Backend
make backend-build      # Build Go binary
make backend-test       # Run Go tests
make backend-lint       # Run Go linters
make backend-fmt        # Format Go code

# Docker
make docker             # Build Docker image
make docker-compose-up  # Start full stack
make docker-compose-down # Stop stack
```

## Documentation

- [Getting Started Guide](docs/GETTING-STARTED.md) - Detailed tutorial
- [Security Model](docs/SECURITY.md) - Threat model and assumptions
- [API Reference](docs/API.md) - HTTP API documentation
- [Design Document](docs/DESIGN.md) - Architecture and design decisions

## Commands

| Command | Description | Who |
|---------|-------------|-----|
| `init` | Initialize as data owner | Owner |
| `join` | Join as backup host | Host |
| `backup` | Create a backup | Owner |
| `snapshots` | List snapshots | Owner |
| `schedule` | Configure backup schedule | Owner |
| `request` | Request restore approval | Owner |
| `pending` | List pending requests | Both |
| `approve` | Approve a request | Host |
| `deny` | Deny a request | Host |
| `restore` | Restore after approval | Owner |
| `status` | Show status | Both |
| `serve` | Run HTTP API + scheduled backups | Both |

## Contributing

Contributions welcome!

```bash
# Development
make build        # Build binary
make test         # Run tests
make backend-lint # Run linters
make help         # Show all targets
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

Built with:
- [restic](https://restic.net) - Backup engine
- [restic-rest-server](https://github.com/restic/rest-server) - Append-only storage
- [React](https://react.dev) + [Vite](https://vitejs.dev) - Frontend
- Shamir's Secret Sharing - Key splitting

---

*Airgapper: Because backups should survive even if your machine doesn't.*
