package app

import (
	"context"
	"fmt"

	"flow-anything/core/agentcore"
	coreconfig "flow-anything/core/config"
	"flow-anything/core/connector"
	"flow-anything/core/flowengine"
	"flow-anything/core/runtimecontext"
	"flow-anything/core/tools"
	"flow-anything/core/trace"
	"flow-anything/core/workflow"
)

// Host is the new runtime composition root.
//
// It intentionally wires core packages directly. Product concerns such as HTTP,
// database persistence, tenant auth, and secret management should be added as
// adapters around this host rather than mixed into core execution logic.
type Host struct {
	catalog coreconfig.RuntimeCatalog

	agentRunner      *agentcore.Runner
	agentGraphRunner *agentcore.GraphRunner
	toolRuntime      *tools.Runtime
	connectorRuntime *connector.Runtime
	workflowService  *WorkflowService

	traceCollector *trace.Collector
	traceStore     trace.Store
}

type hostBuilder struct {
	traceStore         trace.Store
	toolExecutors      []tools.ToolExecutor
	connectorExecutors []connector.ProtocolExecutor
}

// HostOption customizes the runtime host without changing core packages.
type HostOption func(*hostBuilder) error

// WithTraceStore lets server deployments provide durable trace persistence.
func WithTraceStore(store trace.Store) HostOption {
	return func(builder *hostBuilder) error {
		builder.traceStore = store
		return nil
	}
}

// WithToolExecutor registers an implementation adapter such as MCP, script, or
// native platform functions.
func WithToolExecutor(executor tools.ToolExecutor) HostOption {
	return func(builder *hostBuilder) error {
		if executor == nil {
			return fmt.Errorf("tool executor is nil")
		}
		builder.toolExecutors = append(builder.toolExecutors, executor)
		return nil
	}
}

// WithConnectorProtocolExecutor registers an external protocol adapter such as
// HTTP, gRPC, or a local mock protocol for tests.
func WithConnectorProtocolExecutor(executor connector.ProtocolExecutor) HostOption {
	return func(builder *hostBuilder) error {
		if executor == nil {
			return fmt.Errorf("connector protocol executor is nil")
		}
		builder.connectorExecutors = append(builder.connectorExecutors, executor)
		return nil
	}
}

// NewHost compiles config-as-code into core runtime objects and wires event
// hooks into a shared trace collector.
func NewHost(bundle coreconfig.BundleSpec, model agentcore.ModelClient, opts ...HostOption) (*Host, error) {
	builder := hostBuilder{traceStore: trace.NewMemoryStore()}
	for _, opt := range opts {
		if err := opt(&builder); err != nil {
			return nil, err
		}
	}

	catalog, err := coreconfig.CompileRuntimeCatalog(bundle)
	if err != nil {
		return nil, err
	}

	traceCollector := trace.NewCollector(builder.traceStore)
	connectorRuntime, err := buildConnectorRuntime(catalog, traceCollector, builder.connectorExecutors)
	if err != nil {
		return nil, err
	}

	workflowService := &WorkflowService{}
	toolRuntime, err := buildToolRuntime(catalog, traceCollector, connectorRuntime, workflowService, builder.toolExecutors)
	if err != nil {
		return nil, err
	}

	capabilities := agentcore.NewMapCapabilityRegistry()
	agentRunner := agentcore.NewRunner(
		model,
		agentcore.WithCapabilities(capabilities),
		agentcore.WithEventHook(traceCollector),
	)
	agentGraphRunner := agentcore.NewGraphRunner(
		model,
		agentcore.WithGraphCapabilities(capabilities),
		agentcore.WithGraphEventHook(traceCollector),
	)

	host := &Host{
		catalog:          catalog,
		agentRunner:      agentRunner,
		agentGraphRunner: agentGraphRunner,
		toolRuntime:      toolRuntime,
		connectorRuntime: connectorRuntime,
		workflowService:  workflowService,
		traceCollector:   traceCollector,
		traceStore:       builder.traceStore,
	}

	if err := host.installWorkflowRuntime(workflowService); err != nil {
		return nil, err
	}
	if err := host.registerAgentCapabilities(capabilities); err != nil {
		return nil, err
	}
	return host, nil
}

func (h *Host) Catalog() coreconfig.RuntimeCatalog {
	return h.catalog
}

func (h *Host) TraceStore() trace.Store {
	return h.traceStore
}

func (h *Host) TraceCollector() *trace.Collector {
	return h.traceCollector
}

func buildConnectorRuntime(catalog coreconfig.RuntimeCatalog, collector *trace.Collector, executors []connector.ProtocolExecutor) (*connector.Runtime, error) {
	repository := connector.NewMemoryRepository()
	for _, spec := range catalog.Connectors {
		if err := repository.RegisterConnector(spec); err != nil {
			return nil, err
		}
	}
	for _, operation := range catalog.ConnectorOperations {
		if err := repository.RegisterOperation(operation); err != nil {
			return nil, err
		}
	}

	registry := connector.NewExecutorRegistry()
	for _, executor := range executors {
		if err := registry.Register(executor); err != nil {
			return nil, err
		}
	}
	return connector.NewRuntime(repository, registry, connector.WithEventHook(collector)), nil
}

func buildToolRuntime(catalog coreconfig.RuntimeCatalog, collector *trace.Collector, connectors *connector.Runtime, workflows *WorkflowService, executors []tools.ToolExecutor) (*tools.Runtime, error) {
	repository := tools.NewMemoryToolRepository()
	for _, spec := range catalog.Tools {
		if err := repository.Register(spec); err != nil {
			return nil, err
		}
	}

	registry := tools.NewExecutorRegistry()
	for _, executor := range []tools.ToolExecutor{
		connectorToolExecutor{connectors: connectors},
		workflowToolExecutor{workflows: workflows},
	} {
		if err := registry.Register(executor); err != nil {
			return nil, err
		}
	}
	for _, executor := range executors {
		if err := registry.Register(executor); err != nil {
			return nil, err
		}
	}
	return tools.NewRuntime(repository, registry, tools.WithEventHook(collector)), nil
}

func (h *Host) installWorkflowRuntime(service *WorkflowService) error {
	registry, err := workflow.NewDefaultWorkflowRegistry(workflow.NodeRuntime{
		Connectors: connectorNodeInvoker{runtime: h.connectorRuntime},
		Tools:      toolNodeInvoker{runtime: h.toolRuntime},
		Agents:     agentNodeRunner{host: h},
	})
	if err != nil {
		return err
	}
	store := flowengine.NewMemoryInstanceStore()
	engine := flowengine.NewStatefulExecutor(registry, store, flowengine.WithStatefulEventHook(h.traceCollector))
	service.catalog = h.catalog
	service.compiler = workflow.NewCompiler(registry)
	service.runtime = workflow.NewRuntime(engine)
	return nil
}

func (h *Host) registerAgentCapabilities(registry *agentcore.MapCapabilityRegistry) error {
	for _, skill := range h.catalog.Skills {
		if err := registry.Register(agentcore.NewSkillCapability(h.skillSpec(skill), h.agentRunner)); err != nil {
			return err
		}
	}
	for _, spec := range h.catalog.Tools {
		spec := spec
		if err := registry.Register(agentcore.CapabilityFunc{
			Desc: agentcore.CapabilityDescriptor{
				ID:           spec.ID,
				Type:         string(coreconfig.ResourceTool),
				Name:         spec.Name,
				Description:  spec.Description,
				InputSchema:  spec.InputSchema,
				OutputSchema: spec.OutputSchema,
			},
			Fn: func(ctx agentcore.Context, call agentcore.CapabilityCall) (agentcore.CapabilityResult, error) {
				return h.invokeToolCapability(ctx, spec.ID, call)
			},
		}); err != nil {
			return err
		}
	}
	for _, document := range h.catalog.Workflows {
		document := document
		if err := registry.Register(agentcore.CapabilityFunc{
			Desc: agentcore.CapabilityDescriptor{
				ID:          document.ID,
				Type:        string(coreconfig.ResourceWorkflow),
				Name:        document.Spec.Name,
				Description: document.Spec.Name,
			},
			Fn: func(ctx agentcore.Context, call agentcore.CapabilityCall) (agentcore.CapabilityResult, error) {
				return h.invokeWorkflowCapability(ctx, document.ID, call)
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Host) skillSpec(skill coreconfig.SkillConfig) agentcore.SkillSpec {
	capabilities := make([]agentcore.CapabilityDescriptor, 0, len(skill.Tools))
	for _, binding := range skill.Tools {
		if binding.Disabled {
			continue
		}
		if tool, ok := h.catalog.Tools[binding.Ref.ID]; ok {
			capabilities = append(capabilities, agentcore.CapabilityDescriptor{
				ID:           tool.ID,
				Type:         string(coreconfig.ResourceTool),
				Name:         tool.Name,
				Description:  tool.Description,
				InputSchema:  tool.InputSchema,
				OutputSchema: tool.OutputSchema,
			})
		}
	}
	maxActions := intFromMetadata(skill.Metadata, "max_tool_calls", 6)
	return agentcore.SkillSpec{
		ID:            skill.ID,
		Name:          skill.Name,
		Description:   skill.Description,
		Prompt:        coreconfig.BuildPromptText(skill.Prompt),
		ReasoningMode: skillReasoningMode(capabilities),
		Model:         h.defaultModelConfig(),
		InputSchema:   skill.InputSchema,
		OutputSchema:  skill.OutputSchema,
		Capabilities:  capabilities,
		Policy: agentcore.AgentPolicy{
			MaxIterations: maxActions,
			MaxActions:    maxActions,
		},
	}
}

func skillReasoningMode(capabilities []agentcore.CapabilityDescriptor) string {
	if len(capabilities) == 0 {
		return agentcore.DirectStrategy{}.Name()
	}
	return agentcore.ReWOOStrategy{}.Name()
}

func (h *Host) defaultModelConfig() agentcore.ModelConfig {
	for _, model := range h.catalog.Bundle.Resources.Models {
		if runtimeModel, ok := h.catalog.Models[model.ID]; ok {
			return runtimeModel
		}
	}
	for _, model := range h.catalog.Models {
		return model
	}
	return agentcore.ModelConfig{}
}

func intFromMetadata(metadata map[string]any, key string, fallback int) int {
	value, ok := metadata[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case float32:
		if typed > 0 {
			return int(typed)
		}
	}
	return fallback
}

func (h *Host) invokeToolCapability(ctx agentcore.Context, toolID string, call agentcore.CapabilityCall) (agentcore.CapabilityResult, error) {
	result, err := h.InvokeTool(contextFromAgent(ctx), ToolRequest{
		ToolID:       toolID,
		Input:        call.Input,
		TraceID:      call.TraceID,
		TraceContext: childTraceContext(call.TraceContext),
	})
	if err != nil {
		return agentcore.CapabilityResult{}, err
	}
	return agentcore.CapabilityResult{
		ID:     toolID,
		Type:   call.Type,
		Output: result.Output,
		Raw:    result.Raw,
	}, nil
}

func (h *Host) invokeWorkflowCapability(ctx agentcore.Context, workflowID string, call agentcore.CapabilityCall) (agentcore.CapabilityResult, error) {
	result, err := h.RunWorkflow(contextFromAgent(ctx), WorkflowRequest{
		WorkflowID:   workflowID,
		Input:        call.Input,
		TraceContext: childTraceContext(call.TraceContext),
	})
	if err != nil {
		return agentcore.CapabilityResult{}, err
	}
	return agentcore.CapabilityResult{
		ID:     workflowID,
		Type:   string(coreconfig.ResourceWorkflow),
		Output: result.Output,
		Raw:    result.Instance,
	}, nil
}

func contextFromAgent(ctx agentcore.Context) context.Context {
	if standard, ok := ctx.(context.Context); ok {
		return standard
	}
	return context.Background()
}

func traceContextFrom(ctx context.Context) runtimecontext.TraceContext {
	traceContext, _ := runtimecontext.TraceContextFrom(ctx)
	return traceContext
}

func childTraceContext(parent runtimecontext.TraceContext) runtimecontext.TraceContext {
	return runtimecontext.TraceContext{
		TraceID:       parent.TraceID,
		ParentSpanID:  parent.SpanID,
		CorrelationID: parent.CorrelationID,
	}
}
