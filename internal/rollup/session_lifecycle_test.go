package rollup

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplySessionStart_EmitsMetadataUpsert(t *testing.T) {
	ev := domain.Event{
		TS:        1000,
		SessionID: "s1",
		EventName: "claude_code.session_start",
		Attrs: map[string]any{
			"project.name": "demo",
			"project.cwd":  "/tmp/demo",
			"app.version":  "1.2.3",
			"os.type":      "darwin",
			"user.id":      "u1",
		},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if !strings.Contains(ops[0].Query, "INSERT INTO sessions") ||
		!strings.Contains(ops[0].Query, "ON CONFLICT(session_id)") ||
		!strings.Contains(ops[0].Query, "COALESCE(project_name") {
		t.Errorf("unexpected query: %s", ops[0].Query)
	}
	wantArgs := []any{"s1", int64(1000), int64(1000), "demo", "/tmp/demo", "1.2.3", "darwin", "u1"}
	if len(ops[0].Args) != len(wantArgs) {
		t.Fatalf("args len = %d want %d", len(ops[0].Args), len(wantArgs))
	}
	for i := range wantArgs {
		if ops[0].Args[i] != wantArgs[i] {
			t.Errorf("arg[%d] = %v want %v", i, ops[0].Args[i], wantArgs[i])
		}
	}
}

func TestApplySessionStart_MissingMetadataPassesEmptyStrings(t *testing.T) {
	ev := domain.Event{
		TS:        500,
		SessionID: "s2",
		EventName: "claude_code.session_start",
		Attrs:     map[string]any{},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("got %d ops", len(ops))
	}
	args := ops[0].Args
	for i := 3; i < len(args); i++ { // metadata strings
		if args[i] != "" {
			t.Errorf("arg[%d] expected empty string, got %v", i, args[i])
		}
	}
}

func TestApplySessionEnd_EmitsUpdateWithEndedAt(t *testing.T) {
	ev := domain.Event{
		TS:        2000,
		SessionID: "s1",
		EventName: "claude_code.session_end",
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if !strings.Contains(ops[0].Query, "INSERT INTO sessions") ||
		!strings.Contains(ops[0].Query, "ended_at") {
		t.Errorf("unexpected query: %s", ops[0].Query)
	}
	// args: session_id, started_at, last_seen_at, ended_at
	want := []any{"s1", int64(2000), int64(2000), int64(2000)}
	if len(ops[0].Args) != len(want) {
		t.Fatalf("args len %d want %d", len(ops[0].Args), len(want))
	}
	for i := range want {
		if ops[0].Args[i] != want[i] {
			t.Errorf("arg[%d] = %v want %v", i, ops[0].Args[i], want[i])
		}
	}
}
