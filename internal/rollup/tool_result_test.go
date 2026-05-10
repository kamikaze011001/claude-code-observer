package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyToolResult_BumpsToolCallsOnSessionAndPrompt(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1", PromptID: "p1",
		EventName: "tool_result",
	}
	ops := Apply(ev)
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	// Op 0 prompt: index 10 = tool_calls
	if ops[0].Args[10] != int64(1) {
		t.Errorf("prompt tool_calls = %v want 1", ops[0].Args[10])
	}
	// Op 1 session: index 12 = tool_calls
	if ops[1].Args[12] != int64(1) {
		t.Errorf("session tool_calls = %v want 1", ops[1].Args[12])
	}
}

func TestApplyToolResult_NoPromptIDOnlySession(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: "tool_result",
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
}
