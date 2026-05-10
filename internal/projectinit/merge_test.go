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
