package projectinit

import (
	"encoding/json"
	"testing"
)

func TestOrderedObject_UnmarshalPreservesOrder(t *testing.T) {
	in := []byte(`{"b":1,"a":"x","c":{"nested":true}}`)
	o := NewOrderedObject()
	if err := json.Unmarshal(in, o); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keys := o.Keys()
	want := []string{"b", "a", "c"}
	if len(keys) != len(want) {
		t.Fatalf("len(keys)=%d, want %d", len(keys), len(want))
	}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("keys[%d]=%q, want %q", i, keys[i], k)
		}
	}
}

func TestOrderedObject_RoundTripBytes(t *testing.T) {
	in := []byte(`{"b":1,"a":"x","c":{"nested":true}}`)
	o := NewOrderedObject()
	if err := json.Unmarshal(in, o); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != string(in) {
		t.Errorf("round-trip diff:\n got: %s\nwant: %s", out, in)
	}
}

func TestOrderedObject_SetAppendsNewKeysPreservesExisting(t *testing.T) {
	o := NewOrderedObject()
	if err := json.Unmarshal([]byte(`{"a":1,"b":2}`), o); err != nil {
		t.Fatal(err)
	}
	o.Set("c", json.RawMessage(`3`))
	o.Set("a", json.RawMessage(`99`))
	out, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":99,"b":2,"c":3}`
	if string(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
}

func TestOrderedObject_GetAbsentReturnsFalse(t *testing.T) {
	o := NewOrderedObject()
	if _, ok := o.Get("missing"); ok {
		t.Error("expected ok=false for absent key")
	}
}

func TestOrderedObject_RejectsNonObject(t *testing.T) {
	o := NewOrderedObject()
	if err := json.Unmarshal([]byte(`[1,2,3]`), o); err == nil {
		t.Error("expected error for non-object")
	}
}
