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
