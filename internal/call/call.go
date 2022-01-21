// The call package helps support tracking latency and other metrics for calls.
package call

import (
	"context"
	"time"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/metrics"
)

// Type tracks the call type.
type Type string

const (
	// TypeHTTP is a constant that denotes the call type being an HTTP
	// request.
	TypeHTTP Type = "http"

	// TypeGRPC is a constant that denotes the call type being a gRPC
	// request.
	TypeGRPC Type = "grpc"

	// TypeOutbound is a constant that denotes the call type being an
	// outbound request.
	TypeOutbound Type = "outbound"
)

// Info tracks information about an ongoing synchronous call.
type Info struct {
	Name string
	Type Type
	Kind metrics.CallKind
	Args []logf.Marshaler

	events.Times
	events.Durations

	ErrInfo *events.ErrorInfo
}

func (info *Info) Start(ctx context.Context, name string) {
	info.Name = name
	if info.Kind == "" {
		info.Kind = metrics.CallKindInternal
	}
	info.Times.Started = time.Now()
}

func (info *Info) End(ctx context.Context) {
	info.Times.Finished = time.Now()
	info.Durations = *info.Times.Durations()
}

func (info *Info) ReportLatency(ctx context.Context) {
	var err error
	if info.ErrInfo != nil {
		err = info.ErrInfo.RawError
	}

	name, kind := app.Info().Name, metrics.WithCallKind(info.Kind)
	switch info.Type {
	case TypeHTTP:
		metrics.ReportHTTPLatency(name, info.Name, info.ServiceSeconds, err, kind)
	case TypeGRPC:
		metrics.ReportGRPCLatency(name, info.Name, info.ServiceSeconds, err, kind)
	case TypeOutbound:
		metrics.ReportOutboundLatency(name, info.Name, info.ServiceSeconds, err, kind)
	default:
		// do not report anything.
	}
}

func (info *Info) AddArgs(ctx context.Context, args ...logf.Marshaler) {
	info.Args = append(info.Args, args...)
}

func (info *Info) SetStatus(ctx context.Context, err error) {
	info.ErrInfo = events.NewErrorInfo(err)
}

func (info *Info) MarshalLog(addField func(key string, value interface{})) {
	info.Times.MarshalLog(addField)
	info.Durations.MarshalLog(addField)
	logf.Many(info.Args).MarshalLog(addField)
	info.ErrInfo.MarshalLog(addField)
}

// Tracker helps manage a call info via the context.
type Tracker struct {
}

func (t *Tracker) StartCall(ctx context.Context, name string, args []logf.Marshaler) context.Context {
	var info Info
	info.Start(ctx, name)
	info.AddArgs(ctx, args...)
	return context.WithValue(ctx, t, &info)
}

func (t *Tracker) Info(ctx context.Context) *Info {
	if v := ctx.Value(t); v != nil {
		return v.(*Info)
	}
	return nil
}

func (t *Tracker) EndCall(ctx context.Context) {
	info := t.Info(ctx)
	if r := recover(); r != nil {
		info.ErrInfo = events.NewErrorInfoFromPanic(r)

		// rethrow at end of the function
		defer panic(r)
	}

	info.End(ctx)
}
