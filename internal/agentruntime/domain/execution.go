package domain

import (
	"time"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

type Execution struct {
	Call      tool.Call
	StartedAt time.Time
}

func NewExecution(call tool.Call) (Execution, error) {
	if call.ToolID.Empty() && call.Name == "" {
		return Execution{}, apperrors.New(apperrors.CodeInvalidArgument, "tool_id or tool name is required")
	}

	return Execution{
		Call:      call,
		StartedAt: time.Now().UTC(),
	}, nil
}

func (e Execution) Failure(code string, reason string) tool.Result {
	return tool.Result{
		CallID:      e.Call.ID,
		ToolID:      e.Call.ToolID,
		Success:     false,
		ErrorCode:   code,
		ErrorReason: reason,
		StartedAt:   e.StartedAt,
		FinishedAt:  time.Now().UTC(),
	}
}
