package flowengine

import (
	"fmt"
	"reflect"
	"strings"

	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

func ValidateSpec(spec workflow.Spec) error {
	if spec.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if spec.ID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "workflow_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "workflow name is required")
	}
	switch spec.Status {
	case "", workflow.StatusDraft, workflow.StatusEnabled, workflow.StatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported workflow status")
	}
	switch spec.Profile {
	case workflow.ProfileToolWorkflow, workflow.ProfileAgentWorkflow:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported workflow profile")
	}
	if err := ValidateGraph(spec.Graph); err != nil {
		return err
	}
	return validateProfileNodeTypes(spec)
}

func ValidateGraph(graph workflow.Graph) error {
	if graph.EntryNodeID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "entry_node_id is required")
	}
	if len(graph.Nodes) == 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "at least one node is required")
	}
	if _, ok := graph.Nodes[graph.EntryNodeID]; !ok {
		return apperrors.New(apperrors.CodeInvalidArgument, "entry node does not exist")
	}
	for nodeID, node := range graph.Nodes {
		if node.ID.Empty() {
			return apperrors.New(apperrors.CodeInvalidArgument, "node id is required")
		}
		if nodeID != node.ID {
			return apperrors.New(apperrors.CodeInvalidArgument, "node map key must match node id")
		}
		if node.Type == "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "node type is required")
		}
	}
	for _, edge := range graph.Edges {
		if edge.FromNodeID.Empty() || edge.ToNodeID.Empty() {
			return apperrors.New(apperrors.CodeInvalidArgument, "edge endpoints are required")
		}
		if _, ok := graph.Nodes[edge.FromNodeID]; !ok {
			return apperrors.New(apperrors.CodeInvalidArgument, "edge from_node_id does not exist")
		}
		if _, ok := graph.Nodes[edge.ToNodeID]; !ok {
			return apperrors.New(apperrors.CodeInvalidArgument, "edge to_node_id does not exist")
		}
	}
	if hasCycle(graph) {
		return apperrors.New(apperrors.CodeInvalidArgument, "workflow graph must be acyclic")
	}
	return nil
}

func validateProfileNodeTypes(spec workflow.Spec) error {
	allowed := map[workflow.NodeType]bool{
		workflow.NodeTypeStart:     true,
		workflow.NodeTypeEnd:       true,
		workflow.NodeTypeJoin:      true,
		workflow.NodeTypeTransform: true,
		workflow.NodeTypeCondition: true,
	}
	switch spec.Profile {
	case workflow.ProfileToolWorkflow:
		allowed[workflow.NodeTypeConnectorOperation] = true
	case workflow.ProfileAgentWorkflow:
		allowed[workflow.NodeTypeTool] = true
		allowed[workflow.NodeTypeSkill] = true
		allowed[workflow.NodeTypeAgent] = true
	}
	for _, node := range spec.Graph.Nodes {
		if !allowed[node.Type] {
			return apperrors.New(apperrors.CodeInvalidArgument, fmt.Sprintf("node type %q is not allowed in %s", node.Type, spec.Profile))
		}
	}
	return nil
}

func edgeMatches(condition *workflow.EdgeCondition, ctx RunContext) bool {
	if condition == nil {
		return true
	}
	value, exists := ctx.Read(condition.Path)
	if condition.Exists != nil {
		return exists == *condition.Exists
	}
	if !exists {
		return false
	}
	if condition.Equals == nil {
		return truthy(value)
	}
	if reflect.DeepEqual(value, condition.Equals) {
		return true
	}
	return fmt.Sprint(value) == fmt.Sprint(condition.Equals)
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

func hasCycle(graph workflow.Graph) bool {
	visiting := map[id.ID]bool{}
	visited := map[id.ID]bool{}
	var visit func(nodeID id.ID) bool
	visit = func(nodeID id.ID) bool {
		if visiting[nodeID] {
			return true
		}
		if visited[nodeID] {
			return false
		}
		visiting[nodeID] = true
		for _, edge := range graph.Edges {
			if edge.FromNodeID == nodeID && visit(edge.ToNodeID) {
				return true
			}
		}
		visiting[nodeID] = false
		visited[nodeID] = true
		return false
	}
	for nodeID := range graph.Nodes {
		if visit(nodeID) {
			return true
		}
	}
	return false
}
