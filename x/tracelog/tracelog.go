// tracelog is an experimental trace/call logger.
package tracelog

import (
	"context"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
	"github.com/getoutreach/gobox/pkg/trace"
)

// Option specifies an option to the tracelogger.
type Option func(p *Provider)

// New returns a tracelogger.  Use  this with trace.AddProvider.
func New(opts ...Option) *Provider {
	result := &Provider{
		extractf: func(info *call.Info) logf.Marshaler {
			return nil
		},
	}
	for _, opt := range opts {
		opt(result)
	}
	return result
}

// Provider implements a trace/call provider.
type Provider struct {
	extractf func(info *call.Info) log.Marshaler
}

func (p *Provider) Start(ctx context.Context, info *call.Info) context.Context {
	log.Info(ctx, "calling: "+info.Name, info, trace.IDs(ctx), p.extractf(info))
	return ctx
}

func (p *Provider) End(ctx context.Context, info *call.Info) {
	if info.ErrInfo != nil {
		switch category := orerr.ExtractErrorStatusCategory(info.ErrInfo.RawError); category {
		case statuscodes.CategoryClientError:
			log.Warn(ctx, "called: "+info.Name, info, trace.IDs(ctx), p.extractf(info))
		case statuscodes.CategoryServerError:
			log.Error(ctx, "called: "+info.Name, info, trace.IDs(ctx), p.extractf(info))
		case statuscodes.CategoryOK: // just in case if someone will return non-nil error on success
			log.Info(ctx, "called: "+info.Name, info, trace.IDs(ctx), p.extractf(info))
		}
	} else {
		log.Info(ctx, "called: "+info.Name, info, trace.IDs(ctx), p.extractf(info))
	}
}

// WithAllInheritedArgs adds the args from all the parent context into the
// current logs.
func WithAllInheritedArgs() Option {
	extractf := func(info *call.Info) log.Marshaler {
		all := log.Many{}
		for info.Parent != nil {
			info = info.Parent
			all = append(all, info.Args...)
		}
		return all
	}
	return func(p *Provider) {
		p.extractf = extractf
	}
}
