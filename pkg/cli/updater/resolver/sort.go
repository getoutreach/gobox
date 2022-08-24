// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains sorting helpers for a list
// of versions.

package resolver

import "sort"

// Versions represents multiple versions.
type Versions []Version

// Len returns length of version collection
func (s Versions) Len() int {
	return len(s)
}

// Swap swaps two versions inside the collection by its indices
func (s Versions) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less checks if version at index i is less than version at index j
func (s Versions) Less(i, j int) bool {
	// if either of these versions are mutable, they are always
	// ranked less than the other.
	if s[i].mutable {
		// less than j
		return true
	}

	if s[j].mutable {
		// greater than i
		return false
	}

	return s[i].sv.LT(s[j].sv)
}

// Sort sorts a slice of versions
func Sort(versions []Version) {
	sort.Sort(Versions(versions))
}
