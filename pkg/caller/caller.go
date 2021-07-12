// package caller provides info on the caller
package caller

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// nolint:gochecknoglobals
var trimPathsRe = regexp.MustCompile(`(github\.com(/getoutreach?)|golang\.org|go/src)/?`)

// FileLine returns the caller file:line, skipping the specified number of frames.
func FileLine(skip int) string {
	pc := []uintptr{0}
	if runtime.Callers(skip, pc) == 1 {
		// due to being acquired by runtime.Callers, pc is offset by 1
		file, line, _ := FileLineNameForPC(pc[0] - 1)
		return fmt.Sprintf("%s:%d", file, line)
	}
	return "unknown:0"
}

// FileLineNameforPC returns the file, line and name for PC.
func FileLineNameForPC(pc uintptr) (file string, line int, name string) {
	fn := runtime.FuncForPC(pc)
	file, line = fn.FileLine(pc)
	name = fn.Name()

	return trimPaths(pkgPath(file, name)), line, pkgName(name)
}

// pkgName returns the package relative name of a function or method.
func pkgName(funcname string) string {
	if i := strings.LastIndex(funcname, "/"); i != -1 {
		funcname = funcname[i+1:]
	}
	return funcname
}

// pkgPath returns the filepath relative to the compile-time GOPATH, or
// go-module, ignoring any renamed imports.
func pkgPath(filepath, funcname string) string {
	// extract the package path out of the function name, trimming off the
	// import-specific final segment.
	if i := strings.LastIndex(funcname, "/"); i != -1 {
		funcname = funcname[:i]
	}

	// extract the package and relative file name from the filepath.
	if i := nthLastIndex(filepath, '/', 1); i != -1 {
		filepath = filepath[i:]
	}

	return funcname + filepath
}

// nthLastIndex is strings.LastIndex, but supports skipping any number of
// occurrences of sep before returning the result.
func nthLastIndex(s string, sep rune, skip int) int {
	return strings.LastIndexFunc(s, func(r rune) bool {
		if r == sep {
			if skip == 0 {
				return true
			}
			skip--
		}
		return false
	})
}

// trimPaths removes known import paths from a string
func trimPaths(s string) string {
	return trimPathsRe.ReplaceAllString(s, "")
}
