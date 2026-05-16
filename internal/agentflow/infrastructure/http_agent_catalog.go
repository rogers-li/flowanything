package infrastructure

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type HTTPAgentCatalog struct {
	client *httpclient.Client
}

func NewHTTPAgentCatalog(platformAPIBaseURL string) *HTTPAgentCatalog {
	return &HTTPAgentCatalog{
		client: httpclient.New(platformAPIBaseURL, 10*time.Second),
	}
}

func NewHTTPAgentCatalogWithClient(platformAPIBaseURL string, client *http.Client) *HTTPAgentCatalog {
	return &HTTPAgentCatalog{
		client: httpclient.NewWithHTTPClient(platformAPIBaseURL, client),
	}
}

func (c *HTTPAgentCatalog) GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error) {
	query := url.Values{}
	query.Set("tenant_id", tenantID.String())

	var profile agent.Profile
	if err := c.client.GetJSON(ctx, "/v1/agents/"+url.PathEscape(agentID.String()), query, &profile); err != nil {
		return agent.Profile{}, err
	}
	return profile, nil
}
