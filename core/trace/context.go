package trace

import (
	"context"

	"flow-anything/core/runtimecontext"
)

// TraceContext is kept as a type alias for callers that import core/trace
// directly. The canonical propagation protocol lives in core/runtimecontext.
type TraceContext = runtimecontext.TraceContext

func WithTraceContext(ctx context.Context, traceContext TraceContext) context.Context {
	return runtimecontext.WithTraceContext(ctx, traceContext)
}

func ContextFrom(ctx interface{ Value(key any) any }) (TraceContext, bool) {
	return runtimecontext.TraceContextFrom(ctx)
}
