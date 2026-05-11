package rollup

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApply_UnknownEventNameReturnsNil(t *testing.T) {
	ops := Apply(domain.Event{EventName: "something_we_dont_handle"})
	if ops != nil {
		t.Fatalf("expected nil ops for unknown event, got %d ops", len(ops))
	}
}

func TestApply_EmptyEventNameReturnsNil(t *testing.T) {
	if ops := Apply(domain.Event{}); ops != nil {
		t.Fatalf("expected nil for empty EventName")
	}
}

func TestApply_PrependsMetadataUpsertWhenResourceAttrsPresent(t *testing.T) {
	ev := domain.Event{
		TS:        1000,
		SessionID: "s1",
		PromptID:  "p1",
		EventName: domain.EventUserPrompt,
		Attrs: map[string]any{
			"project.name": "demo",
			"project.cwd":  "/tmp/demo",
			"app.version":  "1.2.3",
			"os.type":      "darwin",
			"user.id":      "u1",
		},
	}
	ops := Apply(ev)
	if len(ops) < 2 {
		t.Fatalf("expected metadata op + updater ops, got %d", len(ops))
	}
	if !strings.Contains(ops[0].Query, "COALESCE(project_name") {
		t.Fatalf("first op should be metadata upsert, got: %s", ops[0].Query)
	}
	want := []any{"s1", int64(1000), int64(1000), "demo", "/tmp/demo", "1.2.3", "darwin", "u1"}
	if len(ops[0].Args) != len(want) {
		t.Fatalf("metadata args len = %d want %d", len(ops[0].Args), len(want))
	}
	for i := range want {
		if ops[0].Args[i] != want[i] {
			t.Errorf("metadata arg[%d] = %v want %v", i, ops[0].Args[i], want[i])
		}
	}
}

func TestApply_NoMetadataWhenProjectNameMissing(t *testing.T) {
	ev := domain.Event{
		TS:        1000,
		SessionID: "s1",
		PromptID:  "p1",
		EventName: domain.EventUserPrompt,
		Attrs:     map[string]any{},
	}
	ops := Apply(ev)
	for _, op := range ops {
		if strings.Contains(op.Query, "COALESCE(project_name") {
			t.Fatalf("did not expect metadata upsert when project.name absent, got: %s", op.Query)
		}
	}
}

func TestApply_SessionStartSkipsExtraMetadataPrepend(t *testing.T) {
	ev := domain.Event{
		TS:        1000,
		SessionID: "s1",
		EventName: domain.EventSessionStart,
		Attrs: map[string]any{
			"project.name": "demo",
		},
	}
	ops := Apply(ev)
	metaCount := 0
	for _, op := range ops {
		if strings.Contains(op.Query, "COALESCE(project_name") {
			metaCount++
		}
	}
	if metaCount != 1 {
		t.Fatalf("expected exactly 1 metadata upsert for session_start, got %d", metaCount)
	}
}

func TestApply_AllDomainEventsHaveAHandler(t *testing.T) {
	for _, name := range domain.AllEventNames {
		if _, ok := updaters[name]; !ok {
			t.Errorf("rollup.updaters has no entry for %q (declared in domain.AllEventNames)", name)
		}
	}
}
