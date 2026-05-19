package app

import (
	"context"

	"flow-anything/core/connector"
	"flow-anything/core/tools"
	"flow-anything/core/workflow"
)

type toolNodeInvoker struct {
	runtime *tools.Runtime
}

type connectorNodeInvoker struct {
	runtime *connector.Runtime
}

func (i toolNodeInvoker) InvokeTool(ctx context.Context, req workflow.ToolInvokeRequest) (workflow.ToolInvokeResult, error) {
	result, err := i.runtime.Invoke(ctx, tools.ToolCall{
		ToolID:       req.ToolID,
		Input:        req.Input,
		Metadata:     req.Metadata,
		TraceContext: req.TraceContext,
	})
	if err != nil {
		return workflow.ToolInvokeResult{}, err
	}
	return workflow.ToolInvokeResult{Output: result.Output, Raw: result.Raw}, nil
}

func (i connectorNodeInvoker) InvokeConnector(ctx context.Context, req workflow.ConnectorInvokeRequest) (workflow.ConnectorInvokeResult, error) {
	result, err := i.runtime.Invoke(ctx, connector.InvokeRequest{
		OperationID:  req.OperationID,
		Input:        req.Input,
		Metadata:     req.Metadata,
		TraceContext: req.TraceContext,
	})
	if err != nil {
		return workflow.ConnectorInvokeResult{}, err
	}
	return workflow.ConnectorInvokeResult{Output: result.Output, Raw: result.Raw}, nil
}
