package domain

import (
	"fmt"
	"strings"

	flowdomain "flow-anything/internal/agentflow/domain"
	"flow-anything/internal/platform/contracts/agentflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

func ValidateAgentFlow(spec agentflow.Spec) error {
	if spec.TenantID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "tenant_id is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "agent flow name is required")
	}
	switch spec.Status {
	case "", agentflow.StatusDraft, agentflow.StatusEnabled, agentflow.StatusDisabled:
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent flow status")
	}
	switch spec.OrchestrationMode {
	case "", agentflow.OrchestrationModeWorkflow:
	case agentflow.OrchestrationModeSupervisor:
		if err := validateExistingAgentNodesAreLeaves(spec.Graph); err != nil {
			return err
		}
		if spec.Status == agentflow.StatusEnabled {
			if err := validateSupervisorSpec(spec.Supervisor); err != nil {
				return err
			}
		}
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent flow orchestration mode")
	}
	if err := flowdomain.ValidateGraph(spec.Graph); err != nil {
		return err
	}
	return nil
}

func validateExistingAgentNodesAreLeaves(graph flowdomain.FlowGraph) error {
	outgoingCounts := map[string]int{}
	for _, edge := range graph.Edges {
		outgoingCounts[edge.FromNodeID.String()]++
	}
	for _, node := range graph.Nodes {
		if !isExistingAgentFlowNode(node) || outgoingCounts[node.ID.String()] == 0 {
			continue
		}
		nodeName := strings.TrimSpace(node.Name)
		if nodeName == "" {
			nodeName = node.ID.String()
		}
		return apperrors.New(
			apperrors.CodeInvalidArgument,
			fmt.Sprintf("existing agent node %q must be a leaf node; use a local agent node when it needs sub-agents", nodeName),
		)
	}
	return nil
}

func isExistingAgentFlowNode(node flowdomain.Node) bool {
	if node.Type != flowdomain.NodeTypeAgent && node.Type != flowdomain.NodeTypeSupervisor {
		return false
	}
	mode := stringConfig(node.Config, "agent_mode")
	if mode == "existing" {
		return true
	}
	_, hasLocalAgent := node.Config["local_agent"]
	return mode == "" && stringConfig(node.Config, "agent_id") != "" && !hasLocalAgent
}

func stringConfig(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func validateSupervisorSpec(spec agentflow.SupervisorSpec) error {
	if spec.SupervisorAgentID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "supervisor agent is required")
	}
	if spec.MaxDepth < 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "max_depth cannot be negative")
	}
	if spec.MaxDepth > 16 {
		return apperrors.New(apperrors.CodeInvalidArgument, "agent graph max_depth exceeds platform limit")
	}
	seen := map[string]struct{}{}
	for _, subAgentID := range spec.SubAgentIDs {
		if subAgentID.Empty() {
			return apperrors.New(apperrors.CodeInvalidArgument, "sub-agent id cannot be empty")
		}
		if subAgentID == spec.SupervisorAgentID {
			return apperrors.New(apperrors.CodeInvalidArgument, "supervisor agent cannot be bound as its own sub-agent")
		}
		if _, ok := seen[subAgentID.String()]; ok {
			return apperrors.New(apperrors.CodeInvalidArgument, "duplicate sub-agent id")
		}
		seen[subAgentID.String()] = struct{}{}
	}
	return nil
}
