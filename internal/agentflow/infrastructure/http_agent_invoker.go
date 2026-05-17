package infrastructure

import (
	"context"
	"net/http"
	"strings"
	"time"

	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/contracts/event"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
)

const runtimeSystemPromptPayloadKey = "runtime_system_prompt"

type HTTPAgentInvoker struct {
	client *httpclient.Client
}

func NewHTTPAgentInvoker(aiOrchestratorBaseURL string) *HTTPAgentInvoker {
	return NewHTTPAgentInvokerWithClient(aiOrchestratorBaseURL, nil)
}

func NewHTTPAgentInvokerWithClient(aiOrchestratorBaseURL string, client *http.Client) *HTTPAgentInvoker {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &HTTPAgentInvoker{
		client: httpclient.NewWithHTTPClient(aiOrchestratorBaseURL, client),
	}
}

func (i *HTTPAgentInvoker) InvokeAgent(ctx context.Context, request ports.AgentInvocationRequest) (ports.AgentInvocationResult, error) {
	if request.AgentID.Empty() {
		return ports.AgentInvocationResult{}, apperrors.New(apperrors.CodeInvalidArgument, "agent invocation requires agent_id")
	}
	if strings.TrimSpace(request.Task) == "" {
		return ports.AgentInvocationResult{}, apperrors.New(apperrors.CodeInvalidArgument, "agent invocation requires task text")
	}

	evt := event.Event{
		ID:         id.New("evt"),
		TenantID:   request.Run.TenantID,
		TraceID:    nonEmpty(request.TraceID, id.New("trace").String()),
		UserID:     nonEmpty(request.UserID, "agent_flow"),
		SessionID:  request.SessionID,
		TaskID:     request.Run.ID,
		AgentID:    request.AgentID,
		Type:       event.TypeUserMessageCommitted,
		Channel:    event.ChannelText,
		Payload:    invocationPayload(request.Task, request.Payload, request.RuntimeSystemPrompt),
		OccurredAt: time.Now().UTC(),
	}
	if evt.SessionID.Empty() {
		evt.SessionID = id.New("agentflow_session")
	}

	var response event.Response
	if err := i.client.PostJSON(ctx, "/v1/events", evt, &response); err != nil {
		return ports.AgentInvocationResult{}, err
	}

	return ports.AgentInvocationResult{
		Text:     responseText(response.Actions),
		Response: response,
		Actions:  response.Actions,
		TraceID:  response.TraceID,
	}, nil
}

func invocationPayload(task string, payload map[string]any, runtimeSystemPrompt string) map[string]any {
	result := make(map[string]any, len(payload)+2)
	for key, value := range payload {
		if key == runtimeSystemPromptPayloadKey {
			continue
		}
		result[key] = value
	}
	result["text"] = task
	if prompt := strings.TrimSpace(runtimeSystemPrompt); prompt != "" {
		result[runtimeSystemPromptPayloadKey] = prompt
	}
	return result
}

func responseText(actions []event.Action) string {
	for _, preferred := range []event.ActionType{
		event.ActionSpeak,
		event.ActionDisplayText,
		event.ActionAskQuestion,
		event.ActionAskConfirmation,
	} {
		for _, action := range actions {
			if action.Type == preferred && strings.TrimSpace(action.Text) != "" {
				return action.Text
			}
		}
	}
	for _, action := range actions {
		if strings.TrimSpace(action.Text) != "" {
			return action.Text
		}
	}
	return ""
}

func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
