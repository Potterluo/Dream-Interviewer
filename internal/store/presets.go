package store

import (
	"context"
	"encoding/json"
	"time"
)

// presets.go stores admin-created interview presets.
//
// The built-in presets are compiled into internal/interview and are NOT
// rows here. That split is deliberate: the curated set is part of the
// product and should not be editable (or deletable) by accident, while an
// admin's own presets are data. The server concatenates the two and marks
// which is which, so "why can't I edit this one?" has an obvious answer.

// Preset is an admin-authored shared interview preset.
type Preset struct {
	ID            string
	Role          string
	Level         string
	InterviewType string
	Difficulty    string
	QuestionCount int
	FocusAreas    []string
	JDSample      string
	Description   string
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PresetStore is the admin preset surface.
type PresetStore interface {
	ListPresets(ctx context.Context) ([]Preset, error)
	GetPreset(ctx context.Context, id string) (*Preset, error)
	CreatePreset(ctx context.Context, p *Preset) error
	UpdatePreset(ctx context.Context, p *Preset) error
	DeletePreset(ctx context.Context, id string) error
}

const presetColumns = `id, role, level, interview_type, difficulty, question_count,
	focus_areas, jd_sample, description, created_by, created_at, updated_at`

func scanPreset(scanner interface{ Scan(dest ...any) error }) (*Preset, error) {
	var (
		p     Preset
		focus string
	)
	if err := scanner.Scan(
		&p.ID, &p.Role, &p.Level, &p.InterviewType, &p.Difficulty,
		&p.QuestionCount, &focus, &p.JDSample, &p.Description,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	// A malformed focus list must not take down the presets page: it is
	// display data, and an empty list is a usable degradation.
	if focus != "" {
		_ = json.Unmarshal([]byte(focus), &p.FocusAreas)
	}
	if p.FocusAreas == nil {
		p.FocusAreas = []string{}
	}
	return &p, nil
}

func encodeFocusAreas(list []string) string {
	if len(list) == 0 {
		return "[]"
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (d *DBStore) ListPresets(ctx context.Context) ([]Preset, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT `+presetColumns+` FROM interview_presets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Preset{}
	for rows.Next() {
		p, err := scanPreset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (d *DBStore) GetPreset(ctx context.Context, id string) (*Preset, error) {
	row := d.db.QueryRowContext(ctx, d.rebind(
		`SELECT `+presetColumns+` FROM interview_presets WHERE id = ?`), id)
	p, err := scanPreset(row)
	return p, scanErr(err)
}

func (d *DBStore) CreatePreset(ctx context.Context, p *Preset) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now
	_, err := d.db.ExecContext(ctx, d.rebind(
		`INSERT INTO interview_presets (`+presetColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`),
		p.ID, p.Role, p.Level, p.InterviewType, p.Difficulty, p.QuestionCount,
		encodeFocusAreas(p.FocusAreas), p.JDSample, p.Description,
		p.CreatedBy, p.CreatedAt, p.UpdatedAt)
	if uniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

func (d *DBStore) UpdatePreset(ctx context.Context, p *Preset) error {
	p.UpdatedAt = time.Now().UTC()
	res, err := d.db.ExecContext(ctx, d.rebind(
		`UPDATE interview_presets SET role = ?, level = ?, interview_type = ?,
		        difficulty = ?, question_count = ?, focus_areas = ?, jd_sample = ?,
		        description = ?, updated_at = ?
		 WHERE id = ?`),
		p.Role, p.Level, p.InterviewType, p.Difficulty, p.QuestionCount,
		encodeFocusAreas(p.FocusAreas), p.JDSample, p.Description,
		p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DBStore) DeletePreset(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx, d.rebind(
		`DELETE FROM interview_presets WHERE id = ?`), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
