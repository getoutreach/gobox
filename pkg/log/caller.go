// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides log entry generation for Caller

package log

import "github.com/getoutreach/gobox/pkg/caller"

// Caller returns a log entry of the form F{"caller": "fileName:nn"}
func Caller() Marshaler {
	return F{"caller": caller.FileLine(3)}
}
