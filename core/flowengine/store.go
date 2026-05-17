package flowengine

import (
	"context"
	"fmt"
	"sync"
)

// InstanceStore persists stateful flow instances and append-only events.
type InstanceStore interface {
	Create(ctx context.Context, instance FlowInstance) error
	Get(ctx context.Context, instanceID string) (FlowInstance, error)
	Save(ctx context.Context, instance FlowInstance) error
	AppendEvent(ctx context.Context, event FlowInstanceEvent) error
}

// MemoryInstanceStore is a simple in-memory store for tests and local demos.
type MemoryInstanceStore struct {
	mu        sync.Mutex
	instances map[string]FlowInstance
	events    []FlowInstanceEvent
}

func NewMemoryInstanceStore() *MemoryInstanceStore {
	return &MemoryInstanceStore{
		instances: map[string]FlowInstance{},
		events:    []FlowInstanceEvent{},
	}
}

func (s *MemoryInstanceStore) Create(_ context.Context, instance FlowInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if instance.InstanceID == "" {
		return fmt.Errorf("instance id is required")
	}
	if _, exists := s.instances[instance.InstanceID]; exists {
		return fmt.Errorf("flow instance %q already exists", instance.InstanceID)
	}
	s.instances[instance.InstanceID] = cloneFlowInstance(instance)
	return nil
}

func (s *MemoryInstanceStore) Get(_ context.Context, instanceID string) (FlowInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	instance, ok := s.instances[instanceID]
	if !ok {
		return FlowInstance{}, fmt.Errorf("flow instance %q not found", instanceID)
	}
	return cloneFlowInstance(instance), nil
}

func (s *MemoryInstanceStore) Save(_ context.Context, instance FlowInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if instance.InstanceID == "" {
		return fmt.Errorf("instance id is required")
	}
	s.instances[instance.InstanceID] = cloneFlowInstance(instance)
	return nil
}

func (s *MemoryInstanceStore) AppendEvent(_ context.Context, event FlowInstanceEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func cloneFlowInstance(instance FlowInstance) FlowInstance {
	cloned := instance
	cloned.Context = cloneDataContext(instance.Context)
	cloned.Tokens = append([]ExecutionToken(nil), instance.Tokens...)
	for i := range cloned.Tokens {
		cloned.Tokens[i].Payload = cloneMapRecursive(instance.Tokens[i].Payload)
		cloned.Tokens[i].WaitingFor = append([]WaitCondition(nil), instance.Tokens[i].WaitingFor...)
	}
	cloned.NodeStates = make(map[string]NodeState, len(instance.NodeStates))
	for key, state := range instance.NodeStates {
		state.Input = cloneMapRecursive(state.Input)
		state.Output = cloneMapRecursive(state.Output)
		state.WaitingFor = append([]WaitCondition(nil), state.WaitingFor...)
		cloned.NodeStates[key] = state
	}
	cloned.JoinStates = make(map[string]JoinState, len(instance.JoinStates))
	for key, state := range instance.JoinStates {
		state.ExpectedNodes = append([]string(nil), state.ExpectedNodes...)
		state.ArrivedNodes = cloneBoolMap(state.ArrivedNodes)
		cloned.JoinStates[key] = state
	}
	return cloned
}

func cloneDataContext(data *DataContext) *DataContext {
	if data == nil {
		return NewDataContext(nil)
	}
	return &DataContext{
		FlowInput:   cloneMapRecursive(data.FlowInput),
		FlowOutput:  cloneMapRecursive(data.FlowOutput),
		Variables:   cloneMapRecursive(data.Variables),
		NodeContext: cloneMapRecursive(data.NodeContext),
	}
}

func cloneMapRecursive(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = cloneAny(value)
	}
	return cloned
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMapRecursive(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAny(item)
		}
		return out
	default:
		return value
	}
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return map[string]bool{}
	}
	cloned := make(map[string]bool, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
