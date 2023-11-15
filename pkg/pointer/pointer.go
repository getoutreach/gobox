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

func ToPtr[T any](object T) *T {
	return &object
}

func ToSlicePtr[T any](objects []T) []*T {
	ptrObj := make([]*T, 0, len(objects))
	for _, o := range objects {
		ptrObj = append(ptrObj, ToPtr(o))
	}
	return ptrObj
}

func ToMapPtr[K comparable, T any](objectMap map[K]T) map[K]*T {
	ptrObj := make(map[K]*T, len(objectMap))
	for k, o := range objectMap {
		ptrObj[k] = ToPtr(o)
	}
	return ptrObj
}

func ToValue[T any](ptr *T) (res T) {
	if ptr == nil {
		return res
	}
	return *ptr
}

func ToSliceValue[T any](ptrs []*T) []T {
	res := make([]T, 0, len(ptrs))
	for _, ptr := range ptrs {
		res = append(res, ToValue(ptr))
	}
	return res
}

func ToMapValue[K comparable, T any](ptrs map[K]*T) map[K]T {
	res := make(map[K]T, len(ptrs))
	for v, ptr := range ptrs {
		res[v] = ToValue(ptr)
	}
	return res
}
