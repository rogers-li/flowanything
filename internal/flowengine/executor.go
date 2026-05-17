package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

const (
	defaultMaxSteps       = 64
	defaultMaxParallelism = 4
)

type Executor struct {
	logger   *slog.Logger
	store    RunStore
	registry *NodeRegistry
}

func NewExecutor(logger *slog.Logger, store RunStore, registry *NodeRegistry) *Executor {
	if logger == nil {
		logger = slog.Default()
	}
	if registry == nil {
		registry = NewNodeRegistry()
	}
	return &Executor{logger: logger, store: store, registry: registry}
}

func (e *Executor) RegisterNodeExecutor(nodeType workflow.NodeType, executor NodeExecutor) {
	e.registry.Register(nodeType, executor)
}

// Execute runs a workflow DAG using the workflow-level context contract. Nodes
// exchange data by explicit ctx writes; raw node responses are recorded on the
// node run for debugging and trace inspection.
func (e *Executor) Execute(ctx context.Context, spec workflow.Spec, input map[string]any, initialContext map[string]any, traceID string) (workflow.Run, error) {
	if err := ValidateSpec(spec); err != nil {
		return workflow.Run{}, err
	}
	if e.store == nil {
		return workflow.Run{}, apperrors.New(apperrors.CodeUnavailable, "workflow run store is not configured")
	}

	run := workflow.Run{
		ID:         id.New("wfrun"),
		TenantID:   spec.TenantID,
		WorkflowID: spec.ID,
		Version:    spec.Version,
		Status:     workflow.RunStatusRunning,
		Input:      cloneMap(input),
		Context:    cloneMap(initialContext),
		TraceID:    traceID,
		StartedAt:  time.Now().UTC(),
	}
	if err := e.store.CreateRun(ctx, run); err != nil {
		return workflow.Run{}, err
	}

	runCtx := NewRunContext(input, initialContext)
	state := newExecutionState(spec.Graph, runCtx)
	err := e.executeGraph(ctx, &run, spec, state)
	now := time.Now().UTC()
	run.FinishedAt = &now
	run.Context = cloneMap(state.context.Ctx)
	if err != nil {
		run.Status = workflow.RunStatusFailed
		run.Error = err.Error()
	} else {
		run.Status = workflow.RunStatusSucceeded
		run.Output = state.context.Output()
	}
	if updateErr := e.store.UpdateRun(ctx, run); updateErr != nil && err == nil {
		err = updateErr
	}
	return run, err
}

func (e *Executor) executeGraph(ctx context.Context, run *workflow.Run, spec workflow.Spec, state *executionState) error {
	maxSteps := spec.Policy.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	maxParallelism := spec.Policy.MaxParallelism
	if maxParallelism <= 0 {
		maxParallelism = defaultMaxParallelism
	}

	if err := state.schedule(spec.Graph.EntryNodeID); err != nil {
		return err
	}
	for state.completedCount() < state.scheduledCount() {
		if state.completedCount() >= maxSteps {
			return apperrors.New(apperrors.CodeInvalidArgument, "workflow exceeded max steps")
		}
		ready := state.readyNodes()
		if len(ready) == 0 {
			return apperrors.New(apperrors.CodeConflict, "workflow cannot progress because no scheduled node is ready")
		}
		if len(ready) > maxParallelism {
			ready = ready[:maxParallelism]
		}
		if err := e.executeReadyNodes(ctx, run, spec, state, ready); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) executeReadyNodes(ctx context.Context, run *workflow.Run, spec workflow.Spec, state *executionState, nodeIDs []id.ID) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(nodeIDs))
	results := make(chan nodeExecutionResult, len(nodeIDs))

	for _, nodeID := range nodeIDs {
		nodeID := nodeID
		node := spec.Graph.Nodes[nodeID]
		state.markStarted(nodeID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := e.executeNode(ctx, *run, spec.Graph, node, state.context)
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
		nextNodeIDs, err := e.nextNodes(spec.Graph, result.nodeID, result.result, state.context)
		if err != nil {
			return err
		}
		for _, nextNodeID := range nextNodeIDs {
			if err := state.schedule(nextNodeID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Executor) executeNode(ctx context.Context, run workflow.Run, graph workflow.Graph, node workflow.Node, runContext RunContext) (NodeResult, error) {
	executor, err := e.registry.ExecutorFor(node.Type)
	if err != nil {
		return NodeResult{}, err
	}

	nodeCtx := ctx
	cancel := func() {}
	if node.TimeoutMillis > 0 {
		nodeCtx, cancel = context.WithTimeout(ctx, time.Duration(node.TimeoutMillis)*time.Millisecond)
	}
	defer cancel()

	input, err := buildNodeInput(node, runContext)
	if err != nil {
		return NodeResult{}, err
	}

	nodeRun := workflow.NodeRun{
		ID:         id.New("wfnoderun"),
		TenantID:   run.TenantID,
		RunID:      run.ID,
		WorkflowID: run.WorkflowID,
		NodeID:     node.ID,
		NodeType:   node.Type,
		NodeName:   node.Name,
		Status:     workflow.NodeRunStatusRunning,
		Input:      cloneMap(input),
		Context:    cloneMap(runContext.Ctx),
		StartedAt:  time.Now().UTC(),
	}
	if err := e.store.RecordNodeRun(ctx, nodeRun); err != nil {
		return NodeResult{}, err
	}

	rawResult, err := executor.ExecuteNode(nodeCtx, NodeExecutionRequest{
		Run:     run,
		Graph:   graph,
		Node:    node,
		Context: runContext,
		Input:   input,
	})
	now := time.Now().UTC()
	nodeRun.FinishedAt = &now
	if err != nil {
		if errors.Is(nodeCtx.Err(), context.DeadlineExceeded) {
			err = apperrors.Wrap(apperrors.CodeUnavailable, "workflow node execution timed out", err)
		}
		nodeRun.Status = workflow.NodeRunStatusFailed
		nodeRun.Error = err.Error()
		if len(rawResult.Output) > 0 {
			nodeRun.Output = cloneMap(rawResult.Output)
		}
	} else {
		mapped, mapErr := applyOutputMappings(node, runContext, rawResult.Output)
		if mapErr != nil {
			err = mapErr
			nodeRun.Status = workflow.NodeRunStatusFailed
			nodeRun.Error = err.Error()
		} else {
			mapped.NextNodeIDs = rawResult.NextNodeIDs
			mapped.Stop = rawResult.Stop
			for key, value := range rawResult.ContextWrites {
				if mapped.ContextWrites == nil {
					mapped.ContextWrites = map[string]any{}
				}
				mapped.ContextWrites[key] = value
			}
			for key, value := range rawResult.ResponseWrites {
				if mapped.ResponseWrites == nil {
					mapped.ResponseWrites = map[string]any{}
				}
				mapped.ResponseWrites[key] = value
			}
			rawResult = mapped
			nodeRun.Status = workflow.NodeRunStatusSucceeded
			nodeRun.Output = cloneMap(rawResult.Output)
		}
	}
	if recordErr := e.store.RecordNodeRun(ctx, nodeRun); recordErr != nil && err == nil {
		err = recordErr
	}
	if err != nil {
		return NodeResult{}, err
	}
	return rawResult, nil
}

func (e *Executor) nextNodes(graph workflow.Graph, nodeID id.ID, result NodeResult, runContext RunContext) ([]id.ID, error) {
	if result.Stop {
		return nil, nil
	}
	if result.NextNodeIDs != nil {
		allowed := outgoingTargets(graph, nodeID)
		next := make([]id.ID, 0, len(result.NextNodeIDs))
		for _, nextNodeID := range result.NextNodeIDs {
			if _, ok := allowed[nextNodeID]; !ok {
				return nil, apperrors.New(
					apperrors.CodeInvalidArgument,
					fmt.Sprintf("workflow node selected next node %q that is not connected by an outgoing edge", nextNodeID),
				)
			}
			next = append(next, nextNodeID)
		}
		return next, nil
	}

	next := make([]id.ID, 0)
	for _, edge := range graph.Edges {
		if edge.FromNodeID != nodeID {
			continue
		}
		if edge.Condition == nil || edgeMatches(edge.Condition, runContext) {
			next = append(next, edge.ToNodeID)
		}
	}
	return next, nil
}

func outgoingTargets(graph workflow.Graph, nodeID id.ID) map[id.ID]struct{} {
	result := map[id.ID]struct{}{}
	for _, edge := range graph.Edges {
		if edge.FromNodeID == nodeID {
			result[edge.ToNodeID] = struct{}{}
		}
	}
	return result
}

type nodeExecutionResult struct {
	nodeID id.ID
	result NodeResult
}
