package server

import (
	"strings"
	"testing"
)

func TestNewCodeShapeAndAlphabet(t *testing.T) {
	code := newCode(cryptoBytes())
	if len(code) != 6 {
		t.Fatalf("code length = %d, want 6", len(code))
	}
	for _, r := range code {
		if !strings.ContainsRune(codeAlphabet, r) {
			t.Fatalf("code %q contains out-of-alphabet rune %q", code, r)
		}
	}
	// no visually ambiguous symbols leak in (O, 0, I, 1 are excluded; L required for 32=2^5 chars)
	for _, bad := range []rune{'O', '0', 'I', '1'} {
		if strings.ContainsRune(codeAlphabet, bad) {
			t.Fatalf("alphabet must not contain ambiguous %q", bad)
		}
	}
}

func TestNewCodeDeterministicForFixedSource(t *testing.T) {
	src := func() func() byte {
		buf := []byte{0, 1, 2, 3, 30, 31, 40, 41}
		i := 0
		return func() byte { b := buf[i%len(buf)]; i++; return b }
	}
	a := newCode(src())
	b := newCode(src())
	if a != b {
		t.Fatalf("same byte source must yield same code: %q vs %q", a, b)
	}
	// index masks to 5 bits: 0->'2', 1->'3', 2->'4', 3->'5', 30->'Y', 31->'Z'
	if a != "2345YZ" {
		t.Fatalf("deterministic code = %q, want 2345YZ", a)
	}
}
