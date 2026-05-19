package app

import (
	"context"
	"fmt"

	"flow-anything/core/connector"
	"flow-anything/core/tools"
)

const (
	toolImplementationConnector = "connector"
	toolImplementationWorkflow  = "workflow"
)

type connectorToolExecutor struct {
	connectors *connector.Runtime
}

func (e connectorToolExecutor) Kind() string { return toolImplementationConnector }

func (e connectorToolExecutor) Validate(tool tools.ToolSpec) error {
	if tool.Implementation.Ref == "" {
		return fmt.Errorf("connector tool %q requires implementation.ref operation id", tool.ID)
	}
	if e.connectors == nil {
		return fmt.Errorf("connector runtime is not configured")
	}
	return nil
}

func (e connectorToolExecutor) Execute(ctx tools.Context, req tools.ToolExecutionRequest) (tools.ToolExecutionResult, error) {
	result, err := e.connectors.Invoke(contextFromTool(ctx), connector.InvokeRequest{
		OperationID:  req.Tool.Implementation.Ref,
		Input:        req.Input,
		TraceID:      req.Call.TraceID,
		TraceContext: childTraceContext(req.Call.TraceContext),
	})
	if err != nil {
		return tools.ToolExecutionResult{}, err
	}
	return tools.ToolExecutionResult{Output: result.Output, Raw: result}, nil
}

type workflowToolExecutor struct {
	workflows *WorkflowService
}

func (e workflowToolExecutor) Kind() string { return toolImplementationWorkflow }

func (e workflowToolExecutor) Validate(tool tools.ToolSpec) error {
	if tool.Implementation.Ref == "" {
		return fmt.Errorf("workflow tool %q requires implementation.ref workflow id", tool.ID)
	}
	if e.workflows == nil {
		return fmt.Errorf("workflow runtime is not configured")
	}
	return nil
}

func (e workflowToolExecutor) Execute(ctx tools.Context, req tools.ToolExecutionRequest) (tools.ToolExecutionResult, error) {
	result, err := e.workflows.Run(contextFromTool(ctx), WorkflowRequest{
		WorkflowID:   req.Tool.Implementation.Ref,
		Input:        req.Input,
		TraceContext: childTraceContext(req.Call.TraceContext),
	})
	if err != nil {
		return tools.ToolExecutionResult{}, err
	}
	return tools.ToolExecutionResult{Output: result.Output, Raw: result.Instance}, nil
}

func contextFromTool(ctx tools.Context) context.Context {
	if standard, ok := ctx.(context.Context); ok {
		return standard
	}
	return context.Background()
}
