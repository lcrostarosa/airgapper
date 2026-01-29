// Package sss implements Shamir's Secret Sharing for key splitting
package sss

import (
	"crypto/rand"
	"errors"
	"fmt"
)

// Share represents a single share of a split secret
type Share struct {
	Index byte   // Share index (1-255)
	Data  []byte // Share data
}

// Split splits a secret into n shares, requiring k shares to reconstruct
// For Airgapper MVP, we use k=2, n=2 (both parties required)
func Split(secret []byte, k, n int) ([]Share, error) {
	if k < 2 {
		return nil, errors.New("threshold k must be at least 2")
	}
	if n < k {
		return nil, errors.New("n must be >= k")
	}
	if n > 255 {
		return nil, errors.New("n must be <= 255")
	}

	shares := make([]Share, n)
	for i := range shares {
		shares[i] = Share{
			Index: byte(i + 1),
			Data:  make([]byte, len(secret)),
		}
	}

	// For each byte of the secret, create a random polynomial and evaluate
	for byteIdx, secretByte := range secret {
		// Generate k-1 random coefficients for the polynomial
		// polynomial: f(x) = secret + a1*x + a2*x^2 + ... + a(k-1)*x^(k-1)
		coefficients := make([]byte, k)
		coefficients[0] = secretByte

		// Generate random coefficients
		randomBytes := make([]byte, k-1)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random coefficients: %w", err)
		}
		copy(coefficients[1:], randomBytes)

		// Evaluate polynomial at each share index
		for i := 0; i < n; i++ {
			x := shares[i].Index
			shares[i].Data[byteIdx] = evaluatePolynomial(coefficients, x)
		}
	}

	return shares, nil
}

// Combine reconstructs the secret from k or more shares
func Combine(shares []Share) ([]byte, error) {
	if len(shares) < 2 {
		return nil, errors.New("need at least 2 shares to reconstruct")
	}

	// All shares must have the same length
	secretLen := len(shares[0].Data)
	for _, s := range shares {
		if len(s.Data) != secretLen {
			return nil, errors.New("all shares must have the same length")
		}
	}

	secret := make([]byte, secretLen)

	// For each byte position, use Lagrange interpolation
	for byteIdx := 0; byteIdx < secretLen; byteIdx++ {
		// Collect the (x, y) points for this byte
		points := make([][2]byte, len(shares))
		for i, share := range shares {
			points[i] = [2]byte{share.Index, share.Data[byteIdx]}
		}

		// Lagrange interpolation at x=0 gives us the secret
		secret[byteIdx] = lagrangeInterpolate(points, 0)
	}

	return secret, nil
}

// evaluatePolynomial evaluates a polynomial in GF(256)
func evaluatePolynomial(coefficients []byte, x byte) byte {
	if x == 0 {
		return coefficients[0]
	}

	result := byte(0)
	xPower := byte(1)

	for _, coeff := range coefficients {
		result = gfAdd(result, gfMul(coeff, xPower))
		xPower = gfMul(xPower, x)
	}

	return result
}

// lagrangeInterpolate performs Lagrange interpolation at x in GF(256)
func lagrangeInterpolate(points [][2]byte, x byte) byte {
	result := byte(0)

	for i := 0; i < len(points); i++ {
		xi, yi := points[i][0], points[i][1]

		// Calculate Lagrange basis polynomial
		basis := byte(1)
		for j := 0; j < len(points); j++ {
			if i == j {
				continue
			}
			xj := points[j][0]

			// basis *= (x - xj) / (xi - xj)
			num := gfAdd(x, xj)
			den := gfAdd(xi, xj)
			basis = gfMul(basis, gfMul(num, gfInverse(den)))
		}

		result = gfAdd(result, gfMul(yi, basis))
	}

	return result
}

// GF(256) arithmetic using the irreducible polynomial x^8 + x^4 + x^3 + x + 1

func gfAdd(a, b byte) byte {
	return a ^ b
}

func gfMul(a, b byte) byte {
	var result byte = 0
	for b > 0 {
		if b&1 != 0 {
			result ^= a
		}
		highBit := a & 0x80
		a <<= 1
		if highBit != 0 {
			a ^= 0x1b // x^8 + x^4 + x^3 + x + 1
		}
		b >>= 1
	}
	return result
}

func gfInverse(a byte) byte {
	if a == 0 {
		return 0
	}
	// Use extended Euclidean algorithm or lookup table
	// For simplicity, we use exponentiation: a^254 = a^(-1) in GF(256)
	result := a
	for i := 0; i < 6; i++ {
		result = gfMul(result, result)
		result = gfMul(result, a)
	}
	result = gfMul(result, result)
	return result
}
