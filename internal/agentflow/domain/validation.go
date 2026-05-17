package domain

import (
	"strings"

	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateGraph(graph FlowGraph) error {
	if graph.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if graph.ID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "flow_id is required")
	}
	if strings.TrimSpace(graph.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "flow name is required")
	}
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
	entryOutgoingCount := 0
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
		if edge.Condition != nil && strings.TrimSpace(edge.Condition.Path) == "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "edge condition path is required")
		}
		if edge.FromNodeID == graph.EntryNodeID {
			entryOutgoingCount++
		}
	}
	if entryOutgoingCount > 1 {
		return apperrors.New(apperrors.CodeInvalidArgument, "entry node can only connect to one next node")
	}
	return nil
}
