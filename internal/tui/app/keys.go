package app

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys is the set of keys the shell intercepts before forwarding
// any message to the active view.
type GlobalKeys struct {
	Quit    key.Binding
	Back    key.Binding
	Refresh key.Binding
	Help    key.Binding
}

// DefaultKeys returns the locked v1 keymap.
func DefaultKeys() GlobalKeys {
	return GlobalKeys{
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Back:    key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b", "back")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "about")),
	}
}
