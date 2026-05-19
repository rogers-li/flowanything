package app

import (
	"context"
	"encoding/json"
	"fmt"

	"flow-anything/core/agentcore"
	coreconfig "flow-anything/core/config"
	"flow-anything/core/flowengine"
	"flow-anything/core/runtimecontext"
)

type AgentGraphRequest struct {
	AgentFlowID  string
	Input        map[string]any
	TraceContext runtimecontext.TraceContext
}

type AgentGraphResult struct {
	InstanceID string
	Status     string
	Output     map[string]any
	Text       string
	RootNodeID string
	Raw        any
}

func (h *Host) RunAgentGraph(ctx context.Context, req AgentGraphRequest) (AgentGraphResult, error) {
	workflowConfig, ok := h.agentFlowWorkflowConfig(req.AgentFlowID)
	if !ok {
		return AgentGraphResult{}, fmt.Errorf("agent graph %q not found", req.AgentFlowID)
	}
	if orchestrationMode(workflowConfig) != "supervisor" {
		return AgentGraphResult{}, fmt.Errorf("agent flow %q is not an agent graph", req.AgentFlowID)
	}
	graph, err := h.agentGraphSpecFromWorkflow(workflowConfig)
	if err != nil {
		return AgentGraphResult{}, err
	}
	userMessage := stringValue(req.Input, "user_request")
	result, err := h.agentGraphRunner.Run(ctx, agentcore.AgentGraphRunRequest{
		Graph:        graph,
		UserMessage:  userMessage,
		Input:        req.Input,
		TraceID:      req.TraceContext.TraceID,
		TraceContext: req.TraceContext,
	})
	if err != nil {
		return AgentGraphResult{}, err
	}
	return AgentGraphResult{
		InstanceID: "agent_graph_run_" + graph.ID,
		Status:     "succeeded",
		Output:     result.Output,
		Text:       result.Text,
		RootNodeID: result.RootNodeID,
		Raw:        result.Root.Raw,
	}, nil
}

func (h *Host) agentFlowWorkflowConfig(id string) (coreconfig.WorkflowConfig, bool) {
	for _, workflowConfig := range h.catalog.Bundle.Resources.Workflows {
		if workflowConfig.ID == id {
			return workflowConfig, true
		}
	}
	return coreconfig.WorkflowConfig{}, false
}

func (h *Host) agentGraphSpecFromWorkflow(config coreconfig.WorkflowConfig) (agentcore.AgentGraphSpec, error) {
	nodes := make([]agentcore.AgentGraphNode, 0, len(config.Spec.Nodes))
	for _, node := range config.Spec.Nodes {
		nodeType := agentGraphNodeType(config, node)
		graphNode := agentcore.AgentGraphNode{
			ID:          node.ID,
			Type:        nodeType,
			Name:        node.Name,
			Description: nodeDescription(config.UI, node.ID),
		}
		if nodeType != "start" && nodeType != "end" {
			agent, err := h.agentSpecFromGraphNode(node)
			if err != nil {
				return agentcore.AgentGraphSpec{}, fmt.Errorf("compile agent graph node %q: %w", node.ID, err)
			}
			agent = normalizeAgentGraphAgentSpec(agent)
			graphNode.Agent = agent
		}
		nodes = append(nodes, graphNode)
	}
	edges := make([]agentcore.AgentGraphEdge, 0, len(config.Spec.Edges))
	for _, edge := range config.Spec.Edges {
		edges = append(edges, agentcore.AgentGraphEdge{From: edge.From, To: edge.To})
	}
	return agentcore.AgentGraphSpec{
		ID:          config.ID,
		Name:        config.Name,
		Description: config.Description,
		EntryNodeID: agentGraphEntryNodeID(config),
		Nodes:       nodes,
		Edges:       edges,
		Policy: agentcore.AgentGraphPolicy{
			MaxDepth: intFromUI(config.UI, "supervisor", "maxDepth"),
		},
	}, nil
}

func (h *Host) agentSpecFromGraphNode(node flowengine.NodeSpec) (agentcore.AgentSpec, error) {
	if rawAgent, ok := node.Config["agent"]; ok && rawAgent != nil {
		var agent agentcore.AgentSpec
		if err := decodeMapValue(rawAgent, &agent); err != nil {
			return agentcore.AgentSpec{}, err
		}
		if agent.ID != "" {
			return agent, nil
		}
	}
	if agentID := stringValue(node.Config, "agent_id"); agentID != "" {
		if agent, ok := h.catalog.Agents[agentID]; ok {
			return agent, nil
		}
		return agentcore.AgentSpec{}, fmt.Errorf("agent %q not found", agentID)
	}
	return agentcore.AgentSpec{}, fmt.Errorf("agent config is required")
}

func normalizeAgentGraphAgentSpec(agent agentcore.AgentSpec) agentcore.AgentSpec {
	// Agent Graph executes agents recursively. ReAct's observe-and-replan loop can
	// multiply model calls at every graph level, so graph nodes default to ReWOO:
	// one plan, one bounded execution pass, and one final solve.
	if agent.ReasoningMode == "" || agent.ReasoningMode == "react" {
		agent.ReasoningMode = agentcore.ReWOOStrategy{}.Name()
	}
	if agent.Policy.MaxIterations <= 0 || agent.Policy.MaxIterations > 1 {
		agent.Policy.MaxIterations = 1
	}
	return agent
}

func orchestrationMode(config coreconfig.WorkflowConfig) string {
	if value := stringValue(config.UI, "orchestration_mode"); value != "" {
		return value
	}
	return "workflow"
}

func agentGraphEntryNodeID(config coreconfig.WorkflowConfig) string {
	if value := stringValue(config.UI, "entry_node_id"); value != "" {
		return value
	}
	return "start"
}

func agentGraphNodeType(config coreconfig.WorkflowConfig, node flowengine.NodeSpec) string {
	if metadata := nodeUIMetadata(config.UI, node.ID); metadata != nil {
		if value := stringValue(metadata, "type"); value != "" {
			return value
		}
	}
	switch node.Type {
	case "control.start":
		return "start"
	case "control.end":
		return "end"
	case "workflow.agent":
		return "agent_node"
	default:
		return node.Type
	}
}

func nodeDescription(ui map[string]any, nodeID string) string {
	if metadata := nodeUIMetadata(ui, nodeID); metadata != nil {
		return stringValue(metadata, "description")
	}
	return ""
}

func nodeUIMetadata(ui map[string]any, nodeID string) map[string]any {
	nodesValue, ok := ui["nodes"]
	if !ok {
		return nil
	}
	nodes, ok := nodesValue.(map[string]any)
	if !ok {
		return nil
	}
	value, ok := nodes[nodeID]
	if !ok {
		return nil
	}
	metadata, _ := value.(map[string]any)
	return metadata
}

func intFromUI(ui map[string]any, section string, key string) int {
	value, ok := ui[section]
	if !ok {
		return 0
	}
	record, ok := value.(map[string]any)
	if !ok {
		return 0
	}
	switch number := record[key].(type) {
	case int:
		return number
	case float64:
		return int(number)
	default:
		return 0
	}
}

func decodeMapValue(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func stringValue(record map[string]any, key string) string {
	if record == nil {
		return ""
	}
	value, ok := record[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}
