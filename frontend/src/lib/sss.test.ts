import { describe, it, expect } from "vitest";
import {
  split,
  combine,
  generatePassword,
  toHex,
  fromHex,
  type Share,
} from "./sss";

describe("Shamir's Secret Sharing", () => {
  describe("split and combine", () => {
    it("should split and recombine a secret", () => {
      const secret = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8]);
      const shares = split(secret, 2, 2);

      expect(shares).toHaveLength(2);
      expect(shares[0].index).toBe(1);
      expect(shares[1].index).toBe(2);

      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should work with random passwords", () => {
      const password = generatePassword();
      expect(password).toHaveLength(32);

      const shares = split(password, 2, 2);
      const recovered = combine(shares);

      expect(recovered).toEqual(password);
    });

    it("should work with shares in any order", () => {
      const secret = generatePassword();
      const shares = split(secret, 2, 2);

      // Reverse order
      const recovered = combine([shares[1], shares[0]]);
      expect(recovered).toEqual(secret);
    });

    it("should produce different shares each time", () => {
      const secret = new Uint8Array([1, 2, 3, 4]);
      const shares1 = split(secret, 2, 2);
      const shares2 = split(secret, 2, 2);

      // Shares should be different (with overwhelming probability)
      expect(shares1[0].data).not.toEqual(shares2[0].data);

      // But both should reconstruct to the same secret
      expect(combine(shares1)).toEqual(secret);
      expect(combine(shares2)).toEqual(secret);
    });
  });

  describe("empty inputs", () => {
    it("should handle empty secret", () => {
      const secret = new Uint8Array(0);
      const shares = split(secret, 2, 2);

      expect(shares).toHaveLength(2);
      expect(shares[0].data).toHaveLength(0);
      expect(shares[1].data).toHaveLength(0);

      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should convert empty bytes to empty hex", () => {
      const empty = new Uint8Array(0);
      expect(toHex(empty)).toBe("");
    });

    it("should convert empty hex to empty bytes", () => {
      const bytes = fromHex("");
      expect(bytes).toEqual(new Uint8Array(0));
    });
  });

  describe("hex conversion", () => {
    it("should convert to/from hex correctly", () => {
      const bytes = new Uint8Array([0, 1, 255, 16, 32]);
      const hex = toHex(bytes);
      expect(hex).toBe("0001ff1020");

      const back = fromHex(hex);
      expect(back).toEqual(bytes);
    });

    it("should handle uppercase hex input", () => {
      const bytes = fromHex("AABBCC");
      expect(bytes).toEqual(new Uint8Array([170, 187, 204]));
    });

    it("should handle mixed case hex input", () => {
      const bytes = fromHex("aAbBcC");
      expect(bytes).toEqual(new Uint8Array([170, 187, 204]));
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

  describe("threshold schemes (k-of-n)", () => {
    it("should work with minimum threshold k=2, n=2", () => {
      const secret = new Uint8Array([42]);
      const shares = split(secret, 2, 2);

      expect(shares).toHaveLength(2);
      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should work with 3-of-5 scheme", () => {
      const secret = generatePassword();
      const shares = split(secret, 3, 5);

      expect(shares).toHaveLength(5);

      // Any 3 shares should reconstruct
      const recovered1 = combine([shares[0], shares[1], shares[2]]);
      const recovered2 = combine([shares[2], shares[3], shares[4]]);
      const recovered3 = combine([shares[0], shares[2], shares[4]]);

      expect(recovered1).toEqual(secret);
      expect(recovered2).toEqual(secret);
      expect(recovered3).toEqual(secret);
    });

    it("should work with 2-of-5 scheme using any 2 shares", () => {
      const secret = new Uint8Array([1, 2, 3, 4]);
      const shares = split(secret, 2, 5);

      expect(shares).toHaveLength(5);

      // Test multiple combinations of 2 shares
      expect(combine([shares[0], shares[1]])).toEqual(secret);
      expect(combine([shares[0], shares[4]])).toEqual(secret);
      expect(combine([shares[2], shares[3]])).toEqual(secret);
      expect(combine([shares[1], shares[4]])).toEqual(secret);
    });

    it("should work with 5-of-5 scheme (all shares required)", () => {
      const secret = new Uint8Array([10, 20, 30]);
      const shares = split(secret, 5, 5);

      expect(shares).toHaveLength(5);
      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should fail to correctly reconstruct with fewer than k shares", () => {
      const secret = generatePassword();
      const shares = split(secret, 3, 5);

      // With only 2 shares when k=3, reconstruction should fail
      // (result will be garbage, not the original secret)
      const wrongRecovered = combine([shares[0], shares[1]]);
      expect(wrongRecovered).not.toEqual(secret);
    });
  });

  describe("boundary conditions", () => {
    it("should handle single byte secret", () => {
      const secret = new Uint8Array([0xff]);
      const shares = split(secret, 2, 2);
      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should handle all-zero secret", () => {
      const secret = new Uint8Array([0, 0, 0, 0]);
      const shares = split(secret, 2, 3);
      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should handle all-255 secret", () => {
      const secret = new Uint8Array([255, 255, 255, 255]);
      const shares = split(secret, 2, 3);
      const recovered = combine(shares);
      expect(recovered).toEqual(secret);
    });

    it("should handle large secret (1KB)", () => {
      const secret = new Uint8Array(1024);
      crypto.getRandomValues(secret);

      const shares = split(secret, 2, 3);
      const recovered = combine([shares[0], shares[2]]);
      expect(recovered).toEqual(secret);
    });

    it("should produce shares with correct indices 1 to n", () => {
      const secret = new Uint8Array([1]);
      const shares = split(secret, 2, 10);

      for (let i = 0; i < 10; i++) {
        expect(shares[i].index).toBe(i + 1);
      }
    });
  });

  describe("error cases", () => {
    it("should throw when k < 2", () => {
      const secret = new Uint8Array([1, 2, 3]);
      expect(() => split(secret, 1, 2)).toThrow("threshold k must be at least 2");
    });

    it("should throw when n < k", () => {
      const secret = new Uint8Array([1, 2, 3]);
      expect(() => split(secret, 3, 2)).toThrow("n must be >= k");
    });

    it("should throw when n > 255", () => {
      const secret = new Uint8Array([1, 2, 3]);
      expect(() => split(secret, 2, 256)).toThrow("n must be <= 255");
    });

    it("should throw when combining fewer than 2 shares", () => {
      const secret = new Uint8Array([1, 2, 3]);
      const shares = split(secret, 2, 2);
      expect(() => combine([shares[0]])).toThrow(
        "need at least 2 shares to reconstruct"
      );
    });

    it("should throw when combining zero shares", () => {
      expect(() => combine([])).toThrow("need at least 2 shares to reconstruct");
    });

    it("should throw when shares have different lengths", () => {
      const share1: Share = { index: 1, data: new Uint8Array([1, 2, 3]) };
      const share2: Share = { index: 2, data: new Uint8Array([4, 5]) };
      expect(() => combine([share1, share2])).toThrow(
        "all shares must have the same length"
      );
    });
  });

  describe("share reconstruction order independence", () => {
    it("should reconstruct correctly regardless of share order with 2-of-2", () => {
      const secret = generatePassword();
      const shares = split(secret, 2, 2);

      const order1 = combine([shares[0], shares[1]]);
      const order2 = combine([shares[1], shares[0]]);

      expect(order1).toEqual(secret);
      expect(order2).toEqual(secret);
    });

    it("should reconstruct correctly regardless of share order with 3-of-5", () => {
      const secret = generatePassword();
      const shares = split(secret, 3, 5);

      // Test various orderings of the same 3 shares
      const s0 = shares[0],
        s2 = shares[2],
        s4 = shares[4];

      expect(combine([s0, s2, s4])).toEqual(secret);
      expect(combine([s0, s4, s2])).toEqual(secret);
      expect(combine([s2, s0, s4])).toEqual(secret);
      expect(combine([s2, s4, s0])).toEqual(secret);
      expect(combine([s4, s0, s2])).toEqual(secret);
      expect(combine([s4, s2, s0])).toEqual(secret);
    });

    it("should allow duplicate shares but may produce wrong result", () => {
      const secret = new Uint8Array([1, 2, 3]);
      const shares = split(secret, 2, 3);

      // Using the same share twice won't throw, but will give wrong result
      const duplicateResult = combine([shares[0], shares[0]]);
      // Result is deterministic but wrong (Lagrange interpolation with duplicate x)
      expect(duplicateResult).not.toEqual(secret);
    });
  });

  describe("generatePassword", () => {
    it("should generate 32-byte password", () => {
      const password = generatePassword();
      expect(password).toHaveLength(32);
    });

    it("should generate different passwords each time", () => {
      const p1 = generatePassword();
      const p2 = generatePassword();
      expect(p1).not.toEqual(p2);
    });

    it("should generate valid bytes (0-255)", () => {
      const password = generatePassword();
      for (const byte of password) {
        expect(byte).toBeGreaterThanOrEqual(0);
        expect(byte).toBeLessThanOrEqual(255);
      }
    });
  });
});
