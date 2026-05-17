package flowengine

import (
	"context"
	"fmt"
)

// NodeExecutor executes one node type.
type NodeExecutor interface {
	Type() string
	Validate(ctx context.Context, node NodeSpec) error
	Execute(ctx context.Context, req NodeRequest) (NodeResult, error)
}

// NodeFunc is a convenience adapter for tests and simple nodes.
type NodeFunc func(ctx context.Context, req NodeRequest) (NodeResult, error)

// FuncNodeExecutor adapts a function into a NodeExecutor.
type FuncNodeExecutor struct {
	NodeType string
	Fn       NodeFunc
}

func (e FuncNodeExecutor) Type() string { return e.NodeType }

func (e FuncNodeExecutor) Validate(context.Context, NodeSpec) error {
	if e.NodeType == "" {
		return fmt.Errorf("node type is required")
	}
	if e.Fn == nil {
		return fmt.Errorf("node function is required")
	}
	return nil
}

func (e FuncNodeExecutor) Execute(ctx context.Context, req NodeRequest) (NodeResult, error) {
	return e.Fn(ctx, req)
}

// Registry maps node type names to executors.
type Registry struct {
	executors map[string]NodeExecutor
}

func NewRegistry() *Registry {
	return &Registry{executors: map[string]NodeExecutor{}}
}

func (r *Registry) Register(executor NodeExecutor) error {
	if executor == nil {
		return fmt.Errorf("node executor is nil")
	}
	if executor.Type() == "" {
		return fmt.Errorf("node executor type is required")
	}
	r.executors[executor.Type()] = executor
	return nil
}

func (r *Registry) Get(nodeType string) (NodeExecutor, bool) {
	executor, ok := r.executors[nodeType]
	return executor, ok
}
