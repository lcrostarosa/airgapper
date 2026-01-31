import { describe, it, expect } from "vitest";
import { split, combine, generatePassword, toHex, fromHex } from "./sss";

describe("Shamir's Secret Sharing", () => {
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

  it("should convert to/from hex correctly", () => {
    const bytes = new Uint8Array([0, 1, 255, 16, 32]);
    const hex = toHex(bytes);
    expect(hex).toBe("0001ff1020");

    const back = fromHex(hex);
    expect(back).toEqual(bytes);
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
