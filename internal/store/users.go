package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// users.go implements the auth-related slices of Store: accounts, login
// cookie sessions, and API keys. One file per domain; items.go shows the
// same pattern for a business entity.

const userColumns = `id, username, email, password_hash, display_name, role, status, created_at, updated_at`

func scanUser(scanner interface{ Scan(dest ...any) error }) (*User, error) {
	var u User
	if err := scanner.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// --- Users ---

func (d *DBStore) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (d *DBStore) CreateUser(ctx context.Context, u *User) error {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO users (id, username, email, password_hash, display_name, role, status, created_at, updated_at)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)`,
			d.ph(1), d.ph(2), d.ph(3), d.ph(4), d.ph(5), d.ph(6), d.ph(7), d.ph(8), d.ph(9)),
		u.ID, u.Username, u.Email, u.PasswordHash, u.DisplayName, u.Role, u.Status, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		if uniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (d *DBStore) GetUser(ctx context.Context, id string) (*User, error) {
	row := d.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT `+userColumns+` FROM users WHERE id = %s`, d.ph(1)), id)
	u, err := scanUser(row)
	return u, scanErr(err)
}

func (d *DBStore) GetUserByLogin(ctx context.Context, usernameOrEmail string) (*User, error) {
	row := d.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT `+userColumns+` FROM users WHERE username = %s OR email = %s LIMIT 1`, d.ph(1), d.ph(2)),
		usernameOrEmail, usernameOrEmail)
	u, err := scanUser(row)
	return u, scanErr(err)
}

func (d *DBStore) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := d.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM users ORDER BY created_at ASC`, userColumns))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// UpdateUser rewrites the mutable columns from the record. Callers load
// with GetUser, mutate fields, and save — read-modify-write keeps the
// SQL surface tiny.
func (d *DBStore) UpdateUser(ctx context.Context, u *User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE users SET username = %s, email = %s, password_hash = %s, display_name = %s, role = %s, status = %s, updated_at = %s WHERE id = %s`,
			d.ph(1), d.ph(2), d.ph(3), d.ph(4), d.ph(5), d.ph(6), d.ph(7), d.ph(8)),
		u.Username, u.Email, u.PasswordHash, u.DisplayName, u.Role, u.Status, u.UpdatedAt, u.ID)
	return err
}

func (d *DBStore) DeleteUser(ctx context.Context, id string) error {
	// Cascade by hand in one transaction. There are no foreign keys by
	// design (see db.go), so nothing cascades on its own — every table
	// that owns user rows must be listed here, or deleting an account
	// silently orphans them.
	//
	// interview_turns goes before interviews (it points at them) and both
	// are listed because a user's interviews are theirs alone.
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM web_sessions WHERE user_id = ` + d.ph(1),
		`DELETE FROM apikeys WHERE user_id = ` + d.ph(1),
		`DELETE FROM interview_turns WHERE user_id = ` + d.ph(1),
		`DELETE FROM interviews WHERE user_id = ` + d.ph(1),
		`DELETE FROM files WHERE user_id = ` + d.ph(1),
		`DELETE FROM users WHERE id = ` + d.ph(1),
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Web sessions ---

func (d *DBStore) CreateWebSession(ctx context.Context, s *WebSession) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO web_sessions (sid, user_id, created_at, expires_at) VALUES (%s, %s, %s, %s)`,
			d.ph(1), d.ph(2), d.ph(3), d.ph(4)),
		s.SID, s.UserID, s.CreatedAt, s.ExpiresAt)
	return err
}

func (d *DBStore) GetWebSession(ctx context.Context, sid string) (*WebSession, error) {
	row := d.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT sid, user_id, created_at, expires_at FROM web_sessions WHERE sid = %s`, d.ph(1)), sid)
	var s WebSession
	err := row.Scan(&s.SID, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, scanErr(err)
	}
	return &s, nil
}

func (d *DBStore) DeleteWebSession(ctx context.Context, sid string) error {
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM web_sessions WHERE sid = %s`, d.ph(1)), sid)
	return err
}

func (d *DBStore) DeleteExpiredWebSessions(ctx context.Context, before time.Time) error {
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM web_sessions WHERE expires_at < %s`, d.ph(1)), before)
	return err
}

// --- API keys ---

func scanAPIKey(scanner interface{ Scan(dest ...any) error }) (*APIKey, error) {
	var k APIKey
	if err := scanner.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Type, &k.CreatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

const apiKeyColumns = `id, user_id, name, key_hash, key_prefix, type, created_at`

func (d *DBStore) CreateAPIKey(ctx context.Context, k *APIKey) error {
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now().UTC()
	}
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO apikeys (id, user_id, name, key_hash, key_prefix, type, created_at) VALUES (%s, %s, %s, %s, %s, %s, %s)`,
			d.ph(1), d.ph(2), d.ph(3), d.ph(4), d.ph(5), d.ph(6), d.ph(7)),
		k.ID, k.UserID, k.Name, k.KeyHash, k.KeyPrefix, k.Type, k.CreatedAt)
	return err
}

func (d *DBStore) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	row := d.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT `+apiKeyColumns+` FROM apikeys WHERE id = %s`, d.ph(1)), id)
	k, err := scanAPIKey(row)
	return k, scanErr(err)
}

func (d *DBStore) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := d.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT `+apiKeyColumns+` FROM apikeys WHERE user_id = %s ORDER BY created_at ASC`, d.ph(1)), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (d *DBStore) DeleteAPIKey(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM apikeys WHERE id = %s`, d.ph(1)), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RotateAPIKey swaps the token in place: same key id, new secret.
func (d *DBStore) RotateAPIKey(ctx context.Context, id, keyHash, keyPrefix string) error {
	res, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE apikeys SET key_hash = %s, key_prefix = %s WHERE id = %s`, d.ph(1), d.ph(2), d.ph(3)),
		keyHash, keyPrefix, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// LookupAPIKeyByHash is the auth hot path: hash the presented token and
// hit the unique index.
func (d *DBStore) LookupAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	row := d.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT `+apiKeyColumns+` FROM apikeys WHERE key_hash = %s`, d.ph(1)), keyHash)
	k, err := scanAPIKey(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return k, err
}
