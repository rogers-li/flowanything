package domain

import (
	"fmt"

	"flow-anything/internal/platform/contracts/connector"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

func ValidateInvokeRequest(req connector.InvokeRequest) error {
	if req.OperationID.Empty() {
		return apperrors.New(apperrors.CodeInvalidArgument, "operation_id is required")
	}
	return nil
}

func NewOperationNotEnabledError(operationID id.ID, status connector.OperationStatus) error {
	return apperrors.New(
		apperrors.CodeInvalidArgument,
		fmt.Sprintf("connector operation %s is not enabled: status=%s", operationID.String(), status),
	)
}
