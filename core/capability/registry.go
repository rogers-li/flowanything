package capability

import (
	"fmt"
	"sort"
)

type Filter struct {
	Kinds           []Kind
	IncludeDisabled bool
}

type Registry interface {
	Register(capability Capability) error
	Get(id string) (Capability, bool)
	List(filter Filter) []Descriptor
}

type MapRegistry struct {
	items map[string]Capability
}

func NewMapRegistry(capabilities ...Capability) (*MapRegistry, error) {
	registry := &MapRegistry{items: map[string]Capability{}}
	for _, item := range capabilities {
		if err := registry.Register(item); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *MapRegistry) Register(capability Capability) error {
	if r.items == nil {
		r.items = map[string]Capability{}
	}
	descriptor := capability.Descriptor()
	if descriptor.ID == "" {
		return fmt.Errorf("capability id is required")
	}
	if _, exists := r.items[descriptor.ID]; exists {
		return fmt.Errorf("duplicate capability %q", descriptor.ID)
	}
	r.items[descriptor.ID] = capability
	return nil
}

func (r *MapRegistry) Get(id string) (Capability, bool) {
	item, ok := r.items[id]
	return item, ok
}

func (r *MapRegistry) List(filter Filter) []Descriptor {
	allowed := map[Kind]bool{}
	for _, kind := range filter.Kinds {
		allowed[kind] = true
	}
	out := make([]Descriptor, 0, len(r.items))
	for _, item := range r.items {
		descriptor := item.Descriptor()
		if len(allowed) > 0 && !allowed[descriptor.Kind] {
			continue
		}
		if descriptor.Disabled && !filter.IncludeDisabled {
			continue
		}
		out = append(out, descriptor)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
