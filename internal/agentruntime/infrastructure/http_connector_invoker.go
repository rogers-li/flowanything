package infrastructure

import (
	"context"
	"net/http"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPConnectorInvoker struct {
	client *httpclient.Client
}

func NewHTTPConnectorInvoker(connectorServiceBaseURL string) *HTTPConnectorInvoker {
	return &HTTPConnectorInvoker{
		// The Agent Runtime already wraps adapter execution with the tool's
		// TimeoutMillis. Avoid a shorter transport-level timeout that would
		// mask the configured tool timeout for slower external APIs.
		client: httpclient.NewWithHTTPClient(connectorServiceBaseURL, &http.Client{}),
	}
}

func (i *HTTPConnectorInvoker) Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error) {
	var result connector.InvokeResult
	if err := i.client.PostJSON(ctx, "/v1/connector/invoke", req, &result); err != nil {
		return connector.InvokeResult{}, err
	}

	return result, nil
}
