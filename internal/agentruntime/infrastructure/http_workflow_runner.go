package infrastructure

import (
	"context"
	"time"

	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/kernel/httpclient"
)

type HTTPWorkflowRunner struct {
	client *httpclient.Client
}

func NewHTTPWorkflowRunner(workflowServiceBaseURL string) *HTTPWorkflowRunner {
	return &HTTPWorkflowRunner{
		client: httpclient.New(workflowServiceBaseURL, 30*time.Second),
	}
}

func (r *HTTPWorkflowRunner) Run(ctx context.Context, req tool.WorkflowRunRequest) (tool.BackendResult, error) {
	var result tool.BackendResult
	if err := r.client.PostJSON(ctx, "/v1/tools/workflows/run", req, &result); err != nil {
		return tool.BackendResult{}, err
	}

	return result, nil
}
