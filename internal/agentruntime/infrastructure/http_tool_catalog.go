package infrastructure

import (
	"context"
	"net/url"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPToolCatalog struct {
	client *httpclient.Client
}

func NewHTTPToolCatalog(platformAPIBaseURL string) *HTTPToolCatalog {
	return &HTTPToolCatalog{
		client: httpclient.New(platformAPIBaseURL, 10*time.Second),
	}
}

func (c *HTTPToolCatalog) GetTool(ctx context.Context, call tool.Call) (tool.Spec, error) {
	query := url.Values{}
	query.Set("tenant_id", call.TenantID.String())

	var spec tool.Spec
	if err := c.client.GetJSON(ctx, "/v1/tools/"+url.PathEscape(call.ToolID.String()), query, &spec); err != nil {
		return tool.Spec{}, err
	}

	return spec, nil
}
