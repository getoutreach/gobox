// Package app has the static app info
package app

import (
	"os"
	"runtime/debug"
	"strings"
)

// Version needs to be set at build time using -ldflags "-X github.com/getoutreach/gobox/pkg/app.Version=something"
// nolint:gochecknoglobals
var Version = "Please see http://github.com/getoutreach/go-outreach/blob/master/docs/version.md"

// nolint:gochecknoglobals
var appName = "unknown"

// Info returns the static app info
//
// This struct is used mainly to provide tags to append to logs.  It's also used
// by a handful of infrastructure-y packages like Mint or orgservice that have
// special needs.  Most services will never need to access it directly.
func Info() *Data {
	mainModule := ""
	namespace := ""
	serviceAccount := ""
	bento := ""

	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		mainModule = buildInfo.Main.Path
	}

	// The namespace and service account env vars are set by bootstrap
	// generated deployment scripts.
	if ns := os.Getenv("MY_NAMESPACE"); ns != "" {
		namespace = ns
	}

	if sa := os.Getenv("MY_POD_SERVICE_ACCOUNT"); sa != "" {
		serviceAccount = sa
	}

	parts := strings.Split(namespace, "--")
	if len(parts) == 2 {
		bento = parts[1]
	}

	return &Data{
		Name:    appName,
		Version: Version,

		MainModule: mainModule,

		ServiceAccount: serviceAccount,
		Namespace:      namespace,

		Bento: bento,
	}
}

// SetName sets the app name
//
// Should only be called from tests and app initialization
func SetName(name string) {
	appName = name
}

// Data provides the global app info
type Data struct {
	Name    string
	Version string

	MainModule string

	Namespace      string
	ServiceAccount string

	Bento string
}

// MarshalLog marshals the struct for logging
func (d *Data) MarshalLog(addField func(key string, v interface{})) {
	if d.Name != "unknown" {
		addField("app.name", d.Name)
	}
	if d.Version != "" {
		addField("app.version", d.Version)
	}
	if d.Namespace != "" {
		addField("deployment.namespace", d.Namespace)
	}
}
