package log

import (
	"context"
	"regexp"
	"sync"
)

// registeredCorrelationFields holds app-registered fields from all its components/shared libs;
// value is always 'true' to simplify key presence check
var registeredCorrelationFields map[string]bool

// registeredCorrelationFieldsMu protects registeredCorrelationFields, optimized for reads
var registeredCorrelationFieldsMu sync.RWMutex

// correlationFieldPattern allows only lower-case keys, separated by dots,
// with at least one dot to enforce presence of a namespace (or repo or app name)
var correlationFieldPattern = regexp.MustCompile(`^([a-z][a-z0-9_:]*)(\.[a-z][a-z0-9_:]*)+$`)

// correlationStateType is an internal context.Context key type to store correlation state
type correlationStateType int

// correlationState is a singleton of correlationStateType
const correlationState correlationStateType = iota

// CorrelationKey is a pre-declared log field key that this service can use for log correlation of its logs with
// the libraries used by this service. These fields MUST be declared once per-app as static variables.
// The final key will always have the app name appended to it, if such is present.
type CorrelationKey struct {
	name string
}

func MustRegisterCorrelationKey(name string) CorrelationKey {
	if !correlationFieldPattern.MatchString(name) {
		panic("correlation field name must be in form of namespace_or_app.lower_case_key'")
	}

	registeredCorrelationFieldsMu.Lock()
	defer registeredCorrelationFieldsMu.Unlock()

	if registeredCorrelationFields[name] {
		// crash is intentional - correlation key name must include namespace, hope this drammatically reduces chances of collision
		// if it still happens, crash to alert devs - it might as well a buggy copy-pasted code
		panic("duplicate registration detected for key " + name)
	}
	registeredCorrelationFields[name] = true
	return CorrelationKey{name: name}
}

type CorrelationKV struct {
	Key CorrelationKey
	// TODO: check with VK, we should probably restrict the value here to a string to reduce need for capturing and enable cross-network correlation
	Value Marshaler
}

func WithContext(ctx context.Context, ckvs ...CorrelationKV) context.Context {
	captured := make(F)

	// clone inherited fields, if present
	for k, v := range correlationFields(ctx) {
		captured[k] = v
	}

	for _, ckv := range ckvs {
		if !registered(ckv.Key) {
			// since the only valid way to create CorrelationKey is thru MustRegisterCorrelationKey, this is unlikely to happen
			// if it does happen, there is a bug that must be fixed
			// correlation logs are important logs - rather be strict and panic now than sorry later
			panic("correlation key " + ckv.Key.name + " is used without registration")
		}

		// captured.Set will recursively process values that implement MarshalLog and ensure only the final result is stored
		ckv.Value.MarshalLog(captured.Set)

		// TODO: we should probably also immediately expand all complex types into strings,
		// to avoid JSON marshalling when logging inside shared libraries
	}

	return context.WithValue(ctx, correlationState, captured)
}

func registered(k CorrelationKey) bool {
	registeredCorrelationFieldsMu.RLock()
	defer registeredCorrelationFieldsMu.RUnlock()
	return registeredCorrelationFields[k.name]
}

func correlationFields(ctx context.Context) F {
	return ctx.Value(correlationState).(F)
}
