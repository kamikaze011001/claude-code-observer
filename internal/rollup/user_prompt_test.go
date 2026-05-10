package rollup

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyUserPrompt_EmitsPromptMetaAndSessionCounter(t *testing.T) {
	ev := domain.Event{
		TS:        1000,
		SessionID: "s1",
		PromptID:  "p1",
		EventName: "user_prompt",
		Attrs: map[string]any{
			"prompt_length":  float64(42),
			"command_name":   "edit",
			"command_source": "builtin",
		},
	}
	ops := Apply(ev)
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	// Op 0: prompts metadata upsert
	if !strings.Contains(ops[0].Query, "INSERT INTO prompts") ||
		!strings.Contains(ops[0].Query, "command_source") {
		t.Errorf("op[0] unexpected query: %s", ops[0].Query)
	}
	wantArgs0 := []any{"p1", "s1", int64(1000), int64(42), "edit", "builtin"}
	if !argsEqual(ops[0].Args, wantArgs0) {
		t.Errorf("op[0] args = %v want %v", ops[0].Args, wantArgs0)
	}

	// Op 1: sessions counter — prompts += 1 (last positional arg is the prompts increment)
	if !strings.Contains(ops[1].Query, "INSERT INTO sessions") ||
		!strings.Contains(ops[1].Query, "ON CONFLICT(session_id)") {
		t.Errorf("op[1] unexpected query: %s", ops[1].Query)
	}
	// sessionCounterArgs places `prompts` at index 14 (last)
	if ops[1].Args[14] != int64(1) {
		t.Errorf("session prompts increment = %v want 1", ops[1].Args[14])
	}
}

func TestApplyUserPrompt_MissingPromptIDOmitsPromptOp(t *testing.T) {
	ev := domain.Event{
		TS:        1000,
		SessionID: "s1",
		EventName: "user_prompt",
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op (session only), got %d", len(ops))
	}
	if !strings.Contains(ops[0].Query, "INSERT INTO sessions") {
		t.Errorf("op[0] should be sessions: %s", ops[0].Query)
	}
}

// argsEqual is a shared test helper; lives in helpers_test.go siblings.
func argsEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
