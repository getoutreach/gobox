package trace

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

// provider is a generic interface for tracking calls.
type provider interface {
	Start(ctx context.Context, info *call.Info) context.Context
	End(ctx context.Context, info *call.Info)
}

// manyProviders just wraps multiple providers
type manyProviders []provider

func (px manyProviders) Start(ctx context.Context, info *call.Info) context.Context {
	for _, p := range px {
		ctx = p.Start(ctx, info)
	}
	return ctx
}

func (px manyProviders) End(ctx context.Context, info *call.Info) {
	for idx := range px {
		px[len(px)-1-idx].End(ctx, info)
	}
}

var providers = manyProviders{&defaultCallProvider{}}

// AddProvider is an experimental interface to add a provider.
//
// Note that the provider interface is private as this is still experimental.
// Use x/tracelog.New for an experimental trace log provider.
func AddProvider(p provider) {
	providers = append(providers, p)
}

type defaultCallProvider struct{}

func (d *defaultCallProvider) Start(ctx context.Context, info *call.Info) context.Context {
	log.Debug(ctx, fmt.Sprintf("calling: %s", info.Name), info.Args...)
	return ctx
}

func (d *defaultCallProvider) End(ctx context.Context, info *call.Info) {
	addDefaultTracerInfo(ctx, info)
	info.ReportLatency(ctx)

	if info.ErrInfo != nil {
		switch category := orerr.ExtractErrorStatusCategory(info.ErrInfo.RawError); category {
		case statuscodes.CategoryClientError:
			log.Warn(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
		case statuscodes.CategoryServerError:
			log.Error(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
		case statuscodes.CategoryOK: // just in case if someone will return non-nil error on success
			log.Info(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
		}
	} else {
		log.Info(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
	}
}
