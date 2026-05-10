package rollup

import "testing"

func TestAttrInt64(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		key   string
		want  int64
	}{
		{"missing key", map[string]any{}, "x", 0},
		{"float value", map[string]any{"x": float64(42)}, "x", 42},
		{"int value", map[string]any{"x": 42}, "x", 42},
		{"int64 value", map[string]any{"x": int64(42)}, "x", 42},
		{"string value ignored", map[string]any{"x": "42"}, "x", 0},
		{"nil attrs", nil, "x", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := attrInt64(tc.attrs, tc.key); got != tc.want {
				t.Errorf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestAttrFloat64(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		key   string
		want  float64
	}{
		{"missing", map[string]any{}, "cost_usd", 0},
		{"float", map[string]any{"cost_usd": 1.25}, "cost_usd", 1.25},
		{"int coerced", map[string]any{"cost_usd": 2}, "cost_usd", 2},
		{"string ignored", map[string]any{"cost_usd": "1.25"}, "cost_usd", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := attrFloat64(tc.attrs, tc.key); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestAttrString(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		key   string
		want  string
	}{
		{"missing", map[string]any{}, "k", ""},
		{"string", map[string]any{"k": "v"}, "k", "v"},
		{"non-string ignored", map[string]any{"k": 1}, "k", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := attrString(tc.attrs, tc.key); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
