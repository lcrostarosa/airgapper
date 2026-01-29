# Airgapper Security Model

This document describes Airgapper's threat model, trust assumptions, and security properties.

## Threat Model

### Attacker Goals

| Goal | Description |
|------|-------------|
| **Read backup data** | Access the content of backups |
| **Delete backups** | Remove or corrupt backup snapshots |
| **Ransomware backups** | Encrypt or overwrite backup data |
| **Unauthorized restore** | Restore data without owner knowledge |
| **Impersonate party** | Pretend to be Alice or Bob |
| **Steal encryption key** | Obtain the full repository password |

### Mitigations

| Attack | Mitigation | Strength |
|--------|------------|----------|
| Read backup data | AES-256-CTR + Poly1305 (restic) | Strong |
| Delete backups | Append-only storage | Strong |
| Ransomware backups | Can't delete old snapshots | Strong |
| Unauthorized restore | 2-of-2 key consensus | Strong |
| Impersonate party | Out-of-band verification | Moderate |
| Steal encryption key | Key split via SSS | Strong |

## Trust Assumptions

### Alice (Data Owner)

**Trusts:**
- Her own machine during backup (must hold password)
- Bob to respond to restore requests
- Bob not to collude with attackers

**Does NOT trust:**
- Bob to read her data
- Bob to delete her backups
- Storage to be reliable (has her share to recover)

### Bob (Backup Host)

**Trusts:**
- Alice's restore requests are legitimate (after verification)
- His own storage integrity

**Does NOT trust:**
- Alice's encrypted data (can't read it)
- Alice to delete backups (append-only)

### Both Parties

**Assume:**
- At least one party is not compromised during restore
- Out-of-band communication (phone) is authentic
- Local config files (`~/.airgapper/`) are secure

## Key Security Properties

### 1. No Single Point of Compromise

```
┌─────────────────────────────────────────────────────┐
│              Password P = "abc123..."               │
│                        │                            │
│              Split via 2-of-2 SSS                   │
│                   ╱         ╲                       │
│              Share 1      Share 2                   │
│              (Alice)       (Bob)                    │
│                                                     │
│  To reconstruct P, need BOTH shares                 │
│  Neither party alone can decrypt                    │
└─────────────────────────────────────────────────────┘
```

### 2. Append-Only Storage

```
┌─────────────────────────────────────────────────────┐
│            restic-rest-server --append-only         │
│                                                     │
│  ALLOWED:                                           │
│    ✅ Write new snapshots                           │
│    ✅ Read existing snapshots (with password)       │
│                                                     │
│  BLOCKED:                                           │
│    ❌ Delete snapshots                              │
│    ❌ Modify existing data                          │
│    ❌ Truncate/corrupt repository                   │
│                                                     │
│  Even if Alice's machine is compromised,            │
│  old backups cannot be destroyed.                   │
└─────────────────────────────────────────────────────┘
```

### 3. Consensus-Based Restore

```
┌─────────────────────────────────────────────────────┐
│                  RESTORE FLOW                       │
│                                                     │
│  1. Alice requests restore                          │
│  2. Bob sees request (CLI or API)                   │
│  3. Bob verifies with Alice OUT-OF-BAND            │
│     (phone call, video chat, in person)             │
│  4. Bob approves → releases his share               │
│  5. Alice combines shares → password                │
│  6. Restore proceeds                                │
│                                                     │
│  Attack scenario:                                   │
│    Ransomware on Alice's machine requests restore   │
│    Bob calls Alice: "Did you request this?"         │
│    Alice: "No, my machine is encrypted!"            │
│    Bob: Denies request                              │
│    Attacker cannot restore                          │
└─────────────────────────────────────────────────────┘
```

## Attack Scenarios

### Scenario 1: Alice's Machine Compromised

**Attack:** Ransomware encrypts Alice's drive and tries to destroy backups

**Protection:**
- Ransomware cannot delete old backups (append-only)
- Ransomware cannot request restore without Bob approving
- Bob verifies out-of-band before approving

**Result:** Old backups survive; ransomware cannot complete attack

### Scenario 2: Bob is Malicious

**Attack:** Bob wants to read Alice's data or hold it hostage

**Protection:**
- Bob only has encrypted ciphertext (no password)
- Bob cannot reconstruct password (needs Alice's share)
- If Bob refuses to approve, Alice can ask: why?

**Result:** Data remains encrypted; worst case is denial of service

### Scenario 3: Storage Compromise

**Attack:** Attacker gains access to Bob's NAS

**Protection:**
- All data is encrypted at rest
- Attacker sees only ciphertext
- Cannot delete (append-only enforced by server)
- Cannot decrypt (no shares on storage)

**Result:** Confidentiality maintained

### Scenario 4: Man-in-the-Middle

**Attack:** Attacker intercepts share during initial setup

**Protection:**
- Shares should be transferred out-of-band (in person, encrypted chat)
- Shares alone are useless (need both)

**Mitigation:** Use secure channel for share transfer

### Scenario 5: Insider Threat

**Attack:** Bob colludes with attacker or is coerced

**Protection:**
- Bob alone cannot decrypt
- If Bob releases share improperly, still need Alice's share
- Audit log tracks all actions

**Mitigation:** 2-of-3 scheme with trusted third party (future)

## Cryptographic Details

### Shamir's Secret Sharing

```
Parameters:
- k = 2 (threshold)
- n = 2 (total shares)

Implementation:
- GF(2^8) arithmetic
- Random polynomial of degree k-1
- Each byte processed independently

Security:
- Information-theoretic security
- k-1 shares reveal NOTHING about secret
- With 2-of-2, both shares are required
```

### Restic Encryption

```
Algorithm: AES-256-CTR + Poly1305
Key derivation: scrypt
Deduplication: Content-defined chunking

The repository password encrypts:
- Master key (used for data)
- All backup content
- Metadata and tree structures
```

## What's NOT Protected

### Out of Scope

| Threat | Why | Mitigation |
|--------|-----|------------|
| Hardware attacks | Physical access defeats crypto | Physical security |
| Nation-state adversaries | Beyond threat model | Different tool |
| Denial of service | Bob can refuse to host | Legal agreements |
| Key share loss | Both shares required | Backup shares |
| Coercion of both parties | Consensus can be forced | 2-of-3 with escrow |

### Known Limitations

1. **No forward secrecy** - If shares are stolen, past data can be decrypted
2. **Trust on first use** - Initial share transfer is a critical moment
3. **Single repository** - Shares are tied to one repo
4. **No key rotation** - Password cannot be changed without new setup

## Best Practices

### For Setup

1. Transfer shares in person or via encrypted channel
2. Verify peer identity before accepting shares
3. Store config backup securely (paper in safe)
4. Use TLS for REST server in production

### For Operations

1. Always verify restore requests out-of-band (phone call)
2. Regularly test restore process
3. Monitor for unexpected restore requests
4. Keep audit logs

### For Production

1. Enable TLS on restic-rest-server
2. Use authentication (`--htpasswd-file`)
3. Consider 2-of-3 for redundancy
4. Implement monitoring and alerting

## Future Improvements

- **2-of-3 and m-of-n** - More flexible threshold
- **Key rotation** - Periodic password refresh
- **Identity verification** - Cryptographic identities
- **Time-locked recovery** - Auto-release after timeout
- **Hardware key support** - TPM/HSM integration
- **Audit log replication** - Tamper-evident logging
