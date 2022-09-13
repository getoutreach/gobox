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
	return s[i].LessThan(&s[j])
}

// Sort sorts a slice of versions
func Sort(versions []Version) {
	sort.Sort(Versions(versions))
}

// LessThan returns true if the version is less than the other
func (v *Version) LessThan(other *Version) bool {
	// If we're using a mutable version it's never less
	// than another version
	if v.Mutable {
		return false
	}

	// If the other version is mutable, then it is always
	// greater than our version.
	if other.Mutable {
		return true
	}

	// otherwise fall back to standard semantic-version
	// comparison
	return v.sv.LessThan(other.sv)
}

// Equal returns true if the version is equal to the other
func (v *Version) Equal(other *Version) bool {
	if v.Mutable {
		// if we're mutable, then we're only equal to other
		// mutable versions if the commit and channel match
		if v.Commit == other.Commit && v.Channel == other.Channel {
			return true
		}

		// otherwise they are not equal
		return false
	}

	if other.Mutable {
		// if the other version is mutable but we are not, then we are not equal
		return false
	}

	// otherwise, fall back to standard semantic-version
	// comparison
	return v.sv.Equal(other.sv)
}
