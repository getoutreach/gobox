// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: Implements the  maps package.

// Package maps provides a bunch of functions to work with maps
// This is originally intended to remove repeated code such as merging maps
package maps

import "maps"

// Merge takes two maps a and b and return the merged result.
// If overwrite is true then b will overwrite values in a on conflicting keys
func Merge[K comparable, T any](a, b map[K]T, overwrite bool) map[K]T {
	result := make(map[K]T)
	maps.Copy(result, a)
	for k, v := range b {
		if _, ok := result[k]; ok && !overwrite {
			continue
		}
		result[k] = v
	}
	return result
}
