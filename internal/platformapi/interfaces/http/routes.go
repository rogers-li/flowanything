package httpapi

import (
	"encoding/json"
	"net/http"

	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/httpserver"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
	"flow-anything/internal/platformapi/application"
)

func RegisterRoutes(mux *http.ServeMux, app *application.Service) {
	registerMCPRoutes(mux)

	mux.HandleFunc("POST /v1/agents", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req agent.Profile
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_json",
					"message": "request body must be a valid agent profile json",
				},
			})
			return
		}

		resp, err := app.CreateAgent(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/agents", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		resp, err := app.ListAgents(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/agents/{agent_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		agentID := id.ID(r.PathValue("agent_id"))
		resp, err := app.GetAgent(r.Context(), tenantID, agentID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/agents/{agent_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		agentID := id.ID(r.PathValue("agent_id"))
		var req agent.Profile
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid agent profile json")
			return
		}
		req.TenantID = tenantID
		req.ID = agentID

		resp, err := app.UpdateAgent(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/agents/{agent_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		agentID := id.ID(r.PathValue("agent_id"))
		resp, err := app.SetAgentStatus(r.Context(), tenantID, agentID, agent.StatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/agents/{agent_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		agentID := id.ID(r.PathValue("agent_id"))
		resp, err := app.SetAgentStatus(r.Context(), tenantID, agentID, agent.StatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /v1/agents/{agent_id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		agentID := id.ID(r.PathValue("agent_id"))
		resp, err := app.GetAgentDependencies(r.Context(), tenantID, agentID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/agent-flows", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req agentflow.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid agent flow spec json")
			return
		}

		resp, err := app.CreateAgentFlow(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/agent-flows", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		status := agentflow.Status(r.URL.Query().Get("status"))
		resp, err := app.ListAgentFlows(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		if status != "" {
			resp = filterAgentFlowsByStatus(resp, status)
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/agent-flows/{flow_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		flowID := id.ID(r.PathValue("flow_id"))
		resp, err := app.GetAgentFlow(r.Context(), tenantID, flowID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/agent-flows/{flow_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		flowID := id.ID(r.PathValue("flow_id"))
		var req agentflow.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid agent flow spec json")
			return
		}
		req.ID = flowID
		if !tenantID.Empty() {
			req.TenantID = tenantID
		}

		resp, err := app.UpdateAgentFlow(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/agent-flows/{flow_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		flowID := id.ID(r.PathValue("flow_id"))
		resp, err := app.SetAgentFlowStatus(r.Context(), tenantID, flowID, agentflow.StatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/agent-flows/{flow_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		flowID := id.ID(r.PathValue("flow_id"))
		resp, err := app.SetAgentFlowStatus(r.Context(), tenantID, flowID, agentflow.StatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/workflows", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req workflow.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid workflow spec json")
			return
		}

		resp, err := app.CreateWorkflow(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/workflows", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		status := workflow.Status(r.URL.Query().Get("status"))
		profile := workflow.Profile(r.URL.Query().Get("profile"))
		resp, err := app.ListWorkflows(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		if status != "" {
			resp = filterWorkflows(resp, status, "")
		}
		if profile != "" {
			resp = filterWorkflows(resp, "", profile)
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/workflows/{workflow_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		workflowID := id.ID(r.PathValue("workflow_id"))
		resp, err := app.GetWorkflow(r.Context(), tenantID, workflowID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/workflows/{workflow_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		workflowID := id.ID(r.PathValue("workflow_id"))
		var req workflow.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid workflow spec json")
			return
		}
		req.ID = workflowID
		if !tenantID.Empty() {
			req.TenantID = tenantID
		}

		resp, err := app.UpdateWorkflow(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/workflows/{workflow_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		workflowID := id.ID(r.PathValue("workflow_id"))
		resp, err := app.SetWorkflowStatus(r.Context(), tenantID, workflowID, workflow.StatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/workflows/{workflow_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		workflowID := id.ID(r.PathValue("workflow_id"))
		resp, err := app.SetWorkflowStatus(r.Context(), tenantID, workflowID, workflow.StatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connectors", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req connector.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid connector json")
			return
		}

		resp, err := app.CreateConnector(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/connectors", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		resp, err := app.ListConnectors(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/connectors/{connector_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		connectorID := id.ID(r.PathValue("connector_id"))
		resp, err := app.GetConnector(r.Context(), tenantID, connectorID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/connectors/{connector_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		connectorID := id.ID(r.PathValue("connector_id"))
		var req connector.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid connector json")
			return
		}
		req.ID = connectorID
		if !tenantID.Empty() {
			req.TenantID = tenantID
		}

		resp, err := app.UpdateConnector(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connectors/{connector_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		connectorID := id.ID(r.PathValue("connector_id"))
		resp, err := app.SetConnectorStatus(r.Context(), tenantID, connectorID, connector.OperationStatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connectors/{connector_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		connectorID := id.ID(r.PathValue("connector_id"))
		resp, err := app.SetConnectorStatus(r.Context(), tenantID, connectorID, connector.OperationStatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connector-operations", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req connector.OperationSpec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid connector operation json")
			return
		}

		resp, err := app.CreateConnectorOperation(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/connector-operations", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		resp, err := app.ListConnectorOperations(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/connector-operations/{operation_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		operationID := id.ID(r.PathValue("operation_id"))
		resp, err := app.GetConnectorOperation(r.Context(), tenantID, operationID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/connector-operations/{operation_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		operationID := id.ID(r.PathValue("operation_id"))
		var req connector.OperationSpec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid connector operation json")
			return
		}
		req.ID = operationID
		if !tenantID.Empty() {
			req.TenantID = tenantID
		}

		resp, err := app.UpdateConnectorOperation(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connector-operations/{operation_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		operationID := id.ID(r.PathValue("operation_id"))
		resp, err := app.SetConnectorOperationStatus(r.Context(), tenantID, operationID, connector.OperationStatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connector-operations/{operation_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		operationID := id.ID(r.PathValue("operation_id"))
		resp, err := app.SetConnectorOperationStatus(r.Context(), tenantID, operationID, connector.OperationStatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /v1/connector-operations/{operation_id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		operationID := id.ID(r.PathValue("operation_id"))
		resp, err := app.GetConnectorOperationDependencies(r.Context(), tenantID, operationID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/connectors/{connector_id}/tools/import", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		connectorID := id.ID(r.PathValue("connector_id"))
		var req importConnectorToolsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid connector tool import json")
			return
		}

		resp, err := app.ImportConnectorTools(r.Context(), tenantID, connectorID, req.Tools)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("POST /v1/tools", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req tool.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid tool spec json")
			return
		}

		resp, err := app.CreateTool(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/tools", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		status := tool.Status(r.URL.Query().Get("status"))
		resp, err := app.ListTools(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}
		if status != "" {
			resp = filterToolsByStatus(resp, status)
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/tools/{tool_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		toolID := id.ID(r.PathValue("tool_id"))
		resp, err := app.GetTool(r.Context(), tenantID, toolID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/tools/{tool_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		toolID := id.ID(r.PathValue("tool_id"))
		var req tool.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid tool spec json")
			return
		}
		req.ID = toolID
		if !tenantID.Empty() {
			req.TenantID = tenantID
		}

		resp, err := app.UpdateTool(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/tools/{tool_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		toolID := id.ID(r.PathValue("tool_id"))
		resp, err := app.SetToolStatus(r.Context(), tenantID, toolID, tool.StatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/tools/{tool_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		toolID := id.ID(r.PathValue("tool_id"))
		resp, err := app.SetToolStatus(r.Context(), tenantID, toolID, tool.StatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /v1/tools/{tool_id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		toolID := id.ID(r.PathValue("tool_id"))
		resp, err := app.GetToolDependencies(r.Context(), tenantID, toolID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req skill.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid skill spec json")
			return
		}

		resp, err := app.CreateSkill(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusCreated, resp)
	})

	mux.HandleFunc("GET /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		resp, err := app.ListSkills(r.Context(), tenantID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"items": resp,
		})
	})

	mux.HandleFunc("GET /v1/skills/{skill_id}", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		skillID := id.ID(r.PathValue("skill_id"))
		resp, err := app.GetSkill(r.Context(), tenantID, skillID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("PUT /v1/skills/{skill_id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		skillID := id.ID(r.PathValue("skill_id"))
		var req skill.Spec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidJSON(w, "request body must be a valid skill spec json")
			return
		}
		req.TenantID = tenantID
		req.ID = skillID

		resp, err := app.UpdateSkill(r.Context(), req)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/skills/{skill_id}/enable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		skillID := id.ID(r.PathValue("skill_id"))
		resp, err := app.SetSkillStatus(r.Context(), tenantID, skillID, skill.StatusEnabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /v1/skills/{skill_id}/disable", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		skillID := id.ID(r.PathValue("skill_id"))
		resp, err := app.SetSkillStatus(r.Context(), tenantID, skillID, skill.StatusDisabled)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /v1/skills/{skill_id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.ID(r.URL.Query().Get("tenant_id"))
		skillID := id.ID(r.PathValue("skill_id"))
		resp, err := app.GetSkillDependencies(r.Context(), tenantID, skillID)
		if err != nil {
			httpserver.WriteError(w, err)
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, resp)
	})
}

type importConnectorToolsRequest struct {
	Tools []tool.Spec `json:"tools"`
}

func filterToolsByStatus(items []tool.Spec, status tool.Status) []tool.Spec {
	result := make([]tool.Spec, 0, len(items))
	for _, item := range items {
		if item.Status == status {
			result = append(result, item)
		}
	}
	return result
}

func filterAgentFlowsByStatus(items []agentflow.Spec, status agentflow.Status) []agentflow.Spec {
	result := make([]agentflow.Spec, 0, len(items))
	for _, item := range items {
		if item.Status == status {
			result = append(result, item)
		}
	}
	return result
}

func filterWorkflows(items []workflow.Spec, status workflow.Status, profile workflow.Profile) []workflow.Spec {
	result := make([]workflow.Spec, 0, len(items))
	for _, item := range items {
		if status != "" && item.Status != status {
			continue
		}
		if profile != "" && item.Profile != profile {
			continue
		}
		result = append(result, item)
	}
	return result
}

func writeInvalidJSON(w http.ResponseWriter, message string) {
	httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
		"error": map[string]string{
			"code":    "invalid_json",
			"message": message,
		},
	})
}
