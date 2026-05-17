package main

import (
	"os"

	agentflowapp "flow-anything/internal/agentflow/application"
	"flow-anything/internal/agentflow/domain"
	agentflowinfra "flow-anything/internal/agentflow/infrastructure"
	agentflowhttp "flow-anything/internal/agentflow/interfaces/http"
	"flow-anything/internal/flowengine"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
	"flow-anything/internal/workflowruntime"
)

func main() {
	logger := logging.New("agent-flow-runtime")

	store := agentflowinfra.NewMemoryRunStore()
	registry := agentflowapp.NewNodeRegistry()
	platformAPIURL := config.String("PLATFORM_API_URL", "http://localhost:8080")
	agentInvoker := agentflowinfra.NewHTTPAgentInvoker(config.String("AI_ORCHESTRATOR_URL", "http://localhost:8081"))
	registry.Register(domain.NodeTypeAgent, agentflowapp.NewAgentNodeExecutor(
		agentInvoker,
	))
	workflowRegistry := flowengine.NewNodeRegistry()
	workflowRegistry.Register(workflow.NodeTypeConnectorOperation, workflowruntime.NewConnectorOperationNodeExecutor(
		workflowruntime.NewHTTPConnectorInvoker(config.String("CONNECTOR_SERVICE_URL", "http://localhost:8083")),
	))
	workflowRegistry.Register(workflow.NodeTypeTool, workflowruntime.NewToolNodeExecutor(
		workflowruntime.NewHTTPToolRuntime(config.String("AGENT_RUNTIME_URL", "http://localhost:8082")),
	))
	workflowRegistry.Register(workflow.NodeTypeTransform, workflowruntime.TransformNodeExecutor{})
	workflowRegistry.Register(workflow.NodeTypeCondition, workflowruntime.ConditionNodeExecutor{})

	executor := agentflowapp.NewExecutor(logger, store, registry, nil).WithFlowEngineRegistry(workflowRegistry)
	supervisorRunner := agentflowapp.NewSupervisorRunner(
		logger,
		store,
		agentInvoker,
		agentflowinfra.NewHTTPAgentCatalog(platformAPIURL),
		nil,
	).WithAgentCapabilityCatalog(agentflowinfra.NewHTTPAgentCapabilityCatalog(platformAPIURL))

	server := httpserver.New(
		"agent-flow-runtime",
		httpserver.AddrFromEnv("AGENT_FLOW_RUNTIME_ADDR", ":8086"),
		logger,
	)
	agentflowhttp.RegisterRoutes(
		server.Mux(),
		executor,
		supervisorRunner,
		store,
		agentflowinfra.NewHTTPAgentFlowConfigLoader(platformAPIURL),
	)

	workflowApp := workflowruntime.NewService(
		logger,
		workflowruntime.NewHTTPWorkflowLoader(platformAPIURL),
		flowengine.NewMemoryRunStore(),
		workflowRegistry,
	)
	workflowruntime.RegisterRoutes(server.Mux(), workflowApp)

	ctx, stop := httpserver.SignalContext()
	defer stop()

	if err := server.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
