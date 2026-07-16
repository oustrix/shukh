package server

import crand "crypto/rand"

// codeAlphabet is 32 unambiguous symbols — no O/0, I/1/L. 32 = 2^5, so masking a
// byte to its low 5 bits selects a symbol without modulo bias.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const codeLen = 6

// newCode builds a 6-char room code (§5.1), drawing each symbol from the
// unambiguous alphabet via 5 bits of rng. Injecting rng makes it deterministic in
// tests; production passes cryptoBytes.
func newCode(rng func() byte) string {
	var b [codeLen]byte
	for i := range b {
		b[i] = codeAlphabet[rng()&0x1f]
	}
	return string(b[:])
}

// cryptoBytes returns a byte source backed by crypto/rand. A crypto/rand failure is
// unrecoverable, so it panics rather than return a weak code.
func cryptoBytes() func() byte {
	return func() byte {
		var b [1]byte
		if _, err := crand.Read(b[:]); err != nil {
			panic("server: crypto/rand failed: " + err.Error())
		}
		return b[0]
	}
}
