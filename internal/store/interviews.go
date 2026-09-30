package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// interviews.go is the persistence slice for the AI interviewer domain:
// one row per interview (config + plan + report) and one row per question
// turn (question, answer, grade).
//
// Everything the model produced is stored as JSON text in a column
// (plan_json / grade_json / report_json) rather than exploded into
// relational tables. That is a deliberate trade: the shapes are owned by
// internal/interview and change with prompt iterations, while the columns
// that must be queried or aggregated — status, score, recommendation,
// role, timings — are real columns. The result is that analytics never
// parses JSON and schema migrations never chase a prompt change.

// Interview is one interview session.
type Interview struct {
	ID             string
	UserID         string
	Title          string
	Role           string
	Level          string
	InterviewType  string
	Language       string
	Difficulty     string
	QuestionCount  int
	ResumeText     string
	JDText         string
	Status         string
	PlanJSON       string
	CurrentSeq     int
	TurnCount      int
	OverallScore   float64
	HRSatisfaction float64
	Recommendation string
	ReportJSON     string
	Summary        string
	Model          string
	Engine         string
	DurationSec    int
	StartedAt      *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// InterviewTurn is one question (planned or follow-up) and its answer.
type InterviewTurn struct {
	ID            string
	InterviewID   string
	UserID        string
	Seq           int
	Kind          string
	Dimension     string
	Question      string
	Intent        string
	Weight        float64
	Answer        string
	Score         float64
	GradeJSON     string
	Feedback      string
	AnswerSeconds int
	CreatedAt     time.Time
	AnsweredAt    *time.Time
}

// InterviewSummary is the lean projection used by list pages and the
// analytics aggregator: no plan/report/resume payloads.
type InterviewSummary struct {
	ID             string
	Title          string
	Role           string
	Level          string
	InterviewType  string
	Status         string
	OverallScore   float64
	HRSatisfaction float64
	Recommendation string
	TurnCount      int
	DurationSec    int
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

const interviewColumns = `id, user_id, title, role, level, interview_type, language, difficulty,
	question_count, resume_text, jd_text, status, plan_json, current_seq, turn_count,
	overall_score, hr_satisfaction, recommendation, report_json, summary, model, engine,
	duration_sec, started_at, completed_at, created_at, updated_at`

const interviewTurnColumns = `id, interview_id, user_id, seq, kind, dimension, question, intent,
	weight, answer, score, grade_json, feedback, answer_seconds, created_at, answered_at`

// nullTime converts a nullable timestamp column into *time.Time.
func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

func scanInterview(scanner interface{ Scan(dest ...any) error }) (*Interview, error) {
	var (
		iv          Interview
		startedAt   sql.NullTime
		completedAt sql.NullTime
	)
	if err := scanner.Scan(
		&iv.ID, &iv.UserID, &iv.Title, &iv.Role, &iv.Level, &iv.InterviewType,
		&iv.Language, &iv.Difficulty, &iv.QuestionCount, &iv.ResumeText, &iv.JDText,
		&iv.Status, &iv.PlanJSON, &iv.CurrentSeq, &iv.TurnCount, &iv.OverallScore,
		&iv.HRSatisfaction, &iv.Recommendation, &iv.ReportJSON, &iv.Summary,
		&iv.Model, &iv.Engine, &iv.DurationSec, &startedAt, &completedAt,
		&iv.CreatedAt, &iv.UpdatedAt,
	); err != nil {
		return nil, err
	}
	iv.StartedAt = nullTime(startedAt)
	iv.CompletedAt = nullTime(completedAt)
	return &iv, nil
}

func scanInterviewTurn(scanner interface{ Scan(dest ...any) error }) (*InterviewTurn, error) {
	var (
		t          InterviewTurn
		answeredAt sql.NullTime
	)
	if err := scanner.Scan(
		&t.ID, &t.InterviewID, &t.UserID, &t.Seq, &t.Kind, &t.Dimension, &t.Question,
		&t.Intent, &t.Weight, &t.Answer, &t.Score, &t.GradeJSON, &t.Feedback,
		&t.AnswerSeconds, &t.CreatedAt, &answeredAt,
	); err != nil {
		return nil, err
	}
	t.AnsweredAt = nullTime(answeredAt)
	return &t, nil
}

// --- interviews -------------------------------------------------------------

func (d *DBStore) CreateInterview(ctx context.Context, iv *Interview) error {
	now := time.Now().UTC()
	iv.CreatedAt = now
	iv.UpdatedAt = now
	if iv.QuestionCount == 0 {
		iv.QuestionCount = 6
	}
	if iv.Status == "" {
		iv.Status = "draft"
	}
	_, err := d.db.ExecContext(ctx, d.rebind(
		`INSERT INTO interviews (`+interviewColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`),
		iv.ID, iv.UserID, iv.Title, iv.Role, iv.Level, iv.InterviewType,
		iv.Language, iv.Difficulty, iv.QuestionCount, iv.ResumeText, iv.JDText,
		iv.Status, iv.PlanJSON, iv.CurrentSeq, iv.TurnCount, iv.OverallScore,
		iv.HRSatisfaction, iv.Recommendation, iv.ReportJSON, iv.Summary,
		iv.Model, iv.Engine, iv.DurationSec, iv.StartedAt, iv.CompletedAt,
		iv.CreatedAt, iv.UpdatedAt)
	return err
}

func (d *DBStore) GetInterview(ctx context.Context, id string) (*Interview, error) {
	row := d.db.QueryRowContext(ctx, d.rebind(
		`SELECT `+interviewColumns+` FROM interviews WHERE id = ?`), id)
	iv, err := scanInterview(row)
	return iv, scanErr(err)
}

// ListInterviews returns the user's interviews, newest first. An empty
// userID means "all owners" — admin-only, enforced by the caller.
func (d *DBStore) ListInterviews(ctx context.Context, userID string) ([]Interview, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if userID == "" {
		rows, err = d.db.QueryContext(ctx,
			`SELECT `+interviewColumns+` FROM interviews ORDER BY created_at DESC`)
	} else {
		rows, err = d.db.QueryContext(ctx, d.rebind(
			`SELECT `+interviewColumns+` FROM interviews WHERE user_id = ? ORDER BY created_at DESC`), userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Interview{}
	for rows.Next() {
		iv, err := scanInterview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *iv)
	}
	return out, rows.Err()
}

// ListInterviewSummaries is the lean read for lists and analytics.
func (d *DBStore) ListInterviewSummaries(ctx context.Context, userID string) ([]InterviewSummary, error) {
	q := `SELECT id, title, role, level, interview_type, status, overall_score,
	             hr_satisfaction, recommendation, turn_count, duration_sec,
	             created_at, completed_at
	      FROM interviews`
	args := []any{}
	if userID != "" {
		q += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := d.db.QueryContext(ctx, d.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InterviewSummary{}
	for rows.Next() {
		var (
			s           InterviewSummary
			completedAt sql.NullTime
		)
		if err := rows.Scan(&s.ID, &s.Title, &s.Role, &s.Level, &s.InterviewType, &s.Status,
			&s.OverallScore, &s.HRSatisfaction, &s.Recommendation, &s.TurnCount,
			&s.DurationSec, &s.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		s.CompletedAt = nullTime(completedAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListInterviewReportJSON returns the report_json blobs of the user's
// completed interviews. Analytics needs per-dimension scores, which live
// only inside the report; this reads that one column so the aggregation
// does not drag résumé and JD text through memory.
func (d *DBStore) ListInterviewReportJSON(ctx context.Context, userID string) ([]string, error) {
	q := `SELECT report_json FROM interviews WHERE status = 'completed' AND report_json <> ''`
	args := []any{}
	if userID != "" {
		q += ` AND user_id = ?`
		args = append(args, userID)
	}
	rows, err := d.db.QueryContext(ctx, d.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var blob string
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		out = append(out, blob)
	}
	return out, rows.Err()
}

func (d *DBStore) UpdateInterview(ctx context.Context, iv *Interview) error {
	iv.UpdatedAt = time.Now().UTC()
	res, err := d.db.ExecContext(ctx, d.rebind(
		`UPDATE interviews SET title = ?, role = ?, level = ?, interview_type = ?,
		        language = ?, difficulty = ?, question_count = ?, resume_text = ?, jd_text = ?,
		        status = ?, plan_json = ?, current_seq = ?, turn_count = ?, overall_score = ?,
		        hr_satisfaction = ?, recommendation = ?, report_json = ?, summary = ?,
		        model = ?, engine = ?, duration_sec = ?, started_at = ?, completed_at = ?,
		        updated_at = ?
		 WHERE id = ?`),
		iv.Title, iv.Role, iv.Level, iv.InterviewType, iv.Language, iv.Difficulty,
		iv.QuestionCount, iv.ResumeText, iv.JDText, iv.Status, iv.PlanJSON,
		iv.CurrentSeq, iv.TurnCount, iv.OverallScore, iv.HRSatisfaction,
		iv.Recommendation, iv.ReportJSON, iv.Summary, iv.Model, iv.Engine,
		iv.DurationSec, iv.StartedAt, iv.CompletedAt, iv.UpdatedAt, iv.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteInterview removes an interview and every turn it owns, in one
// transaction. Turns have no foreign key (the schema carries no FKs by
// design), so orphaning them here would silently corrupt analytics.
func (d *DBStore) DeleteInterview(ctx context.Context, id string) error {
	return d.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, d.rebind(
			`DELETE FROM interview_turns WHERE interview_id = ?`), id); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, d.rebind(`DELETE FROM interviews WHERE id = ?`), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// --- turns ------------------------------------------------------------------

func (d *DBStore) CreateInterviewTurn(ctx context.Context, t *InterviewTurn) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if t.Weight <= 0 {
		t.Weight = 1
	}
	_, err := d.db.ExecContext(ctx, d.rebind(
		`INSERT INTO interview_turns (`+interviewTurnColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`),
		t.ID, t.InterviewID, t.UserID, t.Seq, t.Kind, t.Dimension, t.Question,
		t.Intent, t.Weight, t.Answer, t.Score, t.GradeJSON, t.Feedback,
		t.AnswerSeconds, t.CreatedAt, t.AnsweredAt)
	return err
}

func (d *DBStore) GetInterviewTurn(ctx context.Context, id string) (*InterviewTurn, error) {
	row := d.db.QueryRowContext(ctx, d.rebind(
		`SELECT `+interviewTurnColumns+` FROM interview_turns WHERE id = ?`), id)
	t, err := scanInterviewTurn(row)
	return t, scanErr(err)
}

// GetLatestInterviewTurn returns the highest-seq turn — the one awaiting
// an answer.
func (d *DBStore) GetLatestInterviewTurn(ctx context.Context, interviewID string) (*InterviewTurn, error) {
	row := d.db.QueryRowContext(ctx, d.rebind(
		`SELECT `+interviewTurnColumns+` FROM interview_turns
		 WHERE interview_id = ? ORDER BY seq DESC LIMIT 1`), interviewID)
	t, err := scanInterviewTurn(row)
	return t, scanErr(err)
}

func (d *DBStore) ListInterviewTurns(ctx context.Context, interviewID string) ([]InterviewTurn, error) {
	rows, err := d.db.QueryContext(ctx, d.rebind(
		`SELECT `+interviewTurnColumns+` FROM interview_turns WHERE interview_id = ? ORDER BY seq ASC`),
		interviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InterviewTurn{}
	for rows.Next() {
		t, err := scanInterviewTurn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// AnswerInterviewTurn persists the answer and grade for a turn, but ONLY
// while that turn is still open (`answered_at IS NULL`).
//
// That predicate is the compare-and-set which stops two concurrent
// `answer` requests — a double-clicked button, a client retry, two tabs —
// from grading the same question twice. Without it both requests would
// call the model (paying twice) and the later write would silently win.
// ErrNotFound means "somebody already answered this".
func (d *DBStore) AnswerInterviewTurn(ctx context.Context, t *InterviewTurn) error {
	res, err := d.db.ExecContext(ctx, d.rebind(
		`UPDATE interview_turns SET kind = ?, dimension = ?, question = ?, intent = ?,
		        weight = ?, answer = ?, score = ?, grade_json = ?, feedback = ?,
		        answer_seconds = ?, answered_at = ?
		 WHERE id = ? AND answered_at IS NULL`),
		t.Kind, t.Dimension, t.Question, t.Intent, t.Weight, t.Answer, t.Score,
		t.GradeJSON, t.Feedback, t.AnswerSeconds, t.AnsweredAt, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// inTx runs fn inside a transaction, rolling back on error or panic.
func (d *DBStore) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
