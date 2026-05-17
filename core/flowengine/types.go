package flowengine

import "time"

// FlowSpec is the immutable runtime definition of a flow.
//
// The engine owns control flow, context reads/writes, node lifecycle, and trace
// emission. Business capabilities are supplied through NodeExecutor plugins.
type FlowSpec struct {
	ID            string
	Name          string
	Version       string
	ContextSchema ContextSchema
	Nodes         []NodeSpec
	Edges         []EdgeSpec
	Policies      FlowPolicies
}

// FlowPolicies defines cross-node runtime behavior.
type FlowPolicies struct {
	MaxNodeExecutions int
}

// NodeSpec is the engine-level configuration protocol for one node.
type NodeSpec struct {
	ID            string
	Type          string
	Name          string
	Config        map[string]any
	InputMappings []FieldBinding
	OutputWrites  []ContextWrite
	Timeout       time.Duration
	RetryPolicy   RetryPolicy
}

// RetryPolicy is reserved for retry-capable executors. The first core version
// keeps it in the public protocol so product code does not need another schema
// migration later.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

// EdgeSpec declares a directed connection between nodes.
type EdgeSpec struct {
	From string
	To   string
}

// FieldSchema describes an input/output/context field in a human-readable and
// machine-validated form.
type FieldSchema struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Children    []FieldSchema
}

// ContextSchema is the standard context protocol exposed to workflow authors.
type ContextSchema struct {
	FlowInput   []FieldSchema
	FlowOutput  []FieldSchema
	Variables   []FieldSchema
	NodeContext map[string][]FieldSchema
}

// ValueSource declares where a mapping value comes from.
type ValueSource struct {
	Type  ValueSourceType
	Path  string
	Value any
}

// ValueSourceType keeps mapping semantics explicit and easy to validate.
type ValueSourceType string

const (
	SourceContext    ValueSourceType = "context"
	SourceConst      ValueSourceType = "const"
	SourceNodeOutput ValueSourceType = "node_output"
)

// FieldBinding maps one node input field from context or a constant.
type FieldBinding struct {
	Field   string
	Source  ValueSource
	Enabled bool
}

// ContextWrite writes one value from node output or a constant back to context.
type ContextWrite struct {
	Target  string
	Source  ValueSource
	Enabled bool
}

// NodeRequest is passed to a node executor.
type NodeRequest struct {
	RunID   string
	Flow    FlowSpec
	Node    NodeSpec
	Input   map[string]any
	Context *DataContext
}

// NodeResult is returned by a node executor.
type NodeResult struct {
	Output map[string]any
	// NextNodeIDs controls downstream execution. nil means "use static outgoing
	// edges"; an empty non-nil slice means "stop here".
	NextNodeIDs []string
}

// RunResult is the final execution result.
type RunResult struct {
	RunID     string
	Context   *DataContext
	NodeOrder []string
	Events    []FlowEvent
}
