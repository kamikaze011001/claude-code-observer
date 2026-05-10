package eventparser

import (
	"errors"
	"fmt"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// ErrDrop signals the caller that the record should be skipped (logged at WARN)
// without failing the surrounding batch. Returned for records missing required
// identity fields like session.id.
var ErrDrop = errors.New("eventparser: drop record")

// resourceAttrAllowlist is the subset of resource attributes that are merged
// into Event.Attrs. Keep this tight — they end up in the events.attrs JSON
// blob and we don't want to balloon row size.
var resourceAttrAllowlist = map[string]struct{}{
	"project.name":    {},
	"project.cwd":     {},
	"app.version":     {},
	"service.version": {},
	"os.type":         {},
	"os.version":      {},
	"host.arch":       {},
	"user.id":         {},
	"user.email":      {},
	"organization.id": {},
}

// Parse converts an OTLP LogRecord (plus its enclosing Resource) into a
// domain.Event. Returns ErrDrop when session.id is missing.
func Parse(rec *logspb.LogRecord, resource *resourcepb.Resource) (domain.Event, error) {
	if rec == nil {
		return domain.Event{}, fmt.Errorf("eventparser: nil record: %w", ErrDrop)
	}
	flat := FlattenKVs(rec.GetAttributes())
	sessionID, _ := flat["session.id"].(string)
	if sessionID == "" {
		return domain.Event{}, fmt.Errorf("eventparser: missing session.id: %w", ErrDrop)
	}
	promptID, _ := flat["prompt.id"].(string)
	eventName := eventNameOf(rec, flat)

	// Move identity fields out of attrs — they are first-class columns.
	delete(flat, "session.id")
	delete(flat, "prompt.id")
	delete(flat, "event.name")

	if resource != nil {
		for _, kv := range resource.GetAttributes() {
			if _, ok := resourceAttrAllowlist[kv.GetKey()]; !ok {
				continue
			}
			if _, exists := flat[kv.GetKey()]; exists {
				continue // record-level attr wins
			}
			flat[kv.GetKey()] = anyValueToGo(kv.GetValue())
		}
	}

	return domain.Event{
		TS:        int64(rec.GetTimeUnixNano()),
		SessionID: sessionID,
		PromptID:  promptID,
		EventName: eventName,
		Attrs:     flat,
	}, nil
}

// eventNameOf checks (in order) the LogRecord.EventName field, then the
// "event.name" attribute. If neither is set, returns the empty string.
func eventNameOf(rec *logspb.LogRecord, flat map[string]any) string {
	if n := rec.GetEventName(); n != "" {
		return n
	}
	if s, ok := flat["event.name"].(string); ok {
		return s
	}
	return ""
}
