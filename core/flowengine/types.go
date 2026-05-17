package flowengine

import (
	"time"

	"flow-anything/core/expression"
	"flow-anything/core/schema"
)

// FlowSpec is the immutable runtime definition of a flow.
//
// The engine owns control flow, context reads/writes, node lifecycle, and trace
// emission. Business capabilities are supplied through NodeExecutor plugins.
type FlowSpec struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Version       string        `json:"version"`
	ContextSchema ContextSchema `json:"context_schema"`
	Nodes         []NodeSpec    `json:"nodes"`
	Edges         []EdgeSpec    `json:"edges"`
	Policies      FlowPolicies  `json:"policies"`
}

// FlowPolicies defines cross-node runtime behavior.
type FlowPolicies struct {
	MaxNodeExecutions int `json:"max_node_executions"`
}

// NodeSpec is the engine-level configuration protocol for one node.
type NodeSpec struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	Config        map[string]any `json:"config"`
	InputMappings []FieldBinding `json:"input_mappings"`
	OutputWrites  []ContextWrite `json:"output_writes"`
	Timeout       time.Duration  `json:"timeout"`
	RetryPolicy   RetryPolicy    `json:"retry_policy"`
}

// RetryPolicy is reserved for retry-capable executors. The first core version
// keeps it in the public protocol so product code does not need another schema
// migration later.
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`
}

// EdgeSpec declares a directed connection between nodes.
type EdgeSpec struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FieldSchema describes an input/output/context field in a human-readable and
// machine-validated form. The concrete schema semantics live in core/schema.
type FieldSchema = schema.Field

// ContextSchema is the standard context protocol exposed to workflow authors.
type ContextSchema struct {
	FlowInput   []FieldSchema            `json:"flow_input"`
	FlowOutput  []FieldSchema            `json:"flow_output"`
	Variables   []FieldSchema            `json:"variables"`
	NodeContext map[string][]FieldSchema `json:"node_context"`
}

// ValueSource declares where a mapping value comes from. The concrete mapping
// semantics live in core/expression.
type ValueSource = expression.ValueSource

// ValueSourceType keeps mapping semantics explicit and easy to validate.
type ValueSourceType = expression.SourceType

const (
	SourceContext    ValueSourceType = expression.SourceContext
	SourceConst      ValueSourceType = expression.SourceConst
	SourceNodeOutput ValueSourceType = expression.SourceNodeOutput
)

// FieldBinding maps one node input field from context or a constant.
type FieldBinding = expression.FieldBinding

// ContextWrite writes one value from node output or a constant back to context.
type ContextWrite = expression.ContextWrite

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
