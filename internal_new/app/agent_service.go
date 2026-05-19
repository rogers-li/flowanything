package app

import (
	"context"
	"fmt"

	"flow-anything/core/agentcore"
	"flow-anything/core/runtimecontext"
	"flow-anything/core/workflow"
)

type AgentRequest struct {
	AgentID      string
	UserMessage  string
	Conversation []agentcore.Message
	Context      map[string]any
	TraceID      string
	TraceContext runtimecontext.TraceContext
}

type AgentResult = agentcore.AgentRunResult

// RunAgent executes one configured agent through core/agentcore.
func (h *Host) RunAgent(ctx context.Context, req AgentRequest) (AgentResult, error) {
	agent, ok := h.catalog.Agents[req.AgentID]
	if !ok {
		return AgentResult{}, fmt.Errorf("agent %q not found", req.AgentID)
	}
	return h.agentRunner.Run(ctx, agentcore.AgentRunRequest{
		Agent:        agent,
		UserMessage:  req.UserMessage,
		Conversation: req.Conversation,
		Context:      req.Context,
		TraceID:      req.TraceID,
		TraceContext: req.TraceContext,
	})
}

type agentNodeRunner struct {
	host *Host
}

func (r agentNodeRunner) RunAgent(ctx context.Context, req workflow.AgentRunRequest) (workflow.AgentRunResult, error) {
	message := req.Message
	if message == "" {
		message = messageFromInput(req.Input)
	}
	result, err := r.host.agentRunner.Run(ctx, agentcore.AgentRunRequest{
		Agent:        req.Agent,
		UserMessage:  message,
		Context:      req.Input,
		TraceContext: req.TraceContext,
	})
	if err != nil {
		return workflow.AgentRunResult{}, err
	}
	output := result.Output
	if output == nil {
		output = map[string]any{}
	}
	text := userFacingText(output)
	if text == "" {
		text = result.Text
	}
	if text != "" {
		output["text"] = text
	}
	return workflow.AgentRunResult{
		Output: output,
		Text:   text,
		Raw:    result.Raw,
	}, nil
}

func messageFromInput(input map[string]any) string {
	for _, key := range []string{"user_request", "task", "message", "text", "query"} {
		if value, ok := input[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
