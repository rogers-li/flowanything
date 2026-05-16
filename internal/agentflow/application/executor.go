package application

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/flowengine"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

const (
	defaultMaxSteps       = 64
	defaultMaxParallelism = 4
)

type Executor struct {
	logger   *slog.Logger
	store    ports.RunStore
	tracer   ports.TraceEmitter
	registry *NodeRegistry

	flowEngineRegistry *flowengine.NodeRegistry
}

func NewExecutor(logger *slog.Logger, store ports.RunStore, registry *NodeRegistry, tracer ports.TraceEmitter) *Executor {
	if registry == nil {
		registry = NewNodeRegistry()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Executor{
		logger:   logger,
		store:    store,
		tracer:   tracer,
		registry: registry,
	}
}

func (e *Executor) RegisterNodeExecutor(nodeType domain.NodeType, executor ports.NodeExecutor) {
	e.registry.Register(nodeType, executor)
}

func (e *Executor) WithFlowEngineRegistry(registry *flowengine.NodeRegistry) *Executor {
	e.flowEngineRegistry = registry
	return e
}

// Execute runs an Agent Flow graph to completion using deterministic graph
// scheduling. The first implementation is synchronous, but every run and node
// transition goes through RunStore so the API can evolve toward durable async
// execution without changing node executor contracts.
func (e *Executor) Execute(ctx context.Context, graph domain.FlowGraph, input map[string]any) (domain.FlowRun, error) {
	if err := domain.ValidateGraph(graph); err != nil {
		return domain.FlowRun{}, err
	}
	if e.store == nil {
		return domain.FlowRun{}, apperrors.New(apperrors.CodeUnavailable, "agent flow run store is not configured")
	}
	if e.canExecuteWithFlowEngine(graph) {
		return e.executeWithFlowEngine(ctx, graph, input)
	}

	run := domain.FlowRun{
		ID:          id.New("flowrun"),
		TenantID:    graph.TenantID,
		FlowID:      graph.ID,
		FlowVersion: graph.Version,
		Status:      domain.RunStatusRunning,
		Input:       cloneStringMap(input),
		StartedAt:   time.Now().UTC(),
	}
	if err := e.store.CreateRun(ctx, run); err != nil {
		return domain.FlowRun{}, err
	}
	if e.tracer != nil {
		e.tracer.FlowRunStarted(ctx, run)
	}

	state := newExecutionState(graph, domain.NewRunContext(input))
	err := e.executeGraph(ctx, &run, graph, state)
	now := time.Now().UTC()
	run.FinishedAt = &now
	if err != nil {
		run.Status = domain.RunStatusFailed
		run.Error = err.Error()
	} else {
		run.Status = domain.RunStatusSucceeded
		run.Output = state.context.Variables
	}
	if updateErr := e.store.UpdateRun(ctx, run); updateErr != nil && err == nil {
		err = updateErr
	}
	if e.tracer != nil {
		e.tracer.FlowRunFinished(ctx, run)
	}
	return run, err
}

func (e *Executor) executeGraph(ctx context.Context, run *domain.FlowRun, graph domain.FlowGraph, state *executionState) error {
	maxSteps := graph.Policy.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	maxParallelism := graph.Policy.MaxParallelism
	if maxParallelism <= 0 {
		maxParallelism = defaultMaxParallelism
	}

	if err := state.schedule(graph.EntryNodeID); err != nil {
		return err
	}
	for state.completedCount() < state.scheduledCount() {
		if state.completedCount() >= maxSteps {
			return apperrors.New(apperrors.CodeInvalidArgument, "agent flow exceeded max steps")
		}

		ready := state.readyNodes()
		if len(ready) == 0 {
			return apperrors.New(apperrors.CodeConflict, "agent flow cannot progress because no scheduled node is ready")
		}
		if len(ready) > maxParallelism {
			ready = ready[:maxParallelism]
		}
		if err := e.executeReadyNodes(ctx, run, graph, state, ready); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) executeReadyNodes(ctx context.Context, run *domain.FlowRun, graph domain.FlowGraph, state *executionState, nodeIDs []id.ID) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(nodeIDs))
	results := make(chan nodeExecutionResult, len(nodeIDs))

	for _, nodeID := range nodeIDs {
		nodeID := nodeID
		node := graph.Nodes[nodeID]
		state.markStarted(nodeID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := e.executeNode(ctx, *run, node, state.context)
			if err != nil {
				errs <- err
				return
			}
			results <- nodeExecutionResult{nodeID: nodeID, result: result}
		}()
	}

	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	for result := range results {
		state.complete(result.nodeID, result.result)
		for _, nextNodeID := range e.nextNodes(graph, result.nodeID, result.result, state.context) {
			if err := state.schedule(nextNodeID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Executor) executeNode(ctx context.Context, run domain.FlowRun, node domain.Node, runContext domain.RunContext) (domain.NodeResult, error) {
	executor, err := e.registry.ExecutorFor(node.Type)
	if err != nil {
		return domain.NodeResult{}, err
	}

	nodeCtx := ctx
	cancel := func() {}
	if node.TimeoutMillis > 0 {
		nodeCtx, cancel = context.WithTimeout(ctx, time.Duration(node.TimeoutMillis)*time.Millisecond)
	}
	defer cancel()

	nodeRun := domain.NodeRun{
		ID:       id.New("noderun"),
		TenantID: run.TenantID,
		RunID:    run.ID,
		FlowID:   run.FlowID,
		NodeID:   node.ID,
		NodeType: node.Type,
		NodeName: node.Name,
		Status:   domain.NodeRunStatusRunning,
		Input: map[string]any{
			"input":     runContext.Input,
			"variables": runContext.Variables,
		},
		StartedAt: time.Now().UTC(),
	}
	if err := e.store.RecordNodeRun(ctx, nodeRun); err != nil {
		return domain.NodeResult{}, err
	}
	if e.tracer != nil {
		e.tracer.NodeRunStarted(ctx, nodeRun)
	}

	result, err := executor.ExecuteNode(nodeCtx, ports.NodeExecutionRequest{
		Run:     run,
		Node:    node,
		Context: runContext,
	})
	now := time.Now().UTC()
	nodeRun.FinishedAt = &now
	if err != nil {
		if errors.Is(nodeCtx.Err(), context.DeadlineExceeded) {
			err = apperrors.Wrap(apperrors.CodeUnavailable, "agent flow node execution timed out", err)
		}
		nodeRun.Status = domain.NodeRunStatusFailed
		nodeRun.Error = err.Error()
	} else {
		nodeRun.Status = domain.NodeRunStatusSucceeded
		nodeRun.Output = result.Output
	}
	if recordErr := e.store.RecordNodeRun(ctx, nodeRun); recordErr != nil && err == nil {
		err = recordErr
	}
	if e.tracer != nil {
		e.tracer.NodeRunFinished(ctx, nodeRun)
	}
	if err != nil {
		return domain.NodeResult{}, err
	}
	return result, nil
}

func (e *Executor) nextNodes(graph domain.FlowGraph, nodeID id.ID, result domain.NodeResult, runContext domain.RunContext) []id.ID {
	if result.Stop {
		return nil
	}
	if result.NextNodeIDs != nil {
		return result.NextNodeIDs
	}

	next := make([]id.ID, 0)
	for _, edge := range graph.Edges {
		if edge.FromNodeID != nodeID {
			continue
		}
		if edge.Condition == nil || edge.Condition.Matches(runContext) {
			next = append(next, edge.ToNodeID)
		}
	}
	return next
}

type nodeExecutionResult struct {
	nodeID id.ID
	result domain.NodeResult
}

func cloneStringMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
