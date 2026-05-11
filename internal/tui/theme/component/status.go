package component

import "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"

// Status identifies which pill to render.
type Status int

const (
	StatusLive Status = iota
	StatusStale
	StatusNoDaemon
)

// StatusPill renders a colored pill for the connection state.
func StatusPill(t *theme.Theme, s Status) string {
	switch s {
	case StatusLive:
		return t.PillLiveS.Render(t.Glyphs.StatusOK + " LIVE")
	case StatusStale:
		return t.PillStaleS.Render("STALE")
	case StatusNoDaemon:
		return t.PillNoDaemon.Render(t.Glyphs.StatusErr + " NO DAEMON")
	}
	return ""
}
