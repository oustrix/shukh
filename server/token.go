package server

import (
	crand "crypto/rand"
	"encoding/base64"
)

// Token is an opaque per-seat reconnect secret (~128 bits, L2-7). It lives in an
// HttpOnly cookie; the server maps it to a PlayerID. Knowing the token grants the
// seat — like a share link to a document. No JWT, no signature: state is server-side.
type Token string

// newToken mints 16 crypto-random bytes as base64url (no padding, URL-safe).
func newToken() (Token, error) {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "", err
	}
	return Token(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
