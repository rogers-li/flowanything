package connector

import (
	"context"
	"time"

	connectorcontract "flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type Invoker interface {
	Invoke(ctx context.Context, req connectorcontract.InvokeRequest) (connectorcontract.InvokeResult, error)
}

type Adapter struct {
	invoker Invoker
}

func New(invoker Invoker) *Adapter {
	return &Adapter{invoker: invoker}
}

func (a *Adapter) Supports(kind tool.ImplementationType) bool {
	return kind == tool.ImplementationConnector
}

func (a *Adapter) Execute(ctx context.Context, spec tool.Spec, call tool.Call) (tool.Result, error) {
	startedAt := time.Now().UTC()
	if a.invoker == nil {
		return tool.Result{}, apperrors.New(apperrors.CodeUnavailable, "connector invoker is not configured")
	}
	if spec.Binding.ConnectorOperationID.Empty() {
		return tool.Result{}, apperrors.New(apperrors.CodeInvalidArgument, "connector operation binding is required")
	}

	req := connectorcontract.InvokeRequest{
		ID:          id.New("connreq"),
		TenantID:    call.TenantID,
		OperationID: spec.Binding.ConnectorOperationID,
		Args:        call.Args,
		TraceID:     call.TraceID,
	}
	resp, err := a.invoker.Invoke(ctx, req)
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		CallID:     call.ID,
		ToolID:     spec.ID,
		Success:    resp.Success,
		Data:       resp.Data,
		ErrorCode:  resp.ErrorCode,
		StartedAt:  startedAt,
		FinishedAt: resp.FinishedAt,
	}, nil
}
