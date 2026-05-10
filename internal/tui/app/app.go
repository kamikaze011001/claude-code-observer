package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// App is the Bubble Tea root model. It owns the view stack, the global
// keymap, the theme, and per-tick error accounting.
type App struct {
	stack      []View
	theme      theme.Theme
	keys       GlobalKeys
	width      int
	height     int
	lastErr    error
	consecErrs int
}

// New constructs an App with no views. Push at least one before running.
func New(th theme.Theme) *App {
	return &App{theme: th, keys: DefaultKeys()}
}

// Push adds a view to the top of the stack and calls its Init.
func (a *App) Push(v View) tea.Cmd {
	a.stack = append(a.stack, v)
	return v.Init()
}

// StackDepth is exposed for tests.
func (a *App) StackDepth() int { return len(a.stack) }

// ConsecutiveErrors is exposed for tests and the chrome renderer.
func (a *App) ConsecutiveErrors() int { return a.consecErrs }

// Theme exposes the active palette.
func (a *App) Theme() theme.Theme { return a.theme }

// Init returns the initial tea.Cmd: start the 1 s ticker.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{tickEvery()}
	for _, v := range a.stack {
		if c := v.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// Update routes a tea.Msg.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(m, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(m, a.keys.Back):
			if len(a.stack) > 1 {
				a.stack = a.stack[:len(a.stack)-1]
			}
			return a, nil
		case key.Matches(m, a.keys.Refresh):
			return a.forwardTop(TickMsg(time.Now()))
		}
	case TickMsg:
		model, cmd := a.forwardTop(m)
		return model, tea.Batch(cmd, tickEvery())
	case PushViewMsg:
		initCmd := a.Push(m.V)
		return a, initCmd
	case PopViewMsg:
		if len(a.stack) > 1 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	case ErrMsg:
		a.lastErr = m.Err
		a.consecErrs++
		return a, nil
	}
	return a.forwardTop(msg)
}

// View renders the full chrome (header + body + footer pill).
func (a *App) View() string {
	if len(a.stack) == 0 {
		return ""
	}
	top := a.stack[len(a.stack)-1]
	inner := a.safeRender(top)
	return a.renderChrome(top, inner)
}

func (a *App) safeRender(v View) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = a.theme.ErrorText.Render("⚠ VIEW ERROR — b TO RETURN")
		}
	}()
	return v.View(a.width, a.height)
}

func (a *App) renderChrome(v View, body string) string {
	title := a.theme.Heading.Render("CCO  │  " + v.Title())
	pill := a.theme.Pill(v.Status())
	helps := []string{}
	for _, k := range v.ShortHelp() {
		h := k.Help()
		helps = append(helps, "["+h.Key+"] "+h.Desc)
	}
	footer := strings.Join(helps, "  ") + "    " + pill
	return strings.Join([]string{title, body, footer}, "\n")
}

func (a *App) forwardTop(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(a.stack) == 0 {
		return a, nil
	}
	panicked := false
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			a.lastErr = fmt.Errorf("view panic: %v", r)
			a.consecErrs++
		}
	}()
	top := a.stack[len(a.stack)-1]
	updated, cmd := top.Update(msg)
	a.stack[len(a.stack)-1] = updated
	if !panicked {
		a.consecErrs = 0
	}
	return a, cmd
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
