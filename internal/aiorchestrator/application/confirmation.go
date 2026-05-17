package application

import (
	"errors"
	"strings"

	"flow-anything/internal/platform/contracts/event"
	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

const confirmationPrompt = "该操作需要你的确认后才能执行。"

type confirmationRequiredError struct {
	call tool.Call
	err  error
}

func (e *confirmationRequiredError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return confirmationPrompt
}

func (e *confirmationRequiredError) Unwrap() error {
	return e.err
}

func newConfirmationRequiredError(call tool.Call, err error) error {
	return &confirmationRequiredError{call: call, err: err}
}

func confirmationFromError(err error) (tool.Call, bool) {
	var confirmationErr *confirmationRequiredError
	if errors.As(err, &confirmationErr) {
		return confirmationErr.call, true
	}
	return tool.Call{}, false
}

func isRuntimeConfirmationRequired(err error) bool {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return false
	}
	return appErr.Code == apperrors.CodeForbidden &&
		strings.Contains(strings.ToLower(appErr.Message), "requires confirmation")
}

func confirmationResponse(evt event.Event, call tool.Call) event.Response {
	if call.TenantID.Empty() {
		call.TenantID = evt.TenantID
	}
	if call.TraceID == "" {
		call.TraceID = evt.TraceID
	}

	return event.Response{
		EventID: evt.ID,
		TraceID: evt.TraceID,
		Actions: []event.Action{
			{
				Type:     event.ActionAskConfirmation,
				Text:     confirmationPrompt,
				ToolCall: &call,
			},
			{
				Type: event.ActionSpeak,
				Text: confirmationPrompt,
			},
			{
				Type: event.ActionEndTurn,
			},
		},
	}
}
