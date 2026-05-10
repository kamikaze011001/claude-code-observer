package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApply_UnknownEventNameReturnsNil(t *testing.T) {
	ops := Apply(domain.Event{EventName: "claude_code.something_we_dont_handle"})
	if ops != nil {
		t.Fatalf("expected nil ops for unknown event, got %d ops", len(ops))
	}
}

func TestApply_EmptyEventNameReturnsNil(t *testing.T) {
	if ops := Apply(domain.Event{}); ops != nil {
		t.Fatalf("expected nil for empty EventName")
	}
}

func TestApply_AllDomainEventsHaveAHandler(t *testing.T) {
	for _, name := range domain.AllEventNames {
		if _, ok := updaters[name]; !ok {
			t.Errorf("rollup.updaters has no entry for %q (declared in domain.AllEventNames)", name)
		}
	}
}
