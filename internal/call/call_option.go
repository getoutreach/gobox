// Copyright 2022 Outreach Corporation. All Rights Reserved.
// Description: This file contains the call option definition used by trace.
package call

// CallOption defines the call Info adjustment function
type CallOption func(c *Info)

// MarshalLog is defined for being compliant with trace.StartCall contract
func (CallOption) MarshalLog(addField func(string, interface{})) {}
