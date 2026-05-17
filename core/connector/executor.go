package connector

import (
	"context"
	"fmt"
)

// ProtocolExecutor executes operations for one connector protocol kind.
type ProtocolExecutor interface {
	Kind() string
	ValidateConnector(connector ConnectorSpec) error
	ValidateOperation(connector ConnectorSpec, operation OperationSpec) error
	Execute(ctx context.Context, req ProtocolRequest) (ProtocolResult, error)
}

type ProtocolRequest struct {
	Connector ConnectorSpec
	Operation OperationSpec
	Call      InvokeRequest
	Input     map[string]any
}

type ProtocolResult struct {
	Output map[string]any
	Raw    any
}

type ExecutorRegistry struct {
	items map[string]ProtocolExecutor
}

func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{items: map[string]ProtocolExecutor{}}
}

func (r *ExecutorRegistry) Register(executor ProtocolExecutor) error {
	if executor == nil {
		return fmt.Errorf("protocol executor is nil")
	}
	if executor.Kind() == "" {
		return fmt.Errorf("protocol executor kind is required")
	}
	r.items[executor.Kind()] = executor
	return nil
}

func (r *ExecutorRegistry) Get(kind string) (ProtocolExecutor, bool) {
	executor, ok := r.items[kind]
	return executor, ok
}

// ProtocolFunc adapts a function into a ProtocolExecutor.
type ProtocolFunc struct {
	ProtocolKind          string
	ValidateConnectorFunc func(connector ConnectorSpec) error
	ValidateOperationFunc func(connector ConnectorSpec, operation OperationSpec) error
	ExecuteFunc           func(ctx context.Context, req ProtocolRequest) (ProtocolResult, error)
}

func (f ProtocolFunc) Kind() string { return f.ProtocolKind }

func (f ProtocolFunc) ValidateConnector(connector ConnectorSpec) error {
	if f.ValidateConnectorFunc != nil {
		return f.ValidateConnectorFunc(connector)
	}
	return nil
}

func (f ProtocolFunc) ValidateOperation(connector ConnectorSpec, operation OperationSpec) error {
	if f.ValidateOperationFunc != nil {
		return f.ValidateOperationFunc(connector, operation)
	}
	return nil
}

func (f ProtocolFunc) Execute(ctx context.Context, req ProtocolRequest) (ProtocolResult, error) {
	if f.ExecuteFunc == nil {
		return ProtocolResult{}, fmt.Errorf("protocol execute function is required")
	}
	return f.ExecuteFunc(ctx, req)
}
