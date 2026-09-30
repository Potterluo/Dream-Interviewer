package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/store"
)

// API key lifecycle: generation, hashing, and lookup. Rules this
// template keeps from its production ancestor:
//
//   - The plaintext token is shown to the user exactly ONCE at
//     create/rotate time. The store keeps only SHA-256(token); the
//     plaintext cannot be recovered, so a database leak does not leak
//     usable credentials.
//   - A 10-char prefix ("sk_0123456789") is kept for list displays —
//     enough to recognize a key, far below brute-force feasibility.
//   - Any failure in LookupAPIKeyByToken collapses to a single
//     ErrInvalidCredentials so responses can't distinguish "unknown"
//     from "disabled" for probing clients.

// ErrInvalidCredentials is the ONLY error LookupAPIKeyByToken returns
// for bad tokens.
var ErrInvalidCredentials = errors.New("invalid credentials")

// APIKey is the public representation of a key. Key holds the masked
// display string on list responses, or the freshly issued plaintext on
// create/rotate. KeyHash never leaves the backend.
type APIKey struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name,omitempty"`
	Key       string    `json:"key"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
}

// Resolved is what bearer-auth needs in one round-trip.
type Resolved struct {
	Key  *store.APIKey
	User *store.User
}

// CreateAPIKey issues a new key for userID and returns the APIKey with
// Key set to the plaintext token (show once, never store). keyType must
// be store.APIKeyTypeAdmin or APIKeyTypeUser; the caller is responsible
// for policy (only admins may issue admin-tier keys).
func CreateAPIKey(ctx context.Context, st store.Store, userID, name, keyType string) (*APIKey, error) {
	if userID == "" {
		return nil, errors.New("userID is required")
	}
	if keyType != store.APIKeyTypeAdmin && keyType != store.APIKeyTypeUser {
		return nil, errors.New("invalid key type (want admin|user)")
	}
	id, err := newRandomHex(12)
	if err != nil {
		return nil, err
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	rec := &store.APIKey{
		ID:        "k_" + id,
		UserID:    userID,
		Name:      name,
		KeyHash:   HashToken(token),
		KeyPrefix: keyPrefix(token),
		Type:      keyType,
	}
	if err := st.CreateAPIKey(ctx, rec); err != nil {
		return nil, err
	}
	out := maskedAPIKey(rec)
	out.Key = token
	return out, nil
}

// RotateAPIKey replaces the token of key id. Old token dies immediately.
// Returns the new plaintext (show once).
func RotateAPIKey(ctx context.Context, st store.Store, id string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	if err := st.RotateAPIKey(ctx, id, HashToken(token), keyPrefix(token)); err != nil {
		return "", err
	}
	return token, nil
}

// LookupAPIKeyByToken is the bearer auth hot path. Returns
// ErrInvalidCredentials for ANY failure so the middleware can't leak
// account state.
func LookupAPIKeyByToken(ctx context.Context, st store.Store, token string) (*Resolved, error) {
	if token == "" {
		return nil, ErrInvalidCredentials
	}
	rec, err := st.LookupAPIKeyByHash(ctx, HashToken(token))
	if err != nil {
		if err == store.ErrNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	user, err := st.GetUser(ctx, rec.UserID)
	if err != nil || user.Status != store.StatusActive {
		return nil, ErrInvalidCredentials
	}
	return &Resolved{Key: rec, User: user}, nil
}

// MaskedAPIKey converts a store record to its public form with the Key
// field masked for list responses.
func MaskedAPIKey(rec *store.APIKey) *APIKey { return maskedAPIKey(rec) }

func maskedAPIKey(rec *store.APIKey) *APIKey {
	masked := rec.KeyPrefix + "****"
	return &APIKey{
		ID:        rec.ID,
		UserID:    rec.UserID,
		Name:      rec.Name,
		Key:       masked,
		Type:      rec.Type,
		CreatedAt: rec.CreatedAt,
	}
}

// HashToken is the one-way storage form of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// keyPrefix keeps a recognizable slice of the plaintext for UI display.
func keyPrefix(token string) string {
	if len(token) <= 10 {
		return token
	}
	return token[:10]
}

// newToken mints a bearer token: "sk_" + 32 random bytes as hex.
func newToken() (string, error) {
	h, err := newRandomHex(32)
	if err != nil {
		return "", err
	}
	return "sk_" + h, nil
}
