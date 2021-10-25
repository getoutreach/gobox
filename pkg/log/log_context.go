package log

import (
	"context"
	"fmt"
)

type logInfo struct {
	Fields map[string]interface{}
}

func (li *logInfo) MarshalLog(addField func(field string, value interface{})) {
	if li == nil {
		return
	}

	for k, v := range li.Fields {
		addField(k, v)
	}
}

func (li *logInfo) addField(key string, v interface{}) {
	if li == nil {
		return
	}

	li.Fields[key] = convertValue(v)
}

func (li *logInfo) addArgFields(args []Marshaler) {
	for _, arg := range args {
		arg.MarshalLog(li.addField)
	}
}

func convertValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return val
	case bool:
		return val
	case int:
		return val
	case int8:
		return val
	case int16:
		return val
	case int32:
		return val
	case int64:
		return val
	case uint:
		return val
	case uint8:
		return val
	case uint16:
		return val
	case uint32:
		return val
	case uint64:
		return val
	case float32:
		return val
	case float64:
		return val
	default:
		vStringer, ok := v.(fmt.Stringer)
		if ok {
			return vStringer.String()
		}
	}

	panic("try to add unsupported field to call info")
}

// nolint:gochecknoglobals
var infoKey = &logInfo{}

// Creates a new Value context to store fields that should be attached to all logs
func NewLogContext(ctx context.Context) context.Context {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		return ctx
	}

	return context.WithValue(ctx, infoKey, &logInfo{
		Fields: map[string]interface{}{},
	})
}

// Add arguments to all logs. Return true if this is a log info context after args are added
// MarshalLog is invokved immediately on all args to reduce risk of hard to debug issues
// and arbitrary code running during logging.
//
// If the current context is not the log info contex, AddInfo does nothing and return false.
func AddInfo(ctx context.Context, args ...Marshaler) bool {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		logInfo := infoKeyVal.(*logInfo)
		logInfo.addArgFields(args)
		return true
	}

	return false
}

func getLogInfo(ctx context.Context) Marshaler {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		logInfo := infoKeyVal.(*logInfo)
		return logInfo
	}

	return nil
}
