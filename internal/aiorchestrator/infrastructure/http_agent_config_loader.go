package infrastructure

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type HTTPAgentConfigLoader struct {
	client *httpclient.Client
}

func NewHTTPAgentConfigLoader(platformAPIBaseURL string) *HTTPAgentConfigLoader {
	return &HTTPAgentConfigLoader{
		client: httpclient.New(platformAPIBaseURL, 10*time.Second),
	}
}

func NewHTTPAgentConfigLoaderWithClient(platformAPIBaseURL string, client *http.Client) *HTTPAgentConfigLoader {
	return &HTTPAgentConfigLoader{
		client: httpclient.NewWithHTTPClient(platformAPIBaseURL, client),
	}
}

func (l *HTTPAgentConfigLoader) LoadAgentConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (domain.AgentConfig, error) {
	query := url.Values{}
	query.Set("tenant_id", tenantID.String())

	var profile agent.Profile
	if err := l.client.GetJSON(ctx, "/v1/agents/"+url.PathEscape(agentID.String()), query, &profile); err != nil {
		return domain.AgentConfig{}, err
	}
	if !profile.RuntimeEnabled() {
		return domain.AgentConfig{}, apperrors.New(apperrors.CodeForbidden, "agent is not enabled")
	}

	skills := make([]skill.Spec, 0, len(profile.SkillIDs))
	seenToolIDs := make(map[string]struct{})
	tools := make([]tool.Spec, 0)

	loadTool := func(toolID id.ID) error {
		if _, ok := seenToolIDs[toolID.String()]; ok {
			return nil
		}
		var toolSpec tool.Spec
		if err := l.client.GetJSON(ctx, "/v1/tools/"+url.PathEscape(toolID.String()), query, &toolSpec); err != nil {
			return err
		}
		seenToolIDs[toolID.String()] = struct{}{}
		tools = append(tools, toolSpec)
		return nil
	}

	for _, toolID := range profile.ToolIDs {
		if err := loadTool(toolID); err != nil {
			return domain.AgentConfig{}, err
		}
	}

	for _, skillID := range profile.SkillIDs {
		var spec skill.Spec
		if err := l.client.GetJSON(ctx, "/v1/skills/"+url.PathEscape(skillID.String()), query, &spec); err != nil {
			return domain.AgentConfig{}, err
		}
		if !spec.RuntimeEnabled() {
			continue
		}
		skills = append(skills, spec)

		for _, toolID := range spec.ToolIDs {
			if err := loadTool(toolID); err != nil {
				return domain.AgentConfig{}, err
			}
		}
	}

	return domain.AgentConfig{
		Agent:  profile,
		Skills: skills,
		Tools:  tools,
	}, nil
}
