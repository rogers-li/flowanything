package workflow

import (
	"fmt"

	"flow-anything/core/flowengine"
)

// RegisterWorkflowNodes installs platform workflow node executors into a
// flowengine registry. Control-flow nodes are also registered for convenience.
func RegisterWorkflowNodes(registry *flowengine.Registry, runtime NodeRuntime) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if err := flowengine.RegisterControlNodes(registry); err != nil {
		return err
	}
	if runtime.Transforms == nil {
		runtime.Transforms = NewDefaultTransformRegistry()
	}
	for _, executor := range []flowengine.NodeExecutor{
		NewTransformNodeExecutor(runtime.Transforms),
		NewConnectorNodeExecutor(runtime.Connectors),
		NewToolNodeExecutor(runtime.Tools),
		NewAgentNodeExecutor(runtime.Agents),
	} {
		if err := registry.Register(executor); err != nil {
			return err
		}
	}
	return nil
}

func NewDefaultWorkflowRegistry(runtime NodeRuntime) (*flowengine.Registry, error) {
	registry := flowengine.NewDefaultRegistry()
	if err := RegisterWorkflowNodes(registry, runtime); err != nil {
		return nil, err
	}
	return registry, nil
}
