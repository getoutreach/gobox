// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the helper utilities for marshaling log fields.

// package logf has the log.F implementation
package logf

import "strings"

// Marshaler is the same interface as log.Marshaler.
type Marshaler interface {
	MarshalLog(addField func(key string, v interface{}))
}

// Marshal is a helper that checks if the interface provided
// implements Marshaler.  If it does, it recursively calls MarshalLog
// building up the prefixes long (combining them with ".").
func Marshal(prefix string, v interface{}, setField func(key string, value interface{})) {
	if rm, ok := v.(interface{ MarshalRoot() Marshaler }); ok {
		Marshal("", rm.MarshalRoot(), setField)
	}

	if m, ok := v.(Marshaler); ok {
		m.MarshalLog(func(inner string, val interface{}) {
			if prefix == "" {
				Marshal(inner, val, setField)
			} else {
				Marshal(prefix+"."+inner, val, setField)
			}
		})
	} else if prefix != "" {
		setField(prefix, v)
	}
}

// F implements a generic log.Marshaler interface
type F map[string]interface{}

// Set writes the field value into F. If the value implements
// interface { MarshalRoot() log.Marshaler } then it marshals it
// from the root level. If the value is a
// log.Marshaler, it recursively marshals that value into F.
func (f F) Set(field string, value interface{}) {
	Marshal(field, value, func(key string, value interface{}) {
		if f["level"] == "FATAL" && strings.HasPrefix(key, "error.") {
			// if this is a FATAL, make room for the root call stack
			key = "error.cause." + key[6:]
		}
		f[key] = value
	})
}

// MarshalLog implements the Marshaler interface for F
func (f F) MarshalLog(addField func(field string, value interface{})) {
	for k, v := range f {
		addField(k, v)
	}
}

// Many aggregates marshaling of many items
//
// This avoids having to build an append list and also simplifies code
type Many []Marshaler

// MarshalLog calls MarshalLog on all the individual elements
func (m Many) MarshalLog(addField func(key string, v interface{})) {
	for _, item := range m {
		if item != nil {
			item.MarshalLog(addField)
		}
	}
}
