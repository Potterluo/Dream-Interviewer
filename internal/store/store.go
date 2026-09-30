// Package store is the persistence layer: a single Store interface with
// two drivers (SQLite via modernc.org/sqlite — pure Go, no CGO — and
// PostgreSQL via lib/pq).
//
// Design rules this template follows (copied from a production codebase):
//
//   - One interface, defined in terms of plain record structs. Handlers
//     never touch *sql.DB; tests can fake the interface.
//   - Queries are written once with `?` placeholders; ph() rewrites them
//     to $n for postgres so there is exactly one copy of each statement.
//   - Schema lives in migrationSQL() as idempotent CREATE TABLE IF NOT
//     EXISTS statements. Column additions to existing installs go through
//     imperative migrate* steps guarded by tableHasColumn — see
//     migrateItemsAddDoneColumn for the recipe.
//   - Missing rows surface as ErrNotFound, never sql.ErrNoRows.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Get*/lookup methods when no row matches.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned when an INSERT violates a UNIQUE constraint
// (e.g. username/email already taken). Drivers report this differently;
// the store normalizes it so handlers can map it to a 409 cleanly.
var ErrDuplicate = errors.New("duplicate")

// Roles. Admin manages users and can read every user's rows; user is the
// default.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Account statuses.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// API key tiers.
//   - admin: platform authority; can hit /api/users/*. Only admins may
//     issue one.
//   - user:  acts as its owner; the everyday programmatic credential.
const (
	APIKeyTypeAdmin = "admin"
	APIKeyTypeUser  = "user"
)

// User is one account row. PasswordHash is bcrypt — never leave the
// store package in API responses (handlers map to their own DTO).
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	DisplayName  string
	Role         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// WebSession backs the login cookie. SID is a 32-byte random hex string;
// only the SID is stored in the browser.
type WebSession struct {
	SID       string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// APIKey is a programmatic credential. KeyHash is SHA-256 of the
// plaintext token — the plaintext is shown exactly once at create time
// and cannot be recovered. KeyPrefix keeps a short recognizable slice
// ("sk_0123456789") for list displays.
type APIKey struct {
	ID        string
	UserID    string
	Name      string
	KeyHash   string
	KeyPrefix string
	Type      string
	CreatedAt time.Time
}

// Store is the entire persistence surface. Keep methods coarse (one per
// use case) and let the interface grow per domain — see interviews.go for
// the per-entity file convention.
type Store interface {
	Close() error

	// Users.
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, u *User) error
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByLogin(ctx context.Context, usernameOrEmail string) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUser(ctx context.Context, u *User) error
	DeleteUser(ctx context.Context, id string) error

	// Web sessions.
	CreateWebSession(ctx context.Context, s *WebSession) error
	GetWebSession(ctx context.Context, sid string) (*WebSession, error)
	DeleteWebSession(ctx context.Context, sid string) error
	DeleteExpiredWebSessions(ctx context.Context, before time.Time) error

	// API keys.
	CreateAPIKey(ctx context.Context, k *APIKey) error
	GetAPIKey(ctx context.Context, id string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
	RotateAPIKey(ctx context.Context, id, keyHash, keyPrefix string) error
	LookupAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error)

	// Interviews — the product's core domain (see interviews.go).
	// userID "" means "all owners", which is admin-only and enforced by
	// the handler, never by the store.
	InterviewStore

	// Runtime configuration + admin-authored presets (settings.go,
	// presets.go). Both are admin-only at the HTTP layer.
	SettingsStore
	PresetStore

	// File metadata (bytes live on disk; see handlers_files.go).
	CreateFileRecord(ctx context.Context, f *FileRecord) error
	GetFileRecord(ctx context.Context, id string) (*FileRecord, error)
	ListFileRecords(ctx context.Context, userID string) ([]FileRecord, error) // userID "" = all (admin)
	DeleteFileRecord(ctx context.Context, id string) error

	// Generated entities embed their Store interfaces here — see
	// `generator entity` (cmd/generator). Each entity file defines a
	// <Name>Store interface + the DBStore methods implementing it.
	// --- gen:store-interfaces ---
}

// InterviewStore is the interview domain's slice of Store, defined next
// to its records in interviews.go and embedded above.
type InterviewStore interface {
	CreateInterview(ctx context.Context, iv *Interview) error
	GetInterview(ctx context.Context, id string) (*Interview, error)
	ListInterviews(ctx context.Context, userID string) ([]Interview, error)
	ListInterviewSummaries(ctx context.Context, userID string) ([]InterviewSummary, error)
	ListInterviewReportJSON(ctx context.Context, userID string) ([]string, error)
	UpdateInterview(ctx context.Context, iv *Interview) error
	DeleteInterview(ctx context.Context, id string) error

	CreateInterviewTurn(ctx context.Context, t *InterviewTurn) error
	GetInterviewTurn(ctx context.Context, id string) (*InterviewTurn, error)
	GetLatestInterviewTurn(ctx context.Context, interviewID string) (*InterviewTurn, error)
	ListInterviewTurns(ctx context.Context, interviewID string) ([]InterviewTurn, error)
	// AnswerInterviewTurn is a compare-and-set: it writes the answer and
	// grade only while the turn is still open, and returns ErrNotFound if
	// it was already answered. See its implementation for why.
	AnswerInterviewTurn(ctx context.Context, t *InterviewTurn) error
}
