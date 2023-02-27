// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Provides the ability to discover the module+function who is calling you.

// Package callerinfo provides the GetCallerFunction to get the name of the module and function
// that has called you.
package callerinfo

import (
	"fmt"
	"runtime"
)

var moduleLookupByPC = make(map[uintptr]string)

// Returns the name of the function, including module if applicable, that called GetCallerFunction.
// skipFrames determines how many frames above that to skip.  If you want to know who is calling your
// function, you would pass in skipFrames of 1.  skipFrames of 0 will return yourself.
func GetCallerFunction(skipFrames uint16) (string, error) {
	// We only care about the one stack frame above the caller, so skip at least two:
	// 1. runtime.Callers
	// 2. GetCallerModule
	pc := make([]uintptr, 1)
	skipTotal := 2 + int(skipFrames)
	num := runtime.Callers(skipTotal, pc)
	if num == 0 {
		return "", fmt.Errorf("no frames returned from skip %d", skipTotal)
	}

	if mod, valid := moduleLookupByPC[pc[0]]; valid {
		// Found it in the cache
		return mod, nil
	}

	// Not cached -- have to do the slow lookup
	frames := runtime.CallersFrames(pc)
	frame, _ := frames.Next()

	// Cache for later
	moduleLookupByPC[pc[0]] = frame.Function
	return frame.Function, nil
}
