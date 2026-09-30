package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// newTestStore opens a throwaway SQLite store in the test's temp dir.
// Tests hit the real driver and the real migrations — the SQL is the thing
// under test, so faking the store here would test nothing.
func newTestStore(t *testing.T) Store {
	t.Helper()
	st, err := New(&StorageConfig{Type: "sqlite", AutoMigrate: true}, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func seedInterview(t *testing.T, st Store, id, userID string) *Interview {
	t.Helper()
	iv := &Interview{
		ID: id, UserID: userID, Title: "t", Role: "后端", Level: "senior",
		InterviewType: "tech", Language: "zh", Difficulty: "normal",
		QuestionCount: 3, Status: "draft",
	}
	if err := st.CreateInterview(context.Background(), iv); err != nil {
		t.Fatalf("CreateInterview: %v", err)
	}
	return iv
}

// The guard that stops a double-clicked Answer button (or a retried
// request, or a second tab) from grading the same question twice.
func TestAnswerInterviewTurnIsACompareAndSet(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	seedInterview(t, st, "iv_1", "u_1")

	turn := &InterviewTurn{
		ID: "tr_1", InterviewID: "iv_1", UserID: "u_1", Seq: 1,
		Kind: "question", Dimension: "accuracy", Question: "Q1", Weight: 1,
	}
	if err := st.CreateInterviewTurn(ctx, turn); err != nil {
		t.Fatalf("CreateInterviewTurn: %v", err)
	}

	now := time.Now().UTC()
	turn.Answer = "first answer"
	turn.Score = 80
	turn.Feedback = "good"
	turn.AnsweredAt = &now
	if err := st.AnswerInterviewTurn(ctx, turn); err != nil {
		t.Fatalf("first answer must be accepted: %v", err)
	}

	// Second writer, same turn: must be rejected, not silently applied.
	turn.Answer = "second answer"
	turn.Score = 5
	err := st.AnswerInterviewTurn(ctx, turn)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("second answer: err = %v, want ErrNotFound", err)
	}

	got, err := st.GetInterviewTurn(ctx, "tr_1")
	if err != nil {
		t.Fatalf("GetInterviewTurn: %v", err)
	}
	if got.Answer != "first answer" || got.Score != 80 {
		t.Fatalf("the first answer was overwritten: answer=%q score=%v", got.Answer, got.Score)
	}
	if got.AnsweredAt == nil {
		t.Fatal("answered_at was not persisted")
	}
}

func TestPendingTurnSemanticsSurviveASkip(t *testing.T) {
	// A skipped turn keeps an EMPTY answer but counts as answered, which is
	// why "pending" is defined by answered_at rather than by empty text.
	st := newTestStore(t)
	ctx := context.Background()
	seedInterview(t, st, "iv_2", "u_1")

	turn := &InterviewTurn{ID: "tr_2", InterviewID: "iv_2", UserID: "u_1", Seq: 1,
		Kind: "question", Question: "Q1", Weight: 1}
	if err := st.CreateInterviewTurn(ctx, turn); err != nil {
		t.Fatal(err)
	}
	latest, err := st.GetLatestInterviewTurn(ctx, "iv_2")
	if err != nil {
		t.Fatalf("GetLatestInterviewTurn: %v", err)
	}
	if latest.AnsweredAt != nil {
		t.Fatal("a freshly created turn must be open")
	}

	now := time.Now().UTC()
	turn.AnsweredAt = &now
	turn.Score = 0
	turn.Answer = ""
	if err := st.AnswerInterviewTurn(ctx, turn); err != nil {
		t.Fatalf("skip must be accepted: %v", err)
	}
	latest, _ = st.GetLatestInterviewTurn(ctx, "iv_2")
	if latest.AnsweredAt == nil {
		t.Fatal("a skipped turn must be marked answered, or it stays pending forever")
	}
	if latest.Answer != "" {
		t.Fatalf("a skip should carry no text, got %q", latest.Answer)
	}
}

func TestDeleteInterviewCascadesTurns(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	seedInterview(t, st, "iv_3", "u_1")
	for i := 1; i <= 3; i++ {
		if err := st.CreateInterviewTurn(ctx, &InterviewTurn{
			ID: "tr_3_" + string(rune('0'+i)), InterviewID: "iv_3", UserID: "u_1",
			Seq: i, Kind: "question", Question: "Q", Weight: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := st.DeleteInterview(ctx, "iv_3"); err != nil {
		t.Fatalf("DeleteInterview: %v", err)
	}
	turns, err := st.ListInterviewTurns(ctx, "iv_3")
	if err != nil {
		t.Fatalf("ListInterviewTurns: %v", err)
	}
	if len(turns) != 0 {
		t.Fatalf("%d turns survived the interview delete (they would corrupt analytics)", len(turns))
	}
	if _, err := st.GetInterview(ctx, "iv_3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("interview still readable: %v", err)
	}
}

func TestOwningUserFilterAndSummaryProjection(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	seedInterview(t, st, "iv_a", "u_1")
	seedInterview(t, st, "iv_b", "u_2")

	mine, err := st.ListInterviews(ctx, "u_1")
	if err != nil {
		t.Fatalf("ListInterviews: %v", err)
	}
	if len(mine) != 1 || mine[0].ID != "iv_a" {
		t.Fatalf("owner filter leaked rows: %+v", mine)
	}

	// Empty userID means "all owners" — admin-only, enforced by handlers.
	all, err := st.ListInterviews(ctx, "")
	if err != nil {
		t.Fatalf("ListInterviews(all): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("admin view returned %d rows, want 2", len(all))
	}

	summaries, err := st.ListInterviewSummaries(ctx, "u_1")
	if err != nil {
		t.Fatalf("ListInterviewSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
}

func TestReportBlobsOnlyFromCompletedInterviews(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	a := seedInterview(t, st, "iv_r1", "u_1")
	a.Status = "completed"
	a.ReportJSON = `{"overallScore":70}`
	if err := st.UpdateInterview(ctx, a); err != nil {
		t.Fatal(err)
	}
	b := seedInterview(t, st, "iv_r2", "u_1")
	b.Status = "in_progress"
	b.ReportJSON = `{"overallScore":10}`
	if err := st.UpdateInterview(ctx, b); err != nil {
		t.Fatal(err)
	}

	blobs, err := st.ListInterviewReportJSON(ctx, "u_1")
	if err != nil {
		t.Fatalf("ListInterviewReportJSON: %v", err)
	}
	if len(blobs) != 1 {
		t.Fatalf("got %d blobs, want only the completed one: %v", len(blobs), blobs)
	}
	if blobs[0] != `{"overallScore":70}` {
		t.Fatalf("wrong blob: %q", blobs[0])
	}
}

// Migrate runs on every boot, so it must be safe to call repeatedly — and
// the layer-2 backfill must reconcile a turn_count that drifted.
func TestMigrateIsIdempotentAndRepairsTurnCount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	db, ok := st.(*DBStore)
	if !ok {
		t.Fatalf("expected *DBStore, got %T", st)
	}

	seedInterview(t, st, "iv_m", "u_1")
	for i := 1; i <= 2; i++ {
		if err := st.CreateInterviewTurn(ctx, &InterviewTurn{
			ID: "tr_m_" + string(rune('0'+i)), InterviewID: "iv_m", UserID: "u_1",
			Seq: i, Kind: "question", Question: "Q", Weight: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	iv, _ := st.GetInterview(ctx, "iv_m")
	if iv.TurnCount != 0 {
		t.Fatalf("precondition: turn_count should still be the stale 0, got %d", iv.TurnCount)
	}

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate must be a no-op, got: %v", err)
	}

	iv, _ = st.GetInterview(ctx, "iv_m")
	if iv.TurnCount != 2 {
		t.Fatalf("backfill did not repair turn_count: got %d, want 2", iv.TurnCount)
	}
}

// The template's cascade still deleted from the example `items` table
// after that domain was removed, so EVERY `DELETE /api/users/{id}`
// aborted the transaction with "no such table: items", returned 500, and
// left the account's interviews, turns and files orphaned. This pins the
// real list of user-owned tables.
func TestDeleteUserCascadesOwnedRows(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	u := &User{ID: "u_del", Username: "delme", Email: "del@example.com",
		Role: RoleUser, Status: StatusActive}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	seedInterview(t, st, "iv_del", "u_del")
	if err := st.CreateInterviewTurn(ctx, &InterviewTurn{
		ID: "tr_del", InterviewID: "iv_del", UserID: "u_del",
		Seq: 1, Kind: "question", Question: "Q", Weight: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateFileRecord(ctx, &FileRecord{
		ID: "f_del", UserID: "u_del", Name: "resume.pdf", Size: 12,
	}); err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteUser(ctx, "u_del"); err != nil {
		t.Fatalf("DeleteUser must not fail (it used to 500 on the deleted items table): %v", err)
	}
	if _, err := st.GetUser(ctx, "u_del"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("user survived: %v", err)
	}
	if _, err := st.GetInterview(ctx, "iv_del"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the user's interview was orphaned: %v", err)
	}
	turns, err := st.ListInterviewTurns(ctx, "iv_del")
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 0 {
		t.Fatalf("%d turns were orphaned", len(turns))
	}
	files, err := st.ListFileRecords(ctx, "u_del")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("%d file records were orphaned", len(files))
	}
}

func TestInterviewRoundTripPreservesNullableTimestamps(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	iv := seedInterview(t, st, "iv_ts", "u_1")
	if iv.StartedAt != nil || iv.CompletedAt != nil {
		t.Fatal("a fresh interview must have NULL timestamps")
	}

	start := time.Now().UTC().Truncate(time.Millisecond)
	iv.Status = "in_progress"
	iv.StartedAt = &start
	if err := st.UpdateInterview(ctx, iv); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetInterview(ctx, "iv_ts")
	if err != nil {
		t.Fatal(err)
	}
	if got.StartedAt == nil {
		t.Fatal("started_at did not round-trip")
	}
	if !got.StartedAt.Equal(start) {
		t.Fatalf("started_at drifted: got %v want %v", got.StartedAt, start)
	}
	if got.CompletedAt != nil {
		t.Fatal("completed_at must stay NULL until the interview finishes")
	}
}

func TestUpdateInterviewOnMissingRowIsNotFound(t *testing.T) {
	st := newTestStore(t)
	err := st.UpdateInterview(context.Background(), &Interview{ID: "nope", UserID: "u_1"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// Guards the DSN path the tests above depend on, and would catch a
// migration that silently fails to create the interview tables.
func TestMigrateCreatesInterviewTables(t *testing.T) {
	st := newTestStore(t)
	db := st.(*DBStore)
	for _, table := range []string{"interviews", "interview_turns"} {
		has, err := db.tableHasColumn(context.Background(), table, "id")
		if err != nil {
			t.Fatalf("tableHasColumn(%s): %v", table, err)
		}
		if !has {
			t.Fatalf("migration did not create %s", table)
		}
	}
}
