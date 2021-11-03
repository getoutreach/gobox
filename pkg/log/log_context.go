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

func filterAllowList(info F) F {
	result := F{}
	if allowList == nil {
		panic("AllowContextFields must be set in order to use log.Context")
	}

	for k, v := range info {
		_, ok := allowList[k]

		if !ok {
			continue
		}

		result[k] = v
	}

	return result
}

type logContextKeyType int

const currentLogContextKey logContextKeyType = iota

// Creates a new Value context to store fields that should be attached to all logs
// MarshalLog is invoked immediately on all args to reduce risk of hard to debug issues
// and arbitrary code running during logging.
func NewContext(ctx context.Context, args ...Marshaler) context.Context {
	temp := F{}
	infoKeyVal := ctx.Value(currentLogContextKey)
	if infoKeyVal != nil {
		logInfo := infoKeyVal.(Marshaler)
		logInfo.MarshalLog(temp.Set)
	}

	many := Many(args)
	many.MarshalLog(temp.Set)
	temp = filterAllowList(temp)
	return context.WithValue(ctx, currentLogContextKey, temp) //nolint:revive, staticcheck
}

func getLogInfo(ctx context.Context) Marshaler {
	if infoKeyVal := ctx.Value(currentLogContextKey); infoKeyVal != nil {
		logInfo := infoKeyVal.(Marshaler)
		return logInfo
	}

	return nil
}
