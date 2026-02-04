import { describe, it, expect } from "vitest";
import {
  isEd25519Supported,
  generateKeyPair,
  sign,
  verify,
  keyId,
  hashRestoreRequest,
  signRestoreRequest,
  verifyRestoreRequestSignature,
  toHex,
  fromHex,
} from "./crypto";

describe("Ed25519 Crypto Utilities", () => {
  describe("isEd25519Supported", () => {
    it("should return a boolean", async () => {
      const supported = await isEd25519Supported();
      expect(typeof supported).toBe("boolean");
    });

    it("should return true in modern environments", async () => {
      // In Node.js with crypto support, this should be true
      const supported = await isEd25519Supported();
      expect(supported).toBe(true);
    });
  });

  describe("generateKeyPair", () => {
    it("should generate a valid key pair", async () => {
      const keyPair = await generateKeyPair();

      expect(keyPair.publicKey).toBeDefined();
      expect(keyPair.privateKey).toBeDefined();
    });

    it("should generate hex-encoded keys", async () => {
      const keyPair = await generateKeyPair();

      // Valid hex strings contain only 0-9a-f
      expect(keyPair.publicKey).toMatch(/^[0-9a-f]+$/);
      expect(keyPair.privateKey).toMatch(/^[0-9a-f]+$/);
    });

    it("should generate 32-byte public key (64 hex chars)", async () => {
      const keyPair = await generateKeyPair();
      expect(keyPair.publicKey).toHaveLength(64);
    });

    it("should generate 32-byte private key seed (64 hex chars)", async () => {
      const keyPair = await generateKeyPair();
      expect(keyPair.privateKey).toHaveLength(64);
    });

    it("should generate different key pairs each time", async () => {
      const keyPair1 = await generateKeyPair();
      const keyPair2 = await generateKeyPair();

      expect(keyPair1.publicKey).not.toBe(keyPair2.publicKey);
      expect(keyPair1.privateKey).not.toBe(keyPair2.privateKey);
    });
  });

  describe("sign and verify", () => {
    it("should sign a message and verify the signature", async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode("Hello, World!");

      const signature = await sign(keyPair.privateKey, message);
      const isValid = await verify(keyPair.publicKey, message, signature);

      expect(isValid).toBe(true);
    });

    it("should return hex-encoded signature", async () => {
      const keyPair = await generateKeyPair();
      const message = new Uint8Array([1, 2, 3, 4]);

      const signature = await sign(keyPair.privateKey, message);

      // Ed25519 signatures are 64 bytes = 128 hex chars
      expect(signature).toMatch(/^[0-9a-f]+$/);
      expect(signature).toHaveLength(128);
    });

    it("should fail verification with wrong public key", async () => {
      const keyPair1 = await generateKeyPair();
      const keyPair2 = await generateKeyPair();
      const message = new TextEncoder().encode("Test message");

      const signature = await sign(keyPair1.privateKey, message);
      const isValid = await verify(keyPair2.publicKey, message, signature);

      expect(isValid).toBe(false);
    });

    it("should fail verification with modified message", async () => {
      const keyPair = await generateKeyPair();
      const originalMessage = new TextEncoder().encode("Original");
      const modifiedMessage = new TextEncoder().encode("Modified");

      const signature = await sign(keyPair.privateKey, originalMessage);
      const isValid = await verify(keyPair.publicKey, modifiedMessage, signature);

      expect(isValid).toBe(false);
    });

    it("should fail verification with tampered signature", async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode("Test");

      const signature = await sign(keyPair.privateKey, message);
      // Tamper with the signature by changing first character
      const tamperedSignature =
        signature[0] === "a" ? "b" + signature.slice(1) : "a" + signature.slice(1);

      const isValid = await verify(keyPair.publicKey, message, tamperedSignature);
      expect(isValid).toBe(false);
    });

    it("should sign empty message", async () => {
      const keyPair = await generateKeyPair();
      const emptyMessage = new Uint8Array(0);

      const signature = await sign(keyPair.privateKey, emptyMessage);
      const isValid = await verify(keyPair.publicKey, emptyMessage, signature);

      expect(signature).toHaveLength(128);
      expect(isValid).toBe(true);
    });

    it("should sign large message", async () => {
      const keyPair = await generateKeyPair();
      const largeMessage = new Uint8Array(10000);
      crypto.getRandomValues(largeMessage);

      const signature = await sign(keyPair.privateKey, largeMessage);
      const isValid = await verify(keyPair.publicKey, largeMessage, signature);

      expect(isValid).toBe(true);
    });

    it("should produce different signatures for different messages", async () => {
      const keyPair = await generateKeyPair();
      const message1 = new TextEncoder().encode("Message 1");
      const message2 = new TextEncoder().encode("Message 2");

      const signature1 = await sign(keyPair.privateKey, message1);
      const signature2 = await sign(keyPair.privateKey, message2);

      expect(signature1).not.toBe(signature2);
    });

    it("should produce same signature for same message with same key", async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode("Deterministic");

      const signature1 = await sign(keyPair.privateKey, message);
      const signature2 = await sign(keyPair.privateKey, message);

      // Ed25519 is deterministic
      expect(signature1).toBe(signature2);
    });
  });

  describe("verify error handling", () => {
    it("should return false for invalid public key format", async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode("Test");
      const signature = await sign(keyPair.privateKey, message);

      // Invalid hex (too short)
      const isValid = await verify("invalid", message, signature);
      expect(isValid).toBe(false);
    });

    it("should return false for invalid signature format", async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode("Test");

      // Invalid signature
      const isValid = await verify(keyPair.publicKey, message, "invalid");
      expect(isValid).toBe(false);
    });

    it("should return false for truncated signature", async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode("Test");
      const signature = await sign(keyPair.privateKey, message);

      // Truncate signature
      const isValid = await verify(keyPair.publicKey, message, signature.slice(0, 64));
      expect(isValid).toBe(false);
    });
  });

  describe("keyId", () => {
    it("should generate a key ID from public key", async () => {
      const keyPair = await generateKeyPair();
      const id = await keyId(keyPair.publicKey);

      expect(id).toBeDefined();
      expect(typeof id).toBe("string");
    });

    it("should generate 16 hex character key ID", async () => {
      const keyPair = await generateKeyPair();
      const id = await keyId(keyPair.publicKey);

      // First 8 bytes of SHA-256 = 16 hex chars
      expect(id).toHaveLength(16);
      expect(id).toMatch(/^[0-9a-f]+$/);
    });

    it("should generate same key ID for same public key", async () => {
      const keyPair = await generateKeyPair();
      const id1 = await keyId(keyPair.publicKey);
      const id2 = await keyId(keyPair.publicKey);

      expect(id1).toBe(id2);
    });

    it("should generate different key IDs for different public keys", async () => {
      const keyPair1 = await generateKeyPair();
      const keyPair2 = await generateKeyPair();

      const id1 = await keyId(keyPair1.publicKey);
      const id2 = await keyId(keyPair2.publicKey);

      expect(id1).not.toBe(id2);
    });

    it("should handle known input deterministically", async () => {
      // Fixed public key for deterministic test
      const fixedPublicKey = "0".repeat(64); // 32 zero bytes
      const id = await keyId(fixedPublicKey);

      // SHA-256 of 32 zero bytes, first 8 bytes as hex
      // This tests that the hashing works correctly
      expect(id).toHaveLength(16);
      expect(id).toMatch(/^[0-9a-f]+$/);
    });
  });

  describe("hashRestoreRequest", () => {
    const testRequest = {
      requestId: "req-123",
      requester: "owner@example.com",
      snapshotId: "snap-456",
      reason: "Need to restore files",
      keyHolderId: "holder-789",
      paths: ["/data/important", "/config"],
      createdAtUnix: 1704067200,
    };

    it("should return a Uint8Array hash", async () => {
      const hash = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(hash).toBeInstanceOf(Uint8Array);
    });

    it("should return 32-byte SHA-256 hash", async () => {
      const hash = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(hash).toHaveLength(32);
    });

    it("should produce deterministic hash for same inputs", async () => {
      const hash1 = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const hash2 = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(hash1).toEqual(hash2);
    });

    it("should produce different hash for different requestId", async () => {
      const hash1 = await hashRestoreRequest(
        "req-123",
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const hash2 = await hashRestoreRequest(
        "req-456",
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(hash1).not.toEqual(hash2);
    });

    it("should produce different hash for different reason", async () => {
      const hash1 = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        "Reason A",
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const hash2 = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        "Reason B",
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(hash1).not.toEqual(hash2);
    });

    it("should produce same hash regardless of path order (sorted)", async () => {
      const hash1 = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        ["/config", "/data/important"],
        testRequest.createdAtUnix
      );

      const hash2 = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        ["/data/important", "/config"],
        testRequest.createdAtUnix
      );

      expect(hash1).toEqual(hash2);
    });

    it("should handle empty paths array", async () => {
      const hash = await hashRestoreRequest(
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        [],
        testRequest.createdAtUnix
      );

      expect(hash).toHaveLength(32);
    });

    it("should handle empty strings", async () => {
      const hash = await hashRestoreRequest("", "", "", "", "", [], 0);

      expect(hash).toHaveLength(32);
    });
  });

  describe("signRestoreRequest and verifyRestoreRequestSignature", () => {
    const testRequest = {
      requestId: "req-123",
      requester: "owner@example.com",
      snapshotId: "snap-456",
      reason: "Need to restore files",
      keyHolderId: "holder-789",
      paths: ["/data/important", "/config"],
      createdAtUnix: 1704067200,
    };

    it("should sign and verify a restore request", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const isValid = await verifyRestoreRequestSignature(
        keyPair.publicKey,
        signature,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(isValid).toBe(true);
    });

    it("should return hex signature", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(signature).toMatch(/^[0-9a-f]+$/);
      expect(signature).toHaveLength(128);
    });

    it("should fail verification with different requestId", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const isValid = await verifyRestoreRequestSignature(
        keyPair.publicKey,
        signature,
        "different-id", // Different requestId
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(isValid).toBe(false);
    });

    it("should fail verification with different reason", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const isValid = await verifyRestoreRequestSignature(
        keyPair.publicKey,
        signature,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        "Different reason", // Different reason
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(isValid).toBe(false);
    });

    it("should fail verification with different paths", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const isValid = await verifyRestoreRequestSignature(
        keyPair.publicKey,
        signature,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        ["/different/path"], // Different paths
        testRequest.createdAtUnix
      );

      expect(isValid).toBe(false);
    });

    it("should fail verification with different timestamp", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const isValid = await verifyRestoreRequestSignature(
        keyPair.publicKey,
        signature,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix + 1 // Different timestamp
      );

      expect(isValid).toBe(false);
    });

    it("should fail verification with wrong public key", async () => {
      const keyPair1 = await generateKeyPair();
      const keyPair2 = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair1.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      const isValid = await verifyRestoreRequestSignature(
        keyPair2.publicKey, // Wrong public key
        signature,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        testRequest.paths,
        testRequest.createdAtUnix
      );

      expect(isValid).toBe(false);
    });

    it("should verify with paths in any order (canonical sorting)", async () => {
      const keyPair = await generateKeyPair();

      const signature = await signRestoreRequest(
        keyPair.privateKey,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        ["/b", "/a", "/c"],
        testRequest.createdAtUnix
      );

      // Verify with different order
      const isValid = await verifyRestoreRequestSignature(
        keyPair.publicKey,
        signature,
        testRequest.requestId,
        testRequest.requester,
        testRequest.snapshotId,
        testRequest.reason,
        testRequest.keyHolderId,
        ["/c", "/a", "/b"], // Different order
        testRequest.createdAtUnix
      );

      expect(isValid).toBe(true);
    });
  });

  describe("hex conversion (toHex/fromHex)", () => {
    it("should convert bytes to hex correctly", () => {
      const bytes = new Uint8Array([0, 1, 255, 16, 32]);
      const hex = toHex(bytes);
      expect(hex).toBe("0001ff1020");
    });

    it("should convert hex to bytes correctly", () => {
      const bytes = fromHex("0001ff1020");
      expect(bytes).toEqual(new Uint8Array([0, 1, 255, 16, 32]));
    });

    it("should round-trip bytes to hex and back", () => {
      const original = new Uint8Array([0, 127, 128, 255]);
      const hex = toHex(original);
      const back = fromHex(hex);
      expect(back).toEqual(original);
    });

    it("should handle empty bytes", () => {
      const empty = new Uint8Array(0);
      expect(toHex(empty)).toBe("");
    });

    it("should handle empty hex string", () => {
      const bytes = fromHex("");
      expect(bytes).toEqual(new Uint8Array(0));
    });

    it("should handle uppercase hex input", () => {
      const bytes = fromHex("AABBCC");
      expect(bytes).toEqual(new Uint8Array([170, 187, 204]));
    });

    it("should handle mixed case hex input", () => {
      const bytes = fromHex("aAbBcC");
      expect(bytes).toEqual(new Uint8Array([170, 187, 204]));
    });

    it("should pad single-digit hex values with leading zero", () => {
      const bytes = new Uint8Array([0, 1, 15]);
      const hex = toHex(bytes);
      expect(hex).toBe("00010f");
    });

    it("should handle all byte values 0-255", () => {
      const allBytes = new Uint8Array(256);
      for (let i = 0; i < 256; i++) {
        allBytes[i] = i;
      }

      const hex = toHex(allBytes);
      const back = fromHex(hex);

      expect(back).toEqual(allBytes);
    });

    it("should coerce invalid hex characters to 0", () => {
      // fromHex uses parseInt which returns NaN for invalid chars
      // But Uint8Array coerces NaN to 0
      const bytes = fromHex("gg");
      expect(bytes[0]).toBe(0);
    });

    it("should handle odd-length hex (truncates)", () => {
      // fromHex uses hex.length / 2 which truncates odd lengths
      const bytes = fromHex("abc");
      // "abc" -> length 3 / 2 = 1 byte, parses "ab"
      expect(bytes).toHaveLength(1);
      expect(bytes[0]).toBe(0xab);
    });
  });

  describe("crypto round-trip integration", () => {
    it("should work end-to-end: generate, sign, verify", async () => {
      // Generate keys
      const keyPair = await generateKeyPair();

      // Create a message
      const message = new TextEncoder().encode(
        JSON.stringify({ action: "approve", requestId: "123" })
      );

      // Sign
      const signature = await sign(keyPair.privateKey, message);

      // Verify
      const isValid = await verify(keyPair.publicKey, message, signature);
      expect(isValid).toBe(true);

      // Get key ID
      const id = await keyId(keyPair.publicKey);
      expect(id).toHaveLength(16);
    });

    it("should support multiple signers", async () => {
      const signer1 = await generateKeyPair();
      const signer2 = await generateKeyPair();
      const message = new TextEncoder().encode("Multi-party consensus");

      const sig1 = await sign(signer1.privateKey, message);
      const sig2 = await sign(signer2.privateKey, message);

      // Both signatures should verify with their respective public keys
      expect(await verify(signer1.publicKey, message, sig1)).toBe(true);
      expect(await verify(signer2.publicKey, message, sig2)).toBe(true);

      // Cross-verification should fail
      expect(await verify(signer1.publicKey, message, sig2)).toBe(false);
      expect(await verify(signer2.publicKey, message, sig1)).toBe(false);
    });
  });
});
