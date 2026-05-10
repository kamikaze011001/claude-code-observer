package repository

import (
	"context"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestRebuildRollups_MatchesLiveIngest(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	ctx := context.Background()

	events := []domain.Event{
		{TS: 100, SessionID: "s1", EventName: "claude_code.session_start",
			Attrs: map[string]any{"project.name": "demo"}},
		{TS: 200, SessionID: "s1", PromptID: "p1", EventName: "claude_code.user_prompt",
			Attrs: map[string]any{"prompt_length": float64(10)}},
		{TS: 300, SessionID: "s1", PromptID: "p1", EventName: "claude_code.api_request",
			Attrs: map[string]any{"input_tokens": float64(50), "cost_usd": 0.01, "query_source": "main"}},
		{TS: 400, SessionID: "s1", PromptID: "p1", EventName: "claude_code.tool_result"},
	}

	if err := repo.InsertEventsAndApplyRollups(ctx, events); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	var liveCost float64
	var livePrompts, liveAPIReq int64
	if err := repo.db.QueryRow(`SELECT cost_usd, prompts, api_requests FROM sessions WHERE session_id='s1'`).
		Scan(&liveCost, &livePrompts, &liveAPIReq); err != nil {
		t.Fatal(err)
	}

	if err := repo.RebuildRollups(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	var rebCost float64
	var rebPrompts, rebAPIReq int64
	if err := repo.db.QueryRow(`SELECT cost_usd, prompts, api_requests FROM sessions WHERE session_id='s1'`).
		Scan(&rebCost, &rebPrompts, &rebAPIReq); err != nil {
		t.Fatal(err)
	}
	if rebCost != liveCost || rebPrompts != livePrompts || rebAPIReq != liveAPIReq {
		t.Errorf("rebuild diverged: live=(%v,%d,%d) rebuilt=(%v,%d,%d)",
			liveCost, livePrompts, liveAPIReq, rebCost, rebPrompts, rebAPIReq)
	}

	// Truth check per ROADMAP demo: SUM(cost_usd) on sessions equals
	// JSON-extracted sum from events.
	var truth float64
	if err := repo.db.QueryRow(`
		SELECT COALESCE(SUM(json_extract(attrs, '$.cost_usd')), 0)
		FROM events WHERE event_name = 'claude_code.api_request'`).Scan(&truth); err != nil {
		t.Fatal(err)
	}
	var sessTotal float64
	if err := repo.db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM sessions`).Scan(&sessTotal); err != nil {
		t.Fatal(err)
	}
	if sessTotal != truth {
		t.Errorf("truth check: SUM(sessions.cost_usd)=%v vs SUM(json cost_usd)=%v", sessTotal, truth)
	}
}

func TestRebuildRollups_IsIdempotent(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	ctx := context.Background()
	events := []domain.Event{
		{TS: 100, SessionID: "s1", EventName: "claude_code.session_start"},
		{TS: 200, SessionID: "s1", PromptID: "p1", EventName: "claude_code.user_prompt"},
	}
	if err := repo.InsertEventsAndApplyRollups(ctx, events); err != nil {
		t.Fatal(err)
	}
	if err := repo.RebuildRollups(ctx); err != nil {
		t.Fatal(err)
	}
	var first int64
	repo.db.QueryRow(`SELECT prompts FROM sessions WHERE session_id='s1'`).Scan(&first)
	if err := repo.RebuildRollups(ctx); err != nil {
		t.Fatal(err)
	}
	var second int64
	repo.db.QueryRow(`SELECT prompts FROM sessions WHERE session_id='s1'`).Scan(&second)
	if first != second {
		t.Errorf("rebuild not idempotent: %d vs %d", first, second)
	}
}
