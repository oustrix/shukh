package server

import crand "crypto/rand"

// codeAlphabet is 32 symbols: 2-9 and A-Z minus I and O (and no 0/1). L is kept
// (not dropped) so the set totals exactly 32 = 2^5, letting rng()&0x1f pick a
// symbol without modulo bias. L is the least confusable choice once I/1 are gone.
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
