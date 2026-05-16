package flowengine

import (
	"context"
	"fmt"
	"time"

	"flow-anything/core/runtimecontext"
)

// Executor runs a FlowSpec with registered node executors.
type Executor struct {
	registry *Registry
	hooks    []FlowEventHook
	runIDFn  func() string
}

// Option customizes Executor.
type Option func(*Executor)

func WithEventHook(hook FlowEventHook) Option {
	return func(e *Executor) {
		if hook != nil {
			e.hooks = append(e.hooks, hook)
		}
	}
}

func WithEventHookFunc(hook func(context.Context, FlowEvent)) Option {
	return func(e *Executor) {
		if hook != nil {
			e.hooks = append(e.hooks, FlowEventHookFunc(hook))
		}
	}
}

func WithEventSink(sink FlowEventHook) Option {
	return WithEventHook(sink)
}

func WithRunIDFunc(fn func() string) Option {
	return func(e *Executor) { e.runIDFn = fn }
}

func NewExecutor(registry *Registry, opts ...Option) *Executor {
	executor := &Executor{
		registry: registry,
		runIDFn:  func() string { return fmt.Sprintf("run_%d", time.Now().UnixNano()) },
	}
	for _, opt := range opts {
		opt(executor)
	}
	return executor
}

// Execute runs the flow from source nodes to reachable downstream nodes.
func (e *Executor) Execute(ctx context.Context, spec FlowSpec, flowInput map[string]any) (RunResult, error) {
	if e.registry == nil {
		return RunResult{}, fmt.Errorf("node registry is required")
	}
	if err := e.validate(ctx, spec); err != nil {
		return RunResult{}, err
	}

	runID := e.runIDFn()
	parentTrace, _ := runtimecontext.TraceContextFrom(ctx)
	flowTrace := runtimecontext.RootTraceContext(runtimecontext.TraceIDFrom(ctx, runID), runtimecontext.FlowSpanID(runID), parentTrace)
	ctx = runtimecontext.WithTraceContext(ctx, flowTrace)
	data := NewDataContext(flowInput)
	events := []FlowEvent{}
	emit := e.eventEmitter(ctx, runID, spec.ID, flowTrace, &events)
	emit(FlowEvent{Type: EventFlowStarted, Input: cloneMap(flowInput)})

	nodeByID := indexNodes(spec.Nodes)
	outgoing := outgoingEdges(spec.Edges)
	queue := sourceNodes(spec.Nodes, spec.Edges)
	if len(queue) == 0 {
		err := fmt.Errorf("flow has no source node")
		emit(FlowEvent{Type: EventFlowFailed, Error: err.Error()})
		return RunResult{RunID: runID, Context: data, Events: events}, err
	}

	executed := map[string]bool{}
	nodeOrder := []string{}
	maxExecutions := spec.Policies.MaxNodeExecutions
	if maxExecutions <= 0 {
		maxExecutions = len(spec.Nodes) * 2
	}

	for len(queue) > 0 {
		if len(nodeOrder) >= maxExecutions {
			err := fmt.Errorf("flow exceeded max node executions: %d", maxExecutions)
			emit(FlowEvent{Type: EventFlowFailed, Error: err.Error()})
			return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, err
		}
		nodeID := queue[0]
		queue = queue[1:]
		if executed[nodeID] {
			continue
		}
		emit(FlowEvent{Type: EventNodeScheduled, NodeID: nodeID})
		node, ok := nodeByID[nodeID]
		if !ok {
			err := fmt.Errorf("node %q not found", nodeID)
			emit(FlowEvent{Type: EventFlowFailed, NodeID: nodeID, Error: err.Error()})
			return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, err
		}
		executor, ok := e.registry.Get(node.Type)
		if !ok {
			err := fmt.Errorf("node executor for type %q is not registered", node.Type)
			emit(FlowEvent{Type: EventFlowFailed, NodeID: node.ID, NodeType: node.Type, Error: err.Error()})
			return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, err
		}

		input, err := BuildNodeInput(data, node.InputMappings)
		if err != nil {
			emit(FlowEvent{Type: EventNodeFailed, NodeID: node.ID, NodeType: node.Type, Error: err.Error()})
			emit(FlowEvent{Type: EventFlowFailed, NodeID: node.ID, NodeType: node.Type, Error: err.Error()})
			return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, err
		}
		emit(FlowEvent{Type: EventNodeStarted, NodeID: node.ID, NodeType: node.Type, Input: input, Data: map[string]any{"node_name": node.Name}})

		nodeTrace := runtimecontext.TraceContext{
			TraceID:       flowTrace.TraceID,
			SpanID:        runtimecontext.NodeSpanID(runID, node.ID),
			ParentSpanID:  flowTrace.SpanID,
			CorrelationID: flowTrace.CorrelationID,
		}
		nodeCtx := runtimecontext.WithTraceContext(ctx, nodeTrace)
		cancel := func() {}
		if node.Timeout > 0 {
			nodeCtx, cancel = context.WithTimeout(nodeCtx, node.Timeout)
		}
		result, err := executor.Execute(nodeCtx, NodeRequest{
			RunID:   runID,
			Flow:    spec,
			Node:    node,
			Input:   input,
			Context: data,
		})
		cancel()
		if err != nil {
			emit(FlowEvent{Type: EventNodeFailed, NodeID: node.ID, NodeType: node.Type, Input: input, Error: err.Error(), Data: map[string]any{"node_name": node.Name}})
			emit(FlowEvent{Type: EventFlowFailed, NodeID: node.ID, NodeType: node.Type, Error: err.Error()})
			return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, err
		}
		if result.Output == nil {
			result.Output = map[string]any{}
		}
		if err := ApplyContextWrites(data, result.Output, node.OutputWrites); err != nil {
			emit(FlowEvent{Type: EventNodeFailed, NodeID: node.ID, NodeType: node.Type, Input: input, Output: result.Output, Error: err.Error()})
			emit(FlowEvent{Type: EventFlowFailed, NodeID: node.ID, NodeType: node.Type, Error: err.Error()})
			return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, err
		}

		executed[nodeID] = true
		nodeOrder = append(nodeOrder, nodeID)
		emit(FlowEvent{Type: EventNodeCompleted, NodeID: node.ID, NodeType: node.Type, Input: input, Output: result.Output, Data: map[string]any{"node_name": node.Name}})

		nextIDs := result.NextNodeIDs
		if nextIDs == nil {
			nextIDs = outgoing[nodeID]
		}
		for _, nextID := range nextIDs {
			emit(FlowEvent{Type: EventNextNodeSelected, SourceNodeID: node.ID, TargetNodeID: nextID})
		}
		queue = append(queue, nextIDs...)
	}

	emit(FlowEvent{Type: EventFlowCompleted, Output: cloneMap(data.FlowOutput)})
	return RunResult{RunID: runID, Context: data, NodeOrder: nodeOrder, Events: events}, nil
}

func (e *Executor) eventEmitter(ctx context.Context, runID, flowID string, flowTrace runtimecontext.TraceContext, events *[]FlowEvent) func(FlowEvent) {
	return func(event FlowEvent) {
		event.RunID = runID
		event.FlowID = flowID
		event.Timestamp = time.Now()
		event.TraceContext = normalizeFlowEventTraceContext(event, flowTrace)
		*events = append(*events, event)
		eventCtx := runtimecontext.WithTraceContext(ctx, event.TraceContext)
		for _, hook := range e.hooks {
			hook.OnFlowEvent(eventCtx, event)
		}
	}
}

func normalizeFlowEventTraceContext(event FlowEvent, flowTrace runtimecontext.TraceContext) runtimecontext.TraceContext {
	if event.TraceContext.TraceID == "" {
		event.TraceContext.TraceID = flowTrace.TraceID
	}
	if event.TraceContext.CorrelationID == "" {
		event.TraceContext.CorrelationID = flowTrace.CorrelationID
	}
	if event.TraceContext.SpanID != "" {
		return event.TraceContext
	}
	switch event.Type {
	case EventNodeScheduled, EventNodeStarted, EventNodeWaiting, EventNodeCompleted, EventNodeFailed:
		if event.NodeID == "" {
			return flowTrace
		}
		event.TraceContext.SpanID = runtimecontext.NodeSpanID(event.RunID, event.NodeID)
		event.TraceContext.ParentSpanID = flowTrace.SpanID
	case EventNextNodeSelected:
		if event.SourceNodeID == "" {
			return flowTrace
		}
		event.TraceContext.SpanID = runtimecontext.NodeSpanID(event.RunID, event.SourceNodeID)
		event.TraceContext.ParentSpanID = flowTrace.SpanID
	default:
		event.TraceContext.SpanID = flowTrace.SpanID
		event.TraceContext.ParentSpanID = flowTrace.ParentSpanID
	}
	return event.TraceContext
}

func (e *Executor) validate(ctx context.Context, spec FlowSpec) error {
	if spec.ID == "" {
		return fmt.Errorf("flow id is required")
	}
	seen := map[string]bool{}
	for _, node := range spec.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node id is required")
		}
		if seen[node.ID] {
			return fmt.Errorf("duplicate node id %q", node.ID)
		}
		seen[node.ID] = true
		executor, ok := e.registry.Get(node.Type)
		if !ok {
			return fmt.Errorf("node executor for type %q is not registered", node.Type)
		}
		if err := executor.Validate(ctx, node); err != nil {
			return fmt.Errorf("validate node %q: %w", node.ID, err)
		}
	}
	for _, edge := range spec.Edges {
		if !seen[edge.From] || !seen[edge.To] {
			return fmt.Errorf("edge %q -> %q references unknown node", edge.From, edge.To)
		}
	}
	return nil
}

func indexNodes(nodes []NodeSpec) map[string]NodeSpec {
	out := make(map[string]NodeSpec, len(nodes))
	for _, node := range nodes {
		out[node.ID] = node
	}
	return out
}

func outgoingEdges(edges []EdgeSpec) map[string][]string {
	out := map[string][]string{}
	for _, edge := range edges {
		out[edge.From] = append(out[edge.From], edge.To)
	}
	return out
}

func sourceNodes(nodes []NodeSpec, edges []EdgeSpec) []string {
	hasIncoming := map[string]bool{}
	for _, edge := range edges {
		hasIncoming[edge.To] = true
	}
	out := []string{}
	for _, node := range nodes {
		if !hasIncoming[node.ID] {
			out = append(out, node.ID)
		}
	}
	return out
}
