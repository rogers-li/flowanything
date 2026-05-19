package agentcore

import "fmt"

// CapabilityRegistry resolves planned actions to executable capabilities.
type CapabilityRegistry interface {
	Get(id string) (Capability, bool)
	List() []CapabilityDescriptor
}

// MapCapabilityRegistry is an in-memory registry for tests and local adapters.
type MapCapabilityRegistry struct {
	items map[string]Capability
}

func NewMapCapabilityRegistry() *MapCapabilityRegistry {
	return &MapCapabilityRegistry{items: map[string]Capability{}}
}

func (r *MapCapabilityRegistry) Register(capability Capability) error {
	if capability == nil {
		return fmt.Errorf("capability is nil")
	}
	descriptor := capability.Descriptor()
	if descriptor.ID == "" {
		return fmt.Errorf("capability id is required")
	}
	r.items[descriptor.ID] = capability
	return nil
}

func (r *MapCapabilityRegistry) Get(id string) (Capability, bool) {
	item, ok := r.items[id]
	return item, ok
}

func (r *MapCapabilityRegistry) List() []CapabilityDescriptor {
	out := make([]CapabilityDescriptor, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item.Descriptor())
	}
	return out
}

// CompositeCapabilityRegistry overlays multiple registries. Earlier registries
// take precedence, which lets a graph node expose local Sub-Agent capabilities
// without mutating the shared platform registry.
type CompositeCapabilityRegistry struct {
	registries []CapabilityRegistry
}

func NewCompositeCapabilityRegistry(registries ...CapabilityRegistry) *CompositeCapabilityRegistry {
	out := make([]CapabilityRegistry, 0, len(registries))
	for _, registry := range registries {
		if registry != nil {
			out = append(out, registry)
		}
	}
	return &CompositeCapabilityRegistry{registries: out}
}

func (r *CompositeCapabilityRegistry) Get(id string) (Capability, bool) {
	for _, registry := range r.registries {
		if capability, ok := registry.Get(id); ok {
			return capability, true
		}
	}
	return nil, false
}

func (r *CompositeCapabilityRegistry) List() []CapabilityDescriptor {
	seen := map[string]struct{}{}
	out := []CapabilityDescriptor{}
	for _, registry := range r.registries {
		for _, descriptor := range registry.List() {
			if _, exists := seen[descriptor.ID]; exists {
				continue
			}
			seen[descriptor.ID] = struct{}{}
			out = append(out, descriptor)
		}
	}
	return out
}

// CapabilityFunc adapts a function into a Capability.
type CapabilityFunc struct {
	Desc CapabilityDescriptor
	Fn   func(ctx Context, call CapabilityCall) (CapabilityResult, error)
}

func (c CapabilityFunc) Descriptor() CapabilityDescriptor { return c.Desc }

func (c CapabilityFunc) Invoke(ctx Context, call CapabilityCall) (CapabilityResult, error) {
	return c.Fn(ctx, call)
}

// ReasoningStrategy runs one LLM reasoning mode.
type ReasoningStrategy interface {
	Name() string
	Run(ctx Context, runtime StrategyRuntime, req AgentRunRequest) (AgentRunResult, error)
}

// StrategyRegistry stores pluggable reasoning strategies.
type StrategyRegistry struct {
	items map[string]ReasoningStrategy
}

func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{items: map[string]ReasoningStrategy{}}
}

func NewDefaultStrategyRegistry() *StrategyRegistry {
	registry := NewStrategyRegistry()
	_ = registry.Register(DirectStrategy{})
	_ = registry.Register(ActionPlanningStrategy{})
	_ = registry.Register(ReWOOStrategy{})
	_ = registry.Register(ReActStrategy{})
	return registry
}

func (r *StrategyRegistry) Register(strategy ReasoningStrategy) error {
	if strategy == nil {
		return fmt.Errorf("reasoning strategy is nil")
	}
	if strategy.Name() == "" {
		return fmt.Errorf("reasoning strategy name is required")
	}
	r.items[strategy.Name()] = strategy
	return nil
}

func (r *StrategyRegistry) Get(name string) (ReasoningStrategy, bool) {
	strategy, ok := r.items[name]
	return strategy, ok
}
