// Description: This package implements a trace provider for use with pkg/trace.InitTrace.

package metrics

import (
	"context"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/pkg/app"
)

// TraceProvider implements a metrics trace provider for use with pkg/trace.InitTrace.
type TraceProvider struct{}

func (TraceProvider) Start(ctx context.Context, info *call.Info) context.Context {
	return ctx
}

func (TraceProvider) End(ctx context.Context, info *call.Info) {
	var err error
	if info.ErrInfo != nil {
		err = info.ErrInfo.RawError
	}

	name, kind := app.Info().Name, WithCallKind(CallKindInternal)
	switch info.Type {
	case call.TypeHTTP:
		ReportHTTPLatency(name, info.Name, info.ServiceSeconds, err, kind)
	case call.TypeGRPC:
		ReportGRPCLatency(name, info.Name, info.ServiceSeconds, err, kind)
	case call.TypeOutbound:
		ReportOutboundLatency(name, info.Name, info.ServiceSeconds, err, kind)
	default:
		// do not report anything.
	}
}
