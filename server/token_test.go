package server

import (
	"encoding/base64"
	"testing"
)

func TestNewTokenDistinct(t *testing.T) {
	seen := map[Token]bool{}
	for i := 0; i < 100; i++ {
		tok, err := newToken()
		if err != nil {
			t.Fatalf("newToken: %v", err)
		}
		if seen[tok] {
			t.Fatalf("duplicate token minted: %q", tok)
		}
		seen[tok] = true
	}
}

func TestNewTokenDecodesTo16Bytes(t *testing.T) {
	tok, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(string(tok))
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(raw) != 16 {
		t.Fatalf("decoded token length = %d, want 16 (~128 bits, L2-7)", len(raw))
	}
}
