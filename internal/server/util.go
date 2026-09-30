package server

import (
	"crypto/rand"
	"encoding/hex"
)

// randomID mints "<prefix><24 hex chars>" identifiers for rows. Replace
// with UUIDs if your domain prefers them — callers treat IDs as opaques.
func randomID(prefix string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}
