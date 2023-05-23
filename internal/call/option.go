// Copyright 2022 Outreach Corporation. All Rights Reserved.
// Description: This file contains the call option definition used by trace.
package call

// Option defines the call Info adjustment function
type Option func(c *Info)

// MarshalLog is defined for being compliant with trace.StartCall contract
func (Option) MarshalLog(addField func(string, interface{})) {}
