package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	orchestration "flow-anything/internal/agentorchestration"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

const (
	defaultSupervisorMaxSubAgentCalls = 5
	defaultAgentGraphMaxDepth         = 4
	hardAgentGraphMaxDepth            = 16
	agentGraphLiveTraceIDPayloadKey   = "live_trace_id"
)

type SupervisorRunner struct {
	logger       *slog.Logger
	store        ports.RunStore
	invoker      ports.AgentInvoker
	catalog      ports.AgentCatalog
	capabilities ports.AgentCapabilityCatalog
	tracer       ports.TraceEmitter
}

func NewSupervisorRunner(logger *slog.Logger, store ports.RunStore, invoker ports.AgentInvoker, catalog ports.AgentCatalog, tracer ports.TraceEmitter) *SupervisorRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &SupervisorRunner{
		logger:  logger,
		store:   store,
		invoker: invoker,
		catalog: catalog,
		tracer:  tracer,
	}
}

func (r *SupervisorRunner) WithAgentCapabilityCatalog(catalog ports.AgentCapabilityCatalog) *SupervisorRunner {
	r.capabilities = catalog
	return r
}

// Execute runs Agent Flow's recursive Agent Graph mode. Each Agent node uses the
// shared orchestration action model to plan tool, skill, and child-agent actions,
// execute the selected actions, and synthesize a node result.
func (r *SupervisorRunner) Execute(ctx context.Context, spec agentflow.Spec, input map[string]any) (domain.FlowRun, error) {
	if r.store == nil {
		return domain.FlowRun{}, apperrors.New(apperrors.CodeUnavailable, "agent flow run store is not configured")
	}
	if r.invoker == nil {
		return domain.FlowRun{}, apperrors.New(apperrors.CodeUnavailable, "agent invoker is not configured")
	}
	if r.catalog == nil {
		return domain.FlowRun{}, apperrors.New(apperrors.CodeUnavailable, "agent catalog is not configured")
	}
	if input == nil {
		input = map[string]any{}
	}
	if err := validateRuntimeAgentGraph(spec.Graph); err != nil {
		return domain.FlowRun{}, err
	}
	config := normalizedSupervisorConfig(spec)
	if err := validateRuntimeSupervisorConfig(config); err != nil {
		return domain.FlowRun{}, err
	}

	run := domain.FlowRun{
		ID:          id.New("flowrun"),
		TenantID:    spec.TenantID,
		FlowID:      spec.ID,
		FlowVersion: spec.Version,
		Status:      domain.RunStatusRunning,
		Input:       cloneStringMap(input),
		StartedAt:   time.Now().UTC(),
	}
	if err := r.store.CreateRun(ctx, run); err != nil {
		return domain.FlowRun{}, err
	}
	if r.tracer != nil {
		r.tracer.FlowRunStarted(ctx, run)
	}

	output, err := r.executeAgentGraph(ctx, run, spec, config, input)
	now := time.Now().UTC()
	run.FinishedAt = &now
	if err != nil {
		run.Status = domain.RunStatusFailed
		run.Error = err.Error()
	} else {
		run.Status = domain.RunStatusSucceeded
		run.Output = output
	}
	if updateErr := r.store.UpdateRun(ctx, run); updateErr != nil && err == nil {
		err = updateErr
	}
	if r.tracer != nil {
		r.tracer.FlowRunFinished(ctx, run)
	}
	return run, err
}

// executeAgentGraph runs the Supervisor mode as a recursive Agent Graph. In this
// mode graph edges mean "this child Agent is callable by the parent Agent", not
// traditional workflow dataflow sequencing.
func (r *SupervisorRunner) executeAgentGraph(ctx context.Context, run domain.FlowRun, spec agentflow.Spec, config agentflow.SupervisorSpec, input map[string]any) (map[string]any, error) {
	graph, err := buildRuntimeAgentGraph(spec.Graph)
	if err != nil {
		return nil, err
	}
	task := taskFromInput(input)
	sessionID := sessionIDFromInput(input)
	userID := stringFromInput(input, "user_id", "agent_flow")

	rootNode := spec.Graph.Nodes[graph.RootNodeID]
	rootResult, err := r.executeAgentGraphNode(ctx, run, spec, config, graph, rootNode, task, sessionID, userID, 0, map[id.ID]struct{}{})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"return_message":      rootResult.Text,
		"text":                rootResult.Text,
		"root_node_id":        rootResult.NodeID,
		"supervisor_agent_id": rootResult.AgentID,
		"supervisor_trace_id": rootResult.TraceID,
		"agent_graph_result":  rootResult,
		"sub_agent_results":   rootResult.ChildResults,
		"mode":                "agent_graph",
	}, nil
}

// executeAgentGraphNode executes one Agent node with the unified Action model:
// tools, skills, and child agents are all presented as callable actions. The
// node first plans an action list, the runtime executes those actions, and the
// node then synthesizes a response from the observations.
func (r *SupervisorRunner) executeAgentGraphNode(
	ctx context.Context,
	run domain.FlowRun,
	spec agentflow.Spec,
	config agentflow.SupervisorSpec,
	graph runtimeAgentGraph,
	node domain.Node,
	task string,
	sessionID id.ID,
	userID string,
	depth int,
	path map[id.ID]struct{},
) (subAgentResult, error) {
	if depth > config.MaxDepth {
		return subAgentResult{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph exceeded max depth")
	}
	if _, exists := path[node.ID]; exists {
		return subAgentResult{}, apperrors.New(apperrors.CodeConflict, "agent graph contains a recursive cycle")
	}
	path[node.ID] = struct{}{}
	defer delete(path, node.ID)

	capabilityConfig, err := r.loadAgentCapabilityConfig(ctx, run.TenantID, agentIDFromRuntimeNode(node), "agent graph node")
	if err != nil {
		return subAgentResult{}, err
	}
	profile := capabilityConfig.Agent
	children := graph.ChildrenByNodeID[node.ID]
	if len(children) > 0 && depth >= config.MaxDepth {
		return subAgentResult{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph exceeded max depth before executing child agents")
	}
	childOptions, err := r.loadAgentGraphChildren(ctx, run.TenantID, children)
	if err != nil {
		return subAgentResult{}, err
	}
	actionRegistry := orchestration.ActionRegistry{
		Agents:         agentActionSpecsFromChildren(childOptions),
		Skills:         capabilityConfig.Skills,
		Tools:          capabilityConfig.Tools,
		AuthoredPrompt: profile.SystemPrompt,
	}
	planningSystemPrompt := orchestration.BuildActionPlanningSystemPrompt(actionRegistry, config.PlanningPrompt)
	planResult, err := r.invokeAgentStep(ctx, run, domain.Node{
		ID:          agentGraphStepNodeID(node.ID, "planning"),
		Type:        domain.NodeTypeSupervisor,
		Name:        node.Name + " action planning",
		Description: "Plan bounded tool, skill, and agent actions.",
	}, profile, orchestration.BuildActionPlanningTask(task), planningSystemPrompt, sessionID, userID, map[string]any{
		"phase":                         "action_planning",
		"node_id":                       node.ID.String(),
		"agent_id":                      profile.ID.String(),
		"depth":                         depth,
		"available_tool_count":          len(actionRegistry.Tools),
		"available_skill_count":         len(actionRegistry.Skills),
		"child_agent_count":             len(actionRegistry.Agents),
		"agent_flow_id":                 spec.ID.String(),
		agentGraphLiveTraceIDPayloadKey: liveTraceIDFromNode(node),
		orchestration.RuntimeDisableToolsPayloadKey: true,
	})
	if err != nil {
		return subAgentResult{}, err
	}

	plan, err := orchestration.ParseActionPlan(agentResponseText(planResult))
	if err != nil {
		return subAgentResult{}, err
	}
	plan = orchestration.FilterActionPlan(plan, actionRegistry, config.MaxSubAgentCalls)
	if len(plan.Actions) == 0 {
		text := strings.TrimSpace(plan.FinalAnswerIfNoAction)
		if text == "" {
			return subAgentResult{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph node returned no actions or final answer")
		}
		return subAgentResult{
			NodeID:       node.ID.String(),
			AgentID:      profile.ID.String(),
			Name:         profile.Name,
			Task:         task,
			Text:         text,
			TraceID:      planResult.TraceID,
			Depth:        depth,
			Plan:         plan,
			Observations: []orchestration.ActionObservation{},
			ChildResults: []subAgentResult{},
		}, nil
	}

	actionResults := make([]orchestration.ActionObservation, 0, len(plan.Actions))
	subResults := make([]subAgentResult, 0)
	for index, action := range plan.Actions {
		result, err := r.executeAgentGraphAction(ctx, run, spec, config, graph, node, profile, childOptions, capabilityConfig, action, task, sessionID, userID, depth, index, path)
		if err != nil {
			return subAgentResult{}, err
		}
		actionResults = append(actionResults, result)
		if agentResult, ok := result.AgentResult.(subAgentResult); ok {
			subResults = append(subResults, agentResult)
		}
	}

	finalSystemPrompt := orchestration.BuildActionFinalSystemPrompt(config.FinalPrompt)
	finalTask := orchestration.BuildActionFinalTask(task, plan, actionResults)
	finalResult, err := r.invokeAgentStep(ctx, run, domain.Node{
		ID:          agentGraphStepNodeID(node.ID, "final"),
		Type:        domain.NodeTypeSupervisor,
		Name:        node.Name + " final answer",
		Description: "Synthesize action observations.",
	}, profile, finalTask, finalSystemPrompt, sessionID, userID, map[string]any{
		"phase":                         "final_answer",
		"node_id":                       node.ID.String(),
		"agent_id":                      profile.ID.String(),
		"depth":                         depth,
		"agent_flow_id":                 spec.ID.String(),
		"agent_flow_run_id":             run.ID.String(),
		"action_count":                  len(actionResults),
		"sub_agent_call_count":          len(subResults),
		agentGraphLiveTraceIDPayloadKey: liveTraceIDFromNode(node),
		orchestration.RuntimeDisableToolsPayloadKey: true,
	})
	if err != nil {
		return subAgentResult{}, err
	}

	return subAgentResult{
		NodeID:       node.ID.String(),
		AgentID:      profile.ID.String(),
		Name:         profile.Name,
		Task:         task,
		Text:         agentResponseText(finalResult),
		TraceID:      finalResult.TraceID,
		Depth:        depth,
		Plan:         plan,
		Observations: actionResults,
		ChildResults: subResults,
	}, nil
}

func (r *SupervisorRunner) loadAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID, label string) (agent.Profile, error) {
	profile, err := r.catalog.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return agent.Profile{}, err
	}
	if !profile.RuntimeEnabled() {
		return agent.Profile{}, apperrors.New(apperrors.CodeForbidden, label+" is not enabled")
	}
	return profile, nil
}

func (r *SupervisorRunner) loadAgentCapabilityConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID, label string) (ports.AgentCapabilityConfig, error) {
	if r.capabilities != nil {
		config, err := r.capabilities.LoadAgentCapabilityConfig(ctx, tenantID, agentID)
		if err != nil {
			return ports.AgentCapabilityConfig{}, err
		}
		return config, nil
	}
	profile, err := r.loadAgent(ctx, tenantID, agentID, label)
	if err != nil {
		return ports.AgentCapabilityConfig{}, err
	}
	return ports.AgentCapabilityConfig{Agent: profile}, nil
}

func (r *SupervisorRunner) executeAgentGraphAction(
	ctx context.Context,
	run domain.FlowRun,
	spec agentflow.Spec,
	config agentflow.SupervisorSpec,
	graph runtimeAgentGraph,
	parentNode domain.Node,
	profile agent.Profile,
	children []agentGraphChild,
	capabilityConfig ports.AgentCapabilityConfig,
	action orchestration.Action,
	parentTask string,
	sessionID id.ID,
	userID string,
	depth int,
	index int,
	path map[id.ID]struct{},
) (orchestration.ActionObservation, error) {
	switch action.Type {
	case orchestration.ActionKindAgent:
		child, ok := childByNodeID(children, action.NodeID)
		if !ok {
			return orchestration.ActionObservation{}, apperrors.New(apperrors.CodeInvalidArgument, "agent action selected an unknown child node")
		}
		task := strings.TrimSpace(action.Task)
		if task == "" {
			task = parentTask
		}
		result, err := r.executeAgentGraphNode(ctx, run, spec, config, graph, child.Node, task, sessionID, userID, depth+1, path)
		if err != nil {
			return orchestration.ActionObservation{}, err
		}
		result.Reason = action.Reason
		result.CallIndex = index + 1
		return orchestration.ActionObservation{
			Type:        orchestration.ActionKindAgent,
			ID:          child.Profile.ID.String(),
			Name:        child.Profile.Name,
			Task:        task,
			Input:       cloneStringMap(action.Input),
			Reason:      action.Reason,
			Text:        result.Text,
			TraceID:     result.TraceID,
			Success:     true,
			AgentResult: result,
		}, nil
	case orchestration.ActionKindTool:
		toolSpec, ok := orchestration.FindToolAction(capabilityConfig.Tools, action)
		if !ok {
			return orchestration.ActionObservation{}, apperrors.New(apperrors.CodeInvalidArgument, "tool action selected an unknown tool")
		}
		task := strings.TrimSpace(action.Task)
		if task == "" {
			task = "Execute tool action " + toolSpec.Name
		}
		result, err := r.invokeAgentStep(ctx, run, domain.Node{
			ID:          agentGraphStepNodeID(parentNode.ID, fmt.Sprintf("tool_%d", index+1)),
			Type:        domain.NodeTypeAgent,
			Name:        parentNode.Name + " tool " + toolSpec.Name,
			Description: "Execute planned tool action.",
		}, profile, task, "", sessionID, userID, map[string]any{
			"phase":                         "action_execute",
			"action_type":                   string(orchestration.ActionKindTool),
			"tool_id":                       toolSpec.ID.String(),
			"tool_args":                     cloneStringMap(action.Input),
			"node_id":                       parentNode.ID.String(),
			"agent_id":                      profile.ID.String(),
			"depth":                         depth,
			"agent_flow_id":                 spec.ID.String(),
			"agent_flow_run_id":             run.ID.String(),
			"action_index":                  index + 1,
			agentGraphLiveTraceIDPayloadKey: liveTraceIDFromNode(parentNode),
		})
		if err != nil {
			return orchestration.ActionObservation{}, err
		}
		toolResult := toolResultFromActions(agentActions(result))
		return orchestration.ActionObservation{
			Type:       orchestration.ActionKindTool,
			ID:         toolSpec.ID.String(),
			Name:       toolSpec.Name,
			Task:       task,
			Input:      cloneStringMap(action.Input),
			Reason:     action.Reason,
			Text:       agentResponseText(result),
			TraceID:    result.TraceID,
			Success:    toolResult == nil || toolResult.Success,
			ToolResult: toolResult,
		}, nil
	case orchestration.ActionKindSkill:
		skillSpec, ok := orchestration.FindSkillAction(capabilityConfig.Skills, action)
		if !ok {
			return orchestration.ActionObservation{}, apperrors.New(apperrors.CodeInvalidArgument, "skill action selected an unknown skill")
		}
		task := strings.TrimSpace(action.Task)
		if task == "" {
			task = parentTask
		}
		result, err := r.invokeAgentStep(ctx, run, domain.Node{
			ID:          agentGraphStepNodeID(parentNode.ID, fmt.Sprintf("skill_%d", index+1)),
			Type:        domain.NodeTypeAgent,
			Name:        parentNode.Name + " skill " + skillSpec.Name,
			Description: "Execute planned skill action.",
		}, profile, task, orchestration.BuildSkillActionSystemPrompt(skillSpec, action), sessionID, userID, map[string]any{
			"phase":                         "action_execute",
			"action_type":                   string(orchestration.ActionKindSkill),
			"skill_id":                      skillSpec.ID.String(),
			"skill_input":                   cloneStringMap(action.Input),
			"node_id":                       parentNode.ID.String(),
			"agent_id":                      profile.ID.String(),
			"depth":                         depth,
			"agent_flow_id":                 spec.ID.String(),
			"agent_flow_run_id":             run.ID.String(),
			"action_index":                  index + 1,
			agentGraphLiveTraceIDPayloadKey: liveTraceIDFromNode(parentNode),
		})
		if err != nil {
			return orchestration.ActionObservation{}, err
		}
		return orchestration.ActionObservation{
			Type:    orchestration.ActionKindSkill,
			ID:      skillSpec.ID.String(),
			Name:    skillSpec.Name,
			Task:    task,
			Input:   cloneStringMap(action.Input),
			Reason:  action.Reason,
			Text:    agentResponseText(result),
			TraceID: result.TraceID,
			Success: true,
		}, nil
	default:
		return orchestration.ActionObservation{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported agent graph action type")
	}
}

func (r *SupervisorRunner) invokeAgentStep(ctx context.Context, run domain.FlowRun, node domain.Node, profile agent.Profile, task string, runtimeSystemPrompt string, sessionID id.ID, userID string, payload map[string]any) (ports.AgentInvocationResult, error) {
	input := map[string]any{
		"agent_id": profile.ID.String(),
		"task":     task,
		"payload":  payload,
	}
	if strings.TrimSpace(runtimeSystemPrompt) != "" {
		input["runtime_system_prompt"] = runtimeSystemPrompt
	}
	nodeRun := domain.NodeRun{
		ID:        id.New("noderun"),
		TenantID:  run.TenantID,
		RunID:     run.ID,
		FlowID:    run.FlowID,
		NodeID:    node.ID,
		NodeType:  node.Type,
		NodeName:  node.Name,
		Status:    domain.NodeRunStatusRunning,
		Input:     input,
		StartedAt: time.Now().UTC(),
	}
	if err := r.store.RecordNodeRun(ctx, nodeRun); err != nil {
		return ports.AgentInvocationResult{}, err
	}
	if r.tracer != nil {
		r.tracer.NodeRunStarted(ctx, nodeRun)
	}

	result, err := r.invoker.InvokeAgent(ctx, ports.AgentInvocationRequest{
		Run:                 run,
		Node:                node,
		Context:             domain.NewRunContext(map[string]any{"task": task}),
		AgentID:             profile.ID,
		RuntimeSystemPrompt: runtimeSystemPrompt,
		Task:                task,
		Payload:             payload,
		TraceID:             id.New("trace").String(),
		SessionID:           sessionID,
		UserID:              userID,
	})

	now := time.Now().UTC()
	nodeRun.FinishedAt = &now
	if err != nil {
		nodeRun.Status = domain.NodeRunStatusFailed
		nodeRun.Error = err.Error()
	} else {
		nodeRun.Status = domain.NodeRunStatusSucceeded
		nodeRun.Output = map[string]any{
			"agent_id": profile.ID.String(),
			"text":     agentResponseText(result),
			"trace_id": result.TraceID,
			"actions":  agentActions(result),
			"response": result.Response,
		}
	}
	if recordErr := r.store.RecordNodeRun(ctx, nodeRun); recordErr != nil && err == nil {
		err = recordErr
	}
	if r.tracer != nil {
		r.tracer.NodeRunFinished(ctx, nodeRun)
	}
	return result, err
}

func liveTraceIDFromNode(node domain.Node) string {
	if node.Config == nil {
		return ""
	}
	value, _ := node.Config["trace_id"].(string)
	return strings.TrimSpace(value)
}

type subAgentResult struct {
	NodeID       string                            `json:"node_id,omitempty"`
	AgentID      string                            `json:"agent_id"`
	Name         string                            `json:"name"`
	Task         string                            `json:"task"`
	Reason       string                            `json:"reason,omitempty"`
	Text         string                            `json:"text"`
	TraceID      string                            `json:"trace_id,omitempty"`
	Depth        int                               `json:"depth,omitempty"`
	CallIndex    int                               `json:"call_index,omitempty"`
	Plan         orchestration.ActionPlan          `json:"plan,omitempty"`
	Observations []orchestration.ActionObservation `json:"observations,omitempty"`
	ChildResults []subAgentResult                  `json:"child_results,omitempty"`
}

type runtimeAgentGraph struct {
	RootNodeID       id.ID
	ChildrenByNodeID map[id.ID][]domain.Node
}

type agentGraphChild struct {
	Node    domain.Node
	Profile agent.Profile
}

func normalizedSupervisorConfig(spec agentflow.Spec) agentflow.SupervisorSpec {
	config := spec.Supervisor
	if supervisorAgentID, subAgentIDs, ok := supervisorAgentsFromRuntimeGraph(spec.Graph); ok {
		config.SupervisorAgentID = supervisorAgentID
		config.SubAgentIDs = subAgentIDs
	}
	if config.MaxDepth == 0 {
		config.MaxDepth = defaultAgentGraphMaxDepth
	}
	if graphDepth := runtimeAgentGraphDepth(spec.Graph); graphDepth > config.MaxDepth {
		config.MaxDepth = graphDepth
	}
	if config.MaxSubAgentCalls == 0 {
		config.MaxSubAgentCalls = defaultSupervisorMaxSubAgentCalls
	}
	return config
}

func supervisorAgentsFromRuntimeGraph(graph domain.FlowGraph) (id.ID, []id.ID, bool) {
	supervisorNodeID, ok := runtimeSupervisorNodeID(graph)
	if !ok {
		if len(graph.Nodes) > 0 {
			return "", nil, true
		}
		return "", nil, false
	}
	supervisorNode, ok := graph.Nodes[supervisorNodeID]
	if !ok {
		return "", nil, true
	}

	supervisorAgentID := id.ID(strings.TrimSpace(stringConfig(supervisorNode.Config, "agent_id")))
	nodeIDs := make([]string, 0, len(graph.Nodes))
	for nodeID := range graph.Nodes {
		nodeIDs = append(nodeIDs, nodeID.String())
	}
	sort.Strings(nodeIDs)

	seen := map[string]struct{}{}
	subAgentIDs := make([]id.ID, 0)
	for _, nodeID := range nodeIDs {
		node := graph.Nodes[id.ID(nodeID)]
		if node.ID == supervisorNodeID || !isRuntimeAgentNode(node) {
			continue
		}
		agentID := id.ID(strings.TrimSpace(stringConfig(node.Config, "agent_id")))
		if agentID.Empty() || agentID == supervisorAgentID {
			continue
		}
		if _, exists := seen[agentID.String()]; exists {
			continue
		}
		seen[agentID.String()] = struct{}{}
		subAgentIDs = append(subAgentIDs, agentID)
	}
	return supervisorAgentID, subAgentIDs, true
}

func runtimeSupervisorNodeID(graph domain.FlowGraph) (id.ID, bool) {
	entryNodeID := graph.EntryNodeID
	if entryNodeID.Empty() {
		entryNodeID = id.ID("start")
	}
	for _, edge := range graph.Edges {
		if edge.FromNodeID == entryNodeID {
			return edge.ToNodeID, true
		}
	}
	return "", false
}

func isRuntimeAgentNode(node domain.Node) bool {
	return node.Type == domain.NodeTypeAgent || node.Type == domain.NodeTypeSupervisor
}

func validateRuntimeSupervisorConfig(config agentflow.SupervisorSpec) error {
	if config.SupervisorAgentID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "supervisor agent is required")
	}
	if config.MaxDepth < 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "max_depth cannot be negative")
	}
	if config.MaxDepth > hardAgentGraphMaxDepth {
		return apperrors.New(apperrors.CodeInvalidArgument, "agent graph max_depth exceeds platform limit")
	}
	return nil
}

func validateRuntimeAgentGraph(graph domain.FlowGraph) error {
	outgoingCounts := map[id.ID]int{}
	for _, edge := range graph.Edges {
		outgoingCounts[edge.FromNodeID]++
	}
	for _, node := range graph.Nodes {
		if !isExistingRuntimeAgentNode(node) || outgoingCounts[node.ID] == 0 {
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
	if _, err := buildRuntimeAgentGraph(graph); err != nil {
		return err
	}
	return nil
}

func isExistingRuntimeAgentNode(node domain.Node) bool {
	if !isRuntimeAgentNode(node) {
		return false
	}
	mode := strings.TrimSpace(stringConfig(node.Config, "agent_mode"))
	if mode == "existing" {
		return true
	}
	_, hasLocalAgent := node.Config["local_agent"]
	return mode == "" && strings.TrimSpace(stringConfig(node.Config, "agent_id")) != "" && !hasLocalAgent
}

// buildRuntimeAgentGraph compiles the persisted graph into a parent-child map
// used by the recursive runtime. Only agent-to-agent edges are valid here.
func buildRuntimeAgentGraph(graph domain.FlowGraph) (runtimeAgentGraph, error) {
	entryNodeID := graph.EntryNodeID
	if entryNodeID.Empty() {
		entryNodeID = id.ID("start")
	}
	rootNodeID, ok := runtimeSupervisorNodeID(graph)
	if !ok {
		return runtimeAgentGraph{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph root node is required")
	}
	rootNode, ok := graph.Nodes[rootNodeID]
	if !ok {
		return runtimeAgentGraph{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph root node does not exist")
	}
	if !isRuntimeAgentNode(rootNode) {
		return runtimeAgentGraph{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph root must be an agent node")
	}

	childrenByNodeID := map[id.ID][]domain.Node{}
	for _, edge := range graph.Edges {
		if edge.FromNodeID == entryNodeID {
			continue
		}
		fromNode, fromOK := graph.Nodes[edge.FromNodeID]
		toNode, toOK := graph.Nodes[edge.ToNodeID]
		if !fromOK || !toOK {
			continue
		}
		if !isRuntimeAgentNode(fromNode) || !isRuntimeAgentNode(toNode) {
			return runtimeAgentGraph{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph only supports agent-to-agent edges")
		}
		childrenByNodeID[edge.FromNodeID] = append(childrenByNodeID[edge.FromNodeID], toNode)
	}
	result := runtimeAgentGraph{
		RootNodeID:       rootNodeID,
		ChildrenByNodeID: childrenByNodeID,
	}
	if err := validateAcyclicAgentGraph(result); err != nil {
		return runtimeAgentGraph{}, err
	}
	if runtimeAgentGraphDepth(graph) > hardAgentGraphMaxDepth {
		return runtimeAgentGraph{}, apperrors.New(apperrors.CodeInvalidArgument, "agent graph depth exceeds platform limit")
	}
	return result, nil
}

func validateAcyclicAgentGraph(graph runtimeAgentGraph) error {
	state := map[id.ID]int{}
	var visit func(nodeID id.ID) error
	visit = func(nodeID id.ID) error {
		switch state[nodeID] {
		case 1:
			return apperrors.New(apperrors.CodeConflict, "agent graph must be acyclic")
		case 2:
			return nil
		}
		state[nodeID] = 1
		for _, child := range graph.ChildrenByNodeID[nodeID] {
			if err := visit(child.ID); err != nil {
				return err
			}
		}
		state[nodeID] = 2
		return nil
	}
	return visit(graph.RootNodeID)
}

func runtimeAgentGraphDepth(graph domain.FlowGraph) int {
	entryNodeID := graph.EntryNodeID
	if entryNodeID.Empty() {
		entryNodeID = id.ID("start")
	}
	rootNodeID, ok := runtimeSupervisorNodeID(graph)
	if !ok {
		return 0
	}
	childrenByNodeID := map[id.ID][]id.ID{}
	for _, edge := range graph.Edges {
		if edge.FromNodeID == entryNodeID {
			continue
		}
		childrenByNodeID[edge.FromNodeID] = append(childrenByNodeID[edge.FromNodeID], edge.ToNodeID)
	}
	visiting := map[id.ID]bool{}
	var depth func(nodeID id.ID) int
	depth = func(nodeID id.ID) int {
		if visiting[nodeID] {
			return hardAgentGraphMaxDepth + 1
		}
		visiting[nodeID] = true
		defer delete(visiting, nodeID)
		maxChildDepth := 0
		for _, childID := range childrenByNodeID[nodeID] {
			childDepth := 1 + depth(childID)
			if childDepth > maxChildDepth {
				maxChildDepth = childDepth
			}
		}
		return maxChildDepth
	}
	return depth(rootNodeID)
}

func (r *SupervisorRunner) loadAgentGraphChildren(ctx context.Context, tenantID tenant.ID, childNodes []domain.Node) ([]agentGraphChild, error) {
	children := make([]agentGraphChild, 0, len(childNodes))
	for _, childNode := range childNodes {
		profile, err := r.loadAgent(ctx, tenantID, agentIDFromRuntimeNode(childNode), "child agent")
		if err != nil {
			return nil, err
		}
		children = append(children, agentGraphChild{
			Node:    childNode,
			Profile: profile,
		})
	}
	return children, nil
}

func childByNodeID(children []agentGraphChild, nodeID id.ID) (agentGraphChild, bool) {
	for _, child := range children {
		if child.Node.ID == nodeID {
			return child, true
		}
	}
	return agentGraphChild{}, false
}

func agentActionSpecsFromChildren(children []agentGraphChild) []orchestration.AgentActionSpec {
	agents := make([]orchestration.AgentActionSpec, 0, len(children))
	for _, child := range children {
		agents = append(agents, orchestration.AgentActionSpec{
			NodeID:      child.Node.ID,
			AgentID:     child.Profile.ID,
			Name:        child.Profile.Name,
			Description: child.Profile.Description,
		})
	}
	return agents
}

func agentIDFromRuntimeNode(node domain.Node) id.ID {
	return id.ID(strings.TrimSpace(stringConfig(node.Config, "agent_id")))
}

func agentGraphStepNodeID(nodeID id.ID, phase string) id.ID {
	return id.ID(fmt.Sprintf("%s_%s", nodeID, phase))
}

func toolResultFromActions(actions []event.Action) *tool.Result {
	for _, action := range actions {
		if action.ToolResult != nil {
			return action.ToolResult
		}
	}
	return nil
}

func taskFromInput(input map[string]any) string {
	for _, key := range []string{"user_request", "task", "message", "text", "query"} {
		if text := strings.TrimSpace(fmt.Sprint(input[key])); text != "" && text != "<nil>" {
			return text
		}
	}
	payload, err := json.Marshal(input)
	if err != nil || string(payload) == "{}" {
		return ""
	}
	return string(payload)
}

func sessionIDFromInput(input map[string]any) id.ID {
	for _, key := range []string{"session_id", "sessionId"} {
		if text := strings.TrimSpace(fmt.Sprint(input[key])); text != "" && text != "<nil>" {
			return id.ID(text)
		}
	}
	return id.New("agentflow_session")
}

func stringFromInput(input map[string]any, key string, fallback string) string {
	text := strings.TrimSpace(fmt.Sprint(input[key]))
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func safeID(value string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	return replacer.Replace(value)
}
