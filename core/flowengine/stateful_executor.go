package flowengine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"flow-anything/core/runtimecontext"
)

// StatefulExecutor advances persisted FlowInstance state until it is blocked,
// completed, or failed.
type StatefulExecutor struct {
	registry     *Registry
	store        InstanceStore
	hooks        []FlowEventHook
	instanceIDFn func() string
	tokenIDFn    func() string
}

type StatefulOption func(*StatefulExecutor)

func WithStatefulEventHook(hook FlowEventHook) StatefulOption {
	return func(e *StatefulExecutor) {
		if hook != nil {
			e.hooks = append(e.hooks, hook)
		}
	}
}

func WithStatefulEventSink(sink FlowEventHook) StatefulOption {
	return WithStatefulEventHook(sink)
}

func WithInstanceIDFunc(fn func() string) StatefulOption {
	return func(e *StatefulExecutor) { e.instanceIDFn = fn }
}

func WithTokenIDFunc(fn func() string) StatefulOption {
	return func(e *StatefulExecutor) { e.tokenIDFn = fn }
}

func NewStatefulExecutor(registry *Registry, store InstanceStore, opts ...StatefulOption) *StatefulExecutor {
	if registry == nil {
		registry = NewDefaultRegistry()
	} else {
		_ = RegisterControlNodes(registry)
	}
	if store == nil {
		store = NewMemoryInstanceStore()
	}
	executor := &StatefulExecutor{
		registry:     registry,
		store:        store,
		instanceIDFn: func() string { return fmt.Sprintf("instance_%d", time.Now().UnixNano()) },
		tokenIDFn:    func() string { return fmt.Sprintf("token_%d", time.Now().UnixNano()) },
	}
	for _, opt := range opts {
		opt(executor)
	}
	return executor
}

// Start creates a flow instance and runs it until it blocks or completes.
func (e *StatefulExecutor) Start(ctx context.Context, spec FlowSpec, flowInput map[string]any) (FlowInstance, error) {
	return e.StartWithContext(ctx, spec, flowInput, NewDataContext(flowInput))
}

// StartWithContext creates a flow instance using an explicit initial context
// and runs it until it blocks or completes.
func (e *StatefulExecutor) StartWithContext(ctx context.Context, spec FlowSpec, flowInput map[string]any, initialContext *DataContext) (FlowInstance, error) {
	if err := e.validate(ctx, spec); err != nil {
		return FlowInstance{}, err
	}
	sources := sourceNodes(spec.Nodes, spec.Edges)
	if len(sources) == 0 {
		return FlowInstance{}, fmt.Errorf("flow has no source node")
	}
	if initialContext == nil {
		initialContext = NewDataContext(flowInput)
	}
	if initialContext.FlowInput == nil {
		initialContext.FlowInput = cloneMap(flowInput)
	}
	now := time.Now()
	instance := FlowInstance{
		InstanceID:  e.instanceIDFn(),
		FlowID:      spec.ID,
		FlowVersion: spec.Version,
		Status:      InstanceCreated,
		Context:     initialContext,
		Tokens:      make([]ExecutionToken, 0, len(sources)),
		NodeStates:  map[string]NodeState{},
		JoinStates:  map[string]JoinState{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	for _, nodeID := range sources {
		instance.Tokens = append(instance.Tokens, e.newToken(nodeID, "", now))
	}
	ctx = runtimecontext.WithTraceContext(ctx, rootFlowTraceContext(ctx, instance.InstanceID))
	if err := e.store.Create(ctx, instance); err != nil {
		return FlowInstance{}, err
	}
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventFlowStarted, Input: cloneMap(flowInput)})
	return e.RunUntilBlocked(ctx, spec, instance.InstanceID)
}

// RunUntilBlocked advances ready tokens until the instance is waiting,
// completed, failed, or cancelled.
func (e *StatefulExecutor) RunUntilBlocked(ctx context.Context, spec FlowSpec, instanceID string) (FlowInstance, error) {
	if err := e.validate(ctx, spec); err != nil {
		return FlowInstance{}, err
	}
	instance, err := e.store.Get(ctx, instanceID)
	if err != nil {
		return FlowInstance{}, err
	}
	if isTerminalInstance(instance.Status) {
		return instance, nil
	}
	ctx = runtimecontext.WithTraceContext(ctx, rootFlowTraceContext(ctx, instance.InstanceID))
	instance.Status = InstanceRunning

	for {
		index := firstReadyTokenIndex(instance.Tokens)
		if index < 0 {
			instance = e.finalizeBlockedInstance(ctx, spec, instance)
			return instance, e.store.Save(ctx, instance)
		}
		if err := e.executeReadyToken(ctx, spec, &instance, index); err != nil {
			instance.Status = InstanceFailed
			instance.LastError = err.Error()
			instance.UpdatedAt = time.Now()
			_ = e.store.Save(ctx, instance)
			e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventFlowFailed, Error: err.Error()})
			return instance, err
		}
		instance.Version++
		instance.UpdatedAt = time.Now()
		if err := e.store.Save(ctx, instance); err != nil {
			return FlowInstance{}, err
		}
	}
}

// Resume applies an external event to matching waiting tokens and continues
// execution until the next blocked state.
func (e *StatefulExecutor) Resume(ctx context.Context, spec FlowSpec, instanceID string, event ExternalEvent) (FlowInstance, error) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	instance, err := e.store.Get(ctx, instanceID)
	if err != nil {
		return FlowInstance{}, err
	}
	if isTerminalInstance(instance.Status) {
		return instance, fmt.Errorf("flow instance %q is already %s", instanceID, instance.Status)
	}
	ctx = runtimecontext.WithTraceContext(ctx, rootFlowTraceContext(ctx, instance.InstanceID))
	nodeByID := indexNodes(spec.Nodes)
	outgoing := outgoingEdges(spec.Edges)
	matched := false
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{
		Type: EventFlowResumed,
		Data: map[string]any{"event_type": event.Type, "event_key": event.Key, "payload": event.Payload},
	})

	for i := range instance.Tokens {
		token := &instance.Tokens[i]
		if token.Status != TokenWaiting || !waitConditionsMatch(token.WaitingFor, event) {
			continue
		}
		node, ok := nodeByID[token.NodeID]
		if !ok {
			return instance, fmt.Errorf("waiting node %q not found", token.NodeID)
		}
		output := map[string]any{
			"event_type": event.Type,
			"event_key":  event.Key,
			"payload":    event.Payload,
		}
		if err := ApplyContextWrites(instance.Context, output, node.OutputWrites); err != nil {
			return instance, err
		}
		token.Status = TokenConsumed
		token.UpdatedAt = time.Now()
		state := instance.NodeStates[node.ID]
		state.NodeID = node.ID
		state.Status = NodeCompleted
		state.Output = output
		state.WaitingFor = nil
		state.FinishedAt = time.Now()
		instance.NodeStates[node.ID] = state
		e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNodeCompleted, NodeID: node.ID, NodeType: node.Type, Output: output, Data: map[string]any{"resumed": true}})
		e.appendNextTokens(ctx, spec, &instance, node.ID, outgoing[node.ID])
		matched = true
	}
	if !matched {
		return instance, fmt.Errorf("external event %q/%q did not match any waiting token", event.Type, event.Key)
	}
	instance.Status = InstanceRunning
	instance.Version++
	instance.UpdatedAt = time.Now()
	if err := e.store.Save(ctx, instance); err != nil {
		return FlowInstance{}, err
	}
	return e.RunUntilBlocked(ctx, spec, instanceID)
}

func (e *StatefulExecutor) executeReadyToken(ctx context.Context, spec FlowSpec, instance *FlowInstance, tokenIndex int) error {
	nodeByID := indexNodes(spec.Nodes)
	outgoing := outgoingEdges(spec.Edges)
	token := &instance.Tokens[tokenIndex]
	node, ok := nodeByID[token.NodeID]
	if !ok {
		return fmt.Errorf("node %q not found", token.NodeID)
	}
	executor, ok := e.registry.Get(node.Type)
	if !ok {
		return fmt.Errorf("node executor for type %q is not registered", node.Type)
	}

	token.Status = TokenRunning
	token.UpdatedAt = time.Now()
	input, err := BuildNodeInput(instance.Context, node.InputMappings)
	if err != nil {
		return err
	}
	state := instance.NodeStates[node.ID]
	state.NodeID = node.ID
	state.Status = NodeRunning
	state.Attempts++
	state.Input = input
	state.StartedAt = time.Now()
	instance.NodeStates[node.ID] = state
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNodeScheduled, NodeID: node.ID, NodeType: node.Type})
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNodeStarted, NodeID: node.ID, NodeType: node.Type, Input: input})

	if node.Type == NodeTypeWait {
		return e.blockOnWaitNode(ctx, spec, instance, tokenIndex, node, input)
	}
	if node.Type == NodeTypeJoin {
		return e.executeJoinNode(ctx, spec, instance, tokenIndex, node, input)
	}

	nodeTrace := runtimecontext.TraceContext{
		TraceID:       runtimecontext.TraceIDFrom(ctx, instance.InstanceID),
		SpanID:        runtimecontext.NodeSpanID(instance.InstanceID, node.ID),
		ParentSpanID:  runtimecontext.FlowSpanID(instance.InstanceID),
		CorrelationID: traceCorrelationID(ctx),
	}
	nodeCtx := runtimecontext.WithTraceContext(ctx, nodeTrace)
	cancel := func() {}
	if node.Timeout > 0 {
		nodeCtx, cancel = context.WithTimeout(nodeCtx, node.Timeout)
	}
	result, err := executor.Execute(nodeCtx, NodeRequest{
		RunID:   instance.InstanceID,
		Flow:    spec,
		Node:    node,
		Input:   input,
		Context: instance.Context,
	})
	cancel()
	if err != nil {
		return e.failNode(ctx, spec, instance, tokenIndex, node, input, nil, err)
	}
	if result.Output == nil {
		result.Output = map[string]any{}
	}
	if err := ApplyContextWrites(instance.Context, result.Output, node.OutputWrites); err != nil {
		return e.failNode(ctx, spec, instance, tokenIndex, node, input, result.Output, err)
	}
	nextIDs := result.NextNodeIDs
	if nextIDs == nil {
		nextIDs = outgoing[node.ID]
	}
	e.completeNode(ctx, spec, instance, tokenIndex, node, input, result.Output, nextIDs)
	return nil
}

func (e *StatefulExecutor) blockOnWaitNode(ctx context.Context, spec FlowSpec, instance *FlowInstance, tokenIndex int, node NodeSpec, input map[string]any) error {
	config, err := decodeWaitConfig(node.Config, instance.Context)
	if err != nil {
		return e.failNode(ctx, spec, instance, tokenIndex, node, input, nil, err)
	}
	condition := WaitCondition{Type: config.EventType, EventKey: config.EventKey, TimeoutAt: config.TimeoutAt}
	token := &instance.Tokens[tokenIndex]
	token.Status = TokenWaiting
	token.WaitingFor = []WaitCondition{condition}
	token.UpdatedAt = time.Now()

	state := instance.NodeStates[node.ID]
	state.NodeID = node.ID
	state.Status = NodeWaiting
	state.Input = input
	state.WaitingFor = []WaitCondition{condition}
	instance.NodeStates[node.ID] = state
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{
		Type:     EventNodeWaiting,
		NodeID:   node.ID,
		NodeType: node.Type,
		Input:    input,
		Data:     map[string]any{"waiting_for": condition},
	})
	return nil
}

func (e *StatefulExecutor) executeJoinNode(ctx context.Context, spec FlowSpec, instance *FlowInstance, tokenIndex int, node NodeSpec, input map[string]any) error {
	incoming := incomingEdges(spec.Edges)[node.ID]
	if len(incoming) <= 1 {
		e.completeNode(ctx, spec, instance, tokenIndex, node, input, map[string]any{"joined": true}, outgoingEdges(spec.Edges)[node.ID])
		return nil
	}
	token := &instance.Tokens[tokenIndex]
	state := instance.JoinStates[node.ID]
	if state.NodeID == "" {
		state = JoinState{NodeID: node.ID, ExpectedNodes: incoming, ArrivedNodes: map[string]bool{}}
	}
	if state.Completed {
		token.Status = TokenConsumed
		token.UpdatedAt = time.Now()
		instance.JoinStates[node.ID] = state
		return nil
	}
	if token.SourceNodeID != "" {
		state.ArrivedNodes[token.SourceNodeID] = true
	}
	token.Status = TokenConsumed
	token.UpdatedAt = time.Now()
	instance.JoinStates[node.ID] = state

	if !joinArrivedAll(state) {
		nodeState := instance.NodeStates[node.ID]
		nodeState.NodeID = node.ID
		nodeState.Status = NodeWaiting
		nodeState.Input = input
		nodeState.Output = map[string]any{"arrived": cloneBoolMap(state.ArrivedNodes), "expected": state.ExpectedNodes}
		instance.NodeStates[node.ID] = nodeState
		e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{
			Type:     EventNodeWaiting,
			NodeID:   node.ID,
			NodeType: node.Type,
			Input:    input,
			Data:     map[string]any{"arrived": cloneBoolMap(state.ArrivedNodes), "expected": state.ExpectedNodes},
		})
		return nil
	}

	state.Completed = true
	instance.JoinStates[node.ID] = state
	output := map[string]any{"joined": true, "arrived": cloneBoolMap(state.ArrivedNodes)}
	nodeState := instance.NodeStates[node.ID]
	nodeState.NodeID = node.ID
	nodeState.Status = NodeCompleted
	nodeState.Input = input
	nodeState.Output = output
	nodeState.FinishedAt = time.Now()
	instance.NodeStates[node.ID] = nodeState
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNodeCompleted, NodeID: node.ID, NodeType: node.Type, Input: input, Output: output})
	e.appendNextTokens(ctx, spec, instance, node.ID, outgoingEdges(spec.Edges)[node.ID])
	return nil
}

func (e *StatefulExecutor) completeNode(ctx context.Context, spec FlowSpec, instance *FlowInstance, tokenIndex int, node NodeSpec, input, output map[string]any, nextIDs []string) {
	token := &instance.Tokens[tokenIndex]
	token.Status = TokenConsumed
	token.UpdatedAt = time.Now()
	state := instance.NodeStates[node.ID]
	state.NodeID = node.ID
	state.Status = NodeCompleted
	state.Input = input
	state.Output = output
	state.FinishedAt = time.Now()
	instance.NodeStates[node.ID] = state
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNodeCompleted, NodeID: node.ID, NodeType: node.Type, Input: input, Output: output})
	e.appendNextTokens(ctx, spec, instance, node.ID, nextIDs)
}

func (e *StatefulExecutor) failNode(ctx context.Context, spec FlowSpec, instance *FlowInstance, tokenIndex int, node NodeSpec, input, output map[string]any, err error) error {
	token := &instance.Tokens[tokenIndex]
	token.Status = TokenConsumed
	token.UpdatedAt = time.Now()
	state := instance.NodeStates[node.ID]
	state.NodeID = node.ID
	state.Status = NodeFailed
	state.Input = input
	state.Output = output
	state.Error = err.Error()
	state.FinishedAt = time.Now()
	instance.NodeStates[node.ID] = state
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNodeFailed, NodeID: node.ID, NodeType: node.Type, Input: input, Output: output, Error: err.Error()})
	return err
}

func (e *StatefulExecutor) appendNextTokens(ctx context.Context, spec FlowSpec, instance *FlowInstance, sourceNodeID string, nextIDs []string) {
	now := time.Now()
	for _, nextID := range nextIDs {
		instance.Tokens = append(instance.Tokens, e.newToken(nextID, sourceNodeID, now))
		e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventNextNodeSelected, SourceNodeID: sourceNodeID, TargetNodeID: nextID})
	}
}

func (e *StatefulExecutor) newToken(nodeID, sourceNodeID string, now time.Time) ExecutionToken {
	return ExecutionToken{
		TokenID:      e.tokenIDFn(),
		NodeID:       nodeID,
		SourceNodeID: sourceNodeID,
		Status:       TokenReady,
		Payload:      map[string]any{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (e *StatefulExecutor) finalizeBlockedInstance(ctx context.Context, spec FlowSpec, instance FlowInstance) FlowInstance {
	instance.UpdatedAt = time.Now()
	if hasWaitingToken(instance.Tokens) {
		instance.Status = InstanceWaiting
		e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventFlowWaiting})
		return instance
	}
	instance.Status = InstanceCompleted
	e.emit(ctx, instance.InstanceID, spec.ID, FlowEvent{Type: EventFlowCompleted, Output: cloneMap(instance.Context.FlowOutput)})
	return instance
}

func (e *StatefulExecutor) emit(ctx context.Context, instanceID, flowID string, event FlowEvent) {
	event.RunID = instanceID
	event.FlowID = flowID
	event.Timestamp = time.Now()
	flowTrace := rootFlowTraceContext(ctx, instanceID)
	event.TraceContext = normalizeFlowEventTraceContext(event, flowTrace)
	eventCtx := runtimecontext.WithTraceContext(ctx, event.TraceContext)
	for _, hook := range e.hooks {
		hook.OnFlowEvent(eventCtx, event)
	}
	_ = e.store.AppendEvent(eventCtx, FlowInstanceEvent{
		EventID:    fmt.Sprintf("event_%d", time.Now().UnixNano()),
		InstanceID: instanceID,
		Type:       string(event.Type),
		NodeID:     event.NodeID,
		Data:       event.Data,
		Error:      event.Error,
		Timestamp:  event.Timestamp,
	})
}

func rootFlowTraceContext(ctx context.Context, instanceID string) runtimecontext.TraceContext {
	parentTrace, _ := runtimecontext.TraceContextFrom(ctx)
	flowSpanID := runtimecontext.FlowSpanID(instanceID)
	if parentTrace.SpanID == flowSpanID {
		return parentTrace
	}
	return runtimecontext.RootTraceContext(runtimecontext.TraceIDFrom(ctx, instanceID), flowSpanID, parentTrace)
}

func traceCorrelationID(ctx context.Context) string {
	traceContext, _ := runtimecontext.TraceContextFrom(ctx)
	return traceContext.CorrelationID
}

func (e *StatefulExecutor) validate(ctx context.Context, spec FlowSpec) error {
	stateless := &Executor{registry: e.registry}
	return stateless.validate(ctx, spec)
}

type WaitConfig struct {
	EventType      string       `json:"event_type"`
	EventKey       string       `json:"event_key"`
	EventKeySource *ValueSource `json:"event_key_source"`
	TimeoutAt      time.Time    `json:"timeout_at"`
}

func decodeWaitConfig(config map[string]any, data *DataContext) (WaitConfig, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return WaitConfig{}, err
	}
	var decoded WaitConfig
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return WaitConfig{}, err
	}
	if decoded.EventKeySource != nil {
		value, err := ResolveValue(data, nil, *decoded.EventKeySource)
		if err != nil {
			return WaitConfig{}, err
		}
		decoded.EventKey = fmt.Sprint(value)
	}
	if decoded.EventType == "" && decoded.EventKey == "" {
		return WaitConfig{}, fmt.Errorf("wait node requires event_type or event_key")
	}
	return decoded, nil
}

func waitConditionsMatch(conditions []WaitCondition, event ExternalEvent) bool {
	for _, condition := range conditions {
		typeMatches := condition.Type == "" || condition.Type == event.Type
		keyMatches := condition.EventKey == "" || condition.EventKey == event.Key
		if typeMatches && keyMatches {
			return true
		}
	}
	return false
}

func firstReadyTokenIndex(tokens []ExecutionToken) int {
	for i, token := range tokens {
		if token.Status == TokenReady {
			return i
		}
	}
	return -1
}

func hasWaitingToken(tokens []ExecutionToken) bool {
	for _, token := range tokens {
		if token.Status == TokenWaiting {
			return true
		}
	}
	return false
}

func isTerminalInstance(status InstanceStatus) bool {
	return status == InstanceCompleted || status == InstanceFailed || status == InstanceCancelled
}

func incomingEdges(edges []EdgeSpec) map[string][]string {
	out := map[string][]string{}
	for _, edge := range edges {
		out[edge.To] = append(out[edge.To], edge.From)
	}
	return out
}

func joinArrivedAll(state JoinState) bool {
	for _, expected := range state.ExpectedNodes {
		if !state.ArrivedNodes[expected] {
			return false
		}
	}
	return true
}
