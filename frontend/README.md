# Airgapper Web UI

React + TypeScript + Tailwind web interface for Airgapper.

## Features

- **Initialize Vault**: Generate encryption keys and split them using Shamir's Secret Sharing
- **Join as Host**: Enter a share to join as a backup host
- **Dashboard**: View vault status, key shares, and test key reconstruction
- **In-browser SSS**: Full Shamir's Secret Sharing implementation in TypeScript

## Quick Start

```bash
npm install
npm run dev
```

Then open http://localhost:5173

## Build

```bash
npm run build
npm run preview  # Serve production build
```

## Testing

```bash
npm test          # Run tests once
npm run test:watch # Watch mode
```

## Architecture

The web UI implements Shamir's Secret Sharing entirely in the browser using the same GF(256) arithmetic as the Go CLI. This means:

- Keys are generated client-side using `crypto.getRandomValues()`
- No server communication required for key generation
- Shares can be exported for use with the CLI

### Key Files

- `src/lib/sss.ts` - Shamir's Secret Sharing implementation
- `src/components/InitVault.tsx` - Vault initialization flow
- `src/components/JoinVault.tsx` - Host join flow
- `src/components/Dashboard.tsx` - Status and key management

## Storage

Vault configuration is stored in browser `localStorage` under the key `airgapper_vault`. This includes:
- Name and role
- Repository URL
- Local key share (hex encoded)
- Password (for owner role only, hex encoded)

**Security note**: For production use, consider encrypting the localStorage data or using a more secure storage mechanism.

## CLI Integration

The web UI generates shares that are compatible with the CLI:

```bash
# Owner initializes in web UI, gives share to host
airgapper join --name bob --repo "..." --share <hex> --index 2

# Or owner can use CLI and host uses web UI
airgapper init --name alice --repo "..."
# Then paste the share into the web UI's "Join" flow
```
