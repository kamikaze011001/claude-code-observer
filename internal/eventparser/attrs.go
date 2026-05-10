// Package eventparser converts OTLP LogRecord values into domain.Event values.
// It is a pure module: no I/O, no DB, no gRPC concerns. Resource attributes are
// flattened into the event's attrs map so downstream code does not need to join.
package eventparser

import (
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// FlattenKVs collapses an OTLP KeyValue slice into map[string]any. Returns nil
// for nil input. Unknown AnyValue variants are stored as their Go zero-value
// proto representation; this is rare in practice for Claude Code emits.
func FlattenKVs(kvs []*commonpb.KeyValue) map[string]any {
	if kvs == nil {
		return nil
	}
	out := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		out[kv.GetKey()] = anyValueToGo(kv.GetValue())
	}
	return out
}

func anyValueToGo(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return x.DoubleValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_BytesValue:
		return x.BytesValue
	case *commonpb.AnyValue_KvlistValue:
		return FlattenKVs(x.KvlistValue.GetValues())
	case *commonpb.AnyValue_ArrayValue:
		arr := x.ArrayValue.GetValues()
		out := make([]any, len(arr))
		for i, e := range arr {
			out[i] = anyValueToGo(e)
		}
		return out
	default:
		return nil
	}
}
