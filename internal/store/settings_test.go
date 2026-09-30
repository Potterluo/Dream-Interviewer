package store

import (
	"context"
	"errors"
	"testing"
)

// Settings and presets are the admin surfaces added for real RBAC, so they
// get the same store-level coverage as the interview domain.

func TestSettingsRoundTripAndClear(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	all, err := st.AllSettings(ctx)
	if err != nil {
		t.Fatalf("AllSettings: %v", err)
	}
	if all == nil {
		t.Fatal("AllSettings must return an empty map, not nil")
	}
	if len(all) != 0 {
		t.Fatalf("a fresh store already has settings: %v", all)
	}

	if err := st.PutSetting(ctx, "llm.model", "first"); err != nil {
		t.Fatalf("PutSetting: %v", err)
	}
	if err := st.PutSetting(ctx, "llm.base_url", "https://example.test/v1"); err != nil {
		t.Fatal(err)
	}
	// Upsert, not insert-and-fail: the admin panel saves repeatedly.
	if err := st.PutSetting(ctx, "llm.model", "second"); err != nil {
		t.Fatalf("PutSetting must upsert: %v", err)
	}

	all, _ = st.AllSettings(ctx)
	if all["llm.model"] != "second" {
		t.Fatalf("upsert did not replace the value: %q", all["llm.model"])
	}
	if all["llm.base_url"] != "https://example.test/v1" {
		t.Fatalf("unrelated key lost: %v", all)
	}

	if err := st.DeleteSetting(ctx, "llm.model"); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	all, _ = st.AllSettings(ctx)
	if _, ok := all["llm.model"]; ok {
		t.Fatal("the key was not deleted")
	}
	// Deleting an absent key is the desired end state already: not an error.
	if err := st.DeleteSetting(ctx, "llm.model"); err != nil {
		t.Fatalf("deleting an absent key must not error: %v", err)
	}

	if err := st.ClearSettings(ctx); err != nil {
		t.Fatalf("ClearSettings: %v", err)
	}
	all, _ = st.AllSettings(ctx)
	if len(all) != 0 {
		t.Fatalf("ClearSettings left rows: %v", all)
	}
}

func TestPresetCRUDAndFocusAreasRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	p := &Preset{
		ID: "ps_1", Role: "资深数据工程师", Level: "senior",
		InterviewType: "tech", Difficulty: "hard", QuestionCount: 8,
		FocusAreas: []string{"数仓分层", "数据质量", "调度"},
		JDSample:   "负责数据平台建设。", Description: "偏工程",
		CreatedBy: "u_admin",
	}
	if err := st.CreatePreset(ctx, p); err != nil {
		t.Fatalf("CreatePreset: %v", err)
	}

	got, err := st.GetPreset(ctx, "ps_1")
	if err != nil {
		t.Fatalf("GetPreset: %v", err)
	}
	if got.Role != "资深数据工程师" || got.QuestionCount != 8 {
		t.Fatalf("scalar fields did not round-trip: %+v", got)
	}
	// focus_areas is JSON in one column; a round-trip bug here would show up
	// as a silently empty chip list in the UI.
	if len(got.FocusAreas) != 3 || got.FocusAreas[1] != "数据质量" {
		t.Fatalf("focus areas did not round-trip: %#v", got.FocusAreas)
	}
	if got.CreatedBy != "u_admin" {
		t.Fatalf("createdBy = %q", got.CreatedBy)
	}

	got.QuestionCount = 5
	got.FocusAreas = []string{"Kafka"}
	if err := st.UpdatePreset(ctx, got); err != nil {
		t.Fatalf("UpdatePreset: %v", err)
	}
	again, _ := st.GetPreset(ctx, "ps_1")
	if again.QuestionCount != 5 || len(again.FocusAreas) != 1 || again.FocusAreas[0] != "Kafka" {
		t.Fatalf("update did not persist: %+v", again)
	}

	list, err := st.ListPresets(ctx)
	if err != nil {
		t.Fatalf("ListPresets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListPresets returned %d rows", len(list))
	}

	if err := st.DeletePreset(ctx, "ps_1"); err != nil {
		t.Fatalf("DeletePreset: %v", err)
	}
	if _, err := st.GetPreset(ctx, "ps_1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("preset survived deletion: %v", err)
	}
	// Deleting an absent row is a caller error worth surfacing.
	if err := st.DeletePreset(ctx, "ps_1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting an absent preset: err = %v, want ErrNotFound", err)
	}
}

func TestPresetDuplicateIDIsReportedAsDuplicate(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	p := &Preset{ID: "ps_dup", Role: "后端", QuestionCount: 6}
	if err := st.CreatePreset(ctx, p); err != nil {
		t.Fatal(err)
	}
	dup := &Preset{ID: "ps_dup", Role: "另一个", QuestionCount: 6}
	if err := st.CreatePreset(ctx, dup); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err = %v, want ErrDuplicate (the handler maps it to 409)", err)
	}
}

func TestPresetFocusAreasSurviveAMalformedColumn(t *testing.T) {
	// A hand-edited or truncated column must degrade to "no chips" rather
	// than 500 the presets page.
	st := newTestStore(t)
	ctx := context.Background()
	db := st.(*DBStore)
	if err := st.CreatePreset(ctx, &Preset{ID: "ps_bad", Role: "后端", QuestionCount: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, db.rebind(
		`UPDATE interview_presets SET focus_areas = ? WHERE id = ?`), "{not json", "ps_bad"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetPreset(ctx, "ps_bad")
	if err != nil {
		t.Fatalf("a malformed focus_areas column must not fail the read: %v", err)
	}
	if got.FocusAreas == nil {
		t.Fatal("FocusAreas must be non-nil so it serialises as []")
	}
	if len(got.FocusAreas) != 0 {
		t.Fatalf("expected no chips, got %#v", got.FocusAreas)
	}
}

func TestMigrateCreatesAdminTables(t *testing.T) {
	st := newTestStore(t)
	db := st.(*DBStore)
	for _, table := range []string{"app_settings", "interview_presets"} {
		has, err := db.tableHasColumn(context.Background(), table, "id")
		if err != nil {
			t.Fatalf("tableHasColumn(%s): %v", table, err)
		}
		if table == "app_settings" {
			// app_settings is keyed by `key`, not `id`.
			has, err = db.tableHasColumn(context.Background(), "app_settings", "key")
			if err != nil {
				t.Fatal(err)
			}
		}
		if !has {
			t.Fatalf("migration did not create %s", table)
		}
	}
}
