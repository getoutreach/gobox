// Copyright 2022 Outreach Corporation. All Rights Reserved.
// Description: This file contains the call option definition used by trace.
package call

// Option defines the call Info adjustment function
type Option func(c *Info)

// Options contains options for all tracing calls.
type Options struct {
	// EnableInfoLogging turns on per-call info logging if set to true. If
	// true, every successful (statuscodes.CategoryOK) call will have an
	// Info line emitted.  Otherwise, it is omitted.
	EnableInfoLogging bool
}

// MarshalLog is defined for being compliant with trace.StartCall contract
func (Option) MarshalLog(addField func(string, interface{})) {}
