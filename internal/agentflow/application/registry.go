package application

import (
	"context"
	"sync"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

type NodeRegistry struct {
	mu        sync.RWMutex
	executors map[domain.NodeType]ports.NodeExecutor
}

func NewNodeRegistry() *NodeRegistry {
	registry := &NodeRegistry{executors: map[domain.NodeType]ports.NodeExecutor{}}
	registry.Register(domain.NodeTypeStart, NoopNodeExecutor{})
	registry.Register(domain.NodeTypeJoin, NoopNodeExecutor{})
	return registry
}

func (r *NodeRegistry) Register(nodeType domain.NodeType, executor ports.NodeExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[nodeType] = executor
}

func (r *NodeRegistry) ExecutorFor(nodeType domain.NodeType) (ports.NodeExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, ok := r.executors[nodeType]
	if !ok {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "node executor is not registered")
	}
	return executor, nil
}

type NoopNodeExecutor struct{}

func (NoopNodeExecutor) ExecuteNode(ctx context.Context, request ports.NodeExecutionRequest) (domain.NodeResult, error) {
	return domain.NodeResult{Output: map[string]any{}}, nil
}
