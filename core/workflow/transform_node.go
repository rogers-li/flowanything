package workflow

import (
	"context"
	"fmt"

	"flow-anything/core/flowengine"
)

type TransformFunction interface {
	Name() string
	Validate(config TransformNodeConfig) error
	Apply(ctx context.Context, input map[string]any, config TransformNodeConfig) (map[string]any, error)
}

type TransformRegistry struct {
	items map[string]TransformFunction
}

func NewTransformRegistry() *TransformRegistry {
	return &TransformRegistry{items: map[string]TransformFunction{}}
}

func NewDefaultTransformRegistry() *TransformRegistry {
	registry := NewTransformRegistry()
	_ = registry.Register(IdentityTransform{})
	_ = registry.Register(RemoveFieldsTransform{})
	_ = registry.Register(StringConcatTransform{})
	return registry
}

func (r *TransformRegistry) Register(fn TransformFunction) error {
	if fn == nil {
		return fmt.Errorf("transform function is nil")
	}
	if fn.Name() == "" {
		return fmt.Errorf("transform function name is required")
	}
	r.items[fn.Name()] = fn
	return nil
}

func (r *TransformRegistry) Get(name string) (TransformFunction, bool) {
	fn, ok := r.items[name]
	return fn, ok
}

type TransformNodeConfig struct {
	Function string         `json:"function"`
	Args     map[string]any `json:"args"`
}

type TransformNodeExecutor struct {
	registry *TransformRegistry
}

func NewTransformNodeExecutor(registry *TransformRegistry) TransformNodeExecutor {
	if registry == nil {
		registry = NewDefaultTransformRegistry()
	}
	return TransformNodeExecutor{registry: registry}
}

func (e TransformNodeExecutor) Type() string { return NodeTypeTransform }

func (e TransformNodeExecutor) Validate(_ context.Context, node flowengine.NodeSpec) error {
	config, err := decodeNodeConfig[TransformNodeConfig](node.Config)
	if err != nil {
		return err
	}
	if config.Function == "" {
		return fmt.Errorf("transform function is required")
	}
	fn, ok := e.registry.Get(config.Function)
	if !ok {
		return fmt.Errorf("transform function %q is not registered", config.Function)
	}
	return fn.Validate(config)
}

func (e TransformNodeExecutor) Execute(ctx context.Context, req flowengine.NodeRequest) (flowengine.NodeResult, error) {
	config, err := decodeNodeConfig[TransformNodeConfig](req.Node.Config)
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	fn, ok := e.registry.Get(config.Function)
	if !ok {
		return flowengine.NodeResult{}, fmt.Errorf("transform function %q is not registered", config.Function)
	}
	output, err := fn.Apply(ctx, req.Input, config)
	if err != nil {
		return flowengine.NodeResult{}, err
	}
	return flowengine.NodeResult{Output: output}, nil
}

type IdentityTransform struct{}

func (IdentityTransform) Name() string                       { return "identity" }
func (IdentityTransform) Validate(TransformNodeConfig) error { return nil }
func (IdentityTransform) Apply(_ context.Context, input map[string]any, _ TransformNodeConfig) (map[string]any, error) {
	return cloneMap(input), nil
}

type RemoveFieldsTransform struct{}

func (RemoveFieldsTransform) Name() string { return "json.remove_fields" }
func (RemoveFieldsTransform) Validate(config TransformNodeConfig) error {
	_, err := stringSliceArg(config.Args, "fields")
	return err
}
func (RemoveFieldsTransform) Apply(_ context.Context, input map[string]any, config TransformNodeConfig) (map[string]any, error) {
	fields, err := stringSliceArg(config.Args, "fields")
	if err != nil {
		return nil, err
	}
	output := cloneMap(input)
	for _, field := range fields {
		deleteNested(output, field)
	}
	return output, nil
}

type StringConcatTransform struct{}

func (StringConcatTransform) Name() string { return "string.concat" }
func (StringConcatTransform) Validate(config TransformNodeConfig) error {
	_, err := stringSliceArg(config.Args, "fields")
	return err
}
func (StringConcatTransform) Apply(_ context.Context, input map[string]any, config TransformNodeConfig) (map[string]any, error) {
	fields, err := stringSliceArg(config.Args, "fields")
	if err != nil {
		return nil, err
	}
	separator, _ := config.Args["separator"].(string)
	value := ""
	for i, field := range fields {
		if i > 0 {
			value += separator
		}
		value += fmt.Sprint(readNested(input, field))
	}
	outputField, _ := config.Args["output_field"].(string)
	if outputField == "" {
		outputField = "text"
	}
	output := cloneMap(input)
	writeNested(output, outputField, value)
	return output, nil
}
