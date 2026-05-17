package trace

import (
	"context"

	"flow-anything/internal/platform/kernel/id"
)

type ID string

type contextKey struct{}

func New() ID {
	return ID(id.New("trace").String())
}

func (i ID) String() string {
	return string(i)
}

func WithID(ctx context.Context, traceID ID) context.Context {
	return context.WithValue(ctx, contextKey{}, traceID)
}

func FromContext(ctx context.Context) (ID, bool) {
	traceID, ok := ctx.Value(contextKey{}).(ID)
	return traceID, ok
}

func Ensure(ctx context.Context) (context.Context, ID) {
	if traceID, ok := FromContext(ctx); ok && traceID.String() != "" {
		return ctx, traceID
	}

	traceID := New()
	return WithID(ctx, traceID), traceID
}
