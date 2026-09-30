package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// DBStore implements Store on top of database/sql for both dialects.
type DBStore struct {
	db      *sql.DB
	dialect string // "sqlite" | "postgres"
}

// NewDBStore opens the database. The DSN is already final at this point
// (factory.go assembles the tuned sqlite DSN).
func NewDBStore(dialect, dsn string) (*DBStore, error) {
	driver := dialect
	if dialect == "postgres" {
		driver = "postgres"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	// modernc.org/sqlite is happiest with a single write connection; WAL
	// already lets readers proceed concurrently.
	if dialect == "sqlite" {
		db.SetMaxOpenConns(1)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &DBStore{db: db, dialect: dialect}, nil
}

func (d *DBStore) Close() error { return d.db.Close() }

// Dialect reports the active dialect ("sqlite" | "postgres") for the
// rare query that must differ per backend.
func (d *DBStore) Dialect() string { return d.dialect }

// ph returns the placeholder for the nth argument: `?` for sqlite, `$n`
// for postgres. Older files in this package write queries with ph(n);
// newer (generated) files write plain `?` and wrap the query with
// rebind() — both produce identical SQL per dialect.
func (d *DBStore) ph(n int) string {
	if d.dialect == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// rebind rewrites every `?` in query to the postgres `$n` form (no-op on
// sqlite). Use it for queries written as plain string literals.
func (d *DBStore) rebind(query string) string {
	if d.dialect != "postgres" {
		return query
	}
	n := 0
	return placeholderRe.ReplaceAllStringFunc(query, func(string) string {
		n++
		return "$" + strconv.Itoa(n)
	})
}

var placeholderRe = regexp.MustCompile(`\?`)

// scanErr normalizes sql.ErrNoRows into the package-level ErrNotFound so
// callers never depend on driver internals.
func scanErr(err error) error {
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	return err
}

// uniqueViolation reports whether err is a UNIQUE constraint failure,
// matching both drivers by their message (sqlite: "UNIQUE constraint
// failed", postgres: "duplicate key value violates unique constraint").
// Good enough for the few columns that carry UNIQUE indexes; if you add
// many, switch to structured error codes per driver.
func uniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// Migrate brings the schema up to date. Two layers, both idempotent:
//
//  1. migrationSQL() — CREATE TABLE IF NOT EXISTS for every table. Fresh
//     installs are fully covered by this layer.
//  2. Imperative migrate* steps for schema EVOLUTION on installs that
//     already have older tables (add column / backfill / rebuild). Each
//     step checks tableHasColumn (or tableExists) first, so it is safe
//     to run on every boot.
//
// This mirrors how a real product grows: new tables go in layer 1, and
// every change to an existing table becomes one small guarded step in
// layer 2. No migration framework, no version table to babysit.
func (d *DBStore) Migrate(ctx context.Context) error {
	for _, stmt := range d.migrationSQL() {
		if _, err := d.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w (stmt: %.80s...)", err, stmt)
		}
	}

	// Layer 2 — schema evolution and data repair on existing installs.
	// Each step is guarded and idempotent, so it is safe on every boot.
	if err := d.migrateBackfillInterviewTurnCount(ctx); err != nil {
		return err
	}

	// Housekeeping: drop expired sessions on every boot rather than
	// running a janitor goroutine.
	_, _ = d.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM web_sessions WHERE expires_at < %s`, d.ph(1)), time.Now().UTC())
	return nil
}

// migrationSQL returns the full schema as idempotent DDL. Timestamps are
// stored as TIMESTAMP columns in UTC everywhere; go's time.Time round-
// trips through both drivers. Keep table definitions alphabetically
// grouped by domain with a comment saying what owns them.
func (d *DBStore) migrationSQL() []string {
	return []string{
		// --- Auth: accounts ---
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			username      TEXT NOT NULL UNIQUE,
			email         TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL DEFAULT '',
			display_name  TEXT NOT NULL DEFAULT '',
			role          TEXT NOT NULL DEFAULT 'user',
			status        TEXT NOT NULL DEFAULT 'active',
			created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// --- Auth: login cookie sessions ---
		`CREATE TABLE IF NOT EXISTS web_sessions (
			sid        TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_web_sessions_user    ON web_sessions (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_web_sessions_expires ON web_sessions (expires_at)`,
		// --- Auth: programmatic API keys ---
		// type values: "admin" | "user". key_hash is SHA-256 hex of the
		// plaintext token; key_prefix keeps a displayable slice.
		`CREATE TABLE IF NOT EXISTS apikeys (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			name       TEXT NOT NULL DEFAULT '',
			key_hash   TEXT NOT NULL,
			key_prefix TEXT NOT NULL DEFAULT '',
			type       TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_apikeys_user     ON apikeys (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_apikeys_key_hash ON apikeys (key_hash)`,
		// --- Uploads: metadata only; bytes on disk under DataDir/files/ ---
		`CREATE TABLE IF NOT EXISTS files (
			id          TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL,
			name        TEXT NOT NULL,
			size        INTEGER NOT NULL DEFAULT 0,
			content_type TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_user ON files (user_id)`,
		// --- Interviews: the product's core domain -----------------------
		// Config + lifecycle + the model's own artifacts. plan_json and
		// report_json are opaque JSON owned by internal/interview; the
		// columns that get queried or aggregated (status, scores,
		// recommendation, role, timings) are real columns, so analytics
		// never parses JSON and a prompt change never needs a migration.
		`CREATE TABLE IF NOT EXISTS interviews (
			id              TEXT PRIMARY KEY,
			user_id         TEXT NOT NULL,
			title           TEXT NOT NULL DEFAULT '',
			role            TEXT NOT NULL DEFAULT '',
			level           TEXT NOT NULL DEFAULT 'mid',
			interview_type  TEXT NOT NULL DEFAULT 'mixed',
			language        TEXT NOT NULL DEFAULT 'zh',
			difficulty      TEXT NOT NULL DEFAULT 'normal',
			question_count  INTEGER NOT NULL DEFAULT 6,
			resume_text     TEXT NOT NULL DEFAULT '',
			jd_text         TEXT NOT NULL DEFAULT '',
			status          TEXT NOT NULL DEFAULT 'draft',
			plan_json       TEXT NOT NULL DEFAULT '',
			current_seq     INTEGER NOT NULL DEFAULT 0,
			turn_count      INTEGER NOT NULL DEFAULT 0,
			overall_score   REAL NOT NULL DEFAULT 0,
			hr_satisfaction REAL NOT NULL DEFAULT 0,
			recommendation  TEXT NOT NULL DEFAULT '',
			report_json     TEXT NOT NULL DEFAULT '',
			summary         TEXT NOT NULL DEFAULT '',
			model           TEXT NOT NULL DEFAULT '',
			engine          TEXT NOT NULL DEFAULT '',
			duration_sec    INTEGER NOT NULL DEFAULT 0,
			started_at      TIMESTAMP,
			completed_at    TIMESTAMP,
			created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_interviews_user   ON interviews (user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_interviews_status ON interviews (user_id, status)`,
		// --- Interview turns: one row per question asked -----------------
		// grade_json holds the full Grade (dimension map + prose); feedback
		// duplicates its human-readable half so list views can render a
		// snippet without parsing JSON for every row.
		`CREATE TABLE IF NOT EXISTS interview_turns (
			id             TEXT PRIMARY KEY,
			interview_id   TEXT NOT NULL,
			user_id        TEXT NOT NULL,
			seq            INTEGER NOT NULL DEFAULT 0,
			kind           TEXT NOT NULL DEFAULT 'question',
			dimension      TEXT NOT NULL DEFAULT '',
			question       TEXT NOT NULL DEFAULT '',
			intent         TEXT NOT NULL DEFAULT '',
			weight         REAL NOT NULL DEFAULT 1,
			answer         TEXT NOT NULL DEFAULT '',
			score          REAL NOT NULL DEFAULT 0,
			grade_json     TEXT NOT NULL DEFAULT '',
			feedback       TEXT NOT NULL DEFAULT '',
			answer_seconds INTEGER NOT NULL DEFAULT 0,
			created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			answered_at    TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_interview_turns_interview ON interview_turns (interview_id, seq)`,
		// --- Runtime configuration set by an admin in the UI ---------------
		// A flat key/value table: the knobs change as the product grows and
		// a column per knob would mean a migration per knob. An ABSENT key
		// means "no override" (fall back to env/.env), which is why the
		// admin panel deletes rather than blanks a field it clears.
		`CREATE TABLE IF NOT EXISTS app_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// --- Admin-authored shared interview presets -----------------------
		// The built-in preset set is compiled into internal/interview and is
		// deliberately NOT stored here, so the curated set cannot be edited
		// or deleted by accident; this table holds only what an admin added.
		`CREATE TABLE IF NOT EXISTS interview_presets (
			id             TEXT PRIMARY KEY,
			role           TEXT NOT NULL DEFAULT '',
			level          TEXT NOT NULL DEFAULT 'mid',
			interview_type TEXT NOT NULL DEFAULT 'mixed',
			difficulty     TEXT NOT NULL DEFAULT 'normal',
			question_count INTEGER NOT NULL DEFAULT 6,
			focus_areas    TEXT NOT NULL DEFAULT '[]',
			jd_sample      TEXT NOT NULL DEFAULT '',
			description    TEXT NOT NULL DEFAULT '',
			created_by     TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_interview_presets_created ON interview_presets (created_at)`,
		// --- gen:migrations ---
	}
}

// migrateBackfillInterviewTurnCount is the worked example of a layer-2
// step: it is guarded (so it costs one cheap pragma lookup on every boot
// and does nothing on a schema that predates the column), and it repairs
// derived state rather than changing shape.
//
// `interviews.turn_count` is a denormalised count of `interview_turns`
// rows. It exists so list pages and analytics do not need a join, which
// means it can drift if a process dies between writing a turn and
// updating its parent. This reconciles that drift.
//
// For a real COLUMN addition (the other use of this layer), the recipe is
// the same shape: tableHasColumn → if missing, ALTER TABLE. Never
// destructive, never a version table.
func (d *DBStore) migrateBackfillInterviewTurnCount(ctx context.Context) error {
	has, err := d.tableHasColumn(ctx, "interviews", "turn_count")
	if err != nil {
		return err
	}
	if !has {
		return nil
	}
	_, err = d.db.ExecContext(ctx, d.rebind(
		`UPDATE interviews
		    SET turn_count = (SELECT COUNT(*) FROM interview_turns t WHERE t.interview_id = interviews.id)
		  WHERE turn_count <> (SELECT COUNT(*) FROM interview_turns t WHERE t.interview_id = interviews.id)`))
	return err
}

// tableHasColumn reports whether table already has column, across both
// dialects.
func (d *DBStore) tableHasColumn(ctx context.Context, table, column string) (bool, error) {
	var q string
	var args []any
	if d.dialect == "postgres" {
		q = `SELECT COUNT(*) FROM information_schema.columns
		     WHERE table_name = $1 AND column_name = $2`
		args = []any{table, column}
	} else {
		// pragma_table_info takes the table name as a string argument,
		// so it is quoted inline and only the column name is a param.
		q = `SELECT COUNT(*) FROM pragma_table_info(` + quoteIdent(table) + `) WHERE name = ?`
		args = []any{column}
	}
	var n int
	if err := d.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// quoteIdent quotes an identifier for sqlite's pragma functions, which
// take a table-name *string* rather than an identifier.
func quoteIdent(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'')
		} else {
			out = append(out, s[i])
		}
	}
	return string(append(out, '\''))
}
