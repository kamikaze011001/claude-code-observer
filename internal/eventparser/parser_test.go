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

func TestParse_APIError_Synth(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 9,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.api_error"),
			kvStr("session.id", "s"),
			kvStr("prompt.id", "p"),
			kvStr("model", "claude-opus-4-7"),
			kvStr("error_message", "rate limit"),
			kvInt("http_status_code", 429),
			kvInt("attempt", 3),
		},
	}
	ev, err := Parse(rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Attrs["error_message"] != "rate limit" {
		t.Errorf("error_message = %v", ev.Attrs["error_message"])
	}
	if ev.Attrs["http_status_code"] != int64(429) {
		t.Errorf("http_status_code = %v", ev.Attrs["http_status_code"])
	}
}

func TestParse_Compact_Synth(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 1,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.compact"),
			kvStr("session.id", "s"),
			kvStr("prompt.id", "p"),
			kvInt("tokens_before", 100000),
			kvInt("tokens_after", 50000),
		},
	}
	ev, err := Parse(rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.EventName != "claude_code.compact" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if ev.Attrs["tokens_before"] != int64(100000) {
		t.Errorf("tokens_before = %v", ev.Attrs["tokens_before"])
	}
}

func TestParse_SubagentDispatch_Synth(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 1,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.subagent_dispatch"),
			kvStr("session.id", "parent-1"),
			kvStr("parent_session.id", "parent-1"),
			kvStr("child_session.id", "child-1"),
		},
	}
	ev, err := Parse(rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Attrs["child_session.id"] != "child-1" {
		t.Errorf("child_session.id = %v", ev.Attrs["child_session.id"])
	}
}
