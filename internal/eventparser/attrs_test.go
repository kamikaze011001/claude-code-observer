package eventparser

import (
	"reflect"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func TestFlattenKVs(t *testing.T) {
	in := []*commonpb.KeyValue{
		{Key: "s", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "x"}}},
		{Key: "i", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 7}}},
		{Key: "d", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.5}}},
		{Key: "b", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
	}
	got := FlattenKVs(in)
	want := map[string]any{"s": "x", "i": int64(7), "d": 1.5, "b": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestFlattenKVs_Nested(t *testing.T) {
	in := []*commonpb.KeyValue{{
		Key: "kv",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{
			KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{
				{Key: "inner", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "y"}}},
			}},
		}},
	}}
	got := FlattenKVs(in)
	want := map[string]any{"kv": map[string]any{"inner": "y"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestFlattenKVs_Array(t *testing.T) {
	in := []*commonpb.KeyValue{{
		Key: "arr",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{
			ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{
				{Value: &commonpb.AnyValue_StringValue{StringValue: "a"}},
				{Value: &commonpb.AnyValue_IntValue{IntValue: 2}},
			}},
		}},
	}}
	got := FlattenKVs(in)
	want := map[string]any{"arr": []any{"a", int64(2)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestFlattenKVs_NilSafe(t *testing.T) {
	if got := FlattenKVs(nil); got != nil {
		t.Fatalf("nil input → got %#v, want nil", got)
	}
}

func TestFlattenKVs_BytesAndUnknown(t *testing.T) {
	in := []*commonpb.KeyValue{
		{Key: "by", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{1, 2, 3}}}},
		{Key: "nil", Value: nil},
	}
	got := FlattenKVs(in)
	if !reflect.DeepEqual(got["by"], []byte{1, 2, 3}) {
		t.Errorf("bytes mismatch: %#v", got["by"])
	}
	if got["nil"] != nil {
		t.Errorf("nil value not nil: %v", got["nil"])
	}
}
