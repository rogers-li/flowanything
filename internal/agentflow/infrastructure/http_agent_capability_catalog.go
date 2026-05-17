package infrastructure

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type HTTPAgentCapabilityCatalog struct {
	client *httpclient.Client
}

func NewHTTPAgentCapabilityCatalog(platformAPIBaseURL string) *HTTPAgentCapabilityCatalog {
	return &HTTPAgentCapabilityCatalog{
		client: httpclient.New(platformAPIBaseURL, 10*time.Second),
	}
}

func NewHTTPAgentCapabilityCatalogWithClient(platformAPIBaseURL string, client *http.Client) *HTTPAgentCapabilityCatalog {
	return &HTTPAgentCapabilityCatalog{
		client: httpclient.NewWithHTTPClient(platformAPIBaseURL, client),
	}
}

func (c *HTTPAgentCapabilityCatalog) LoadAgentCapabilityConfig(ctx context.Context, tenantID tenant.ID, agentID id.ID) (ports.AgentCapabilityConfig, error) {
	query := url.Values{}
	query.Set("tenant_id", tenantID.String())

	var profile agent.Profile
	if err := c.client.GetJSON(ctx, "/v1/agents/"+url.PathEscape(agentID.String()), query, &profile); err != nil {
		return ports.AgentCapabilityConfig{}, err
	}
	if !profile.RuntimeEnabled() {
		return ports.AgentCapabilityConfig{}, apperrors.New(apperrors.CodeForbidden, "agent is not enabled")
	}

	skills := make([]skill.Spec, 0, len(profile.SkillIDs))
	seenToolIDs := make(map[string]struct{})
	tools := make([]tool.Spec, 0)

	loadTool := func(toolID id.ID) error {
		if toolID.Empty() {
			return nil
		}
		if _, ok := seenToolIDs[toolID.String()]; ok {
			return nil
		}
		var spec tool.Spec
		if err := c.client.GetJSON(ctx, "/v1/tools/"+url.PathEscape(toolID.String()), query, &spec); err != nil {
			return err
		}
		seenToolIDs[toolID.String()] = struct{}{}
		tools = append(tools, spec)
		return nil
	}

	for _, toolID := range profile.ToolIDs {
		if err := loadTool(toolID); err != nil {
			return ports.AgentCapabilityConfig{}, err
		}
	}
	for _, skillID := range profile.SkillIDs {
		if skillID.Empty() {
			continue
		}
		var spec skill.Spec
		if err := c.client.GetJSON(ctx, "/v1/skills/"+url.PathEscape(skillID.String()), query, &spec); err != nil {
			return ports.AgentCapabilityConfig{}, err
		}
		if !spec.RuntimeEnabled() {
			continue
		}
		skills = append(skills, spec)
		for _, toolID := range spec.ToolIDs {
			if err := loadTool(toolID); err != nil {
				return ports.AgentCapabilityConfig{}, err
			}
		}
	}

	return ports.AgentCapabilityConfig{
		Agent:  profile,
		Skills: skills,
		Tools:  tools,
	}, nil
}
