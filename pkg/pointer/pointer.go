// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Implements the pointer package.

// Package pointer is an attempt to provide functions to convert data to pointers using generics.
// inspired by https://pkg.go.dev/github.com/aws/smithy-go/ptr
// This is intended to replace the following patterns that we've seen across our codebase:
// - myVar := "value"; return &myVar
// - ptr.{SomeType}("value")
// Also Enables the possibility to use: var res *myStructType = ToPtr(myStructType{}) and any derivate
// with map / slice
package pointer

// ToPtr returns a pointer to {object} of type *T
func ToPtr[T any](object T) *T {
	return &object
}

// ToSlicePtr returns a slice of pointers of type *T pointing to each element of {objects}
func ToSlicePtr[T any](objects []T) []*T {
	ptrObj := make([]*T, 0, len(objects))
	for _, o := range objects {
		ptrObj = append(ptrObj, ToPtr(o))
	}
	return ptrObj
}

// ToMapPtr returns a map of pointers of type *T pointing to each element of {objectMap}
func ToMapPtr[K comparable, T any](objectMap map[K]T) map[K]*T {
	ptrObj := make(map[K]*T, len(objectMap))
	for k, o := range objectMap {
		ptrObj[k] = ToPtr(o)
	}
	return ptrObj
}

// ToValue returns the value of type T pointed by {ptr}
// if {ptr} is nil, return 0 value of T
func ToValue[T any](ptr *T) (res T) {
	if ptr == nil {
		return res
	}
	return *ptr
}

// ToSliceValue returns a slice of values of type T pointed by each pointers in {ptrs}
// if a pointer in {ptrs} is nil the value at the corresponding index will be the 0 of T
func ToSliceValue[T any](ptrs []*T) []T {
	res := make([]T, 0, len(ptrs))
	for _, ptr := range ptrs {
		res = append(res, ToValue(ptr))
	}
	return res
}

// ToSliceValue returns a map of values of type T pointed by each pointers in {ptrs}
// if a pointer in {ptrs} is nil the value at the corresponding key will be the 0 of T
func ToMapValue[K comparable, T any](ptrs map[K]*T) map[K]T {
	res := make(map[K]T, len(ptrs))
	for v, ptr := range ptrs {
		res[v] = ToValue(ptr)
	}
	return res
}
