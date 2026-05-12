package theme

import "github.com/mattn/go-runewidth"

// VisualWidth returns the number of terminal columns required to display s.
// It handles CJK wide characters and emoji correctly, unlike len(s) or
// utf8.RuneCountInString(s).
func VisualWidth(s string) int {
	return runewidth.StringWidth(s)
}
