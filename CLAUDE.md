# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Airgapper is a consensus-based encrypted backup system that wraps [restic](https://restic.net) and uses Shamir's Secret Sharing (SSS) to split backup encryption keys between parties. The key innovation: neither the data owner nor the backup host can decrypt/restore backups alone - both must agree via a consensus approval flow.

## Build & Development Commands

### Build
```bash
make build              # Build frontend + backend
make backend-build      # Build Go binary to ./bin/airgapper
make frontend-build     # Build React app for production
```

### Development
```bash
make dev                # Run both frontend (:5173) and backend (:8081) in dev mode
make frontend-dev       # Run Vite dev server only
cd backend && go run ./cmd/airgapper serve  # Run backend only (default :8081)

# Set custom port via environment variable
AIRGAPPER_PORT=8081 go run ./cmd/airgapper serve
```

### Testing
```bash
make test               # Run all tests (backend + frontend)
make backend-test       # Run Go tests only
make frontend-test      # Run frontend tests only
make backend-test-cover # Run Go tests with coverage report
```

### Linting & Formatting
```bash
make backend-lint       # Run golangci-lint
make backend-fmt        # Format Go code with go fmt
make frontend-lint      # Run frontend linter
```

### Docker
```bash
make docker             # Build Docker image
docker-compose up -d    # Run full stack (storage + backend + frontend)
docker-compose logs -f  # View logs
```

## Architecture

### Repository Structure

```
airgapper/
├── frontend/          # React + Vite web UI
│   └── src/
│       ├── components/  # InitVault, JoinVault, Dashboard, Welcome
│       ├── lib/         # API client utilities
│       └── types.ts     # TypeScript type definitions
├── backend/           # Go backend
│   ├── cmd/airgapper/ # CLI entrypoint (main.go)
│   ├── internal/      # Internal packages (not for external use)
│   │   ├── api/       # HTTP API server
│   │   ├── config/    # Config management (~/.airgapper/config.json)
│   │   ├── consent/   # Restore request/approval workflow
│   │   ├── restic/    # Restic CLI wrapper
│   │   ├── scheduler/ # Cron-based backup scheduler
│   │   └── sss/       # Shamir's Secret Sharing implementation
│   └── pkg/           # Public packages (none currently)
└── docker/            # Docker configuration
```

### Backend Architecture

The Go backend (`backend/cmd/airgapper/main.go`) implements a CLI with these subcommands:

**Owner (data owner) commands:**
- `init` - Initialize repository, generate password, split it via SSS into 2 shares
- `backup` - Run restic backup (uses full password from config)
- `snapshots` - List backup snapshots
- `request` - Request restore approval from peer
- `restore` - Restore data after peer approval (combines key shares)
- `schedule` - Configure automated backup schedule

**Host (backup host) commands:**
- `join` - Join as backup host, receive key share from owner
- `pending` - List pending restore requests
- `approve` - Approve a restore request (release key share)
- `deny` - Deny a restore request

**Both:**
- `serve` - Run HTTP API server (also runs scheduled backups if configured)
- `status` - Show current configuration and status

### Key Internal Packages

**`internal/config`**
- Manages `~/.airgapper/config.json` with node identity, role (owner/host), repo URL, key shares
- Config includes `ListenAddr` for HTTP API (defaults to `AIRGAPPER_PORT` env var or `:8081`)

**`internal/sss`**
- Implements Shamir's Secret Sharing for splitting/combining the restic password
- Default: 2-of-2 scheme (both parties required to reconstruct password)

**`internal/consent`**
- Manages restore request workflow (request ID, status, requester, approver, timestamps)
- Stores requests in `~/.airgapper/requests/`

**`internal/restic`**
- Wraps restic CLI operations (init, backup, restore, snapshots)
- Executes restic as subprocess with proper password handling

**`internal/scheduler`**
- Implements cron-based scheduled backups using owner's full password
- Supports cron expressions or simple aliases ("daily", "weekly")

**`internal/api`**
- HTTP API server for UI and programmatic access
- Endpoints: `/api/status`, `/api/requests`, `/api/schedule`, etc.

### Frontend Architecture

React + Vite single-page app with components:
- `Welcome.tsx` - Landing page (choose init or join)
- `InitVault.tsx` - Initialize as owner (creates repo, displays key share)
- `JoinVault.tsx` - Join as host (paste key share from owner)
- `Dashboard.tsx` - Main UI (backup, restore, approvals, schedule)

The frontend communicates with the backend via REST API on port 8081.

## Key Concepts

### Shamir's Secret Sharing (SSS)
The restic repository password is split into 2 shares using SSS. Neither share alone can decrypt backups - both must be combined. This ensures:
- Owner can backup freely (has full password in config)
- Neither party can restore alone
- Restore requires explicit approval from both parties

### Consent-Based Restore Flow
1. Owner creates restore request with reason/justification
2. Request stored locally and sent to host
3. Host reviews and approves/denies
4. If approved: owner combines shares to reconstruct password
5. Owner runs restic restore with reconstructed password
6. Reconstructed password discarded after use

### Append-Only Storage
Backups stored on `restic-rest-server --append-only` mode. This prevents:
- Deletion of backup data (even by compromised owner)
- Modification of existing snapshots
- Ransomware from destroying backups

## Development Notes

### Running Tests
```bash
# All backend tests
cd backend && go test ./...

# E2E tests with verbose output
cd backend && go test -v ./internal/e2e/...

# Single test
cd backend && go test -v ./internal/sss -run TestSplitCombine
cd frontend && npm test -- --run "test name pattern"
```

### E2E Test Suite (`internal/e2e/`)
Comprehensive end-to-end tests covering:

**SSS Workflow Tests:**
- `TestE2E_SSS_SplitCombineIntegrity` - Full split/combine with hash validation
- `TestE2E_SSS_PartialSharesCannotReconstruct` - Verifies k-of-n threshold enforcement
- `TestE2E_SSS_TamperDetection` - Detects corrupted shares

**Consensus Mode Tests:**
- `TestE2E_Consensus_SignVerify` - Ed25519 signing/verification
- `TestE2E_Consensus_TamperedRequest` - Rejects modified requests

**Hash Validation Tests:**
- `TestE2E_HashValidation_LargeData` - Tests various payload sizes (64B to 16KB)
- `TestE2E_HashValidation_ThresholdSchemes` - Tests 2-of-2 through 5-of-10
- `TestE2E_HashValidation_AfterDecryption` - Post-decryption integrity checks
- `TestE2E_HashChain` - Verifies hash consistency through all SSS operations
- `TestE2E_CryptoKeyIntegrity` - Key encode/decode integrity

**Full Workflow Tests:**
- `TestE2E_FullWorkflow_SSS` - Complete owner/host backup/restore flow
- `TestE2E_FullWorkflow_Consensus` - Complete m-of-n signing flow
- `TestE2E_DataIntegrity_FileSimulation` - Simulated file backup/restore

### Adding New CLI Commands
1. Add command case in `main.go` switch statement
2. Implement `cmd<Name>(args []string) error` function
3. Add to `printUsage()` help text
4. Update tests in `backend/`

### Modifying Config Schema
When changing `internal/config/config.go` Config struct:
1. Update JSON tags appropriately
2. Consider migration for existing `~/.airgapper/config.json` files
3. Update save/load logic if needed

### API Changes
- Backend API: Modify `internal/api/server.go`
- Frontend API client: Update `frontend/src/lib/` utilities
- Keep in sync with docs/API.md

## Default Ports

- **Frontend dev server**: 5173 (Vite default)
- **Backend API**: 8081 (configurable via `AIRGAPPER_PORT` env var or `--addr` flag)
- **restic-rest-server**: 8000 (in docker-compose.yml)

### Backend Port Configuration Priority
The backend port is determined in this order:
1. `--addr` flag (if provided)
2. `AIRGAPPER_PORT` environment variable
3. Default: `:8081`
