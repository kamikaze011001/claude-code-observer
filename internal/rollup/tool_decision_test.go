package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyToolDecision_RejectBumpsToolDenied(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: domain.EventToolDecision,
		Attrs:     map[string]any{"decision": "reject"},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	// sessionCounterArgs index 13 = tool_denied
	if ops[0].Args[13] != int64(1) {
		t.Errorf("tool_denied = %v want 1", ops[0].Args[13])
	}
}

func TestApplyToolDecision_AcceptDoesNotBumpToolDenied(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: domain.EventToolDecision,
		Attrs:     map[string]any{"decision": "accept"},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Args[13] != int64(0) {
		t.Errorf("tool_denied = %v want 0", ops[0].Args[13])
	}
}

func TestApplyToolDecision_MissingDecisionDoesNotBump(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: domain.EventToolDecision,
	}
	ops := Apply(ev)
	if ops[0].Args[13] != int64(0) {
		t.Errorf("tool_denied = %v want 0", ops[0].Args[13])
	}
}
