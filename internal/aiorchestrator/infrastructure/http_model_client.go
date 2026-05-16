package infrastructure

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/model"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPModelClient struct {
	client *httpclient.Client
}

func NewHTTPModelClient(modelGatewayBaseURL string) *HTTPModelClient {
	return NewHTTPModelClientWithTimeout(modelGatewayBaseURL, 30*time.Second)
}

func NewHTTPModelClientWithTimeout(modelGatewayBaseURL string, timeout time.Duration) *HTTPModelClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPModelClient{
		client: httpclient.New(modelGatewayBaseURL, timeout),
	}
}

func (c *HTTPModelClient) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	var resp model.ChatResponse
	if err := c.client.PostJSON(ctx, "/v1/chat/completions", req, &resp); err != nil {
		return model.ChatResponse{}, err
	}

	return resp, nil
}
