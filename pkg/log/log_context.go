package log

import (
	"context"
)

var allowList map[string]bool

func AllowContextFields(fields ...string) {
	if allowList != nil {
		panic("the log context fields allowed list can only be set once")
	}

	allowList = map[string]bool{}
	for _, v := range fields {
		allowList[v] = true
	}
}

// nolint:gochecknoglobals
var infoKey = &F{}

// Creates a new Value context to store fields that should be attached to all logs
func NewContext(ctx context.Context) context.Context {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		return ctx
	}

	return context.WithValue(ctx, infoKey, &F{})
}

// Add arguments to all logs. Return true if this is a log info context after args are added
// MarshalLog is invokved immediately on all args to reduce risk of hard to debug issues
// and arbitrary code running during logging.
//
// If the current context is not the log info contex, AddInfo does nothing and return false.
func AddInfo(ctx context.Context, args ...Marshaler) {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		logInfo := infoKeyVal.(*F)
		many := Many(args)
		set := logInfo.Set
		if allowList != nil {
			set = func(field string, value interface{}) {
				_, ok := allowList[field]
				if ok {
					logInfo.Set(field, value)
				}
			}
		}
		many.MarshalLog(set)
	}
}

func getLogInfo(ctx context.Context) Marshaler {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		logInfo := infoKeyVal.(*F)
		return logInfo
	}

	return nil
}
