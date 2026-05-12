# CCO Logo & About View — Design Spec

**Date:** 2026-05-12
**Status:** Approved, ready for implementation plan
**Scope:** Brand mark for the `cco` TUI — a full block "CCO" logo in a new about view, plus a refined compact wordmark in the chrome header.

---

## 1. Goal

Replace the plain `"CCO  │  <title>"` chrome header with a styled wordmark, and add a discoverable about view that displays a figlet-block "CCO" logo with version metadata. No startup splash — power users open the tool many times per day, so the big mark lives behind `?`.

## 2. Non-Goals

- No animated logo, no startup splash, no ASCII intro sequence.
- No multi-variant logo (single color treatment, single font).
- No raster/image asset — terminal-rendered Unicode only.

## 3. Components

### 3.1 Splash mark (about view body)

Figlet block "CCO" — 28 cells wide × 6 lines tall, fixed:

```
 ██████╗ ██████╗  ██████╗
██╔════╝██╔════╝ ██╔═══██╗
██║     ██║      ██║   ██║
██║     ██║      ██║   ██║
╚██████╗╚██████╗ ╚██████╔╝
 ╚═════╝ ╚═════╝  ╚═════╝
```

**Styling:**
- Block letters: bold, `t.Palette.Accent` (Catppuccin Mocha mauve).
- Tagline "claude code observer": `t.Subtitle` (muted).
- Version line `vX.Y.Z · local OTLP receiver · github.com/kamikaze011001/cco`: `t.Muted`.
- Footer help `[b] back   [q] quit`: `t.Help`.

**Layout:** centered horizontally via `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)`. Lines stacked tightly with one blank line between the block and the tagline, one between tagline and version.

**Narrow-width fallback (`width < 32`):** render the compact wordmark `✦ CCO` on one line, followed by tagline and version. No figlet block.

### 3.2 Compact header wordmark

Replaces `App.renderChrome`'s current title construction.

```
✦ CCO  │  dashboard                                          [LIVE]
```

**Composition:**
- `✦` — accent color (mauve), from `t.Glyphs.Brand`.
- `CCO` — bold + accent (uppercase).
- `│` — muted separator, surrounded by two spaces on each side.
- View title — subtitle/muted, lowercase (matches existing convention).
- Status pill — unchanged, right-aligned via existing pill renderer.

**Width budget:** brand + separator = ~14 cells. Fits any terminal ≥ 40 cells. No fallback needed — the wordmark is already minimal.

### 3.3 About view

New view implementing the `app.View` interface.

**Files:**
- `internal/tui/about/view.go` — `Model`, `New(th *theme.Theme, version string) *Model`, and the interface methods.
- `internal/tui/about/logo.go` — figlet `const Logo string` literal plus `Render(t *theme.Theme) string` that applies the accent+bold style.

**Interface behavior:**
- `Title() string` → `"about"` (keeps the breadcrumb consistent).
- `Status() component.Status` → propagates the global daemon status (passed in via constructor or queried from a shared readstore — match whatever pattern the existing views use).
- `ShortHelp() []key.Binding` → `[b] back`, `[q] quit`.
- `Update(msg)` → `b` returns `PopViewMsg`; other keys ignored.
- `View(w, h)` → centered logo + metadata via `lipgloss.Place`; narrow fallback as in §3.1.

## 4. Integration

### 4.1 `internal/tui/app/app.go`

- **Keymap:** add `Help key.Binding` to `GlobalKeys`, bound to `?`.
- **Update:** on `key.Matches(m, a.keys.Help)`, push an `about.Model` onto the stack via `PushViewMsg`.
- **renderChrome:** replace
  ```go
  title := a.theme.Title.Render("CCO  │  " + v.Title())
  ```
  with the composed wordmark from §3.2.

### 4.2 `internal/tui/dashboard/view.go`

- Delete `renderHeader` and remove it from the `sections` slice in `View()`. The chrome header is now the only place the brand appears.
- Any tests that diffed against the dashboard's own header line need their goldens regenerated.

### 4.3 Per-view `ShortHelp()`

Each existing view (dashboard, sessions list, session detail, prompt detail) adds a `[?] about` hint to its `ShortHelp()` return value. Position: after the view-specific keys, before `[q] quit`.

### 4.4 Version source

About view receives the version string at construction time. Source: the existing ldflag-injected vars in `cmd/app` (currently used by `cco version`). Either:
- Pass through from `cmd/app` into `app.New(...)` and on to `about.New(...)`, or
- Move the vars into a tiny `internal/version` package importable by both.

Pick whichever matches existing patterns; this is a wiring detail, not a design decision.

## 5. Theme dependencies

No new theme styles. Uses existing:
- `t.Title` (bold accent) — applied to block logo + compact "CCO".
- `t.Subtitle` (muted) — tagline.
- `t.Muted` (muted) — version line + separator `│`.
- `t.Accent` (accent foreground) — brand glyph.
- `t.Help` (muted) — about footer.
- `t.Glyphs.Brand` (`✦`) — already present.

## 6. Testing

- `internal/tui/about/about_test.go`:
  - Golden test for the rendered logo at width 80, height 24.
  - Golden test for the narrow fallback at width 30, height 10.
  - Update test for the `b` key returning `PopViewMsg`.
- `internal/tui/app/app_test.go`:
  - Test that pressing `?` from any pushed view results in the about view being on top of the stack.
- `internal/tui/dashboard/view_test.go`:
  - Regenerate `populated.golden` and `empty.golden` after removing `renderHeader`.

Golden files: use the existing ANSI-strip + TrueColor TestMain harness already in place for dashboard goldens.

## 7. Width discipline

The figlet block is a hard 28 cells wide. `lipgloss.Place` centers it within the available width without wrapping. At widths between 28 and 32, the block fits but with minimal margin — acceptable. Below 28, fall back to the compact wordmark variant.

The chrome wordmark inherits the existing chrome's width discipline; no new math.

## 8. Out of scope (deferred)

- Logo variants per theme (e.g., a different palette mapping for a future light theme).
- Animated transitions when pushing the about view.
- ASCII art for individual views (only "CCO" gets a mark).
- Configurable logo (no flag to disable; the about view is opt-in by `?`).

## 9. Migration

Single PR. No data migration, no config changes. User-visible changes:
1. Chrome header looks slightly different (styled wordmark + glyph).
2. Pressing `?` now opens the about view from any screen.
3. Dashboard body loses its redundant inline `✦ cco · dashboard` line.

No breaking changes for downstream consumers; the TUI is a single binary, no API.
