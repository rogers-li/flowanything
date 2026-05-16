package flowengine

import "time"

// InstanceStatus is the lifecycle status of a stateful flow instance.
type InstanceStatus string

const (
	InstanceCreated   InstanceStatus = "created"
	InstanceRunning   InstanceStatus = "running"
	InstanceWaiting   InstanceStatus = "waiting"
	InstanceCompleted InstanceStatus = "completed"
	InstanceFailed    InstanceStatus = "failed"
	InstanceCancelled InstanceStatus = "cancelled"
)

// TokenStatus is the lifecycle status of one execution token.
type TokenStatus string

const (
	TokenReady     TokenStatus = "ready"
	TokenRunning   TokenStatus = "running"
	TokenConsumed  TokenStatus = "consumed"
	TokenWaiting   TokenStatus = "waiting"
	TokenCancelled TokenStatus = "cancelled"
)

// NodeStatus is the lifecycle status of a node within one flow instance.
type NodeStatus string

const (
	NodePending   NodeStatus = "pending"
	NodeRunning   NodeStatus = "running"
	NodeWaiting   NodeStatus = "waiting"
	NodeCompleted NodeStatus = "completed"
	NodeFailed    NodeStatus = "failed"
	NodeSkipped   NodeStatus = "skipped"
)

// FlowInstance is the durable state of one flow execution.
type FlowInstance struct {
	InstanceID  string
	FlowID      string
	FlowVersion string
	Status      InstanceStatus
	Context     *DataContext
	Tokens      []ExecutionToken
	NodeStates  map[string]NodeState
	JoinStates  map[string]JoinState
	LastError   string
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ExecutionToken represents execution permission currently located at a node.
type ExecutionToken struct {
	TokenID      string
	NodeID       string
	SourceNodeID string
	Status       TokenStatus
	Payload      map[string]any
	WaitingFor   []WaitCondition
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NodeState stores the latest runtime state of a node in an instance.
type NodeState struct {
	NodeID     string
	Status     NodeStatus
	Attempts   int
	Input      map[string]any
	Output     map[string]any
	WaitingFor []WaitCondition
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// WaitCondition describes an external event needed to continue execution.
type WaitCondition struct {
	Type      string
	EventKey  string
	TimeoutAt time.Time
}

// ExternalEvent resumes one or more waiting tokens.
type ExternalEvent struct {
	Type    string
	Key     string
	Payload map[string]any
	At      time.Time
}

// JoinState stores aggregation progress for a join gateway.
type JoinState struct {
	NodeID        string
	ExpectedNodes []string
	ArrivedNodes  map[string]bool
	Completed     bool
}

// FlowInstanceEvent is an append-only stateful execution event for audit and
// replay stores.
type FlowInstanceEvent struct {
	EventID    string
	InstanceID string
	Type       string
	NodeID     string
	TokenID    string
	Data       map[string]any
	Error      string
	Timestamp  time.Time
}
