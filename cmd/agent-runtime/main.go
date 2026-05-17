package main

import (
	"os"
	"time"

	connectoradapter "flow-anything/internal/agentruntime/adapters/connector"
	knowledgeadapter "flow-anything/internal/agentruntime/adapters/knowledge"
	mcpadapter "flow-anything/internal/agentruntime/adapters/mcp"
	pythonadapter "flow-anything/internal/agentruntime/adapters/python"
	workflowadapter "flow-anything/internal/agentruntime/adapters/workflow"
	"flow-anything/internal/agentruntime/application"
	"flow-anything/internal/agentruntime/infrastructure"
	httpapi "flow-anything/internal/agentruntime/interfaces/http"
	"flow-anything/internal/platform/kernel/config"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/logging"
)

func main() {
	logger := logging.New("agent-runtime")
	catalog := infrastructure.NewHTTPToolCatalog(config.String("PLATFORM_API_URL", "http://localhost:8080"))
	connectorInvoker := infrastructure.NewHTTPConnectorInvoker(config.String("CONNECTOR_SERVICE_URL", "http://localhost:8083"))
	knowledgeRetriever := infrastructure.NewHTTPKnowledgeRetriever(config.String("KNOWLEDGE_SERVICE_URL", "http://localhost:8084"))
	mcpCaller := infrastructure.NewHTTPMCPCaller(
		config.String("MCP_GATEWAY_URL", "http://localhost:8085"),
		logger,
		config.Duration("AGENT_RUNTIME_MCP_CALL_TIMEOUT", 75*time.Second),
	)
	pythonRunner := infrastructure.NewHTTPPythonRunner(config.String("CODE_ADAPTER_SERVICE_URL", "http://localhost:8086"))
	workflowRunner := infrastructure.NewHTTPWorkflowRunner(config.String("WORKFLOW_SERVICE_URL", "http://localhost:8086"))
	recorder := infrastructure.NewMemoryExecutionRecorder()
	app := application.New(
		logger,
		catalog,
		recorder,
		connectoradapter.New(connectorInvoker),
		knowledgeadapter.New(knowledgeRetriever),
		mcpadapter.New(mcpCaller),
		pythonadapter.New(pythonRunner),
		workflowadapter.New(workflowRunner),
	)

	server := httpserver.New(
		"agent-runtime",
		httpserver.AddrFromEnv("AGENT_RUNTIME_ADDR", ":8082"),
		logger,
	)
	httpapi.RegisterRoutes(server.Mux(), app)

	ctx, stop := httpserver.SignalContext()
	defer stop()

	if err := server.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
