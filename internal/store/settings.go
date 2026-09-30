package store

import (
	"context"
	"time"
)

// settings.go is the runtime configuration store behind the admin panel
// (docs/API.md §6). It is a flat key/value table on purpose: the settings it
// holds change as the product grows, and a column per knob would mean a
// migration per knob.
//
// Only what an admin explicitly stored lives here. Precedence (DB > env/.env
// > default) is resolved by the server, because that is a product decision,
// not a storage one.

// SettingsStore is the admin-configurable configuration surface.
type SettingsStore interface {
	AllSettings(ctx context.Context) (map[string]string, error)
	PutSetting(ctx context.Context, key, value string) error
	DeleteSetting(ctx context.Context, key string) error
	ClearSettings(ctx context.Context) error
}

// AllSettings returns every stored override. It returns an empty map rather
// than nil when nothing is configured, so callers can range over it safely.
func (d *DBStore) AllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// PutSetting upserts one key. The ON CONFLICT form is understood by both
// SQLite (3.24+) and PostgreSQL, so a single statement serves both dialects.
func (d *DBStore) PutSetting(ctx context.Context, key, value string) error {
	_, err := d.db.ExecContext(ctx, d.rebind(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`),
		key, value, time.Now().UTC())
	return err
}

// DeleteSetting removes one override, reverting that field to the
// environment/default. Deleting an absent key is not an error: the desired
// end state is "no override", which is already true.
func (d *DBStore) DeleteSetting(ctx context.Context, key string) error {
	_, err := d.db.ExecContext(ctx, d.rebind(`DELETE FROM app_settings WHERE key = ?`), key)
	return err
}

// ClearSettings drops every override — the escape hatch for an admin who has
// stored a broken endpoint and can no longer reach a working model.
func (d *DBStore) ClearSettings(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM app_settings`)
	return err
}
