package infrastructure

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"flow-anything/internal/platform/contracts/agentflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type HTTPAgentFlowConfigLoader struct {
	client *httpclient.Client
}

func NewHTTPAgentFlowConfigLoader(platformAPIBaseURL string) *HTTPAgentFlowConfigLoader {
	return &HTTPAgentFlowConfigLoader{
		client: httpclient.New(platformAPIBaseURL, 10*time.Second),
	}
}

func NewHTTPAgentFlowConfigLoaderWithClient(platformAPIBaseURL string, client *http.Client) *HTTPAgentFlowConfigLoader {
	return &HTTPAgentFlowConfigLoader{
		client: httpclient.NewWithHTTPClient(platformAPIBaseURL, client),
	}
}

func (l *HTTPAgentFlowConfigLoader) LoadAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error) {
	query := url.Values{}
	query.Set("tenant_id", tenantID.String())

	var spec agentflow.Spec
	if err := l.client.GetJSON(ctx, "/v1/agent-flows/"+url.PathEscape(flowID.String()), query, &spec); err != nil {
		return agentflow.Spec{}, err
	}
	if !spec.RuntimeEnabled() {
		return agentflow.Spec{}, apperrors.New(apperrors.CodeForbidden, "agent flow is not enabled")
	}

	return spec, nil
}
