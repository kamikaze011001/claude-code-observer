package productivity

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestView_RendersDayRows(t *testing.T) {
	m := New(nil, nil)
	m.days = []readstore.ProductivityDay{
		{Day: "2026-06-14", LinesAdded: 1200, LinesRemoved: 340, Commits: 3, ActiveSec: 1500, EditsAccepted: 9, EditsRejected: 1},
	}
	out := m.View(100, 40)
	if !strings.Contains(out, "2026-06-14") {
		t.Errorf("expected day in output, got:\n%s", out)
	}
	if !strings.Contains(out, "90%") {
		t.Errorf("expected accept-rate 90%%, got:\n%s", out)
	}
}

func TestView_EmptyState(t *testing.T) {
	m := New(nil, nil)
	if out := m.View(100, 40); !strings.Contains(out, "no data") {
		t.Errorf("expected empty-state text, got:\n%s", out)
	}
}
