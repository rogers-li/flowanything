package tools

import "fmt"

// ToolRepository resolves tool specs by id. A product implementation can back
// this with database snapshots, in-memory maps, or remote config service.
type ToolRepository interface {
	GetTool(id string) (ToolSpec, bool)
}

type MemoryToolRepository struct {
	items map[string]ToolSpec
}

func NewMemoryToolRepository() *MemoryToolRepository {
	return &MemoryToolRepository{items: map[string]ToolSpec{}}
}

func (r *MemoryToolRepository) Register(tool ToolSpec) error {
	if tool.ID == "" {
		return fmt.Errorf("tool id is required")
	}
	r.items[tool.ID] = tool
	return nil
}

func (r *MemoryToolRepository) GetTool(id string) (ToolSpec, bool) {
	tool, ok := r.items[id]
	return tool, ok
}

// ToolExecutor executes tools backed by one implementation kind.
type ToolExecutor interface {
	Kind() string
	Validate(tool ToolSpec) error
	Execute(ctx Context, req ToolExecutionRequest) (ToolExecutionResult, error)
}

// Context is compatible with context.Context while keeping public signatures
// compact in this core package.
type Context interface {
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

type ToolExecutionRequest struct {
	Tool  ToolSpec
	Call  ToolCall
	Input map[string]any
}

type ToolExecutionResult struct {
	Output map[string]any
	Raw    any
}

// ExecutorRegistry stores implementation adapters by kind.
type ExecutorRegistry struct {
	items map[string]ToolExecutor
}

func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{items: map[string]ToolExecutor{}}
}

func (r *ExecutorRegistry) Register(executor ToolExecutor) error {
	if executor == nil {
		return fmt.Errorf("tool executor is nil")
	}
	if executor.Kind() == "" {
		return fmt.Errorf("tool executor kind is required")
	}
	r.items[executor.Kind()] = executor
	return nil
}

func (r *ExecutorRegistry) Get(kind string) (ToolExecutor, bool) {
	executor, ok := r.items[kind]
	return executor, ok
}

// ToolFunc adapts a function into a ToolExecutor.
type ToolFunc struct {
	ImplementationKind string
	ValidateFunc       func(tool ToolSpec) error
	ExecuteFunc        func(ctx Context, req ToolExecutionRequest) (ToolExecutionResult, error)
}

func (f ToolFunc) Kind() string { return f.ImplementationKind }

func (f ToolFunc) Validate(tool ToolSpec) error {
	if f.ValidateFunc != nil {
		return f.ValidateFunc(tool)
	}
	return nil
}

func (f ToolFunc) Execute(ctx Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	if f.ExecuteFunc == nil {
		return ToolExecutionResult{}, fmt.Errorf("tool execute function is required")
	}
	return f.ExecuteFunc(ctx, req)
}
