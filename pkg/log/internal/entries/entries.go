// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides an interface to work with Entries

package entries

import (
	"sync"
	"time"
)

// maxItems is the maximum number of debug entries cached
const MaxItems = 200

// maxduration is the age past which a debug entry is considered stale.
const MaxDuration = time.Minute * 2

// New returns a new collection of log entries
func New() *Entries {
	return &Entries{}
}

// Entries holds a limited size buffer of formatted debug entries
type Entries struct { //nolint:gocritic // Why: Will refactor in the future
	sync.Mutex
	items []item
}

func (e *Entries) Append(message string) {
	e.Lock()
	defer e.Unlock()

	e.items = append(e.items, item{message, time.Now()})
	if len(e.items) > MaxItems {
		e.items = e.items[1:]
	}
}

func (e *Entries) Flush(write func(s string)) {
	e.Lock()
	items := e.items
	e.items = nil
	e.Unlock()

	for _, entry := range items {
		if time.Since(entry.ts) <= MaxDuration {
			write(entry.s)
		}
	}
}

func (e *Entries) Purge() {
	e.Lock()
	defer e.Unlock()

	e.items = nil
}

type item struct {
	s  string
	ts time.Time
}
