// parser-debug reads a fixture JSON file (the {resource, scope, record} shape
// used in internal/eventparser/testdata/fixtures) and prints the parsed
// domain.Event to stdout.
//
// Usage: parser-debug path/to/fixture.json
package main

import (
	"encoding/json"
	"fmt"
	"os"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/kamikaze011001/claude-code-observer/internal/eventparser"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: parser-debug <fixture.json>")
		os.Exit(2)
	}
	bs, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail("read: %v", err)
	}
	var wrap struct {
		Resource json.RawMessage `json:"resource"`
		Record   json.RawMessage `json:"record"`
	}
	if err := json.Unmarshal(bs, &wrap); err != nil {
		fail("decode wrapper: %v", err)
	}
	var res resourcepb.Resource
	if len(wrap.Resource) > 0 && string(wrap.Resource) != "null" {
		if err := protojson.Unmarshal(wrap.Resource, &res); err != nil {
			fail("decode resource: %v", err)
		}
	}
	var rec logspb.LogRecord
	if err := protojson.Unmarshal(wrap.Record, &rec); err != nil {
		fail("decode record: %v", err)
	}
	ev, err := eventparser.Parse(&rec, &res)
	if err != nil {
		fail("parse: %v", err)
	}
	out, _ := json.MarshalIndent(ev, "", "  ")
	fmt.Println(string(out))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "parser-debug: "+format+"\n", args...)
	os.Exit(1)
}
