package application

import (
	"context"
	"fmt"
	"strings"

	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

func (s *Service) validateWorkflowForPublish(ctx context.Context, spec workflow.Spec) error {
	issues := make([]string, 0)
	issues = append(issues, validateWorkflowReachability(spec)...)
	issues = append(issues, s.validateWorkflowNodeBindings(ctx, spec)...)
	if len(issues) > 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "workflow publish validation failed: "+strings.Join(issues, "; "))
	}
	return nil
}

func validateWorkflowReachability(spec workflow.Spec) []string {
	issues := make([]string, 0)
	outgoing := map[id.ID][]workflow.Edge{}
	for _, edge := range spec.Graph.Edges {
		outgoing[edge.FromNodeID] = append(outgoing[edge.FromNodeID], edge)
	}
	if len(outgoing[spec.Graph.EntryNodeID]) > 1 && spec.Graph.EntryNodeID == id.ID("start") {
		issues = append(issues, "start node can only have one outgoing edge")
	}

	reachable := map[id.ID]bool{}
	var visit func(nodeID id.ID)
	visit = func(nodeID id.ID) {
		if nodeID.Empty() || reachable[nodeID] {
			return
		}
		if _, ok := spec.Graph.Nodes[nodeID]; !ok {
			return
		}
		reachable[nodeID] = true
		for _, edge := range outgoing[nodeID] {
			visit(edge.ToNodeID)
		}
	}
	visit(spec.Graph.EntryNodeID)
	for nodeID, node := range spec.Graph.Nodes {
		if !reachable[nodeID] {
			issues = append(issues, fmt.Sprintf("node %q is not reachable from entry", workflowNodeName(node)))
		}
		if node.Type == workflow.NodeTypeCondition {
			for _, target := range conditionBranchTargets(node.Config) {
				if _, ok := spec.Graph.Nodes[target]; !ok {
					issues = append(issues, fmt.Sprintf("condition node %q points to missing next node %q", workflowNodeName(node), target.String()))
				}
			}
		}
	}
	return issues
}

func (s *Service) validateWorkflowNodeBindings(ctx context.Context, spec workflow.Spec) []string {
	issues := make([]string, 0)
	for _, node := range spec.Graph.Nodes {
		switch node.Type {
		case workflow.NodeTypeConnectorOperation:
			operationID := configID(node.Config, "connector_operation_id", "operation_id")
			if operationID.Empty() {
				issues = append(issues, fmt.Sprintf("connector node %q must bind a connector operation", workflowNodeName(node)))
				continue
			}
			if s.operations == nil {
				issues = append(issues, "connector operation repository is not configured")
				continue
			}
			operation, err := s.operations.GetConnectorOperation(ctx, spec.TenantID, operationID)
			if err != nil {
				issues = append(issues, fmt.Sprintf("connector node %q references missing operation %q", workflowNodeName(node), operationID.String()))
				continue
			}
			if operation.Status != connector.OperationStatusEnabled {
				issues = append(issues, fmt.Sprintf("connector operation %q is not enabled", operation.Name))
			}
		case workflow.NodeTypeTool:
			toolID := configID(node.Config, "tool_id")
			if toolID.Empty() {
				issues = append(issues, fmt.Sprintf("tool node %q must bind a tool", workflowNodeName(node)))
				continue
			}
			if s.tools == nil {
				issues = append(issues, "tool repository is not configured")
				continue
			}
			specTool, err := s.tools.GetTool(ctx, spec.TenantID, toolID)
			if err != nil {
				issues = append(issues, fmt.Sprintf("tool node %q references missing tool %q", workflowNodeName(node), toolID.String()))
				continue
			}
			if specTool.Status != tool.StatusEnabled {
				issues = append(issues, fmt.Sprintf("tool %q is not enabled", specTool.Name))
			}
		case workflow.NodeTypeSkill:
			skillID := configID(node.Config, "skill_id")
			if skillID.Empty() {
				issues = append(issues, fmt.Sprintf("skill node %q must bind a skill", workflowNodeName(node)))
				continue
			}
			if s.skills == nil {
				issues = append(issues, "skill repository is not configured")
				continue
			}
			specSkill, err := s.skills.GetSkill(ctx, spec.TenantID, skillID)
			if err != nil {
				issues = append(issues, fmt.Sprintf("skill node %q references missing skill %q", workflowNodeName(node), skillID.String()))
				continue
			}
			if specSkill.Status != skill.StatusEnabled {
				issues = append(issues, fmt.Sprintf("skill %q is not enabled", specSkill.Name))
			}
		case workflow.NodeTypeAgent:
			agentID := configID(node.Config, "agent_id")
			if agentID.Empty() {
				issues = append(issues, fmt.Sprintf("agent node %q must bind an agent", workflowNodeName(node)))
				continue
			}
			if s.agents == nil {
				issues = append(issues, "agent repository is not configured")
				continue
			}
			specAgent, err := s.agents.GetAgent(ctx, spec.TenantID, agentID)
			if err != nil {
				issues = append(issues, fmt.Sprintf("agent node %q references missing agent %q", workflowNodeName(node), agentID.String()))
				continue
			}
			if specAgent.Status != agent.StatusEnabled {
				issues = append(issues, fmt.Sprintf("agent %q is not enabled", specAgent.Name))
			}
		case workflow.NodeTypeTransform:
			if configString(node.Config, "function_id") == "" {
				issues = append(issues, fmt.Sprintf("transform node %q must bind a transform function", workflowNodeName(node)))
			}
		}
	}
	return issues
}

func workflowNodeName(node workflow.Node) string {
	if strings.TrimSpace(node.Name) != "" {
		return node.Name
	}
	return node.ID.String()
}

func configID(config map[string]any, keys ...string) id.ID {
	for _, key := range keys {
		value := configString(config, key)
		if value != "" {
			return id.ID(value)
		}
	}
	return ""
}

func conditionBranchTargets(config map[string]any) []id.ID {
	targets := make([]id.ID, 0)
	if branches, ok := config["branches"].([]any); ok {
		for _, raw := range branches {
			record, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if target := configString(record, "next_node_id"); target != "" {
				targets = append(targets, id.ID(target))
			}
		}
	}
	if defaultBranch, ok := config["default_branch"].(map[string]any); ok {
		if target := configString(defaultBranch, "next_node_id"); target != "" {
			targets = append(targets, id.ID(target))
		}
	}
	return targets
}
