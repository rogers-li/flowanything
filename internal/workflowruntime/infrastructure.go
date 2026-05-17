package workflowruntime

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type HTTPWorkflowLoader struct {
	client *httpclient.Client
}

func NewHTTPWorkflowLoader(platformAPIBaseURL string) *HTTPWorkflowLoader {
	return &HTTPWorkflowLoader{client: httpclient.New(platformAPIBaseURL, 10*time.Second)}
}

func (l *HTTPWorkflowLoader) LoadWorkflow(ctx context.Context, tenantID tenant.ID, workflowID id.ID) (workflow.Spec, error) {
	var spec workflow.Spec
	query := url.Values{}
	query.Set("tenant_id", tenantID.String())
	err := l.client.GetJSON(ctx, "/v1/workflows/"+workflowID.String(), query, &spec)
	return spec, err
}

type HTTPConnectorInvoker struct {
	client *httpclient.Client
}

func NewHTTPConnectorInvoker(baseURL string) *HTTPConnectorInvoker {
	// Workflow node execution is bounded by the workflow/node context. Keep the
	// transport client context-bound so long-running connector operations are not
	// cut off by an unrelated fixed timeout.
	return &HTTPConnectorInvoker{client: httpclient.NewWithHTTPClient(baseURL, &http.Client{})}
}

func (i *HTTPConnectorInvoker) Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error) {
	var result connector.InvokeResult
	err := i.client.PostJSON(ctx, "/v1/connector/invoke", req, &result)
	return result, err
}

type HTTPToolRuntime struct {
	client *httpclient.Client
}

func NewHTTPToolRuntime(baseURL string) *HTTPToolRuntime {
	// Tool execution has its own runtime timeout. The workflow runtime should
	// not impose a second, shorter HTTP timeout while waiting for that result.
	return &HTTPToolRuntime{client: httpclient.NewWithHTTPClient(baseURL, &http.Client{})}
}

func (r *HTTPToolRuntime) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var result tool.Result
	err := r.client.PostJSON(ctx, "/v1/tools/execute", call, &result)
	return result, err
}
