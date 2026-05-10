package rollup

// attrInt64 returns the value at key as int64. Accepts float64, int, int64.
// Returns 0 for missing keys, nil maps, or wrong-type values.
func attrInt64(attrs map[string]any, key string) int64 {
	if attrs == nil {
		return 0
	}
	switch v := attrs[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	default:
		return 0
	}
}

// attrFloat64 returns the value at key as float64. Accepts float64, int,
// int64. Returns 0 otherwise.
func attrFloat64(attrs map[string]any, key string) float64 {
	if attrs == nil {
		return 0
	}
	switch v := attrs[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

// attrString returns the string at key, or "" if missing or wrong-typed.
func attrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if s, ok := attrs[key].(string); ok {
		return s
	}
	return ""
}
