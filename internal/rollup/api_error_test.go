package rollup

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyAPIError_BumpsAPIErrorsAndMarksPromptHadError(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1", PromptID: "p1",
		EventName: "api_error",
	}
	ops := Apply(ev)
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	// Op 0 = prompt had_error UPSERT
	if !strings.Contains(ops[0].Query, "INSERT INTO prompts") ||
		!strings.Contains(ops[0].Query, "had_error") {
		t.Errorf("op[0] unexpected: %s", ops[0].Query)
	}
	// args: prompt_id, session_id, started_at, had_error
	want0 := []any{"p1", "s1", int64(1000), int64(1)}
	if !argsEqual(ops[0].Args, want0) {
		t.Errorf("op[0] args = %v want %v", ops[0].Args, want0)
	}
	// Op 1 = sessions counter, api_errors at index 9
	sessArgs := ops[1].Args
	if sessArgs[9] != int64(1) {
		t.Errorf("api_errors = %v want 1", sessArgs[9])
	}
}

func TestApplyAPIError_NoPromptIDStillBumpsSession(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: "api_error",
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if !strings.Contains(ops[0].Query, "INSERT INTO sessions") {
		t.Errorf("expected sessions op")
	}
}
