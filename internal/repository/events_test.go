package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestInsertEvents(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()

	evs := []domain.Event{
		{TS: 100, SessionID: "s1", PromptID: "p1", EventName: "claude_code.user_prompt", Attrs: map[string]any{"prompt_length": int64(10)}},
		{TS: 200, SessionID: "s1", PromptID: "p1", EventName: "claude_code.api_request", Attrs: map[string]any{"cost_usd": 0.01, "model": "claude-opus-4-7"}},
	}
	if err := repo.InsertEvents(context.Background(), evs); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}

	rows, err := repo.DB().QueryContext(context.Background(), "SELECT ts, session_id, COALESCE(prompt_id,''), event_name, attrs FROM events ORDER BY ts")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []domain.Event
	for rows.Next() {
		var e domain.Event
		var attrsJSON string
		if err := rows.Scan(&e.TS, &e.SessionID, &e.PromptID, &e.EventName, &attrsJSON); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if err := json.Unmarshal([]byte(attrsJSON), &e.Attrs); err != nil {
			t.Fatalf("unmarshal attrs: %v", err)
		}
		got = append(got, e)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if got[0].EventName != "claude_code.user_prompt" || got[1].EventName != "claude_code.api_request" {
		t.Fatalf("ordering wrong: %+v", got)
	}
	if got[0].Attrs["prompt_length"].(float64) != 10 {
		t.Fatalf("attr roundtrip failed: %#v", got[0].Attrs)
	}
}

func TestInsertEvents_EmptyIsNoop(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	if err := repo.InsertEvents(context.Background(), nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := repo.InsertEvents(context.Background(), []domain.Event{}); err != nil {
		t.Fatalf("empty: %v", err)
	}
}

func openTempRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}
