package infrastructure

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPToolRuntime struct {
	client *httpclient.Client
}

func NewHTTPToolRuntime(agentRuntimeBaseURL string, timeout time.Duration) *HTTPToolRuntime {
	if timeout <= 0 {
		timeout = 75 * time.Second
	}
	return &HTTPToolRuntime{
		client: httpclient.New(agentRuntimeBaseURL, timeout),
	}
}

func (r *HTTPToolRuntime) ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error) {
	var result tool.Result
	if err := r.client.PostJSON(ctx, "/v1/tools/execute", call, &result); err != nil {
		return tool.Result{}, err
	}

	return result, nil
}
