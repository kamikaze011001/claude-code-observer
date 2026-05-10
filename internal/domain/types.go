// Package domain defines the core data types shared across receiver,
// service, repository, and TUI layers.
package domain

// Event is the in-memory representation of a single OTLP log record after
// parsing. Persisted to the events table; attrs is serialized to JSON.
type Event struct {
	ID        int64
	TS        int64 // unix nanoseconds (OTel time_unix_nano)
	SessionID string
	PromptID  string // empty for session-level events
	EventName string
	Attrs     map[string]any
}

// Session is the rollup row for a single Claude Code invocation.
type Session struct {
	SessionID           string
	ProjectName         string
	ProjectCWD          string
	StartedAt           int64
	LastSeenAt          int64
	EndedAt             *int64
	AppVersion          string
	OSType              string
	UserID              string
	CostUSD             float64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	APIRequests         int64
	APIErrors           int64
	SubagentRequests    int64
	AuxiliaryRequests   int64
	ToolCalls           int64
	ToolDenied          int64
	Prompts             int64
}

// Prompt is the rollup row for a single user turn within a session.
type Prompt struct {
	PromptID            string
	SessionID           string
	StartedAt           int64
	EndedAt             *int64
	PromptLength        int64
	CommandName         string
	CommandSource       string
	CostUSD             float64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	APIRequests         int64
	SubagentRequests    int64
	ToolCalls           int64
	HadError            bool
}

// MetricSnapshot is a single OTLP metric datapoint persisted for sanity
// checking against the events-derived rollups (see ADR-003).
type MetricSnapshot struct {
	ID         int64
	TS         int64
	SessionID  string
	MetricName string
	Value      float64
	Attrs      map[string]any
}
