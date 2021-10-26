package log

import (
	"context"
	"strings"
)

var allowList map[string]interface{}

func AllowContextFields(fields ...string) {
	if allowList != nil {
		panic("the log context fields allowed list can only be set once")
	}

	allowList = map[string]interface{}{}
	for _, v := range fields {
		allowList[v] = true
		parts := strings.Split(v, ".")
		current := allowList
		for _, part := range parts {
			nested := map[string]interface{}{}
			current[part] = nested
			current = nested
		}
	}
}

func filterAllowList(info F, allow map[string]interface{}) F {
	result := F{}
	if allow == nil {
		panic("AllowContextFields must be set in order to use log.Context")
	}

	for k, v := range info {
		allow, ok := allowList[k]

		if !ok {
			continue
		}

		_, ok = allow.(bool)
		if ok {
			result[k] = v
		}

		childAllow, ok := allow.(map[string]interface{})
		childF, fOk := v.(F)
		if ok && fOk {
			childValue := filterAllowList(childF, childAllow)
			if len(childValue) > 0 {
				result[k] = childValue
			}
		}
	}

	return result
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
		temp := F{}
		many := Many(args)
		many.MarshalLog(temp.Set)
		temp = filterAllowList(temp, allowList)
		temp.MarshalLog(logInfo.Set)
	}
}

func getLogInfo(ctx context.Context) Marshaler {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		logInfo := infoKeyVal.(*F)
		return logInfo
	}

	return nil
}
