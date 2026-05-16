package e2e

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeconnector "flow-anything/internal/agentruntime/adapters/connector"
	runtimeapp "flow-anything/internal/agentruntime/application"
	runtimeinfra "flow-anything/internal/agentruntime/infrastructure"
	orchestratorapp "flow-anything/internal/aiorchestrator/application"
	orchestratordomain "flow-anything/internal/aiorchestrator/domain"
	orchestratorinfra "flow-anything/internal/aiorchestrator/infrastructure"
	connectorapp "flow-anything/internal/connector/application"
	connectorinfra "flow-anything/internal/connector/infrastructure"
	"flow-anything/internal/mockbusiness"
	modelapp "flow-anything/internal/modelgateway/application"
	modelinfra "flow-anything/internal/modelgateway/infrastructure"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
	platformapp "flow-anything/internal/platformapi/application"
	platforminfra "flow-anything/internal/platformapi/infrastructure"
)

func TestWeatherQuestionRunsThroughAgentToolConnectorFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := testLogger()
	tenantID := tenant.ID("tenant_1")

	platform := newPlatformService(logger)
	registerWeatherResources(t, ctx, platform, tenantID)

	mockBusinessMux := http.NewServeMux()
	mockbusiness.RegisterRoutes(mockBusinessMux)

	connectorService := connectorapp.New(
		logger,
		platformOperationRepository{platform: platform},
		connectorinfra.NewHTTPOperationInvokerWithClient(handlerBackedHTTPClient(mockBusinessMux)),
	)
	runtimeService := runtimeapp.New(
		logger,
		platformToolCatalog{platform: platform},
		runtimeinfra.NewMemoryExecutionRecorder(),
		runtimeconnector.New(directConnectorInvoker{connector: connectorService}),
	)
	modelService := modelapp.New(logger, modelinfra.NewMockProvider())
	orchestratorService := orchestratorapp.New(
		logger,
		directToolRuntime{runtime: runtimeService},
		directModelClient{model: modelService},
		directAgentConfigLoader{platform: platform},
		orchestratorapp.WithDefaultSystemPrompt("你是企业 AI 中台中的智能助手。"),
		orchestratorapp.WithMaxToolIterations(1),
		orchestratorapp.WithConversationStore(orchestratorinfra.NewMemoryConversationStore()),
	)

	resp, err := orchestratorService.HandleEvent(ctx, event.Event{
		TenantID: tenantID,
		AgentID:  id.ID("agent_weather"),
		Type:     event.TypeUserMessageCommitted,
		Channel:  event.ChannelText,
		Payload: map[string]any{
			"text": "帮我查一下深圳天气",
		},
	})
	if err != nil {
		t.Fatalf("handle weather event: %v", err)
	}

	speech := firstActionText(resp.Actions, event.ActionSpeak)
	if !strings.Contains(speech, `"city":"深圳"`) {
		t.Fatalf("expected speech to include weather city, got %q", speech)
	}
	if !strings.Contains(speech, `"condition":"多云"`) {
		t.Fatalf("expected speech to include weather condition, got %q", speech)
	}
}

func TestWeatherQuestionSupportsFollowUpInSameSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := testLogger()
	tenantID := tenant.ID("tenant_1")
	sessionID := id.ID("session_weather_debug")

	platform := newPlatformService(logger)
	registerWeatherResources(t, ctx, platform, tenantID)

	mockBusinessMux := http.NewServeMux()
	mockbusiness.RegisterRoutes(mockBusinessMux)

	connectorService := connectorapp.New(
		logger,
		platformOperationRepository{platform: platform},
		connectorinfra.NewHTTPOperationInvokerWithClient(handlerBackedHTTPClient(mockBusinessMux)),
	)
	runtimeService := runtimeapp.New(
		logger,
		platformToolCatalog{platform: platform},
		runtimeinfra.NewMemoryExecutionRecorder(),
		runtimeconnector.New(directConnectorInvoker{connector: connectorService}),
	)
	modelService := modelapp.New(logger, modelinfra.NewMockProvider())
	orchestratorService := orchestratorapp.New(
		logger,
		directToolRuntime{runtime: runtimeService},
		directModelClient{model: modelService},
		directAgentConfigLoader{platform: platform},
		orchestratorapp.WithDefaultSystemPrompt("你是企业 AI 中台中的智能助手。"),
		orchestratorapp.WithMaxToolIterations(1),
		orchestratorapp.WithConversationStore(orchestratorinfra.NewMemoryConversationStore()),
	)

	if _, err := orchestratorService.HandleEvent(ctx, event.Event{
		TenantID:  tenantID,
		AgentID:   id.ID("agent_weather"),
		SessionID: sessionID,
		Type:      event.TypeUserMessageCommitted,
		Channel:   event.ChannelText,
		Payload:   map[string]any{"text": "帮我查一下深圳天气"},
	}); err != nil {
		t.Fatalf("handle first weather event: %v", err)
	}

	resp, err := orchestratorService.HandleEvent(ctx, event.Event{
		TenantID:  tenantID,
		AgentID:   id.ID("agent_weather"),
		SessionID: sessionID,
		Type:      event.TypeUserMessageCommitted,
		Channel:   event.ChannelText,
		Payload:   map[string]any{"text": "那北京呢？"},
	})
	if err != nil {
		t.Fatalf("handle follow-up weather event: %v", err)
	}

	speech := firstActionText(resp.Actions, event.ActionSpeak)
	if !strings.Contains(speech, `"city":"北京"`) {
		t.Fatalf("expected follow-up speech to include Beijing weather, got %q", speech)
	}
	if !strings.Contains(speech, `"condition":"晴"`) {
		t.Fatalf("expected follow-up speech to include Beijing condition, got %q", speech)
	}
}

func newPlatformService(logger *slog.Logger) *platformapp.Service {
	return platformapp.New(
		logger,
		platforminfra.NewMemoryAgentRepository(),
		platforminfra.NewMemoryAgentFlowRepository(),
		platforminfra.NewMemoryToolRepository(),
		platforminfra.NewMemorySkillRepository(),
		platforminfra.NewMemoryConnectorRepository(),
		platforminfra.NewMemoryConnectorOperationRepository(),
	)
}

func registerWeatherResources(t *testing.T, ctx context.Context, platform *platformapp.Service, tenantID tenant.ID) {
	t.Helper()

	_, err := platform.CreateConnectorOperation(ctx, connector.OperationSpec{
		ID:                 id.ID("connop_query_weather"),
		TenantID:           tenantID,
		Name:               "query_weather",
		Description:        "查询城市实时天气",
		Type:               connector.OperationTypeHTTP,
		Status:             connector.OperationStatusEnabled,
		ImplementationMode: connector.ImplementationModeSimpleHTTP,
		BaseURL:            "http://mock-business.local",
		Method:             http.MethodGet,
		Path:               "/weather/current",
		Auth: connector.AuthConfig{
			Type: connector.AuthTypeNone,
		},
		InputSchema:   weatherInputSchema(),
		TimeoutMillis: 8000,
	})
	if err != nil {
		t.Fatalf("create weather connector operation: %v", err)
	}

	_, err = platform.CreateTool(ctx, tool.Spec{
		ID:             id.ID("tool_query_weather"),
		TenantID:       tenantID,
		Name:           "query_weather",
		Description:    "查询城市实时天气",
		Status:         tool.StatusEnabled,
		Implementation: tool.ImplementationConnector,
		Binding: tool.Binding{
			ConnectorOperationID: id.ID("connop_query_weather"),
		},
		InputSchema:   weatherInputSchema(),
		SideEffect:    tool.SideEffectRead,
		RiskLevel:     tool.RiskLow,
		TimeoutMillis: 8000,
	})
	if err != nil {
		t.Fatalf("create weather tool: %v", err)
	}

	_, err = platform.CreateSkill(ctx, skill.Spec{
		ID:           id.ID("skill_weather"),
		TenantID:     tenantID,
		Name:         "weather_service",
		Description:  "天气查询能力",
		Status:       skill.StatusEnabled,
		ToolIDs:      []id.ID{id.ID("tool_query_weather")},
		SystemPrompt: "当用户询问天气时，调用 query_weather 查询城市实时天气。如果城市缺失，先追问用户。",
		RiskLevel:    skill.RiskLow,
	})
	if err != nil {
		t.Fatalf("create weather skill: %v", err)
	}

	_, err = platform.CreateAgent(ctx, agent.Profile{
		ID:           id.ID("agent_weather"),
		TenantID:     tenantID,
		Name:         "Weather Agent",
		Description:  "面向用户的天气助手",
		Status:       agent.StatusEnabled,
		SkillIDs:     []id.ID{id.ID("skill_weather")},
		DefaultLang:  "zh-CN",
		SystemPrompt: "你是简洁的天气助手。用户询问天气时，调用 query_weather 并基于工具结果回答。",
	})
	if err != nil {
		t.Fatalf("create weather agent: %v", err)
	}
}

func weatherInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "城市名称",
			},
			"unit": map[string]any{
				"type":        "string",
				"description": "单位，默认 metric",
			},
		},
		"required": []string{"city"},
	}
}

func firstActionText(actions []event.Action, actionType event.ActionType) string {
	for _, action := range actions {
		if action.Type == actionType {
			return action.Text
		}
	}
	return ""
}

type platformToolCatalog struct {
	platform *platformapp.Service
}

func (c platformToolCatalog) GetTool(ctx context.Context, call tool.Call) (tool.Spec, error) {
	return c.platform.GetTool(ctx, call.TenantID, call.ToolID)
}

type platformOperationRepository struct {
	platform *platformapp.Service
}

func (r platformOperationRepository) GetOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error) {
	return r.platform.GetConnectorOperation(ctx, tenantID, operationID)
}

type directConnectorInvoker struct {
	connector *connectorapp.Service
}

func (i directConnectorInvoker) Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error) {
	return i.connector.Invoke(ctx, req)
}

type directToolRuntime struct {
	runtime *runtimeapp.Service
}

func (r directToolRuntime) ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error) {
	return r.runtime.ExecuteTool(ctx, call)
}

type directModelClient struct {
	model *modelapp.Service
}

func (c directModelClient) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	return c.model.Chat(ctx, req)
}

type directAgentConfigLoader struct {
	platform *platformapp.Service
}

func (l directAgentConfigLoader) LoadAgentConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (orchestratordomain.AgentConfig, error) {
	profile, err := l.platform.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return orchestratordomain.AgentConfig{}, err
	}
	if !profile.RuntimeEnabled() {
		return orchestratordomain.AgentConfig{}, fmt.Errorf("agent %s is not enabled", agentID)
	}

	skills := make([]skill.Spec, 0, len(profile.SkillIDs))
	tools := make([]tool.Spec, 0)
	seenToolIDs := make(map[id.ID]struct{})
	for _, skillID := range profile.SkillIDs {
		spec, err := l.platform.GetSkill(ctx, tenantID, skillID)
		if err != nil {
			return orchestratordomain.AgentConfig{}, err
		}
		if !spec.RuntimeEnabled() {
			continue
		}
		skills = append(skills, spec)

		for _, toolID := range spec.ToolIDs {
			if _, ok := seenToolIDs[toolID]; ok {
				continue
			}
			toolSpec, err := l.platform.GetTool(ctx, tenantID, toolID)
			if err != nil {
				return orchestratordomain.AgentConfig{}, err
			}
			seenToolIDs[toolID] = struct{}{}
			tools = append(tools, toolSpec)
		}
	}

	return orchestratordomain.AgentConfig{
		Agent:  profile,
		Skills: skills,
		Tools:  tools,
	}, nil
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func handlerBackedHTTPClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			resp.Request = req
			if resp.Body == nil {
				resp.Body = io.NopCloser(strings.NewReader(""))
			}
			return resp, nil
		}),
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
