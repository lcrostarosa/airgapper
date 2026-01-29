# Airgapper

**Consensus-based encrypted backups with ransomware protection**

Airgapper is a control plane for peer-to-peer NAS backups where no single party can read, delete, or restore the data alone. It wraps [restic](https://restic.net) for encrypted backups and uses Shamir's Secret Sharing to split the decryption key between parties.

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
- A peer (friend, family member, colleague) willing to hold your key share
- (Optional) [restic-rest-server](https://github.com/restic/rest-server) for append-only storage

### Installation

```bash
# From source
go install github.com/lcrostarosa/airgapper/cmd/airgapper@latest

# Or build locally
git clone https://github.com/lcrostarosa/airgapper.git
cd airgapper
make build
./bin/airgapper --help
```

### Workflow Overview

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
airgapper init --name alice --repo rest:http://bob-nas:8000/alice-backup

# Output includes a share to give to Bob:
#   Share:   a1b2c3d4e5f6...
#   Index:   2
```

**2. Bob joins (backup host)**

```bash
# Bob receives Alice's share and joins
airgapper join --name bob \
  --repo rest:http://localhost:8000/alice-backup \
  --share a1b2c3d4e5f6... \
  --index 2
```

**3. Alice backs up (daily, no approval needed)**

```bash
airgapper backup ~/Documents ~/Pictures
```

**4. Alice requests restore (requires Bob's approval)**

```bash
# Something went wrong, Alice needs her data back
airgapper request --snapshot latest --reason "laptop crashed"

# Request ID: abc123...
```

**5. Bob approves**

```bash
# Bob sees the pending request
airgapper pending

# Bob approves after verifying with Alice (phone call, etc.)
airgapper approve abc123
```

**6. Alice restores**

```bash
airgapper restore --request abc123 --target ~/restore/
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              SYSTEM                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────┐                         ┌─────────────────┐        │
│  │   ALICE         │                         │   BOB           │        │
│  │   (Owner)       │                         │   (Host)        │        │
│  │                 │     Encrypted Data      │                 │        │
│  │  ┌───────────┐  │   ──────────────────▶   │  ┌───────────┐  │        │
│  │  │  Restic   │  │     (append-only)       │  │REST Server│  │        │
│  │  └───────────┘  │                         │  └───────────┘  │        │
│  │       │         │                         │       │         │        │
│  │  ┌───────────┐  │   Approval Requests     │  ┌───────────┐  │        │
│  │  │ Airgapper │  │ ◀────────────────────▶  │  │ Airgapper │  │        │
│  │  └───────────┘  │                         │  └───────────┘  │        │
│  │       │         │                         │       │         │        │
│  │  ┌───────────┐  │                         │  ┌───────────┐  │        │
│  │  │Key Share 1│  │                         │  │Key Share 2│  │        │
│  │  │+ Password │  │                         │  │  (only)   │  │        │
│  │  └───────────┘  │                         │  └───────────┘  │        │
│  │                 │                         │                 │        │
│  └─────────────────┘                         └─────────────────┘        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key points:**
- Alice keeps the full password (for backups) AND her key share
- Bob keeps only his key share (cannot backup or read data alone)
- Storage is append-only (neither can delete backups)
- Restore requires combining both shares

## Commands

| Command | Description | Who |
|---------|-------------|-----|
| `init` | Initialize as data owner | Owner |
| `join` | Join as backup host | Host |
| `backup` | Create a backup | Owner |
| `snapshots` | List snapshots | Owner |
| `request` | Request restore approval | Owner |
| `pending` | List pending requests | Both |
| `approve` | Approve a request | Host |
| `deny` | Deny a request | Host |
| `restore` | Restore after approval | Owner |
| `status` | Show status | Both |
| `serve` | Run HTTP API server | Both |

## HTTP API

Run `airgapper serve --addr :8080` to expose a REST API for remote management:

```bash
# Health check
curl http://localhost:8080/health

# Get status
curl http://localhost:8080/api/status

# List pending requests
curl http://localhost:8080/api/requests

# Approve a request
curl -X POST http://localhost:8080/api/requests/abc123/approve
```

See [docs/API.md](docs/API.md) for full API documentation.

## Docker

```bash
# Build the image
make docker

# Run full stack (Alice + Bob + storage)
docker-compose -f docker/docker-compose.yml up -d

# Or use the simple example
docker-compose -f examples/docker-compose.example.yml up -d
```

## Documentation

- [Getting Started Guide](docs/GETTING-STARTED.md) - Detailed tutorial
- [Security Model](docs/SECURITY.md) - Threat model and assumptions
- [API Reference](docs/API.md) - HTTP API documentation
- [Design Document](docs/DESIGN.md) - Architecture and design decisions

## Personas

### Technical Users
Full CLI control, can customize everything. Integrate with existing backup scripts.

### Home Users  
Two friends back up to each other's NAS. Simple setup, phone call to approve restores.

### Small Business
IT backs up to offsite location. Requires manager approval for restores.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Development
make build      # Build binary
make test       # Run tests
make lint       # Run linters
make help       # Show all targets
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

Built with:
- [restic](https://restic.net) - Backup engine
- [restic-rest-server](https://github.com/restic/rest-server) - Append-only storage
- Shamir's Secret Sharing - Key splitting

---

*Airgapper: Because backups should survive even if your machine doesn't.*
