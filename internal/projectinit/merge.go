package projectinit

import (
	"encoding/json"
	"fmt"
)

// Conflict describes an owned key whose existing value differs from the
// canonical value. Reported even when force=true so callers can show what
// was overwritten.
type Conflict struct {
	Key      string
	Existing string
	Proposed string
}

// MergeSettings produces a new top-level OrderedObject with all owned keys
// set to their canonical values. Non-owned env keys and non-env top-level
// keys pass through untouched in their original order.
//
// If force is false, owned keys whose existing value differs from canonical
// are NOT overwritten — they are returned as conflicts and the caller must
// re-call with force=true (after prompting) to apply.
//
// basename is used to set OTEL_RESOURCE_ATTRIBUTES sub-key project.name.
func MergeSettings(existing *OrderedObject, basename string, force bool) (*OrderedObject, []Conflict, error) {
	if existing == nil {
		existing = NewOrderedObject()
	}

	out := NewOrderedObject()
	var env *OrderedObject
	for _, k := range existing.Keys() {
		v, _ := existing.Get(k)
		if k == "env" {
			env = NewOrderedObject()
			if len(v) > 0 && string(v) != "null" {
				if err := json.Unmarshal(v, env); err != nil {
					return nil, nil, fmt.Errorf("parse env block: %w", err)
				}
			}
			out.Set("env", json.RawMessage("{}"))
		} else {
			out.Set(k, v)
		}
	}
	if env == nil {
		env = NewOrderedObject()
		out.Set("env", json.RawMessage("{}"))
	}

	canon := CanonicalValues()
	var conflicts []Conflict
	for _, k := range OwnedKeys() {
		if k == KeyResourceAttrs {
			mergedVal, conflict := mergeResourceAttrs(env, basename)
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
				if !force {
					// Still write the merged value (which adds project.name
					// without overwriting other sub-keys), just skip the force overwrite.
					// Actually per spec: if no force, don't overwrite on conflict.
					// But for resource attrs we only conflict on project.name mismatch.
					// If force=false and conflict, skip setting the key.
					continue
				}
			}
			valBytes, _ := json.Marshal(mergedVal)
			env.Set(k, valBytes)
			continue
		}
		proposed := canon[k]
		if existingRaw, ok := env.Get(k); ok {
			var existingStr string
			if err := json.Unmarshal(existingRaw, &existingStr); err != nil {
				conflicts = append(conflicts, Conflict{Key: k, Existing: string(existingRaw), Proposed: proposed})
				if !force {
					continue
				}
			} else if existingStr == proposed {
				continue
			} else {
				conflicts = append(conflicts, Conflict{Key: k, Existing: existingStr, Proposed: proposed})
				if !force {
					continue
				}
			}
		}
		b, _ := json.Marshal(proposed)
		env.Set(k, b)
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return nil, nil, fmt.Errorf("serialize env: %w", err)
	}
	out.Set("env", envBytes)
	return out, conflicts, nil
}

// mergeResourceAttrs returns the merged value for OTEL_RESOURCE_ATTRIBUTES
// and a conflict (or nil) describing any project.name disagreement.
func mergeResourceAttrs(env *OrderedObject, basename string) (string, *Conflict) {
	var existing string
	if raw, ok := env.Get(KeyResourceAttrs); ok {
		_ = json.Unmarshal(raw, &existing)
	}
	pairs := parseResourceAttrs(existing)
	if v, ok := getSubKey(pairs, SubKeyProjectName); ok && v != basename {
		merged := serializeResourceAttrs(setSubKey(pairs, SubKeyProjectName, basename))
		return merged, &Conflict{Key: KeyResourceAttrs, Existing: existing, Proposed: merged}
	}
	pairs = setSubKey(pairs, SubKeyProjectName, basename)
	return serializeResourceAttrs(pairs), nil
}
