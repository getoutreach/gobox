package log

import "github.com/getoutreach/gobox/internal/caller"

// Caller returns a log entry of the form F{"caller": "fileName:nn"}
func Caller() Marshaler {
	return F{"caller": caller.FileLine(3)}
}
