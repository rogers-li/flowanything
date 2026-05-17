package infrastructure

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPPythonRunner struct {
	client *httpclient.Client
}

func NewHTTPPythonRunner(codeAdapterBaseURL string) *HTTPPythonRunner {
	return &HTTPPythonRunner{
		client: httpclient.New(codeAdapterBaseURL, 30*time.Second),
	}
}

func (r *HTTPPythonRunner) Run(ctx context.Context, req tool.PythonRunRequest) (tool.BackendResult, error) {
	var result tool.BackendResult
	if err := r.client.PostJSON(ctx, "/v1/code-adapter/runs", req, &result); err != nil {
		return tool.BackendResult{}, err
	}

	return result, nil
}
