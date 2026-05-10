package eventparser

import (
	"errors"
	"reflect"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func kvStr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}
func kvInt(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}
func kvFloat(k string, v float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}}}
}

func TestParse_MissingSessionIDReturnsErrDrop(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 100,
		Attributes:   []*commonpb.KeyValue{kvStr("event.name", "claude_code.user_prompt")},
	}
	_, err := Parse(rec, nil)
	if !errors.Is(err, ErrDrop) {
		t.Fatalf("err = %v, want ErrDrop", err)
	}
}

func TestParse_MinimalUserPrompt(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 1700000000000000000,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.user_prompt"),
			kvStr("session.id", "sess-1"),
			kvStr("prompt.id", "pr-1"),
			kvInt("prompt_length", 42),
		},
	}
	res := &resourcepb.Resource{
		Attributes: []*commonpb.KeyValue{
			kvStr("project.name", "demo"),
			kvStr("app.version", "2.x"),
			kvStr("os.type", "darwin"),
		},
	}
	got, err := Parse(rec, res)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.SessionID != "sess-1" || got.PromptID != "pr-1" || got.EventName != "claude_code.user_prompt" {
		t.Fatalf("identity wrong: %+v", got)
	}
	if got.TS != 1700000000000000000 {
		t.Fatalf("ts = %d", got.TS)
	}
	wantAttrs := map[string]any{
		"prompt_length": int64(42),
		"project.name":  "demo",
		"app.version":   "2.x",
		"os.type":       "darwin",
	}
	if !reflect.DeepEqual(got.Attrs, wantAttrs) {
		t.Fatalf("attrs:\n got %#v\nwant %#v", got.Attrs, wantAttrs)
	}
}

func TestParse_UnknownEventNameStoredVerbatim(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 5,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.something_new"),
			kvStr("session.id", "sess-1"),
			kvStr("custom_field", "yo"),
		},
	}
	got, err := Parse(rec, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.EventName != "claude_code.something_new" {
		t.Fatalf("event_name = %q", got.EventName)
	}
	if got.Attrs["custom_field"] != "yo" {
		t.Fatalf("attrs missing custom_field: %#v", got.Attrs)
	}
}
