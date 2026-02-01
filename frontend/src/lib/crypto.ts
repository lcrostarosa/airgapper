/**
 * Ed25519 cryptographic utilities using Web Crypto API
 * Note: Ed25519 was added to Web Crypto API in recent browsers.
 * For older browsers, consider using a library like tweetnacl.
 */

/**
 * Check if Ed25519 is supported by the browser
 */
export async function isEd25519Supported(): Promise<boolean> {
  try {
    const keyPair = await crypto.subtle.generateKey("Ed25519", true, [
      "sign",
      "verify",
    ]);
    return !!keyPair;
  } catch {
    return false;
  }
}

/**
 * Generate an Ed25519 key pair
 * Returns hex-encoded public and private keys
 */
export async function generateKeyPair(): Promise<{
  publicKey: string;
  privateKey: string;
}> {
  const keyPair = await crypto.subtle.generateKey("Ed25519", true, [
    "sign",
    "verify",
  ]);

  // Export keys
  const publicKeyRaw = await crypto.subtle.exportKey(
    "raw",
    keyPair.publicKey
  );
  const privateKeyRaw = await crypto.subtle.exportKey(
    "pkcs8",
    keyPair.privateKey
  );

  // The PKCS8 format has extra header bytes, extract just the key
  // Ed25519 private key in PKCS8 is 48 bytes, with 16 byte header
  const privateKeyBytes = new Uint8Array(privateKeyRaw);
  const privateKeySeed = privateKeyBytes.slice(-32); // Last 32 bytes are the seed

  return {
    publicKey: toHex(new Uint8Array(publicKeyRaw)),
    privateKey: toHex(privateKeySeed),
  };
}

/**
 * Import a private key from hex-encoded seed
 */
async function importPrivateKey(
  privateKeyHex: string
): Promise<CryptoKey> {
  const seed = fromHex(privateKeyHex);

  // Build PKCS8 structure for Ed25519
  // Header: 30 2e 02 01 00 30 05 06 03 2b 65 70 04 22 04 20
  const pkcs8Header = new Uint8Array([
    0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70,
    0x04, 0x22, 0x04, 0x20,
  ]);
  const pkcs8 = new Uint8Array(pkcs8Header.length + seed.length);
  pkcs8.set(pkcs8Header);
  pkcs8.set(seed, pkcs8Header.length);

  return crypto.subtle.importKey(
    "pkcs8",
    pkcs8.buffer as ArrayBuffer,
    "Ed25519",
    false,
    ["sign"]
  );
}

/**
 * Import a public key from hex
 */
async function importPublicKey(publicKeyHex: string): Promise<CryptoKey> {
  const publicKeyBytes = fromHex(publicKeyHex);

  return crypto.subtle.importKey(
    "raw",
    publicKeyBytes.buffer as ArrayBuffer,
    "Ed25519",
    false,
    ["verify"]
  );
}

/**
 * Sign a message with an Ed25519 private key
 * Returns hex-encoded signature
 */
export async function sign(
  privateKeyHex: string,
  message: Uint8Array
): Promise<string> {
  const privateKey = await importPrivateKey(privateKeyHex);
  // Cast to ArrayBuffer to satisfy TypeScript
  const signature = await crypto.subtle.sign("Ed25519", privateKey, message.buffer as ArrayBuffer);
  return toHex(new Uint8Array(signature));
}

/**
 * Verify a signature
 */
export async function verify(
  publicKeyHex: string,
  message: Uint8Array,
  signatureHex: string
): Promise<boolean> {
  try {
    const publicKey = await importPublicKey(publicKeyHex);
    const signature = fromHex(signatureHex);
    // Cast to ArrayBuffer to satisfy TypeScript
    return crypto.subtle.verify("Ed25519", publicKey, signature.buffer as ArrayBuffer, message.buffer as ArrayBuffer);
  } catch {
    return false;
  }
}

/**
 * Generate a key ID from a public key (first 16 hex chars of SHA256)
 */
export async function keyId(publicKeyHex: string): Promise<string> {
  const publicKeyBytes = fromHex(publicKeyHex);
  // Cast to ArrayBuffer to satisfy TypeScript
  const hashBuffer = await crypto.subtle.digest("SHA-256", publicKeyBytes.buffer as ArrayBuffer);
  const hashArray = new Uint8Array(hashBuffer);
  return toHex(hashArray.slice(0, 8));
}

/**
 * Create canonical hash of restore request data for signing
 */
export async function hashRestoreRequest(
  requestId: string,
  requester: string,
  snapshotId: string,
  reason: string,
  keyHolderId: string,
  paths: string[],
  createdAtUnix: number
): Promise<Uint8Array> {
  // Sort paths for canonical ordering
  const sortedPaths = [...paths].sort();

  const data = {
    request_id: requestId,
    requester,
    snapshot_id: snapshotId,
    paths: sortedPaths,
    reason,
    created_at: createdAtUnix,
    key_holder_id: keyHolderId,
  };

  // Create canonical JSON
  const jsonStr = JSON.stringify(data);
  const encoder = new TextEncoder();
  const jsonBytes = encoder.encode(jsonStr);

  // Hash the JSON (cast to ArrayBuffer to satisfy TypeScript)
  const hashBuffer = await crypto.subtle.digest("SHA-256", jsonBytes.buffer as ArrayBuffer);
  return new Uint8Array(hashBuffer);
}

/**
 * Sign a restore request
 */
export async function signRestoreRequest(
  privateKeyHex: string,
  requestId: string,
  requester: string,
  snapshotId: string,
  reason: string,
  keyHolderId: string,
  paths: string[],
  createdAtUnix: number
): Promise<string> {
  const hash = await hashRestoreRequest(
    requestId,
    requester,
    snapshotId,
    reason,
    keyHolderId,
    paths,
    createdAtUnix
  );
  return sign(privateKeyHex, hash);
}

/**
 * Verify a restore request signature
 */
export async function verifyRestoreRequestSignature(
  publicKeyHex: string,
  signatureHex: string,
  requestId: string,
  requester: string,
  snapshotId: string,
  reason: string,
  keyHolderId: string,
  paths: string[],
  createdAtUnix: number
): Promise<boolean> {
  const hash = await hashRestoreRequest(
    requestId,
    requester,
    snapshotId,
    reason,
    keyHolderId,
    paths,
    createdAtUnix
  );
  return verify(publicKeyHex, hash, signatureHex);
}

/**
 * Convert bytes to hex string
 */
export function toHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

/**
 * Convert hex string to bytes
 */
export function fromHex(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(hex.substr(i * 2, 2), 16);
  }
  return bytes;
}
