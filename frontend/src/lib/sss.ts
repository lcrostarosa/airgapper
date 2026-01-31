/**
 * Shamir's Secret Sharing in TypeScript
 * Mirrors the Go implementation in internal/sss/sss.go
 * Uses GF(256) with irreducible polynomial x^8 + x^4 + x^3 + x + 1
 */

export interface Share {
  index: number; // 1-255
  data: Uint8Array;
}

/**
 * Split a secret into n shares, requiring k shares to reconstruct
 */
export function split(secret: Uint8Array, k: number, n: number): Share[] {
  if (k < 2) throw new Error("threshold k must be at least 2");
  if (n < k) throw new Error("n must be >= k");
  if (n > 255) throw new Error("n must be <= 255");

  const shares: Share[] = [];
  for (let i = 0; i < n; i++) {
    shares.push({
      index: i + 1,
      data: new Uint8Array(secret.length),
    });
  }

  // For each byte of the secret, create a random polynomial and evaluate
  for (let byteIdx = 0; byteIdx < secret.length; byteIdx++) {
    const secretByte = secret[byteIdx];

    // Generate k-1 random coefficients for the polynomial
    // polynomial: f(x) = secret + a1*x + a2*x^2 + ... + a(k-1)*x^(k-1)
    const coefficients = new Uint8Array(k);
    coefficients[0] = secretByte;

    // Generate random coefficients
    const randomBytes = new Uint8Array(k - 1);
    crypto.getRandomValues(randomBytes);
    for (let i = 1; i < k; i++) {
      coefficients[i] = randomBytes[i - 1];
    }

    // Evaluate polynomial at each share index
    for (let i = 0; i < n; i++) {
      const x = shares[i].index;
      shares[i].data[byteIdx] = evaluatePolynomial(coefficients, x);
    }
  }

  return shares;
}

/**
 * Combine shares to reconstruct the secret
 */
export function combine(shares: Share[]): Uint8Array {
  if (shares.length < 2) {
    throw new Error("need at least 2 shares to reconstruct");
  }

  const secretLen = shares[0].data.length;
  for (const s of shares) {
    if (s.data.length !== secretLen) {
      throw new Error("all shares must have the same length");
    }
  }

  const secret = new Uint8Array(secretLen);

  // For each byte position, use Lagrange interpolation
  for (let byteIdx = 0; byteIdx < secretLen; byteIdx++) {
    // Collect the (x, y) points for this byte
    const points: [number, number][] = shares.map((share) => [
      share.index,
      share.data[byteIdx],
    ]);

    // Lagrange interpolation at x=0 gives us the secret
    secret[byteIdx] = lagrangeInterpolate(points, 0);
  }

  return secret;
}

function evaluatePolynomial(coefficients: Uint8Array, x: number): number {
  if (x === 0) return coefficients[0];

  let result = 0;
  let xPower = 1;

  for (const coeff of coefficients) {
    result = gfAdd(result, gfMul(coeff, xPower));
    xPower = gfMul(xPower, x);
  }

  return result;
}

function lagrangeInterpolate(points: [number, number][], x: number): number {
  let result = 0;

  for (let i = 0; i < points.length; i++) {
    const [xi, yi] = points[i];

    // Calculate Lagrange basis polynomial
    let basis = 1;
    for (let j = 0; j < points.length; j++) {
      if (i === j) continue;
      const xj = points[j][0];

      // basis *= (x - xj) / (xi - xj)
      const num = gfAdd(x, xj);
      const den = gfAdd(xi, xj);
      basis = gfMul(basis, gfMul(num, gfInverse(den)));
    }

    result = gfAdd(result, gfMul(yi, basis));
  }

  return result;
}

// GF(256) arithmetic using the irreducible polynomial x^8 + x^4 + x^3 + x + 1

function gfAdd(a: number, b: number): number {
  return a ^ b;
}

function gfMul(a: number, b: number): number {
  let result = 0;
  let aa = a;
  let bb = b;

  while (bb > 0) {
    if (bb & 1) {
      result ^= aa;
    }
    const highBit = aa & 0x80;
    aa <<= 1;
    if (highBit) {
      aa ^= 0x1b; // x^8 + x^4 + x^3 + x + 1
    }
    aa &= 0xff;
    bb >>= 1;
  }

  return result;
}

function gfInverse(a: number): number {
  if (a === 0) return 0;
  // Use exponentiation: a^254 = a^(-1) in GF(256)
  let result = a;
  for (let i = 0; i < 6; i++) {
    result = gfMul(result, result);
    result = gfMul(result, a);
  }
  result = gfMul(result, result);
  return result;
}

/**
 * Generate a random password (32 bytes = 64 hex chars)
 */
export function generatePassword(): Uint8Array {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return bytes;
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
