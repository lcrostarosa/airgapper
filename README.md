# Airgapper

**Consensus-based encrypted backup for the paranoid.**

Airgapper wraps [Restic](https://restic.net) with 2-of-2 Shamir's Secret Sharing, ensuring no single party can access backup data without the other's consent.

## Status: MVP (v0.1.0)

This is a minimal working prototype demonstrating the core concepts:
- ✅ Key splitting with Shamir's Secret Sharing
- ✅ Restic repository initialization  
- ✅ Restore request/approval workflow
- ✅ Password reconstruction from shares

### Not yet implemented (see design doc):
- Peer-to-peer networking
- Backup-only keys (currently requires full password)
- Notifications (push/email)
- Key rotation
- Multi-party (m-of-n) support

## Installation

```bash
# Requires Go 1.21+ and Restic
brew install restic  # macOS
# or: apt install restic  # Debian/Ubuntu

# Build
cd airgapper
go build -o airgapper ./cmd/airgapper

# Optional: install to PATH
sudo mv airgapper /usr/local/bin/
```

## Quick Start

### Initialize (Party A - Data Owner)

```bash
# Initialize a new backup repository
airgapper init --name alice --repo /path/to/backup/repo

# Output includes a share for Party B:
# Peer share (index 2): 61ceb5a35c64...
```

### Join (Party B - Storage Host)

```bash
# Join using the share from Party A
airgapper join --name bob --repo /path/to/backup/repo \
  --share 61ceb5a35c64... --index 2
```

### Backup (Party A)

```bash
# Create backups (MVP note: requires password reconstruction)
airgapper backup /important/data
```

### Restore Flow (requires consensus)

```bash
# Party A requests restore
airgapper request --snapshot latest --reason "laptop crashed"
# Output: Request ID: abc123

# Party B sees request
airgapper pending

# Party B approves (releases their share)
airgapper approve abc123

# Party A can now restore
airgapper restore --request abc123 --target /restore/path
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                     AIRGAPPER FLOW                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  INIT:                                                      │
│  1. Generate random Restic password                         │
│  2. Split password using 2-of-2 Shamir                      │
│  3. Store Share 1 locally                                   │
│  4. Give Share 2 to peer                                    │
│  5. Initialize Restic repo with password                    │
│                                                             │
│  RESTORE:                                                   │
│  1. Party A creates restore request                         │
│  2. Party B reviews and approves                            │
│  3. Party B's share is released                             │
│  4. Combine shares → reconstruct password                   │
│  5. Decrypt and restore                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Security Model

- **No single point of failure**: Neither party alone can decrypt
- **Append-only backups**: Restic can be configured with append-only server
- **Untrusted storage**: Host sees only encrypted data
- **Audit trail**: All requests logged locally

## Design Document

See [research/airgapper-design.md](../research/airgapper-design.md) for the full architecture, threat model, and roadmap.

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize new backup repository |
| `join` | Join existing repository with peer's share |
| `backup` | Create a backup |
| `snapshots` | List snapshots |
| `request` | Request restore approval |
| `pending` | List pending requests |
| `approve` | Approve a restore request |
| `deny` | Deny a restore request |
| `restore` | Restore (requires approved request) |
| `status` | Show status |

## License

MIT
