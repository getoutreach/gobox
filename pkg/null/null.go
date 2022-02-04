// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Imports https://github.com/getoutreach/null

// Package null is a library with reasonable options for dealing with nullable SQL and JSON values. Types in null will
// only be considered null on null input, and will JSON encode to null. If you need zero and null be considered
// separate values, use these.
package null

import "github.com/getoutreach/null"

type Bool = null.Bool
type Float = null.Float
type Int = null.Int
type String = null.String
type Time = null.Time
