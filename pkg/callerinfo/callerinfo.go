// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Provides the ability to discover the module+function who is calling you.

// Package callerinfo provides the GetCallerFunction to get the name of the module and function
// that has called you.
package callerinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// CallerInfo holds basic information about a call site:
// Function is a long form module+function name that looks like:
// * main.main for your own main function
// * github.com/getoutreach/gobox/pkg/callerinfo.Test_Callers for a module function
// File is the file path of the call site
// LineNum is the line number inside that File of the call site
// Module is a best-effort attempt to get the module name for the call site (i.e. github.com/getoutreach/gobox)
// ModuleVersion will be the version of the Module used at the call site
type CallerInfo struct {
	Function      string
	File          string
	LineNum       uint
	Module        string
	ModuleVersion string
}

// moduleLookupByPC is a cache of CallerInfo objects keyed by the PC of the function that called us
var moduleLookupByPC = make(map[uintptr]CallerInfo)

// moduleLookupLock is a lock to protect the moduleLookupByPC map from concurrent access issues
var moduleLookupLock = sync.RWMutex{}

var buildInfo *debug.BuildInfo

//nolint:gochecknoinits // Why: Because we need it and the side effects are self-contained and non-mutating
func init() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		buildInfo = bi
	}
}

// GetCallerInfo returns various types of info about the caller of the function.
// skipFrames determines how many frames above the caller to skip.  If you want to know who is calling your
// function, you would pass in skipFrames of 1.  skipFrames of 0 will return yourself.
func GetCallerInfo(skipFrames uint16) (CallerInfo, error) {
	// We only care about the one stack frame above the caller, so skip at least two:
	// 1. runtime.Callers
	// 2. GetCallerModule
	pc := make([]uintptr, 1)
	skipTotal := 2 + int(skipFrames)
	num := runtime.Callers(skipTotal, pc)
	if num == 0 {
		return CallerInfo{}, fmt.Errorf("no frames returned from skip %d", skipTotal)
	}

	moduleLookupLock.RLock()
	mod, valid := moduleLookupByPC[pc[0]]
	moduleLookupLock.RUnlock()
	if valid {
		// Found it in the cache
		return mod, nil
	}

	// Not cached -- have to do the slow lookup
	frames := runtime.CallersFrames(pc)
	frame, _ := frames.Next()

	ci := CallerInfo{
		Function: frame.Function,
		File:     frame.File,
		LineNum:  uint(frame.Line),
	}

	// Attempt to back-calc module name by searching through deps
	if buildInfo != nil {
		for _, mod := range buildInfo.Deps {
			if strings.HasPrefix(ci.Function, mod.Path) {
				ci.Module = mod.Path
				ci.ModuleVersion = mod.Version
				break
			}
		}

		// If we can't find it in the dep list, it must be from the main app (at least as far as all my experimenting
		// has so far shown -- function name looks like "main.main" for example, it doesn't have a module path prefix).
		if ci.Module == "" {
			ci.Module = buildInfo.Main.Path
			ci.ModuleVersion = buildInfo.Main.Version
		}

		// If we still can't find it, attempt to calculate it with a heuristic
		if ci.Module == "" {
			ci.Module = calculateModule(ci.Function)
		}
	}

	// Cache for later, under a brief write lock -- don't use the defer style here in case we add more logic after this someday
	// and hold the lock for too long.
	moduleLookupLock.Lock()
	moduleLookupByPC[pc[0]] = ci
	moduleLookupLock.Unlock()

	return ci, nil
}

// In unit tests (https://github.com/golang/go/issues/33976) or in apps without module info compiled in,
// module info will be blank.  Fall back to parsing out what we can from the function name with some
// heuristics to do the best we can.
func calculateModule(funcName string) string {
	splits := strings.Split(funcName, "/")
	// Pull off the function name/module
	splits = splits[0 : len(splits)-1]
	// Heuristic to pull off pkg/internal dirs to try to get to the root module where we can
	for i := len(splits) - 1; i > 1; i-- {
		switch splits[i] {
		case "pkg", "internal":
			splits = splits[0:i]
		}
	}
	// Put it back together and close your eyes
	return strings.Join(splits, "/")
}
