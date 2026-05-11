// Package component contains pure render helpers used by the TUI views.
// Each function takes a *theme.Theme + data + a target width and returns a
// styled string. Width discipline: outputs satisfy lipgloss.Width(out) ==
// width (for fixed-width components) so views can compose them with
// lipgloss.JoinHorizontal without misalignment.
package component
