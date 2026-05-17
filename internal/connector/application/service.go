package application

import (
	"context"
	"log/slog"

	"flow-anything/internal/connector/domain"
	"flow-anything/internal/connector/ports"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/id"
)

type Service struct {
	logger     *slog.Logger
	operations ports.OperationRepository
	invoker    ports.OperationInvoker
}

func New(logger *slog.Logger, operations ports.OperationRepository, invoker ports.OperationInvoker) *Service {
	return &Service{
		logger:     logger,
		operations: operations,
		invoker:    invoker,
	}
}

func (s *Service) Invoke(ctx context.Context, req connector.InvokeRequest) (connector.InvokeResult, error) {
	if req.ID.Empty() {
		req.ID = id.New("connreq")
	}
	if err := domain.ValidateInvokeRequest(req); err != nil {
		return connector.InvokeResult{}, err
	}
	operation, err := s.operations.GetOperation(ctx, req.TenantID, req.OperationID)
	if err != nil {
		return connector.InvokeResult{}, err
	}
	if operation.Status != "" && operation.Status != connector.OperationStatusEnabled {
		return connector.InvokeResult{}, domain.NewOperationNotEnabledError(operation.ID, operation.Status)
	}

	s.logger.Info("connector invoke accepted",
		"request_id", req.ID.String(),
		"operation_id", req.OperationID.String(),
	)

	return s.invoker.Invoke(ctx, operation, req)
}
