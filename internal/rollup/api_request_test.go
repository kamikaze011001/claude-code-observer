package rollup

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyAPIRequest_MainQuerySource_BumpsTokensCostAndAPIRequests(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1", PromptID: "p1",
		EventName: "claude_code.api_request",
		Attrs: map[string]any{
			"input_tokens":          float64(100),
			"output_tokens":         float64(50),
			"cache_read_tokens":     float64(10),
			"cache_creation_tokens": float64(5),
			"cost_usd":              0.025,
			"query_source":          "main",
		},
	}
	ops := Apply(ev)
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	// Session args: subagent and auxiliary should be 0
	sessArgs := ops[1].Args
	// indices per sessionCounterArgs: 0=session, 1=ts, 2=ts,
	//   3=input, 4=output, 5=cache_read, 6=cache_creation,
	//   7=cost, 8=api_requests, 9=api_errors, 10=subagent, 11=auxiliary,
	//   12=tool_calls, 13=tool_denied, 14=prompts
	if sessArgs[3] != int64(100) || sessArgs[4] != int64(50) ||
		sessArgs[5] != int64(10) || sessArgs[6] != int64(5) ||
		sessArgs[7] != 0.025 || sessArgs[8] != int64(1) ||
		sessArgs[10] != int64(0) || sessArgs[11] != int64(0) {
		t.Errorf("sessions counters wrong: %+v", sessArgs)
	}
}

func TestApplyAPIRequest_SubagentBumpsSubagentRequests(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1", PromptID: "p1",
		EventName: "claude_code.api_request",
		Attrs:     map[string]any{"query_source": "subagent"},
	}
	ops := Apply(ev)
	sessArgs := ops[1].Args
	if sessArgs[10] != int64(1) {
		t.Errorf("subagent_requests = %v want 1", sessArgs[10])
	}
	if sessArgs[11] != int64(0) {
		t.Errorf("auxiliary_requests = %v want 0", sessArgs[11])
	}
	// Prompt op also bumps subagent_requests
	prArgs := ops[0].Args
	// promptCounterArgs indices: 0=prompt, 1=session, 2=ts,
	//   3=input,4=output,5=cache_read,6=cache_creation,
	//   7=cost,8=api_requests,9=subagent,10=tool_calls
	if prArgs[9] != int64(1) {
		t.Errorf("prompt subagent_requests = %v want 1", prArgs[9])
	}
}

func TestApplyAPIRequest_AuxiliaryBumpsAuxiliaryOnSessionOnly(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1", PromptID: "p1",
		EventName: "claude_code.api_request",
		Attrs:     map[string]any{"query_source": "auxiliary"},
	}
	ops := Apply(ev)
	sessArgs := ops[1].Args
	if sessArgs[11] != int64(1) {
		t.Errorf("auxiliary_requests = %v want 1", sessArgs[11])
	}
	if sessArgs[10] != int64(0) {
		t.Errorf("subagent_requests = %v want 0", sessArgs[10])
	}
}

func TestApplyAPIRequest_NoPromptIDOmitsPromptOp(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: "claude_code.api_request",
		Attrs:     map[string]any{"query_source": "main"},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if !strings.Contains(ops[0].Query, "INSERT INTO sessions") {
		t.Errorf("expected sessions op")
	}
}
