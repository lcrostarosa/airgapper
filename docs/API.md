# Airgapper HTTP API Reference

The Airgapper API provides remote control of backup and restore operations.

## Starting the Server

```bash
airgapper serve --addr :8080
```

## Endpoints

### Health Check

```http
GET /health
```

Returns server health status.

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "ok"
  }
}
```

---

### System Status

```http
GET /api/status
```

Returns current system status.

**Response:**
```json
{
  "success": true,
  "data": {
    "name": "alice",
    "role": "owner",
    "repo_url": "rest:http://localhost:8000/backup",
    "has_share": true,
    "share_index": 1,
    "pending_requests": 0,
    "peer": {
      "name": "bob",
      "address": "http://bob:8080"
    }
  }
}
```

---

### List Restore Requests

```http
GET /api/requests
```

Returns all pending restore requests.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "a1b2c3d4",
      "requester": "alice",
      "snapshot_id": "latest",
      "paths": null,
      "reason": "laptop crashed",
      "status": "pending",
      "created_at": "2024-01-25T10:00:00Z",
      "expires_at": "2024-01-26T10:00:00Z"
    }
  ]
}
```

---

### Create Restore Request

```http
POST /api/requests
Content-Type: application/json

{
  "snapshot_id": "latest",
  "paths": ["/home/user/documents"],
  "reason": "need to restore files"
}
```

Creates a new restore request.

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `snapshot_id` | string | No | Snapshot to restore (default: "latest") |
| `paths` | string[] | No | Specific paths to restore |
| `reason` | string | Yes | Reason for restore request |

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "a1b2c3d4",
    "status": "pending",
    "expires_at": "2024-01-26T10:00:00Z"
  }
}
```

---

### Get Request Details

```http
GET /api/requests/{id}
```

Returns details of a specific request.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "a1b2c3d4",
    "requester": "alice",
    "snapshot_id": "latest",
    "paths": null,
    "reason": "laptop crashed",
    "status": "approved",
    "created_at": "2024-01-25T10:00:00Z",
    "expires_at": "2024-01-26T10:00:00Z",
    "approved_at": "2024-01-25T11:00:00Z",
    "approved_by": "bob"
  }
}
```

---

### Approve Request

```http
POST /api/requests/{id}/approve
Content-Type: application/json

{}
```

Approves a restore request and releases the local key share.

**Optional Body:**
```json
{
  "share": "base64-encoded-share",
  "share_index": 2
}
```

If no body provided, uses the locally stored share.

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "approved",
    "message": "Key share released"
  }
}
```

---

### Deny Request

```http
POST /api/requests/{id}/deny
```

Denies a restore request.

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "denied"
  }
}
```

---

### List Snapshots

```http
GET /api/snapshots
```

Lists available snapshots (requires password/authorization).

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "Snapshot listing requires restore approval"
  }
}
```

Note: In the current implementation, snapshot listing requires the full password, which is only available to the owner.

---

### Receive Share (Peer Setup)

```http
POST /api/share
Content-Type: application/json

{
  "share": "base64-encoded-share",
  "share_index": 2,
  "repo_url": "rest:http://localhost:8000/backup",
  "peer_name": "alice"
}
```

Receives and stores a key share from a peer. Used during initial setup.

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "received",
    "message": "Share stored successfully"
  }
}
```

---

## Error Responses

All errors return a consistent format:

```json
{
  "success": false,
  "error": "Error message description"
}
```

**Common HTTP Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad request (invalid input) |
| 404 | Not found |
| 500 | Internal server error |

---

## Examples

### Using curl

```bash
# Health check
curl http://localhost:8080/health

# Get status
curl http://localhost:8080/api/status

# Create restore request
curl -X POST http://localhost:8080/api/requests \
  -H "Content-Type: application/json" \
  -d '{"reason": "need files back", "snapshot_id": "latest"}'

# List pending requests
curl http://localhost:8080/api/requests

# Approve a request
curl -X POST http://localhost:8080/api/requests/a1b2c3d4/approve

# Deny a request
curl -X POST http://localhost:8080/api/requests/a1b2c3d4/deny
```

### Using HTTPie

```bash
# Health check
http :8080/health

# Get status
http :8080/api/status

# Create request
http POST :8080/api/requests reason="need restore" snapshot_id="latest"

# Approve
http POST :8080/api/requests/a1b2c3d4/approve
```

### Using Python

```python
import requests

BASE_URL = "http://localhost:8080"

# Get status
resp = requests.get(f"{BASE_URL}/api/status")
print(resp.json())

# Create request
resp = requests.post(f"{BASE_URL}/api/requests", json={
    "reason": "laptop died",
    "snapshot_id": "latest"
})
request_id = resp.json()["data"]["id"]

# Approve (on Bob's side)
resp = requests.post(f"{BASE_URL}/api/requests/{request_id}/approve")
print(resp.json())
```

---

## CORS

The API includes CORS headers allowing cross-origin requests:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

---

## Security Considerations

1. **No built-in authentication** - The current API has no auth. In production:
   - Use a reverse proxy with authentication
   - Add API keys
   - Use mTLS

2. **TLS recommended** - Use HTTPS in production

3. **Network isolation** - Consider running on a private network

4. **Audit logging** - All API calls are logged to stdout

---

## WebSocket (Future)

Future versions may include WebSocket support for:
- Real-time request notifications
- Backup progress updates
- Peer status monitoring
