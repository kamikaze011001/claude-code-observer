# Phase 4 — Install Ergonomics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `cco init` (project setup wizard), `scripts/com.claude-code-observer.plist` (macOS launchd) + `scripts/claude-code-observer.service` (Linux systemd user unit), and a README install section that takes a fresh user from `git clone` to a populated dashboard in under 5 minutes.

**Architecture:** New pure package `internal/projectinit/` owns all `cco init` logic — parse existing `.claude/settings.json`, merge our seven owned `env` keys (preserving order and unrelated keys), special-case `OTEL_RESOURCE_ATTRIBUTES` as a sub-key map, write back, then probe `localhost:4317` via gRPC. The cobra subcommand in `cmd/app/init.go` is a thin wiring layer that supplies stdio, flags, and a real probe. Service files are static text shipped under `scripts/`; README gains an Install section walking through build → service install → `cco init` → use → open dashboard.

**Tech Stack:** Go 1.25, `encoding/json` (order-preserving custom struct, no new deps), `google.golang.org/grpc` + `go.opentelemetry.io/proto/otlp` (already in `go.mod`) for the daemon probe, `cobra` (existing), table-driven tests.

**Spec:** `docs/superpowers/specs/2026-05-10-phase-4-install-ergonomics-design.md`

---

## File Structure

**Naming deviation from spec:** the spec calls the new module `internal/init/`. Go reserves `init` as a package name (`package init` does not compile). This plan uses `internal/projectinit/` instead and renames the roadmap test-gate line accordingly in Task 11.

**New files:**

- `internal/projectinit/doc.go` — package marker
- `internal/projectinit/owned.go` — owned-key set + canonical values
- `internal/projectinit/settings.go` — `OrderedObject` type + JSON marshal/unmarshal preserving key order
- `internal/projectinit/settings_test.go`
- `internal/projectinit/merge.go` — `MergeSettings(existing, basename) (merged, conflicts)` + `--force` handling
- `internal/projectinit/merge_test.go`
- `internal/projectinit/resource_attrs.go` — `parseResourceAttrs` / `serializeResourceAttrs` for `OTEL_RESOURCE_ATTRIBUTES` sub-key merge
- `internal/projectinit/resource_attrs_test.go`
- `internal/projectinit/probe.go` — `Prober` interface + `GRPCProbe` implementation
- `internal/projectinit/probe_test.go`
- `internal/projectinit/run.go` — `Run(opts Options) error` top-level orchestrator
- `internal/projectinit/run_test.go`
- `cmd/app/init_test.go` — cobra wiring + flag tests
- `scripts/com.claude-code-observer.plist`
- `scripts/claude-code-observer.service`

**Modified files:**

- `cmd/app/init.go` — replace stub body with real wiring (currently 16 lines, lines 1–16)
- `README.md` — replace template content (currently 78 lines) with project-specific README + Install section
- `docs/MANUAL-VERIFICATION.md` — add Phase 4 entries
- `docs/ROADMAP.md:269` — replace `internal/init/` with `internal/projectinit/`

---

## Cross-cutting reference: owned key set

These seven `env` keys are the entire surface `cco init` writes to and prompts for. All other keys (under `env` and at top level) pass through untouched.

```go
// internal/projectinit/owned.go (Task 2 will create this)
package projectinit

const (
    KeyEnableTelemetry      = "CLAUDE_CODE_ENABLE_TELEMETRY"
    KeyMetricsExporter      = "OTEL_METRICS_EXPORTER"
    KeyLogsExporter         = "OTEL_LOGS_EXPORTER"
    KeyOTLPProtocol         = "OTEL_EXPORTER_OTLP_PROTOCOL"
    KeyOTLPEndpoint         = "OTEL_EXPORTER_OTLP_ENDPOINT"
    KeyResourceAttrs        = "OTEL_RESOURCE_ATTRIBUTES"
    KeyMetricExportInterval = "OTEL_METRIC_EXPORT_INTERVAL"

    SubKeyProjectName = "project.name"
)

// CanonicalValues returns the value cco init writes for each owned key
// EXCEPT KeyResourceAttrs, which is merged sub-key-wise (see resource_attrs.go).
func CanonicalValues() map[string]string {
    return map[string]string{
        KeyEnableTelemetry:      "1",
        KeyMetricsExporter:      "otlp",
        KeyLogsExporter:         "otlp",
        KeyOTLPProtocol:         "grpc",
        KeyOTLPEndpoint:         "http://localhost:4317",
        KeyMetricExportInterval: "20000",
    }
}

// OwnedKeys returns the full owned set in stable insertion order. Useful for
// emitting a fresh env block.
func OwnedKeys() []string {
    return []string{
        KeyEnableTelemetry,
        KeyMetricsExporter,
        KeyLogsExporter,
        KeyOTLPProtocol,
        KeyOTLPEndpoint,
        KeyResourceAttrs,
        KeyMetricExportInterval,
    }
}
```

---

## Cross-cutting reference: order-preserving JSON

Go's `map[string]any` does not preserve key order on round-trip. To meet the "re-running `cco init` is a no-op" criterion, Task 2 introduces `OrderedObject`:

```go
// internal/projectinit/settings.go (Task 2 will create this)
package projectinit

import (
    "bytes"
    "encoding/json"
    "fmt"
)

// OrderedObject is a JSON object that preserves insertion order on marshal.
// Values are stored as json.RawMessage so unrelated subtrees pass through
// byte-identical.
type OrderedObject struct {
    keys   []string
    values map[string]json.RawMessage
}

func NewOrderedObject() *OrderedObject {
    return &OrderedObject{values: map[string]json.RawMessage{}}
}

// Get returns the raw JSON value for key and whether it was present.
func (o *OrderedObject) Get(key string) (json.RawMessage, bool) {
    v, ok := o.values[key]
    return v, ok
}

// Set inserts or updates key with value. New keys are appended to the order.
func (o *OrderedObject) Set(key string, value json.RawMessage) {
    if _, ok := o.values[key]; !ok {
        o.keys = append(o.keys, key)
    }
    o.values[key] = value
}

// Keys returns the keys in insertion order. The returned slice is a copy.
func (o *OrderedObject) Keys() []string {
    out := make([]string, len(o.keys))
    copy(out, o.keys)
    return out
}

func (o *OrderedObject) UnmarshalJSON(data []byte) error {
    o.keys = nil
    o.values = map[string]json.RawMessage{}
    dec := json.NewDecoder(bytes.NewReader(data))
    dec.UseNumber()
    tok, err := dec.Token()
    if err != nil {
        return err
    }
    if d, ok := tok.(json.Delim); !ok || d != '{' {
        return fmt.Errorf("expected JSON object, got %v", tok)
    }
    for dec.More() {
        keyTok, err := dec.Token()
        if err != nil {
            return err
        }
        key, ok := keyTok.(string)
        if !ok {
            return fmt.Errorf("expected string key, got %v", keyTok)
        }
        var raw json.RawMessage
        if err := dec.Decode(&raw); err != nil {
            return fmt.Errorf("decode %q: %w", key, err)
        }
        o.keys = append(o.keys, key)
        o.values[key] = raw
    }
    if _, err := dec.Token(); err != nil { // consume '}'
        return err
    }
    return nil
}

func (o *OrderedObject) MarshalJSON() ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('{')
    for i, k := range o.keys {
        if i > 0 {
            buf.WriteByte(',')
        }
        kb, err := json.Marshal(k)
        if err != nil {
            return nil, err
        }
        buf.Write(kb)
        buf.WriteByte(':')
        buf.Write(o.values[k])
    }
    buf.WriteByte('}')
    return buf.Bytes(), nil
}
```

This is the entire JSON layer. Top-level and `env` are both `*OrderedObject`. Anything below `env` (we never look there) stays as `json.RawMessage` and round-trips byte-identical.

For pretty-printing on write, Task 7 uses `json.Indent` with `"  "` (2-space) on the marshaled bytes.

---

## Cross-cutting reference: probe interface

Defined in `internal/projectinit/probe.go` (Task 5):

```go
package projectinit

import (
    "context"
    "time"
)

// Prober reports whether a daemon is reachable at the given endpoint.
// Implementations must respect ctx cancellation. Returning a non-nil error
// means "unreachable" with the error containing the reason; a nil error means
// "reachable".
type Prober interface {
    Probe(ctx context.Context, endpoint string, timeout time.Duration) error
}
```

`run.go` (Task 6) takes a `Prober` so tests can pass a stub. `cmd/app/init.go` (Task 7) wires `GRPCProbe{}`.

---

## Task 1: Scaffold `internal/projectinit/` package

**Files:**
- Create: `internal/projectinit/doc.go`
- Create: `internal/projectinit/owned.go`
- Test: (none yet — pure constants)

- [ ] **Step 1: Create `doc.go`**

```go
// Package projectinit implements `cco init`: writes/updates
// .claude/settings.json in a project directory and probes the daemon.
//
// Boundaries:
//   - Pure file-and-network operations.
//   - No dependency on internal/{receiver,service,repository,domain}.
//   - All I/O behind interfaces so tests stay hermetic.
package projectinit
```

- [ ] **Step 2: Create `owned.go`**

Use the exact code under "Cross-cutting reference: owned key set" above.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/projectinit/`
Expected: exit 0, no output.

- [ ] **Step 4: Commit**

```bash
git add internal/projectinit/doc.go internal/projectinit/owned.go
git commit -m "projectinit: scaffold package with owned key set"
```

---

## Task 2: Order-preserving JSON object

**Files:**
- Create: `internal/projectinit/settings.go`
- Test: `internal/projectinit/settings_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/projectinit/settings_test.go`:

```go
package projectinit

import (
    "encoding/json"
    "testing"
)

func TestOrderedObject_UnmarshalPreservesOrder(t *testing.T) {
    in := []byte(`{"b":1,"a":"x","c":{"nested":true}}`)
    o := NewOrderedObject()
    if err := json.Unmarshal(in, o); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    keys := o.Keys()
    want := []string{"b", "a", "c"}
    if len(keys) != len(want) {
        t.Fatalf("len(keys)=%d, want %d", len(keys), len(want))
    }
    for i, k := range want {
        if keys[i] != k {
            t.Errorf("keys[%d]=%q, want %q", i, keys[i], k)
        }
    }
}

func TestOrderedObject_RoundTripBytes(t *testing.T) {
    in := []byte(`{"b":1,"a":"x","c":{"nested":true}}`)
    o := NewOrderedObject()
    if err := json.Unmarshal(in, o); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    out, err := json.Marshal(o)
    if err != nil {
        t.Fatalf("marshal: %v", err)
    }
    if string(out) != string(in) {
        t.Errorf("round-trip diff:\n got: %s\nwant: %s", out, in)
    }
}

func TestOrderedObject_SetAppendsNewKeysPreservesExisting(t *testing.T) {
    o := NewOrderedObject()
    if err := json.Unmarshal([]byte(`{"a":1,"b":2}`), o); err != nil {
        t.Fatal(err)
    }
    o.Set("c", json.RawMessage(`3`))
    o.Set("a", json.RawMessage(`99`))
    out, err := json.Marshal(o)
    if err != nil {
        t.Fatal(err)
    }
    want := `{"a":99,"b":2,"c":3}`
    if string(out) != want {
        t.Errorf("got %s, want %s", out, want)
    }
}

func TestOrderedObject_GetAbsentReturnsFalse(t *testing.T) {
    o := NewOrderedObject()
    if _, ok := o.Get("missing"); ok {
        t.Error("expected ok=false for absent key")
    }
}

func TestOrderedObject_RejectsNonObject(t *testing.T) {
    o := NewOrderedObject()
    if err := json.Unmarshal([]byte(`[1,2,3]`), o); err == nil {
        t.Error("expected error for non-object")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/projectinit/ -run OrderedObject -v`
Expected: FAIL with "undefined: NewOrderedObject" / "undefined: OrderedObject".

- [ ] **Step 3: Create `settings.go`**

Use the exact code under "Cross-cutting reference: order-preserving JSON" above.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/projectinit/ -run OrderedObject -v`
Expected: all five subtests PASS.

- [ ] **Step 5: Run vet and build**

Run: `go vet ./internal/projectinit/ && go build ./internal/projectinit/`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add internal/projectinit/settings.go internal/projectinit/settings_test.go
git commit -m "projectinit: order-preserving OrderedObject for settings.json"
```

---

## Task 3: `OTEL_RESOURCE_ATTRIBUTES` sub-key merge

**Files:**
- Create: `internal/projectinit/resource_attrs.go`
- Test: `internal/projectinit/resource_attrs_test.go`

The function parses a comma-separated `k=v` list, lets the caller mutate the `project.name` entry, and re-serializes. Order rule: if `project.name` is being inserted (didn't exist before), it goes first; if it already existed, it stays in place.

- [ ] **Step 1: Write the failing tests**

Create `internal/projectinit/resource_attrs_test.go`:

```go
package projectinit

import "testing"

func TestParseResourceAttrs_Empty(t *testing.T) {
    pairs := parseResourceAttrs("")
    if len(pairs) != 0 {
        t.Errorf("got %d pairs, want 0", len(pairs))
    }
}

func TestParseResourceAttrs_Multiple(t *testing.T) {
    pairs := parseResourceAttrs("enduser.id=jdoe,deployment.environment=prod")
    if len(pairs) != 2 {
        t.Fatalf("got %d pairs, want 2", len(pairs))
    }
    if pairs[0].Key != "enduser.id" || pairs[0].Value != "jdoe" {
        t.Errorf("pairs[0]=%+v", pairs[0])
    }
    if pairs[1].Key != "deployment.environment" || pairs[1].Value != "prod" {
        t.Errorf("pairs[1]=%+v", pairs[1])
    }
}

func TestParseResourceAttrs_TrimsSpaces(t *testing.T) {
    pairs := parseResourceAttrs("a=1, b=2 ,c =3")
    want := []struct{ k, v string }{{"a", "1"}, {"b", "2"}, {"c", "3"}}
    for i, w := range want {
        if pairs[i].Key != w.k || pairs[i].Value != w.v {
            t.Errorf("pairs[%d]=%+v, want {%q,%q}", i, pairs[i], w.k, w.v)
        }
    }
}

func TestParseResourceAttrs_SkipsMalformed(t *testing.T) {
    pairs := parseResourceAttrs("a=1,bogus,b=2")
    if len(pairs) != 2 || pairs[0].Key != "a" || pairs[1].Key != "b" {
        t.Errorf("got %+v, want a=1,b=2", pairs)
    }
}

func TestSerializeResourceAttrs_RoundTrip(t *testing.T) {
    in := "enduser.id=jdoe,deployment.environment=prod"
    out := serializeResourceAttrs(parseResourceAttrs(in))
    if out != in {
        t.Errorf("round trip: got %q, want %q", out, in)
    }
}

func TestSetProjectName_InsertsFirstWhenAbsent(t *testing.T) {
    pairs := parseResourceAttrs("enduser.id=jdoe")
    pairs = setSubKey(pairs, SubKeyProjectName, "myproj")
    out := serializeResourceAttrs(pairs)
    want := "project.name=myproj,enduser.id=jdoe"
    if out != want {
        t.Errorf("got %q, want %q", out, want)
    }
}

func TestSetProjectName_UpdatesInPlaceWhenPresent(t *testing.T) {
    pairs := parseResourceAttrs("enduser.id=jdoe,project.name=old,deployment.environment=prod")
    pairs = setSubKey(pairs, SubKeyProjectName, "new")
    out := serializeResourceAttrs(pairs)
    want := "enduser.id=jdoe,project.name=new,deployment.environment=prod"
    if out != want {
        t.Errorf("got %q, want %q", out, want)
    }
}

func TestSetProjectName_OnEmptyList(t *testing.T) {
    pairs := parseResourceAttrs("")
    pairs = setSubKey(pairs, SubKeyProjectName, "myproj")
    out := serializeResourceAttrs(pairs)
    if out != "project.name=myproj" {
        t.Errorf("got %q", out)
    }
}

func TestGetSubKey(t *testing.T) {
    pairs := parseResourceAttrs("a=1,b=2")
    if v, ok := getSubKey(pairs, "b"); !ok || v != "2" {
        t.Errorf("b: got (%q,%v)", v, ok)
    }
    if _, ok := getSubKey(pairs, "missing"); ok {
        t.Error("missing key reported as present")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/projectinit/ -run ResourceAttrs -v`
Expected: FAIL with "undefined: parseResourceAttrs" etc.

- [ ] **Step 3: Create `resource_attrs.go`**

```go
package projectinit

import "strings"

// kvPair is one entry in the OTEL_RESOURCE_ATTRIBUTES comma-separated list.
type kvPair struct {
    Key   string
    Value string
}

// parseResourceAttrs parses an OTEL_RESOURCE_ATTRIBUTES string. Whitespace
// around keys and values is trimmed. Entries without an '=' are silently
// dropped.
func parseResourceAttrs(s string) []kvPair {
    if s == "" {
        return nil
    }
    parts := strings.Split(s, ",")
    out := make([]kvPair, 0, len(parts))
    for _, p := range parts {
        eq := strings.IndexByte(p, '=')
        if eq < 0 {
            continue
        }
        k := strings.TrimSpace(p[:eq])
        v := strings.TrimSpace(p[eq+1:])
        if k == "" {
            continue
        }
        out = append(out, kvPair{Key: k, Value: v})
    }
    return out
}

func serializeResourceAttrs(pairs []kvPair) string {
    parts := make([]string, len(pairs))
    for i, p := range pairs {
        parts[i] = p.Key + "=" + p.Value
    }
    return strings.Join(parts, ",")
}

func getSubKey(pairs []kvPair, key string) (string, bool) {
    for _, p := range pairs {
        if p.Key == key {
            return p.Value, true
        }
    }
    return "", false
}

// setSubKey returns pairs with key=value set. If key is absent it is inserted
// at the front (so cco init's project.name appears first on fresh writes).
// If present it is updated in place.
func setSubKey(pairs []kvPair, key, value string) []kvPair {
    for i, p := range pairs {
        if p.Key == key {
            pairs[i].Value = value
            return pairs
        }
    }
    out := make([]kvPair, 0, len(pairs)+1)
    out = append(out, kvPair{Key: key, Value: value})
    out = append(out, pairs...)
    return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/projectinit/ -run ResourceAttrs -v && go test ./internal/projectinit/ -run SetProjectName -v && go test ./internal/projectinit/ -run GetSubKey -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/projectinit/resource_attrs.go internal/projectinit/resource_attrs_test.go
git commit -m "projectinit: OTEL_RESOURCE_ATTRIBUTES sub-key merge"
```

---

## Task 4: Owned-key merge

**Files:**
- Create: `internal/projectinit/merge.go`
- Test: `internal/projectinit/merge_test.go`

Merge produces (a) the new top-level `OrderedObject`, (b) a list of conflicts the caller must resolve. Conflict = owned key already present with a different value.

- [ ] **Step 1: Write the failing tests**

Create `internal/projectinit/merge_test.go`:

```go
package projectinit

import (
    "encoding/json"
    "strings"
    "testing"
)

// helper: parse JSON or fail
func parse(t *testing.T, s string) *OrderedObject {
    t.Helper()
    o := NewOrderedObject()
    if s == "" {
        return o
    }
    if err := json.Unmarshal([]byte(s), o); err != nil {
        t.Fatalf("parse %q: %v", s, err)
    }
    return o
}

func mustMarshal(t *testing.T, o *OrderedObject) string {
    t.Helper()
    b, err := json.Marshal(o)
    if err != nil {
        t.Fatal(err)
    }
    return string(b)
}

func TestMerge_FreshFile(t *testing.T) {
    out, conflicts, err := MergeSettings(NewOrderedObject(), "myproj", false)
    if err != nil {
        t.Fatal(err)
    }
    if len(conflicts) != 0 {
        t.Errorf("unexpected conflicts: %+v", conflicts)
    }
    s := mustMarshal(t, out)
    // env block must contain all 7 owned keys
    for _, k := range OwnedKeys() {
        if !strings.Contains(s, `"`+k+`"`) {
            t.Errorf("output missing key %s: %s", k, s)
        }
    }
    if !strings.Contains(s, "project.name=myproj") {
        t.Errorf("project.name not set: %s", s)
    }
}

func TestMerge_PreservesNonEnvTopLevelKeys(t *testing.T) {
    in := parse(t, `{"model":"opus","env":{},"theme":"dark"}`)
    out, _, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    keys := out.Keys()
    if keys[0] != "model" || keys[1] != "env" || keys[2] != "theme" {
        t.Errorf("top-level order changed: %v", keys)
    }
}

func TestMerge_PreservesNonOwnedEnvKeys(t *testing.T) {
    in := parse(t, `{"env":{"MY_VAR":"42"}}`)
    out, _, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    var top map[string]json.RawMessage
    _ = json.Unmarshal([]byte(mustMarshal(t, out)), &top)
    var env map[string]string
    _ = json.Unmarshal(top["env"], &env)
    if env["MY_VAR"] != "42" {
        t.Errorf("MY_VAR lost: %v", env)
    }
}

func TestMerge_NoConflictWhenAllValuesMatch(t *testing.T) {
    raw := `{"env":{` +
        `"CLAUDE_CODE_ENABLE_TELEMETRY":"1",` +
        `"OTEL_METRICS_EXPORTER":"otlp",` +
        `"OTEL_LOGS_EXPORTER":"otlp",` +
        `"OTEL_EXPORTER_OTLP_PROTOCOL":"grpc",` +
        `"OTEL_EXPORTER_OTLP_ENDPOINT":"http://localhost:4317",` +
        `"OTEL_RESOURCE_ATTRIBUTES":"project.name=p",` +
        `"OTEL_METRIC_EXPORT_INTERVAL":"20000"}}`
    in := parse(t, raw)
    _, conflicts, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    if len(conflicts) != 0 {
        t.Errorf("expected no conflicts, got %+v", conflicts)
    }
}

func TestMerge_ConflictReportsOwnedDiff(t *testing.T) {
    in := parse(t, `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://other:1234"}}`)
    _, conflicts, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    if len(conflicts) != 1 {
        t.Fatalf("got %d conflicts, want 1", len(conflicts))
    }
    c := conflicts[0]
    if c.Key != KeyOTLPEndpoint || c.Existing != "http://other:1234" || c.Proposed != "http://localhost:4317" {
        t.Errorf("conflict=%+v", c)
    }
}

func TestMerge_ResourceAttrsConflictsOnlyOnProjectName(t *testing.T) {
    // existing has unrelated sub-keys but no project.name → no conflict
    in := parse(t, `{"env":{"OTEL_RESOURCE_ATTRIBUTES":"enduser.id=jdoe"}}`)
    _, conflicts, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    if len(conflicts) != 0 {
        t.Errorf("expected no conflicts, got %+v", conflicts)
    }
}

func TestMerge_ResourceAttrsConflictsWhenProjectNameDiffers(t *testing.T) {
    in := parse(t, `{"env":{"OTEL_RESOURCE_ATTRIBUTES":"project.name=other"}}`)
    _, conflicts, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    if len(conflicts) != 1 || conflicts[0].Key != KeyResourceAttrs {
        t.Errorf("got %+v", conflicts)
    }
}

func TestMerge_ForceOverwritesConflicts(t *testing.T) {
    in := parse(t, `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://other:1234"}}`)
    out, conflicts, err := MergeSettings(in, "p", true)
    if err != nil {
        t.Fatal(err)
    }
    // With force=true, conflicts are still returned (for reporting) but env
    // is overwritten with canonical values.
    if len(conflicts) != 1 {
        t.Errorf("expected 1 reported conflict, got %d", len(conflicts))
    }
    s := mustMarshal(t, out)
    if !strings.Contains(s, `"OTEL_EXPORTER_OTLP_ENDPOINT":"http://localhost:4317"`) {
        t.Errorf("force did not overwrite: %s", s)
    }
}

func TestMerge_PreservesResourceAttrsSubKeys(t *testing.T) {
    in := parse(t, `{"env":{"OTEL_RESOURCE_ATTRIBUTES":"enduser.id=jdoe,deployment.environment=prod"}}`)
    out, _, err := MergeSettings(in, "p", false)
    if err != nil {
        t.Fatal(err)
    }
    s := mustMarshal(t, out)
    if !strings.Contains(s, "enduser.id=jdoe") || !strings.Contains(s, "deployment.environment=prod") {
        t.Errorf("sub-keys lost: %s", s)
    }
    if !strings.Contains(s, "project.name=p") {
        t.Errorf("project.name not added: %s", s)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/projectinit/ -run Merge -v`
Expected: FAIL with "undefined: MergeSettings".

- [ ] **Step 3: Create `merge.go`**

```go
package projectinit

import (
    "encoding/json"
    "fmt"
)

// Conflict describes an owned key whose existing value differs from the
// canonical value. Reported even when force=true so callers can show what
// was overwritten.
type Conflict struct {
    Key      string
    Existing string
    Proposed string
}

// MergeSettings produces a new top-level OrderedObject with all owned keys
// set to their canonical values. Non-owned env keys and non-env top-level
// keys pass through untouched in their original order.
//
// If force is false, owned keys whose existing value differs from canonical
// are NOT overwritten — they are returned as conflicts and the caller must
// re-call with force=true (after prompting) to apply.
//
// basename is used to set OTEL_RESOURCE_ATTRIBUTES sub-key project.name.
func MergeSettings(existing *OrderedObject, basename string, force bool) (*OrderedObject, []Conflict, error) {
    if existing == nil {
        existing = NewOrderedObject()
    }

    // 1. Materialize the env block, creating it if absent. We will return a
    //    new OrderedObject; non-env top-level keys are copied by raw value.
    out := NewOrderedObject()
    var env *OrderedObject
    for _, k := range existing.Keys() {
        v, _ := existing.Get(k)
        if k == "env" {
            env = NewOrderedObject()
            if len(v) > 0 && string(v) != "null" {
                if err := json.Unmarshal(v, env); err != nil {
                    return nil, nil, fmt.Errorf("parse env block: %w", err)
                }
            }
            // placeholder; we re-set after merging
            out.Set("env", json.RawMessage("{}"))
        } else {
            out.Set(k, v)
        }
    }
    if env == nil {
        env = NewOrderedObject()
        out.Set("env", json.RawMessage("{}"))
    }

    // 2. For each canonical key, decide: insert / no-op / conflict.
    canon := CanonicalValues()
    var conflicts []Conflict
    for _, k := range OwnedKeys() {
        if k == KeyResourceAttrs {
            mergedVal, conflict := mergeResourceAttrs(env, basename)
            if conflict != nil {
                conflicts = append(conflicts, *conflict)
                if !force {
                    continue
                }
            }
            valBytes, _ := json.Marshal(mergedVal)
            env.Set(k, valBytes)
            continue
        }
        proposed := canon[k]
        if existingRaw, ok := env.Get(k); ok {
            var existingStr string
            if err := json.Unmarshal(existingRaw, &existingStr); err != nil {
                // non-string value at owned key → treat as conflict
                conflicts = append(conflicts, Conflict{Key: k, Existing: string(existingRaw), Proposed: proposed})
                if !force {
                    continue
                }
            } else if existingStr == proposed {
                continue // already correct
            } else {
                conflicts = append(conflicts, Conflict{Key: k, Existing: existingStr, Proposed: proposed})
                if !force {
                    continue
                }
            }
        }
        b, _ := json.Marshal(proposed)
        env.Set(k, b)
    }

    // 3. Re-serialize env into out.
    envBytes, err := json.Marshal(env)
    if err != nil {
        return nil, nil, fmt.Errorf("serialize env: %w", err)
    }
    out.Set("env", envBytes)
    return out, conflicts, nil
}

// mergeResourceAttrs returns the merged value for OTEL_RESOURCE_ATTRIBUTES
// and a conflict (or nil) describing any project.name disagreement.
func mergeResourceAttrs(env *OrderedObject, basename string) (string, *Conflict) {
    var existing string
    if raw, ok := env.Get(KeyResourceAttrs); ok {
        _ = json.Unmarshal(raw, &existing) // tolerate non-string by treating as empty
    }
    pairs := parseResourceAttrs(existing)
    if v, ok := getSubKey(pairs, SubKeyProjectName); ok && v != basename {
        merged := serializeResourceAttrs(setSubKey(pairs, SubKeyProjectName, basename))
        return merged, &Conflict{Key: KeyResourceAttrs, Existing: existing, Proposed: merged}
    }
    pairs = setSubKey(pairs, SubKeyProjectName, basename)
    return serializeResourceAttrs(pairs), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/projectinit/ -run Merge -v`
Expected: all subtests PASS.

- [ ] **Step 5: Run full package suite + vet**

Run: `go vet ./internal/projectinit/ && go test ./internal/projectinit/ -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/projectinit/merge.go internal/projectinit/merge_test.go
git commit -m "projectinit: owned-key merge with conflict detection"
```

---

## Task 5: Daemon probe

**Files:**
- Create: `internal/projectinit/probe.go`
- Test: `internal/projectinit/probe_test.go`

The probe opens a gRPC client to the endpoint and calls `LogsService.Export` with an empty request. A successful response (or a gRPC error from the server, indicating the server *is* listening and speaks the protocol) means "reachable". Connection refused / timeout means "unreachable".

- [ ] **Step 1: Write the failing tests**

Create `internal/projectinit/probe_test.go`:

```go
package projectinit

import (
    "context"
    "net"
    "testing"
    "time"

    collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
    "google.golang.org/grpc"
)

// stubLogsServer accepts any ExportLogs request with an empty response.
type stubLogsServer struct {
    collogspb.UnimplementedLogsServiceServer
}

func (s *stubLogsServer) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
    return &collogspb.ExportLogsServiceResponse{}, nil
}

// startStubServer returns endpoint string and a cleanup func.
func startStubServer(t *testing.T) (string, func()) {
    t.Helper()
    lis, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("listen: %v", err)
    }
    srv := grpc.NewServer()
    collogspb.RegisterLogsServiceServer(srv, &stubLogsServer{})
    go srv.Serve(lis)
    return lis.Addr().String(), func() { srv.Stop(); _ = lis.Close() }
}

func TestGRPCProbe_Reachable(t *testing.T) {
    addr, stop := startStubServer(t)
    defer stop()
    var p GRPCProbe
    err := p.Probe(context.Background(), addr, 2*time.Second)
    if err != nil {
        t.Errorf("expected reachable, got error: %v", err)
    }
}

func TestGRPCProbe_Unreachable(t *testing.T) {
    // Pick a port that nothing is listening on by binding then closing.
    lis, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatal(err)
    }
    addr := lis.Addr().String()
    _ = lis.Close()
    var p GRPCProbe
    err = p.Probe(context.Background(), addr, 500*time.Millisecond)
    if err == nil {
        t.Error("expected unreachable error, got nil")
    }
}

func TestGRPCProbe_RespectsContextCancel(t *testing.T) {
    // Long timeout, but cancelled context should win.
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    var p GRPCProbe
    err := p.Probe(ctx, "127.0.0.1:1", 30*time.Second)
    if err == nil {
        t.Error("expected error after ctx cancel")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/projectinit/ -run GRPCProbe -v`
Expected: FAIL with "undefined: GRPCProbe".

- [ ] **Step 3: Create `probe.go`**

```go
package projectinit

import (
    "context"
    "fmt"
    "time"

    collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// Prober reports whether a daemon is reachable at the given endpoint.
// A nil error means reachable. Implementations must respect ctx cancellation.
type Prober interface {
    Probe(ctx context.Context, endpoint string, timeout time.Duration) error
}

// GRPCProbe dials endpoint over plaintext gRPC and calls LogsService.Export
// with an empty request. The daemon's own server is what we expect, but any
// listening LogsService implementation counts as "reachable".
type GRPCProbe struct{}

// Probe dials endpoint with a deadline of timeout (or ctx, whichever fires
// first) and issues one Export call. Returns nil iff the call returns
// without a transport error.
func (GRPCProbe) Probe(ctx context.Context, endpoint string, timeout time.Duration) error {
    dialCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    conn, err := grpc.DialContext(
        dialCtx,
        endpoint,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
    )
    if err != nil {
        return fmt.Errorf("dial %s: %w", endpoint, err)
    }
    defer conn.Close()
    client := collogspb.NewLogsServiceClient(conn)
    _, err = client.Export(dialCtx, &collogspb.ExportLogsServiceRequest{})
    if err != nil {
        return fmt.Errorf("export probe: %w", err)
    }
    return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/projectinit/ -run GRPCProbe -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/projectinit/probe.go internal/projectinit/probe_test.go
git commit -m "projectinit: gRPC daemon probe with timeout"
```

---

## Task 6: Top-level `Run` orchestrator

**Files:**
- Create: `internal/projectinit/run.go`
- Test: `internal/projectinit/run_test.go`

`Run` is the public entry point. Inputs are an `Options` struct (paths, flags, prompter, prober, stdout/stderr). Outputs are an exit code via error return. The prompter is an interface so tests can simulate y/n input deterministically.

- [ ] **Step 1: Write the failing tests**

Create `internal/projectinit/run_test.go`:

```go
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
    answers []bool // popped from front
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
    if !strings.Contains(s, `"model":"opus"`) {
        t.Errorf("model lost: %s", s)
    }
    if !strings.Contains(s, `"MY_VAR":"42"`) {
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
    // Prompter must NOT be called when --force.
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
        // Probe failure must NOT make Run return an error.
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/projectinit/ -run TestRun -v`
Expected: FAIL with "undefined: Run", "undefined: Options".

- [ ] **Step 3: Create `run.go`**

```go
package projectinit

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "time"
)

// Prompter asks the user a yes/no question. Returning (true, nil) means yes.
type Prompter interface {
    Confirm(prompt string) (bool, error)
}

// Options configures a single Run invocation.
type Options struct {
    ProjectDir string        // absolute path; .claude/settings.json lives at <ProjectDir>/.claude/settings.json
    Endpoint   string        // daemon endpoint to probe, e.g. "127.0.0.1:4317"
    Force      bool          // overwrite owned-key conflicts without prompting
    Print      bool          // emit merged JSON to Stdout, no FS write, no probe
    Stdout     io.Writer
    Stderr     io.Writer
    Prompter   Prompter
    Prober     Prober
}

const probeTimeout = 500 * time.Millisecond

// Run executes the cco init flow for a single project directory.
func Run(opts Options) error {
    if opts.ProjectDir == "" {
        return errors.New("ProjectDir is required")
    }
    if opts.Stdout == nil {
        opts.Stdout = os.Stdout
    }
    if opts.Stderr == nil {
        opts.Stderr = os.Stderr
    }

    settingsPath := filepath.Join(opts.ProjectDir, ".claude", "settings.json")
    existing, err := loadExisting(settingsPath)
    if err != nil {
        return fmt.Errorf("load existing settings: %w", err)
    }

    basename := filepath.Base(opts.ProjectDir)

    // First merge pass: detect conflicts without applying them.
    merged, conflicts, err := MergeSettings(existing, basename, opts.Force)
    if err != nil {
        return err
    }

    // If not --force and there are conflicts, prompt and re-merge with the
    // user's per-conflict decision. For v1 we keep this simple: one
    // confirm() prompt covering ALL conflicts. If yes → re-merge with
    // force=true. If no → keep current merged (which preserved user values).
    if !opts.Force && !opts.Print && len(conflicts) > 0 {
        for _, c := range conflicts {
            fmt.Fprintf(opts.Stdout, "  %s: %q → %q\n", c.Key, c.Existing, c.Proposed)
        }
        ok, err := opts.Prompter.Confirm("Overwrite the keys above? [y/N]")
        if err != nil {
            return fmt.Errorf("prompt: %w", err)
        }
        if ok {
            merged, _, err = MergeSettings(existing, basename, true)
            if err != nil {
                return err
            }
        }
    }

    // Render output bytes.
    rendered, err := renderIndented(merged)
    if err != nil {
        return fmt.Errorf("render: %w", err)
    }

    if opts.Print {
        _, _ = opts.Stdout.Write(rendered)
        return nil
    }

    // Write file (mkdir -p .claude/).
    if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
        return fmt.Errorf("mkdir .claude: %w", err)
    }
    if err := os.WriteFile(settingsPath, rendered, 0o644); err != nil {
        return fmt.Errorf("write settings: %w", err)
    }
    fmt.Fprintf(opts.Stdout, "✓ wrote %s (%d keys)\n", settingsPath, len(OwnedKeys()))
    fmt.Fprintf(opts.Stdout, "✓ project.name = %s\n", basename)

    // Probe daemon. Failure is informational, not fatal.
    ctx, cancel := context.WithTimeout(context.Background(), probeTimeout+500*time.Millisecond)
    defer cancel()
    if err := opts.Prober.Probe(ctx, opts.Endpoint, probeTimeout); err != nil {
        fmt.Fprintf(opts.Stdout, "✗ daemon not reachable at %s\n", opts.Endpoint)
        fmt.Fprintln(opts.Stdout, "  → start it with: cco serve")
        fmt.Fprintln(opts.Stdout, "    or install as a service: see README §Install")
    } else {
        fmt.Fprintf(opts.Stdout, "✓ daemon reachable at %s\n", opts.Endpoint)
    }
    return nil
}

func loadExisting(path string) (*OrderedObject, error) {
    b, err := os.ReadFile(path)
    if errors.Is(err, fs.ErrNotExist) {
        return NewOrderedObject(), nil
    }
    if err != nil {
        return nil, err
    }
    o := NewOrderedObject()
    if len(bytes.TrimSpace(b)) == 0 {
        return o, nil
    }
    if err := json.Unmarshal(b, o); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    return o, nil
}

func renderIndented(o *OrderedObject) ([]byte, error) {
    raw, err := json.Marshal(o)
    if err != nil {
        return nil, err
    }
    var pretty bytes.Buffer
    if err := json.Indent(&pretty, raw, "", "  "); err != nil {
        return nil, err
    }
    pretty.WriteByte('\n')
    return pretty.Bytes(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/projectinit/ -run TestRun -v`
Expected: all PASS.

- [ ] **Step 5: Run full package suite + vet + coverage check**

Run: `go vet ./internal/projectinit/ && go test -cover ./internal/projectinit/`
Expected: PASS, coverage ≥ 90%.

- [ ] **Step 6: Commit**

```bash
git add internal/projectinit/run.go internal/projectinit/run_test.go
git commit -m "projectinit: Run orchestrator with prompt/print/force flows"
```

---

## Task 7: Wire `cmd/app/init.go`

**Files:**
- Modify: `cmd/app/init.go` (replace lines 1–16)
- Create: `cmd/app/init_test.go`

The cobra subcommand reads `--force` / `--print`, builds an `Options`, supplies a real `Prompter` (reading from `cmd.InOrStdin()`) and `GRPCProbe`, and calls `projectinit.Run`.

- [ ] **Step 1: Write the failing test**

Create `cmd/app/init_test.go`:

```go
package main

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestInit_PrintEmitsJSON(t *testing.T) {
    homeDir = "" // reset global from main.go
    t.Setenv("CCO_HOME", "")

    dir := t.TempDir()
    cwd, err := os.Getwd()
    if err != nil {
        t.Fatal(err)
    }
    if err := os.Chdir(dir); err != nil {
        t.Fatal(err)
    }
    defer os.Chdir(cwd)

    root := newRootCmd()
    registerSubcommands(root)
    var out bytes.Buffer
    root.SetOut(&out)
    root.SetErr(&out)
    root.SetArgs([]string{"init", "--print"})
    if err := root.Execute(); err != nil {
        t.Fatalf("execute: %v", err)
    }
    if !strings.Contains(out.String(), "OTEL_EXPORTER_OTLP_ENDPOINT") {
        t.Errorf("--print missing owned keys: %s", out.String())
    }
    // --print must not write the file
    if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err == nil {
        t.Error("--print created the file")
    }
}

func TestInit_FreshDir_WritesFile(t *testing.T) {
    homeDir = ""
    t.Setenv("CCO_HOME", "")

    dir := t.TempDir()
    cwd, _ := os.Getwd()
    _ = os.Chdir(dir)
    defer os.Chdir(cwd)

    root := newRootCmd()
    registerSubcommands(root)
    var out bytes.Buffer
    root.SetOut(&out)
    root.SetErr(&out)
    root.SetArgs([]string{"init", "--force"}) // --force avoids stdin
    if err := root.Execute(); err != nil {
        t.Fatalf("execute: %v", err)
    }
    b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
    if err != nil {
        t.Fatalf("read settings: %v", err)
    }
    if !strings.Contains(string(b), `"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317"`) {
        t.Errorf("settings missing endpoint: %s", string(b))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/app/ -run Init -v`
Expected: FAIL — current init prints "init not yet wired" and does nothing.

- [ ] **Step 3: Replace `cmd/app/init.go`**

Overwrite the entire file with:

```go
package main

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/spf13/cobra"

    "github.com/kamikaze011001/claude-code-observer/internal/projectinit"
)

func newInitCmd() *cobra.Command {
    var (
        force bool
        print bool
    )
    cmd := &cobra.Command{
        Use:   "init",
        Short: "Write/update .claude/settings.json in the current directory",
        Long: "Configures the current project to export OpenTelemetry to localhost:4317 " +
            "by writing the canonical OTel env block to .claude/settings.json. " +
            "Existing user keys are preserved; conflicts on cco-owned keys prompt for confirmation.",
        RunE: func(cmd *cobra.Command, args []string) error {
            cwd, err := os.Getwd()
            if err != nil {
                return fmt.Errorf("getwd: %w", err)
            }
            opts := projectinit.Options{
                ProjectDir: cwd,
                Endpoint:   "127.0.0.1:4317",
                Force:      force,
                Print:      print,
                Stdout:     cmd.OutOrStdout(),
                Stderr:     cmd.ErrOrStderr(),
                Prompter:   &stdinPrompter{in: cmd.InOrStdin(), out: cmd.OutOrStdout()},
                Prober:     projectinit.GRPCProbe{},
            }
            return projectinit.Run(opts)
        },
    }
    cmd.Flags().BoolVar(&force, "force", false, "Skip prompts; overwrite owned-key conflicts")
    cmd.Flags().BoolVar(&print, "print", false, "Render merged settings.json to stdout, do not write")
    return cmd
}

// stdinPrompter implements projectinit.Prompter against the cobra command's
// configured input stream.
type stdinPrompter struct {
    in  io.Reader
    out io.Writer
}

func (p *stdinPrompter) Confirm(prompt string) (bool, error) {
    fmt.Fprintf(p.out, "%s ", prompt)
    s := bufio.NewScanner(p.in)
    if !s.Scan() {
        return false, s.Err()
    }
    line := strings.ToLower(strings.TrimSpace(s.Text()))
    return line == "y" || line == "yes", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/app/ -run Init -v`
Expected: both subtests PASS.

- [ ] **Step 5: Run all tests + vet + build**

Run: `go vet ./... && go test ./... && go build -o bin/cco ./cmd/app`
Expected: all PASS, binary built.

- [ ] **Step 6: Smoke test by hand**

Run:
```bash
mkdir -p /tmp/cco-init-smoke && cd /tmp/cco-init-smoke
~/Documents/AIBLES/claude-code-observer/bin/cco init --force
cat .claude/settings.json
cd - && rm -rf /tmp/cco-init-smoke
```
Expected: file contains all 7 owned keys, `project.name=cco-init-smoke`, ends with `daemon not reachable` line.

- [ ] **Step 7: Commit**

```bash
git add cmd/app/init.go cmd/app/init_test.go
git commit -m "cmd/app: wire init subcommand to projectinit.Run"
```

---

## Task 8: launchd plist (macOS)

**Files:**
- Create: `scripts/com.claude-code-observer.plist`

The plist uses literal `$HOME` paths resolved at install time (the user copies it into `~/Library/LaunchAgents/`). README documents replacing `__HOME__` if the user's home differs from the assumed path.

- [ ] **Step 1: Create the plist**

Create `scripts/com.claude-code-observer.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.claude-code-observer</string>

    <key>ProgramArguments</key>
    <array>
        <string>__HOME__/.claude-code-observer/bin/cco</string>
        <string>serve</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>WorkingDirectory</key>
    <string>__HOME__/.claude-code-observer</string>

    <key>StandardOutPath</key>
    <string>__HOME__/.claude-code-observer/logs/cco.log</string>

    <key>StandardErrorPath</key>
    <string>__HOME__/.claude-code-observer/logs/cco.log</string>

    <key>ProcessType</key>
    <string>Background</string>
</dict>
</plist>
```

- [ ] **Step 2: Validate XML syntax**

Run: `plutil -lint scripts/com.claude-code-observer.plist`
Expected: `scripts/com.claude-code-observer.plist: OK`

(If `plutil` is unavailable on the dev machine, skip this step and rely on installation as the validation.)

- [ ] **Step 3: Commit**

```bash
git add scripts/com.claude-code-observer.plist
git commit -m "scripts: launchd plist for macOS user agent"
```

---

## Task 9: systemd user unit (Linux)

**Files:**
- Create: `scripts/claude-code-observer.service`

`%h` is systemd's home-directory specifier — resolved at unit-load time, no template substitution required.

- [ ] **Step 1: Create the unit file**

Create `scripts/claude-code-observer.service`:

```ini
[Unit]
Description=Claude Code Observer daemon
After=network.target

[Service]
Type=simple
ExecStart=%h/.claude-code-observer/bin/cco serve
Restart=on-failure
RestartSec=5
WorkingDirectory=%h/.claude-code-observer
StandardOutput=append:%h/.claude-code-observer/logs/cco.log
StandardError=append:%h/.claude-code-observer/logs/cco.log

[Install]
WantedBy=default.target
```

- [ ] **Step 2: Validate (best effort)**

If on Linux:
Run: `systemd-analyze --user verify scripts/claude-code-observer.service`
Expected: no errors (warnings about `ExecStart` path are fine since the binary may not exist on this machine).

If on macOS or Windows, skip — verified at install time on a Linux machine per the M4.2 demo.

- [ ] **Step 3: Commit**

```bash
git add scripts/claude-code-observer.service
git commit -m "scripts: systemd user unit for Linux"
```

---

## Task 10: README install section

**Files:**
- Modify: `README.md` (replace template content)

The current README is a generic ShipWithAI template. Replace with project-specific content centered on the install flow. Keep it short — install section under 60 lines.

- [ ] **Step 1: Replace `README.md`**

Overwrite the entire file with:

```markdown
# claude-code-observer

Local observability for Claude Code via OTLP. A single Go binary ingests OTLP/gRPC telemetry into a SQLite store and renders it in a TUI — costs, prompts, tool calls, errors — all on `localhost`, no cloud.

**Stack:** Go 1.25 · gRPC · SQLite (modernc) · Bubble Tea TUI

## Install

Five steps from clone to dashboard.

### 1. Build

```bash
git clone https://github.com/kamikaze011001/claude-code-observer.git
cd claude-code-observer
mkdir -p ~/.claude-code-observer/bin ~/.claude-code-observer/logs
go build -o ~/.claude-code-observer/bin/cco ./cmd/app
```

Add `~/.claude-code-observer/bin` to your `PATH` if you want `cco` invocable from anywhere.

### 2. Install the service

**macOS (launchd):**

```bash
sed "s|__HOME__|$HOME|g" scripts/com.claude-code-observer.plist \
  > ~/Library/LaunchAgents/com.claude-code-observer.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.claude-code-observer.plist
launchctl kickstart gui/$(id -u)/com.claude-code-observer
```

**Linux (systemd user unit):**

```bash
mkdir -p ~/.config/systemd/user
cp scripts/claude-code-observer.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now claude-code-observer
```

Verify: `cco` (no args, opens TUI) — the dashboard should load without errors. The daemon listens on `127.0.0.1:4317` and writes logs to `~/.claude-code-observer/logs/cco.log`.

### 3. Configure a project

In any project where you use Claude Code:

```bash
cd path/to/your/project
cco init
```

This writes seven OTel env vars under `env` in `.claude/settings.json` and probes the daemon. Existing keys (your `model`, `theme`, `hooks`, etc.) are preserved.

### 4. Use Claude Code

Run any `claude` command in the configured project. Each prompt, API call, tool invocation, and error flows into SQLite within ~20 seconds.

### 5. Open the dashboard

```bash
cco
```

You should see today's cost, prompt count, and the most expensive sessions. Drill in with `Enter`, back out with `b`.

## Troubleshooting

- **macOS daemon logs:** `tail -f ~/.claude-code-observer/logs/cco.log` or `log show --predicate 'subsystem == "com.claude-code-observer"' --last 10m`
- **Linux daemon logs:** `journalctl --user -u claude-code-observer -f` (or the same `cco.log` file)
- **`cco init` says daemon not reachable:** the launchd/systemd unit didn't start. Check the service status (`launchctl print gui/$(id -u)/com.claude-code-observer` or `systemctl --user status claude-code-observer`).
- **Log rotation:** v1 does not rotate `cco.log`. On Linux, drop a `logrotate` config; on macOS, truncate manually.

## Stopping / Uninstall

**macOS:** `launchctl bootout gui/$(id -u)/com.claude-code-observer && rm ~/Library/LaunchAgents/com.claude-code-observer.plist`

**Linux:** `systemctl --user disable --now claude-code-observer && rm ~/.config/systemd/user/claude-code-observer.service`

Data lives in `~/.claude-code-observer/`; remove the directory to wipe state.

## Architecture & Decisions

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — layer boundaries
- [docs/DATA-MODELS.md](docs/DATA-MODELS.md) — schema
- [docs/decisions/](docs/decisions/) — ADRs
- [docs/CLAUDE-CODE-OTEL.md](docs/CLAUDE-CODE-OTEL.md) — what Claude Code emits
- [docs/ROADMAP.md](docs/ROADMAP.md) — milestone tracker

## License

MIT.
```

- [ ] **Step 2: Verify the README renders**

Run: `head -50 README.md`
Expected: Install section starts within first 50 lines, no template placeholders remain.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: project README with Install section"
```

---

## Task 11: Manual verification checklist + roadmap fix

**Files:**
- Modify: `docs/MANUAL-VERIFICATION.md` (append Phase 4 entries)
- Modify: `docs/ROADMAP.md:269` (rename `internal/init/` → `internal/projectinit/`)

- [ ] **Step 1: Read existing MANUAL-VERIFICATION.md**

Run: `cat docs/MANUAL-VERIFICATION.md` (use the Read tool).
Note the existing structure so the new section matches it.

- [ ] **Step 2: Append Phase 4 entries**

Append to `docs/MANUAL-VERIFICATION.md` (preserve existing content):

```markdown

## Phase 4 — Install Ergonomics

### M4.1: cco init

- [ ] In an empty temp dir, `cco init --force` creates `.claude/settings.json` containing all 7 owned keys.
- [ ] `OTEL_RESOURCE_ATTRIBUTES` value contains `project.name=<dirname>`.
- [ ] Re-running `cco init --force` is a no-op (file mtime updates but content is byte-identical via `diff`).
- [ ] In a dir with `.claude/settings.json` containing `{"model":"opus","env":{"MY_VAR":"42"}}`, `cco init --force` preserves both keys.
- [ ] In a dir where an owned key conflicts, plain `cco init` (no flags) prints the conflict, prompts, and respects y/N.
- [ ] `cco init --print` writes nothing to disk; renders valid JSON to stdout.
- [ ] With daemon stopped: output ends with `✗ daemon not reachable` and the hint lines.
- [ ] With daemon running: output ends with `✓ daemon reachable at 127.0.0.1:4317`.

### M4.2: Service files (macOS)

- [ ] After `launchctl bootstrap`, `launchctl print gui/$(id -u)/com.claude-code-observer` shows the service running.
- [ ] Trigger a Claude Code prompt — TUI dashboard updates within 20–30 s.
- [ ] Logout / login — daemon still running.
- [ ] `kill -9 <pid>` — launchd restarts it within 10 s.
- [ ] `launchctl kill TERM gui/$(id -u)/com.claude-code-observer` followed by `launchctl bootout` — clean shutdown, file removed, no zombie.

### M4.2: Service files (Linux)

- [ ] After `systemctl --user enable --now`, `systemctl --user status claude-code-observer` shows `active (running)`.
- [ ] Trigger a Claude Code prompt — TUI dashboard updates within 20–30 s.
- [ ] User session restart — daemon still running.
- [ ] `kill -9 <pid>` — systemd restarts it within 10 s (RestartSec=5).
- [ ] Disable + remove — clean shutdown, file removed.

### M4.2: README dry-run

- [ ] On a fresh user account (or a clean VM), follow README §Install end-to-end without help. Reach the dashboard with at least one populated session in under 5 minutes.
```

- [ ] **Step 3: Update roadmap**

Open `docs/ROADMAP.md` and replace `internal/init/` with `internal/projectinit/` on line 269 (under M4.1's Test gate).

Use Edit:
- old_string: `- Coverage ≥ 90% on \`internal/init/\``
- new_string: `- Coverage ≥ 90% on \`internal/projectinit/\``

- [ ] **Step 4: Commit**

```bash
git add docs/MANUAL-VERIFICATION.md docs/ROADMAP.md
git commit -m "docs: phase 4 manual verification checklist + projectinit rename"
```

---

## Task 12: Final verification

**Files:** none (verification only)

- [ ] **Step 1: Full verification chain**

Run:
```bash
go vet ./... && go test ./... && go build -o bin/cco ./cmd/app
```
Expected: all green.

- [ ] **Step 2: Coverage check on `internal/projectinit/`**

Run: `go test -cover ./internal/projectinit/`
Expected: coverage ≥ 90% per the (renamed) M4.1 test gate.

- [ ] **Step 3: golangci-lint (if installed)**

Run: `golangci-lint run ./internal/projectinit/ ./cmd/app/`
Expected: zero issues.

- [ ] **Step 4: End-to-end smoke test**

In one terminal: `bin/cco serve`
In another:
```bash
mkdir -p /tmp/cco-e2e && cd /tmp/cco-e2e
~/Documents/AIBLES/claude-code-observer/bin/cco init
```
Expected output ends with `✓ daemon reachable at 127.0.0.1:4317`. File is valid JSON. Stop daemon, re-run `cco init` → output ends with the unreachable hint. Cleanup: `cd - && rm -rf /tmp/cco-e2e`. Stop the daemon.

- [ ] **Step 5: Re-run readability check on README**

Open `README.md` in the editor. Confirm: Install section reads top-to-bottom, no jargon left undefined, all commands are copy-pasteable.

---

## Self-review

Spec coverage:

- M4.1 owned keys (7) — Tasks 1, 4 ✓
- M4.1 conflict resolution (silent merge / prompt / `--force`) — Tasks 4, 6, 7 ✓
- M4.1 `OTEL_RESOURCE_ATTRIBUTES` sub-key merge — Tasks 3, 4 ✓
- M4.1 file/dir creation, 2-space indent, trailing newline — Task 6 ✓
- M4.1 `--force` and `--print` flags — Tasks 6, 7 ✓
- M4.1 daemon probe (gRPC, 500ms, non-fatal) — Tasks 5, 6 ✓
- M4.1 happy / daemon-down output — Tasks 6, 7 ✓
- M4.1 `internal/projectinit/` boundaries (no service/repo/receiver deps) — Task 1 (doc.go) + reviewed in Task 6 imports ✓
- M4.1 ≥90% coverage gate — Task 12 step 2 ✓
- M4.2 launchd plist with RunAtLoad + KeepAlive-on-failure + log paths — Task 8 ✓
- M4.2 systemd unit with Restart=on-failure + WantedBy=default.target + log paths — Task 9 ✓
- M4.2 README install section ≤5 min path — Task 10 ✓
- M4.2 troubleshooting subsection — Task 10 ✓
- M4.2 manual verification — Task 11 ✓
- Open question (JSON key-order stability) — resolved by `OrderedObject` in Task 2 ✓
- Open question (install prefix `$HOME/.claude-code-observer/bin/cco`) — documented in Task 8 plist + Task 10 README ✓

Type consistency check:

- `OrderedObject` / `NewOrderedObject` — used identically in Tasks 2, 4, 6 ✓
- `Conflict{Key, Existing, Proposed}` — defined Task 4, used Tasks 4, 6 ✓
- `Prober.Probe(ctx, endpoint, timeout) error` — defined Task 5, used Tasks 6, 7 ✓
- `Prompter.Confirm(prompt) (bool, error)` — defined Task 6, used Tasks 6, 7 ✓
- `Options{ProjectDir, Endpoint, Force, Print, Stdout, Stderr, Prompter, Prober}` — defined Task 6, used Task 7 ✓
- Owned-key constants — defined Task 1, used Tasks 4, 7 ✓

Placeholder scan: no TBD / TODO / "fill in"; every code step has runnable code; every test has runnable tests; every command has expected output.
