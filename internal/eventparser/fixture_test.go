package eventparser

import (
	"os"
	"path/filepath"
	"testing"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// loadFixture parses a single-record JSON file under testdata/fixtures.
// Schema: {"resource": <Resource>, "record": <LogRecord>}.
func loadFixture(t *testing.T, name string) (*resourcepb.Resource, *logspb.LogRecord) {
	t.Helper()
	bs, err := os.ReadFile(filepath.Join("testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	// Manual two-step decode: protojson can't natively unmarshal a struct with
	// two distinct proto fields, so we pull each out by string indexing the JSON.
	resBytes, recBytes := splitFixtureJSON(t, bs)
	var res resourcepb.Resource
	if len(resBytes) > 0 {
		if err := protojson.Unmarshal(resBytes, &res); err != nil {
			t.Fatalf("unmarshal resource in %s: %v", name, err)
		}
	}
	var rec logspb.LogRecord
	if err := protojson.Unmarshal(recBytes, &rec); err != nil {
		t.Fatalf("unmarshal record in %s: %v", name, err)
	}
	return &res, &rec
}

// splitFixtureJSON pulls the "resource" and "record" sub-objects out of the
// top-level fixture JSON. Implemented with encoding/json since protojson does
// not support unknown wrapper structs.
func splitFixtureJSON(t *testing.T, raw []byte) (resJSON, recJSON []byte) {
	t.Helper()
	var wrap struct {
		Resource any `json:"resource"`
		Record   any `json:"record"`
	}
	if err := jsonUnmarshal(raw, &wrap); err != nil {
		t.Fatalf("split fixture: %v", err)
	}
	resJSON, _ = jsonMarshal(wrap.Resource)
	recJSON, _ = jsonMarshal(wrap.Record)
	return resJSON, recJSON
}

func parseFixture(t *testing.T, name string) domain.Event {
	t.Helper()
	res, rec := loadFixture(t, name)
	ev, err := Parse(rec, res)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return ev
}
