package log

import "github.com/getoutreach/gobox/internal/logf"

// Many aggregates marshaling of many items
//
// This avoids having to build an append list and also simplifies code
type Many = logf.Many
