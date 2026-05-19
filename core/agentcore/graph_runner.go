package agentcore

import (
	"fmt"
	"strings"

	"flow-anything/core/runtimecontext"
)

const (
	DefaultGraphMaxDepth = 4
)

type AgentGraphSpec struct {
	ID          string
	Name        string
	Description string
	EntryNodeID string
	Nodes       []AgentGraphNode
	Edges       []AgentGraphEdge
	Policy      AgentGraphPolicy
}

type AgentGraphNode struct {
	ID          string
	Type        string
	Name        string
	Description string
	Agent       AgentSpec
}

type AgentGraphEdge struct {
	From string
	To   string
}

type AgentGraphPolicy struct {
	MaxDepth int
}

type AgentGraphRunRequest struct {
	Graph        AgentGraphSpec
	UserMessage  string
	Input        map[string]any
	Conversation []Message
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

type AgentGraphRunResult struct {
	Text       string
	Output     map[string]any
	RootNodeID string
	Root       AgentRunResult
}

// GraphRunner executes Agent Graph flows with recursive Agent-as-Capability
// semantics. It intentionally does not depend on flowengine: graph edges define
// which Sub-Agents are available to a node, while each Agent's reasoning
// strategy decides whether to invoke those Sub-Agents, tools, or workflows.
type GraphRunner struct {
	model            ModelClient
	baseCapabilities CapabilityRegistry
	strategies       *StrategyRegistry
	contextAssembler ContextAssembler
	memory           MemoryProvider
	hooks            []AgentEventHook
}

type GraphRunnerOption func(*GraphRunner)

func WithGraphCapabilities(registry CapabilityRegistry) GraphRunnerOption {
	return func(r *GraphRunner) { r.baseCapabilities = registry }
}

func WithGraphStrategies(registry *StrategyRegistry) GraphRunnerOption {
	return func(r *GraphRunner) { r.strategies = registry }
}

func WithGraphContextAssembler(assembler ContextAssembler) GraphRunnerOption {
	return func(r *GraphRunner) {
		if assembler != nil {
			r.contextAssembler = assembler
		}
	}
}

func WithGraphMemoryProvider(provider MemoryProvider) GraphRunnerOption {
	return func(r *GraphRunner) { r.memory = provider }
}

func WithGraphEventHook(hook AgentEventHook) GraphRunnerOption {
	return func(r *GraphRunner) {
		if hook != nil {
			r.hooks = append(r.hooks, hook)
		}
	}
}

func NewGraphRunner(model ModelClient, opts ...GraphRunnerOption) *GraphRunner {
	runner := &GraphRunner{
		model:            model,
		baseCapabilities: NewMapCapabilityRegistry(),
		strategies:       NewDefaultStrategyRegistry(),
		contextAssembler: NewDefaultContextAssembler(),
	}
	for _, opt := range opts {
		opt(runner)
	}
	return runner
}

func (r *GraphRunner) Run(ctx Context, req AgentGraphRunRequest) (AgentGraphRunResult, error) {
	if r.model == nil {
		return AgentGraphRunResult{}, fmt.Errorf("model client is required")
	}
	graphIndex, err := newAgentGraphIndex(req.Graph)
	if err != nil {
		return AgentGraphRunResult{}, err
	}
	rootNodeID, err := graphIndex.rootNodeID(req.Graph.EntryNodeID)
	if err != nil {
		return AgentGraphRunResult{}, err
	}
	traceContext := req.TraceContext
	if traceContext.TraceID == "" {
		traceContext.TraceID = firstNonEmpty(req.TraceID, "agent_graph:"+req.Graph.ID)
	}
	result, err := r.runNode(ctx, graphIndex, rootNodeID, req.UserMessage, req.Input, req.Conversation, traceContext, map[string]bool{}, 0)
	if err != nil {
		return AgentGraphRunResult{}, err
	}
	output := cloneAnyMap(result.Output)
	if output == nil {
		output = map[string]any{}
	}
	if _, ok := output["return_message"]; !ok && result.Text != "" {
		output["return_message"] = result.Text
	}
	return AgentGraphRunResult{
		Text:       result.Text,
		Output:     output,
		RootNodeID: rootNodeID,
		Root:       result,
	}, nil
}

func (r *GraphRunner) runNode(
	ctx Context,
	graph *agentGraphIndex,
	nodeID string,
	message string,
	input map[string]any,
	conversation []Message,
	traceContext runtimecontext.TraceContext,
	stack map[string]bool,
	depth int,
) (AgentRunResult, error) {
	if depth > graph.maxDepth {
		return AgentRunResult{}, fmt.Errorf("agent graph max depth %d exceeded at node %q", graph.maxDepth, nodeID)
	}
	if stack[nodeID] {
		return AgentRunResult{}, fmt.Errorf("agent graph cycle detected at node %q", nodeID)
	}
	node, ok := graph.nodes[nodeID]
	if !ok {
		return AgentRunResult{}, fmt.Errorf("agent graph node %q not found", nodeID)
	}
	if node.Agent.ID == "" {
		return AgentRunResult{}, fmt.Errorf("agent graph node %q has no executable agent", nodeID)
	}

	nextStack := cloneBoolMap(stack)
	nextStack[nodeID] = true
	localCapabilities := NewMapCapabilityRegistry()
	childDescriptors := []CapabilityDescriptor{}
	for _, childID := range graph.children[nodeID] {
		child, ok := graph.nodes[childID]
		if !ok || child.Agent.ID == "" {
			continue
		}
		descriptor := CapabilityDescriptor{
			ID:           childID,
			Type:         "agent",
			Name:         firstNonEmpty(child.Agent.Name, child.Name, childID),
			Description:  firstNonEmpty(child.Agent.Description, child.Description),
			InputSchema:  []SchemaField{{Name: "user_request", Type: "string", Description: "Task passed from the parent Agent."}},
			OutputSchema: child.Agent.OutputSchema,
		}
		childDescriptors = append(childDescriptors, descriptor)
		childID := childID
		descriptorCopy := descriptor
		_ = localCapabilities.Register(CapabilityFunc{
			Desc: descriptorCopy,
			Fn: func(callCtx Context, call CapabilityCall) (CapabilityResult, error) {
				childMessage := firstNonEmpty(call.Task, stringFromMap(call.Input, "user_request"), stringFromMap(call.Input, "task"), stringFromMap(call.Input, "message"), message)
				childInput := mergeMaps(input, call.Input)
				childTrace := runtimecontext.TraceContext{
					TraceID:       firstNonEmpty(call.TraceContext.TraceID, traceContext.TraceID),
					ParentSpanID:  call.TraceContext.SpanID,
					CorrelationID: call.TraceContext.CorrelationID,
				}
				childResult, err := r.runNode(callCtx, graph, childID, childMessage, childInput, nil, childTrace, nextStack, depth+1)
				if err != nil {
					return CapabilityResult{}, err
				}
				return CapabilityResult{
					ID:     childID,
					Type:   "agent",
					Text:   childResult.Text,
					Output: childResult.Output,
					Raw:    childResult.Raw,
				}, nil
			},
		})
	}

	agent := node.Agent
	if agent.ReasoningMode == "" {
		agent.ReasoningMode = ReWOOStrategy{}.Name()
	}
	agent.Capabilities = mergeCapabilityDescriptors(agent.Capabilities, childDescriptors)
	capabilities := NewCompositeCapabilityRegistry(localCapabilities, r.baseCapabilities)
	runner := NewRunner(
		r.model,
		WithCapabilities(capabilities),
		WithStrategies(r.strategies),
		WithContextAssembler(r.contextAssembler),
		WithMemoryProvider(r.memory),
	)
	for _, hook := range r.hooks {
		runner.hooks = append(runner.hooks, hook)
	}
	return runner.Run(ctx, AgentRunRequest{
		Agent:        agent,
		UserMessage:  message,
		Conversation: conversation,
		Context:      input,
		TraceID:      traceContext.TraceID,
		TraceContext: traceContext,
	})
}

type agentGraphIndex struct {
	graph    AgentGraphSpec
	nodes    map[string]AgentGraphNode
	children map[string][]string
	maxDepth int
}

func newAgentGraphIndex(graph AgentGraphSpec) (*agentGraphIndex, error) {
	if graph.ID == "" {
		return nil, fmt.Errorf("agent graph id is required")
	}
	index := &agentGraphIndex{
		graph:    graph,
		nodes:    map[string]AgentGraphNode{},
		children: map[string][]string{},
		maxDepth: graph.Policy.MaxDepth,
	}
	if index.maxDepth <= 0 {
		index.maxDepth = DefaultGraphMaxDepth
	}
	for _, node := range graph.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("agent graph node id is required")
		}
		if _, exists := index.nodes[node.ID]; exists {
			return nil, fmt.Errorf("duplicate agent graph node %q", node.ID)
		}
		index.nodes[node.ID] = node
	}
	for _, edge := range graph.Edges {
		if edge.From == "" || edge.To == "" {
			return nil, fmt.Errorf("agent graph edge requires from and to")
		}
		index.children[edge.From] = append(index.children[edge.From], edge.To)
	}
	if err := index.validateAcyclic(); err != nil {
		return nil, err
	}
	return index, nil
}

func (i *agentGraphIndex) validateAcyclic() error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	path := []string{}
	var visit func(nodeID string) error
	visit = func(nodeID string) error {
		if visiting[nodeID] {
			return fmt.Errorf("agent graph cycle detected: %s", cyclePath(path, nodeID))
		}
		if visited[nodeID] {
			return nil
		}
		visiting[nodeID] = true
		path = append(path, nodeID)
		for _, childID := range i.children[nodeID] {
			if _, ok := i.nodes[childID]; !ok {
				return fmt.Errorf("agent graph edge targets missing node %q", childID)
			}
			if err := visit(childID); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		visiting[nodeID] = false
		visited[nodeID] = true
		return nil
	}
	for nodeID := range i.nodes {
		if err := visit(nodeID); err != nil {
			return err
		}
	}
	return nil
}

func cyclePath(path []string, repeatedNodeID string) string {
	start := 0
	for idx, nodeID := range path {
		if nodeID == repeatedNodeID {
			start = idx
			break
		}
	}
	cycle := append([]string{}, path[start:]...)
	cycle = append(cycle, repeatedNodeID)
	return strings.Join(cycle, " -> ")
}

func (i *agentGraphIndex) rootNodeID(entryNodeID string) (string, error) {
	entryNodeID = firstNonEmpty(entryNodeID, "start")
	if node, ok := i.nodes[entryNodeID]; ok && node.Type != "start" {
		if node.Agent.ID == "" {
			return "", fmt.Errorf("entry node %q has no executable agent", entryNodeID)
		}
		return entryNodeID, nil
	}
	children := i.children[entryNodeID]
	if len(children) == 0 {
		return "", fmt.Errorf("agent graph entry node %q has no outgoing agent node", entryNodeID)
	}
	if len(children) > 1 {
		return "", fmt.Errorf("agent graph entry node %q must have exactly one outgoing agent node", entryNodeID)
	}
	return children[0], nil
}

func mergeCapabilityDescriptors(base []CapabilityDescriptor, extra []CapabilityDescriptor) []CapabilityDescriptor {
	out := make([]CapabilityDescriptor, 0, len(base)+len(extra))
	seen := map[string]struct{}{}
	for _, descriptor := range append(base, extra...) {
		if descriptor.ID == "" {
			continue
		}
		if _, exists := seen[descriptor.ID]; exists {
			continue
		}
		seen[descriptor.ID] = struct{}{}
		out = append(out, descriptor)
	}
	return out
}

func mergeMaps(left map[string]any, right map[string]any) map[string]any {
	out := cloneAnyMap(left)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func cloneAnyMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, entry := range value {
		out[key] = entry
	}
	return out
}

func cloneBoolMap(value map[string]bool) map[string]bool {
	out := make(map[string]bool, len(value))
	for key, entry := range value {
		out[key] = entry
	}
	return out
}

func stringFromMap(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	value, ok := input[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}
