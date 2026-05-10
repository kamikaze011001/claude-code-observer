package projectinit

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// OrderedObject is a JSON object that preserves insertion order on marshal.
// Values are stored as json.RawMessage so unrelated subtrees pass through
// byte-identical.
type OrderedObject struct {
	keys   []string
	values map[string]json.RawMessage
}

func NewOrderedObject() *OrderedObject {
	return &OrderedObject{values: map[string]json.RawMessage{}}
}

// Get returns the raw JSON value for key and whether it was present.
func (o *OrderedObject) Get(key string) (json.RawMessage, bool) {
	v, ok := o.values[key]
	return v, ok
}

// Set inserts or updates key with value. New keys are appended to the order.
func (o *OrderedObject) Set(key string, value json.RawMessage) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// Keys returns the keys in insertion order. The returned slice is a copy.
func (o *OrderedObject) Keys() []string {
	out := make([]string, len(o.keys))
	copy(out, o.keys)
	return out
}

func (o *OrderedObject) UnmarshalJSON(data []byte) error {
	o.keys = nil
	o.values = map[string]json.RawMessage{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("expected JSON object, got %v", tok)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("expected string key, got %v", keyTok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("decode %q: %w", key, err)
		}
		o.keys = append(o.keys, key)
		o.values[key] = raw
	}
	if _, err := dec.Token(); err != nil { // consume '}'
		return err
	}
	return nil
}

func (o *OrderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(o.values[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
