package flowengine

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

type NodeExecutionRequest struct {
	Run     workflow.Run
	Graph   workflow.Graph
	Node    workflow.Node
	Context RunContext
	Input   map[string]any
}

type NodeExecutor interface {
	ExecuteNode(ctx context.Context, request NodeExecutionRequest) (NodeResult, error)
}

type NodeRegistry struct {
	mu        sync.RWMutex
	executors map[workflow.NodeType]NodeExecutor
}

func NewNodeRegistry() *NodeRegistry {
	registry := &NodeRegistry{executors: map[workflow.NodeType]NodeExecutor{}}
	registry.Register(workflow.NodeTypeStart, NoopNodeExecutor{})
	registry.Register(workflow.NodeTypeJoin, NoopNodeExecutor{})
	registry.Register(workflow.NodeTypeEnd, EndNodeExecutor{})
	return registry
}

func (r *NodeRegistry) Register(nodeType workflow.NodeType, executor NodeExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[nodeType] = executor
}

func (r *NodeRegistry) ExecutorFor(nodeType workflow.NodeType) (NodeExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, ok := r.executors[nodeType]
	if !ok {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "workflow node executor is not registered")
	}
	return executor, nil
}

type NoopNodeExecutor struct{}

func (NoopNodeExecutor) ExecuteNode(ctx context.Context, request NodeExecutionRequest) (NodeResult, error) {
	return NodeResult{Output: map[string]any{}}, nil
}

type EndNodeExecutor struct{}

func (EndNodeExecutor) ExecuteNode(ctx context.Context, request NodeExecutionRequest) (NodeResult, error) {
	output := cloneMap(request.Context.Ctx)
	if mapping, ok := mapConfig(request.Node.Config, "output_mapping"); ok && len(mapping) > 0 {
		output = map[string]any{}
		for target, source := range mapping {
			value, err := evaluateConfigValue(source, request.Context, nil, nil)
			if err != nil {
				return NodeResult{}, err
			}
			output[target] = value
		}
	}
	return NodeResult{Output: output, Stop: true}, nil
}
