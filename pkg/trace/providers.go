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

var providers = manyProviders{}

// WithProvider adds a provider to the list of providers.
func WithProvider(p provider) Option {
	return func(s *settings) error {
		s.providers = append(s.providers, p)
		return nil
	}
}

type defaultCallLogger struct{}

func (defaultCallLogger) Start(ctx context.Context, info *call.Info) context.Context {
	log.Debug(ctx, fmt.Sprintf("calling: %s", info.Name), info.Args...)
	return ctx
}

func (defaultCallLogger) End(ctx context.Context, info *call.Info) {
	if info.ErrInfo != nil {
		switch category := orerr.ExtractErrorStatusCategory(info.ErrInfo.RawError); category {
		case statuscodes.CategoryClientError:
			log.Warn(ctx, info.Name, info, info.IDs, traceEventMarker{})
		case statuscodes.CategoryServerError:
			log.Error(ctx, info.Name, info, info.IDs, traceEventMarker{})
		case statuscodes.CategoryOK: // just in case if someone will return non-nil error on success
			log.Info(ctx, info.Name, info, info.IDs, traceEventMarker{})
		}
	} else {
		log.Info(ctx, info.Name, info, info.IDs, traceEventMarker{})
	}
}

type traceEventMarker struct{}

func (traceEventMarker) MarshalLog(addField func(k string, v interface{})) {
	addField("event_name", "trace")
}
