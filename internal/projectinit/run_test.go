package projectinit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakePrompter struct {
	answers []bool
}

func (f *fakePrompter) Confirm(prompt string) (bool, error) {
	if len(f.answers) == 0 {
		return false, errors.New("fakePrompter: no answer queued")
	}
	a := f.answers[0]
	f.answers = f.answers[1:]
	return a, nil
}

type fakeProbe struct{ err error }

func (f fakeProbe) Probe(ctx context.Context, endpoint string, timeout time.Duration) error {
	return f.err
}

func newOpts(t *testing.T, dir string) Options {
	t.Helper()
	return Options{
		ProjectDir: dir,
		Endpoint:   "127.0.0.1:4317",
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Prompter:   &fakePrompter{},
		Prober:     fakeProbe{err: errors.New("not running")},
	}
}

func readSettings(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	return string(b)
}

func TestRun_FreshDir_CreatesDirAndFile(t *testing.T) {
	dir := t.TempDir()
	opts := newOpts(t, dir)
	if err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	fi, err := os.Stat(filepath.Join(dir, ".claude"))
	if err != nil || !fi.IsDir() {
		t.Fatalf(".claude/ not created: %v", err)
	}
	s := readSettings(t, dir)
	for _, k := range OwnedKeys() {
		if !strings.Contains(s, `"`+k+`"`) {
			t.Errorf("missing key %s", k)
		}
	}
}

func TestRun_BasenameDerivedFromProjectDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "myproj")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := newOpts(t, dir)
	if err := Run(opts); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if !strings.Contains(s, "project.name=myproj") {
		t.Errorf("basename not used: %s", s)
	}
}

func TestRun_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Run(newOpts(t, dir)); err != nil {
		t.Fatal(err)
	}
	first := readSettings(t, dir)
	if err := Run(newOpts(t, dir)); err != nil {
		t.Fatal(err)
	}
	second := readSettings(t, dir)
	if first != second {
		t.Errorf("not idempotent:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestRun_PreservesNonOwnedKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{"model":"opus","env":{"MY_VAR":"42"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(newOpts(t, dir)); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if !strings.Contains(s, `"model": "opus"`) {
		t.Errorf("model lost: %s", s)
	}
	if !strings.Contains(s, `"MY_VAR": "42"`) {
		t.Errorf("MY_VAR lost: %s", s)
	}
}

func TestRun_ConflictPromptYes_Overwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://other:1234"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := newOpts(t, dir)
	opts.Prompter = &fakePrompter{answers: []bool{true}}
	if err := Run(opts); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if !strings.Contains(s, "http://localhost:4317") {
		t.Errorf("not overwritten: %s", s)
	}
}

func TestRun_ConflictPromptNo_Preserves(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://other:1234"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := newOpts(t, dir)
	opts.Prompter = &fakePrompter{answers: []bool{false}}
	if err := Run(opts); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if !strings.Contains(s, "http://other:1234") {
		t.Errorf("user value lost: %s", s)
	}
}

func TestRun_Force_SkipsPrompts(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://other:1234"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := newOpts(t, dir)
	opts.Force = true
	opts.Prompter = &fakePrompter{}
	if err := Run(opts); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if !strings.Contains(s, "http://localhost:4317") {
		t.Errorf("force did not overwrite: %s", s)
	}
}

func TestRun_Print_DoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	opts := newOpts(t, dir)
	opts.Print = true
	stdout := &bytes.Buffer{}
	opts.Stdout = stdout
	if err := Run(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("--print created the file")
	}
	if !json.Valid(stdout.Bytes()) {
		t.Errorf("--print did not emit valid JSON: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "project.name=") {
		t.Errorf("--print missing project.name: %s", stdout.String())
	}
}

func TestRun_ProbeReachable_PrintsCheckmark(t *testing.T) {
	dir := t.TempDir()
	opts := newOpts(t, dir)
	opts.Prober = fakeProbe{err: nil}
	stdout := &bytes.Buffer{}
	opts.Stdout = stdout
	if err := Run(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "daemon reachable") {
		t.Errorf("expected reachable line: %s", stdout.String())
	}
}

func TestRun_ProbeUnreachable_PrintsHint(t *testing.T) {
	dir := t.TempDir()
	opts := newOpts(t, dir)
	opts.Prober = fakeProbe{err: errors.New("connection refused")}
	stdout := &bytes.Buffer{}
	opts.Stdout = stdout
	if err := Run(opts); err != nil {
		t.Errorf("unreachable probe should not error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "not reachable") || !strings.Contains(out, "cco serve") {
		t.Errorf("expected hint: %s", out)
	}
}

func TestRun_OutputIsTwoSpaceIndented(t *testing.T) {
	dir := t.TempDir()
	if err := Run(newOpts(t, dir)); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if !strings.Contains(s, "\n  \"env\"") {
		t.Errorf("expected 2-space indent on env block, got:\n%s", s)
	}
	if !strings.HasSuffix(s, "\n") {
		t.Error("expected trailing newline")
	}
}

func TestRun_EmptyProjectDir_ReturnsError(t *testing.T) {
	opts := Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	err := Run(opts)
	if err == nil || !strings.Contains(err.Error(), "ProjectDir is required") {
		t.Errorf("expected ProjectDir error, got: %v", err)
	}
}

func TestRun_NilStdoutStderr_UsesDefaults(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		ProjectDir: dir,
		Endpoint:   "127.0.0.1:4317",
		Prompter:   &fakePrompter{},
		Prober:     fakeProbe{err: errors.New("not running")},
		// Stdout and Stderr intentionally nil — should default to os.Stdout/os.Stderr
	}
	if err := Run(opts); err != nil {
		t.Fatalf("Run with nil Stdout/Stderr: %v", err)
	}
}

func TestRun_ConflictPromptError_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://other:1234"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := newOpts(t, dir)
	opts.Prompter = &fakePrompter{} // no answers queued → will error
	err := Run(opts)
	if err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Errorf("expected prompt error, got: %v", err)
	}
}

func TestLoadExisting_EmptyFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("  \n  "), 0o644); err != nil {
		t.Fatal(err)
	}
	obj, err := loadExisting(path)
	if err != nil {
		t.Fatalf("loadExisting: %v", err)
	}
	if len(obj.Keys()) != 0 {
		t.Errorf("expected empty object, got keys: %v", obj.Keys())
	}
}

func TestLoadExisting_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadExisting(path)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestLoadExisting_ReadError_ReturnsError(t *testing.T) {
	// Use a path inside a non-existent directory (not ErrNotExist on the file itself,
	// but permission error by making the directory unreadable).
	dir := t.TempDir()
	subdir := filepath.Join(dir, "noaccess")
	if err := os.Mkdir(subdir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })
	path := filepath.Join(subdir, "settings.json")
	_, err := loadExisting(path)
	// On a read-restricted dir the error is permission denied (not ErrNotExist)
	if err == nil {
		t.Skip("running as root or filesystem ignores permissions")
	}
}

func TestRun_MkdirAllError_ReturnsError(t *testing.T) {
	// Make the parent unwritable so MkdirAll can't create .claude/.
	dir := t.TempDir()
	// Create a .claude dir, then remove write permission from it so WriteFile fails,
	// but first we need to get past loadExisting (no settings.json yet) and MergeSettings.
	// Strategy: create settings.json manually, then make .claude unwritable.
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(claudeDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(claudeDir, 0o755) })
	opts := newOpts(t, dir)
	err := Run(opts)
	if err == nil {
		t.Skip("running as root or filesystem ignores permissions")
	}
	// The error should be about writing settings (either mkdir or write settings)
	if !strings.Contains(err.Error(), "write settings") && !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("expected write/mkdir error, got: %v", err)
	}
}

func TestRun_LoadExistingError_ReturnsError(t *testing.T) {
	// Create settings.json with invalid JSON so loadExisting fails.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte("{bad json}"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := newOpts(t, dir)
	err := Run(opts)
	if err == nil || !strings.Contains(err.Error(), "load existing") {
		t.Errorf("expected load existing error, got: %v", err)
	}
}
