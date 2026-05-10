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
