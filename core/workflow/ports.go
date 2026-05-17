package workflow

import (
	"context"

	"flow-anything/core/agentcore"
	"flow-anything/core/runtimecontext"
)

// ConnectorInvoker adapts external business APIs into workflow connector nodes.
type ConnectorInvoker interface {
	InvokeConnector(ctx context.Context, req ConnectorInvokeRequest) (ConnectorInvokeResult, error)
}

type ConnectorInvokeRequest struct {
	OperationID  string
	Input        map[string]any
	Metadata     map[string]any
	TraceContext runtimecontext.TraceContext
}

type ConnectorInvokeResult struct {
	Output map[string]any
	Raw    any
}

// ToolInvoker adapts platform tools into workflow tool nodes.
type ToolInvoker interface {
	InvokeTool(ctx context.Context, req ToolInvokeRequest) (ToolInvokeResult, error)
}

type ToolInvokeRequest struct {
	ToolID       string
	Input        map[string]any
	Metadata     map[string]any
	TraceContext runtimecontext.TraceContext
}

type ToolInvokeResult struct {
	Output map[string]any
	Raw    any
}

// AgentRunner adapts agentcore or an external agent runtime into workflow agent
// nodes.
type AgentRunner interface {
	RunAgent(ctx context.Context, req AgentRunRequest) (AgentRunResult, error)
}

type AgentRunRequest struct {
	Agent        agentcore.AgentSpec
	Input        map[string]any
	Message      string
	Metadata     map[string]any
	TraceContext runtimecontext.TraceContext
}

type AgentRunResult struct {
	Output      map[string]any
	Text        string
	NextNodeIDs []string
	Raw         any
}
