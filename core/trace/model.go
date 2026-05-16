package trace

import "time"

// Trace is one complete operation, represented as spans.
type Trace struct {
	TraceID string
	Spans   []Span
}

type SpanKind string

const (
	SpanKindFlow      SpanKind = "FLOW"
	SpanKindNode      SpanKind = "NODE"
	SpanKindAgent     SpanKind = "AGENT"
	SpanKindPlanning  SpanKind = "PLANNING"
	SpanKindLLM       SpanKind = "LLM"
	SpanKindTool      SpanKind = "TOOL"
	SpanKindConnector SpanKind = "CONNECTOR"
	SpanKindWorkflow  SpanKind = "WORKFLOW"
	SpanKindInternal  SpanKind = "INTERNAL"
)

type SpanStatus string

const (
	SpanStatusRunning SpanStatus = "running"
	SpanStatusWaiting SpanStatus = "waiting"
	SpanStatusOK      SpanStatus = "ok"
	SpanStatusError   SpanStatus = "error"
)

// Span is intentionally close to the OpenTelemetry span model while keeping
// product-friendly input/output fields for Agent Debug UI.
type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	Kind         SpanKind
	Status       SpanStatus
	StartedAt    time.Time
	FinishedAt   time.Time
	Attributes   map[string]any
	Events       []SpanEvent
	Links        []SpanLink
	Input        map[string]any
	Output       map[string]any
	Error        string
}

type SpanEvent struct {
	Name       string
	Attributes map[string]any
	Timestamp  time.Time
}

type SpanLink struct {
	TraceID    string
	SpanID     string
	Attributes map[string]any
}

type SpanTreeNode struct {
	Span     Span
	Children []SpanTreeNode
}
