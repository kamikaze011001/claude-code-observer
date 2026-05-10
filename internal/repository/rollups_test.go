package repository

import (
	"context"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestInsertEventsAndApplyRollups_BuildsSessionAndPromptRows(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	ctx := context.Background()

	events := []domain.Event{
		{TS: 1000, SessionID: "s1", EventName: "claude_code.session_start",
			Attrs: map[string]any{"project.name": "demo", "app.version": "1.0"}},
		{TS: 1100, SessionID: "s1", PromptID: "p1", EventName: "claude_code.user_prompt",
			Attrs: map[string]any{"prompt_length": float64(25), "command_name": "edit", "command_source": "builtin"}},
		{TS: 1200, SessionID: "s1", PromptID: "p1", EventName: "claude_code.api_request",
			Attrs: map[string]any{
				"input_tokens": float64(100), "output_tokens": float64(50),
				"cache_read_tokens": float64(10), "cache_creation_tokens": float64(5),
				"cost_usd": 0.025, "query_source": "main"}},
		{TS: 1300, SessionID: "s1", PromptID: "p1", EventName: "claude_code.tool_result"},
		{TS: 1400, SessionID: "s1", EventName: "claude_code.tool_decision",
			Attrs: map[string]any{"decision": "deny"}},
		{TS: 1500, SessionID: "s1", PromptID: "p1", EventName: "claude_code.api_error"},
		{TS: 1600, SessionID: "s1", EventName: "claude_code.session_end"},
	}

	if err := repo.InsertEventsAndApplyRollups(ctx, events); err != nil {
		t.Fatalf("InsertEventsAndApplyRollups: %v", err)
	}

	var (
		projectName, appVersion sql_NullString
		started, lastSeen       int64
		ended                   *int64
		input, output, cr, cc   int64
		cost                    float64
		apiReq, apiErr, prompts int64
		toolCalls, toolDenied   int64
	)
	row := repo.db.QueryRowContext(ctx, `
		SELECT project_name, app_version, started_at, last_seen_at, ended_at,
		       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		       cost_usd, api_requests, api_errors, prompts, tool_calls, tool_denied
		FROM sessions WHERE session_id = ?`, "s1")
	if err := row.Scan(&projectName, &appVersion, &started, &lastSeen, &ended,
		&input, &output, &cr, &cc, &cost, &apiReq, &apiErr, &prompts, &toolCalls, &toolDenied); err != nil {
		t.Fatalf("scan sessions: %v", err)
	}

	if projectName.String != "demo" || appVersion.String != "1.0" {
		t.Errorf("metadata: %v %v", projectName, appVersion)
	}
	if started != 1000 || lastSeen != 1600 {
		t.Errorf("started/lastSeen = %d/%d want 1000/1600", started, lastSeen)
	}
	if ended == nil || *ended != 1600 {
		t.Errorf("ended_at = %v want 1600", ended)
	}
	if input != 100 || output != 50 || cr != 10 || cc != 5 || cost != 0.025 {
		t.Errorf("tokens/cost wrong: %d %d %d %d %v", input, output, cr, cc, cost)
	}
	if apiReq != 1 || apiErr != 1 || prompts != 1 || toolCalls != 1 || toolDenied != 1 {
		t.Errorf("counters wrong: req=%d err=%d prompts=%d tool=%d denied=%d",
			apiReq, apiErr, prompts, toolCalls, toolDenied)
	}

	var (
		pl            int64
		hadErr        int64
		promptStarted int64
	)
	row = repo.db.QueryRowContext(ctx, `
		SELECT started_at, prompt_length, had_error FROM prompts WHERE prompt_id = ?`, "p1")
	if err := row.Scan(&promptStarted, &pl, &hadErr); err != nil {
		t.Fatalf("scan prompts: %v", err)
	}
	if promptStarted != 1100 || pl != 25 || hadErr != 1 {
		t.Errorf("prompt: started=%d len=%d had_err=%d", promptStarted, pl, hadErr)
	}
}

func TestInsertEventsAndApplyRollups_EmptyBatchIsNoop(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	if err := repo.InsertEventsAndApplyRollups(context.Background(), nil); err != nil {
		t.Fatalf("nil batch: %v", err)
	}
	if err := repo.InsertEventsAndApplyRollups(context.Background(), []domain.Event{}); err != nil {
		t.Fatalf("empty slice: %v", err)
	}
}

func TestInsertEventsAndApplyRollups_OutOfOrderEventCreatesSessionRow(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	// api_request arrives before any session_start
	events := []domain.Event{
		{TS: 5000, SessionID: "s9", PromptID: "p9", EventName: "claude_code.api_request",
			Attrs: map[string]any{"input_tokens": float64(7), "query_source": "main"}},
	}
	if err := repo.InsertEventsAndApplyRollups(context.Background(), events); err != nil {
		t.Fatalf("err: %v", err)
	}
	var n int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE session_id='s9'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected auto-created session row, got %d", n)
	}
}

// sql_NullString is a tiny wrapper for tests; see helper in a shared
// existing test file or define here.
type sql_NullString struct {
	String string
	Valid  bool
}

func (n *sql_NullString) Scan(v any) error {
	if v == nil {
		n.String, n.Valid = "", false
		return nil
	}
	switch s := v.(type) {
	case string:
		n.String, n.Valid = s, true
	case []byte:
		n.String, n.Valid = string(s), true
	}
	return nil
}
