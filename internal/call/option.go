// Copyright 2022 Outreach Corporation. All Rights Reserved.
// Description: This file contains the call option definition used by trace.
package call

// Option defines the call Info adjustment function
type Option func(c *Info)

// Options contains options for all tracing calls.
type Options struct {
	// DisableInfoLogging determines if info logging should be disabled or not
	// when a call is finished. This is useful for calls that are expected to
	// be very frequent, such as HTTP requests.
	DisableInfoLogging bool
}

// MarshalLog is defined for being compliant with trace.StartCall contract
func (Option) MarshalLog(addField func(string, interface{})) {}
