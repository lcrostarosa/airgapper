# Getting Started with Airgapper

This guide walks you through setting up Airgapper for the first time.

## Prerequisites

### Required
- **restic** - The backup engine ([installation guide](https://restic.net/install/))
- **Go 1.21+** - To build Airgapper (or use pre-built binaries/Docker)

### Recommended
- **restic-rest-server** - For append-only storage
- **A trusted peer** - Friend, family, or colleague to hold your key share

## Scenario: Alice and Bob

We'll use this example throughout:
- **Alice** - Has important data to back up
- **Bob** - Has a NAS and is willing to store Alice's encrypted backups
- Neither trusts the other completely, but they trust each other somewhat

## Step 1: Install Airgapper

### From Source

```bash
git clone https://github.com/lcrostarosa/airgapper.git
cd airgapper
make build
sudo cp bin/airgapper /usr/local/bin/
```

### Using Go

```bash
go install github.com/lcrostarosa/airgapper/cmd/airgapper@latest
```

### Verify Installation

```bash
airgapper version
# airgapper 0.2.0

restic version
# restic 0.16.x
```

## Step 2: Set Up Storage (Bob's Side)

Bob runs the restic-rest-server in append-only mode:

### Using Docker (recommended)

```bash
# Create data directory
mkdir -p /path/to/backup-storage

# Run the server
docker run -d \
  --name restic-rest-server \
  -p 8000:8000 \
  -v /path/to/backup-storage:/data \
  restic/rest-server \
  --append-only \
  --no-auth
```

### From Binary

```bash
# Download from https://github.com/restic/rest-server/releases
rest-server --path /path/to/backup-storage --append-only --no-auth
```

**Why `--append-only`?**
- Alice can write new backups
- Alice cannot delete or overwrite existing backups
- Even if Alice's machine is compromised, old backups are safe

**Why `--no-auth`?**
- For simplicity in this guide
- In production, use `--htpasswd-file` for authentication
- The data is encrypted anyway - server sees only ciphertext

## Step 3: Initialize (Alice's Side)

Alice initializes the backup repository:

```bash
airgapper init \
  --name alice \
  --repo rest:http://bob-nas.local:8000/alice-backup
```

Output:
```
🔐 Airgapper Initialization (Data Owner)
=========================================
Name: alice
Repo: rest:http://bob-nas.local:8000/alice-backup

1. Generated secure repository password
2. Split password into 2 shares (2-of-2 required for restore)
3. Initializing restic repository...
4. Repository initialized successfully
5. Configuration saved to ~/.airgapper/

======================================================================
⚠️  IMPORTANT: Share this with your backup host (Bob):
======================================================================

  Share:   a1b2c3d4e5f6789...
  Index:   2
  Repo:    rest:http://bob-nas.local:8000/alice-backup

They should run:
  airgapper join --name <their-name> --repo 'rest:http://bob-nas.local:8000/alice-backup' \
    --share a1b2c3d4e5f6789... --index 2

======================================================================

✅ Initialization complete!
```

**⚠️ IMPORTANT:** 
- Give the share to Bob through a secure channel (in person, encrypted message)
- The share is sensitive - don't post it publicly
- Alice's config is stored in `~/.airgapper/`

## Step 4: Join as Backup Host (Bob's Side)

Bob receives Alice's share and joins:

```bash
airgapper join \
  --name bob \
  --repo rest:http://localhost:8000/alice-backup \
  --share a1b2c3d4e5f6789... \
  --index 2
```

Output:
```
🔐 Airgapper Join (Backup Host)
================================
Name:  bob
Repo:  rest:http://localhost:8000/alice-backup
Share: 64 bytes, index 2

✅ Joined as backup host!

You are now a key holder for this backup repository.
When the owner requests a restore, you'll need to approve it.

Commands available to you:
  airgapper pending  - List pending restore requests
  airgapper approve  - Approve a restore request
  airgapper deny     - Deny a restore request
  airgapper serve    - Run HTTP API for remote management
```

## Step 5: Configure Scheduled Backups (Alice)

Set up automatic backups:

```bash
# Configure daily backups at 2 AM
airgapper schedule --set daily ~/Documents ~/Pictures

# Or use more specific schedules
airgapper schedule --set "0 3 * * *" ~/Documents  # 3 AM daily (cron)
airgapper schedule --set "every 4h" ~/Documents   # Every 4 hours
airgapper schedule --set hourly ~/Documents       # Every hour

# View current schedule
airgapper schedule --show
```

Output:
```
✅ Schedule configured!
Schedule: daily
Paths:    /home/alice/Documents, /home/alice/Pictures
Next run: 2024-01-26 02:00:00 (in 8.5 hours)

To start scheduled backups, run:
  airgapper serve  # Default port :8081, or set AIRGAPPER_PORT
```

**Run as daemon:**
```bash
# Start the server (runs scheduled backups + HTTP API)
airgapper serve  # Default port :8081
```

## Step 6: Manual Backups (Alice)

You can also run backups manually:

```bash
# Backup specific directories
airgapper backup ~/Documents ~/Pictures

# Backup with restic tags (passed through)
airgapper backup ~/important-project
```

Output:
```
📦 Creating Backup
==================
Repository: rest:http://bob-nas.local:8000/alice-backup
Paths: /home/alice/Documents, /home/alice/Pictures

repository 8a2b3c4d opened successfully
scanning /home/alice/Documents
scanning /home/alice/Pictures
[0:15] 100.00%  3.2 GiB / 3.2 GiB  12345 / 12345 items

snapshot abc12345 saved

✅ Backup complete!
```

**Note:** Backups don't require Bob's approval. Alice has the full password.

## Step 7: Request Restore (Alice)

When Alice needs to restore (laptop died, ransomware, etc.):

```bash
airgapper request \
  --snapshot latest \
  --reason "laptop hard drive failed"
```

Output:
```
📤 Restore Request Created
==========================
Request ID: f7e8d9c0a1b2
Snapshot:   latest
Reason:     laptop hard drive failed
Expires:    2024-01-26 15:30:00

⏳ Waiting for peer approval...
Share request ID with your peer: f7e8d9c0a1b2

Once approved, run:
  airgapper restore --request f7e8d9c0a1b2 --target /restore/path
```

**Communication with Bob:**
- Alice calls Bob: "Hey, my laptop died. Can you approve restore request f7e8d9c0?"
- This out-of-band verification is intentional - it prevents a compromised machine from requesting restores without the human knowing

## Step 8: Approve Request (Bob)

Bob checks pending requests:

```bash
airgapper pending
```

Output:
```
📋 Pending Restore Requests
===========================

ID: f7e8d9c0a1b2
  From:     alice
  Snapshot: latest
  Reason:   laptop hard drive failed
  Expires:  2024-01-26 15:30

To approve: airgapper approve <request-id>
To deny:    airgapper deny <request-id>
```

After verifying with Alice (phone call, video chat, in person):

```bash
airgapper approve f7e8d9c0a1b2
```

Output:
```
Approving request f7e8d9c0a1b2...
Releasing key share (index 2)...

✅ Request approved!
Your key share has been released.
The requester can now restore their data.
```

## Step 9: Restore Data (Alice)

Now Alice can restore:

```bash
airgapper restore \
  --request f7e8d9c0a1b2 \
  --target ~/restore/
```

Output:
```
🔓 Reconstructing password from key shares...
✅ Password reconstructed successfully

📥 Starting restore...
Snapshot: latest
Target:   /home/alice/restore/

restoring <snapshot> to /home/alice/restore/
[0:30] 100.00%  3.2 GiB / 3.2 GiB

✅ Restore complete! Files restored to: /home/alice/restore/
```

## Using the HTTP API

For remote management, both parties can run the API server:

```bash
# Bob runs the server
airgapper serve  # Default port :8081
```

Alice can then interact via HTTP:

```bash
# Check Bob's status
curl http://bob-nas:8081/api/status

# Create request via API
curl -X POST http://bob-nas:8081/api/requests \
  -H "Content-Type: application/json" \
  -d '{"snapshot_id": "latest", "reason": "need restore"}'

# Bob approves via API (or CLI)
curl -X POST http://bob-nas:8081/api/requests/abc123/approve
```

## Running with Docker

Full stack example:

```bash
cd airgapper
docker-compose -f docker/docker-compose.yml up -d

# Alice's API is on :8081
# Bob's API is on :8082
# Storage is on :8000
```

## Troubleshooting

### "restic is not installed"
Install restic from https://restic.net/installation/

### "airgapper not initialized"
Run `airgapper init` or `airgapper join` first

### "request is not approved"
Wait for your peer to approve, then try restore again

### "failed to reconstruct password"
The shares may be corrupted or from different repositories

### Connection refused to REST server
Make sure restic-rest-server is running and accessible

## Next Steps

- Read [Security Model](SECURITY.md) to understand trust assumptions
- See [API Reference](API.md) for HTTP API details
- Check [Design Document](DESIGN.md) for architecture details

## Security Reminders

1. **Verify the share transfer** - Give Bob the share in person or via encrypted channel
2. **Verify restore requests** - Bob should call Alice before approving
3. **Use TLS in production** - The examples use HTTP for simplicity
4. **Keep backups of your config** - `~/.airgapper/` contains your key share
5. **Consider 2-of-3** - Add a third party for redundancy (future feature)
