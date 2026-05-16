package trace

import "context"

type Exporter interface {
	ExportTrace(ctx context.Context, trace Trace) error
}
