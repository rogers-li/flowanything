package domain

import (
	"fmt"
	"reflect"
	"strings"

	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type FlowStatus string

const (
	FlowStatusDraft    FlowStatus = "draft"
	FlowStatusEnabled  FlowStatus = "enabled"
	FlowStatusDisabled FlowStatus = "disabled"
)

type NodeType string

const (
	NodeTypeStart      NodeType = "start"
	NodeTypeSupervisor NodeType = "supervisor_node"
	NodeTypeAgent      NodeType = "agent_node"
	NodeTypePlanner    NodeType = "planner_node"
	NodeTypeRouter     NodeType = "router_node"
	NodeTypeAggregator NodeType = "aggregator_node"
	NodeTypeVerifier   NodeType = "verifier_node"
	NodeTypeJoin       NodeType = "join_node"
	NodeTypeEnd        NodeType = "end"

	NodeTypeConnectorOperation NodeType = "connector_operation"
	NodeTypeTool               NodeType = "tool"
	NodeTypeSkill              NodeType = "skill"
	NodeTypeWorkflowAgent      NodeType = "agent"
	NodeTypeTransform          NodeType = "transform"
	NodeTypeCondition          NodeType = "condition"
	NodeTypeWorkflowJoin       NodeType = "join"
)

type EdgeType string

const (
	EdgeTypeDefault     EdgeType = "default"
	EdgeTypeConditional EdgeType = "conditional"
	EdgeTypeFallback    EdgeType = "fallback"
)

type FlowGraph struct {
	ID          id.ID           `json:"id"`
	TenantID    tenant.ID       `json:"tenant_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Status      FlowStatus      `json:"status"`
	Version     string          `json:"version,omitempty"`
	EntryNodeID id.ID           `json:"entry_node_id"`
	Nodes       map[id.ID]Node  `json:"nodes"`
	Edges       []Edge          `json:"edges,omitempty"`
	Policy      ExecutionPolicy `json:"policy,omitempty"`
}

type Node struct {
	ID            id.ID          `json:"id"`
	Type          NodeType       `json:"type"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	TimeoutMillis int            `json:"timeout_ms,omitempty"`
	RetryPolicy   RetryPolicy    `json:"retry_policy,omitempty"`
}

type Edge struct {
	ID          id.ID          `json:"id"`
	FromNodeID  id.ID          `json:"from_node_id"`
	ToNodeID    id.ID          `json:"to_node_id"`
	Type        EdgeType       `json:"type"`
	Condition   *EdgeCondition `json:"condition,omitempty"`
	Description string         `json:"description,omitempty"`
}

type EdgeCondition struct {
	Path   string `json:"path"`
	Equals any    `json:"equals,omitempty"`
	Exists *bool  `json:"exists,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts   int `json:"max_attempts,omitempty"`
	BackoffMillis int `json:"backoff_ms,omitempty"`
}

type ExecutionPolicy struct {
	MaxSteps       int `json:"max_steps,omitempty"`
	MaxParallelism int `json:"max_parallelism,omitempty"`
	TimeoutMillis  int `json:"timeout_ms,omitempty"`
}

func (c EdgeCondition) Matches(ctx RunContext) bool {
	value, exists := ctx.ValueAtPath(c.Path)
	if c.Exists != nil {
		return exists == *c.Exists
	}
	if !exists {
		return false
	}
	if c.Equals == nil {
		return truthy(value)
	}
	if reflect.DeepEqual(value, c.Equals) {
		return true
	}
	return fmt.Sprint(value) == fmt.Sprint(c.Equals)
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.TrimSpace(typed) != ""
	case nil:
		return false
	default:
		return true
	}
}
