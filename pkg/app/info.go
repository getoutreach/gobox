// Package app has the static app info
package app

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
)

// Version needs to be set at build time using -ldflags "-X github.com/getoutreach/gobox/pkg/app.Version=something"
// nolint:gochecknoglobals
var Version = "Please see http://github.com/getoutreach/gobox/blob/master/docs/version.md"

// nolint:gochecknoglobals
var appName = "unknown"

var appInfo struct {
	*Data
	mu sync.Mutex // guarding Data to be set initialized concurrently
}

// Info returns the static app info
//
// This struct is used mainly to provide tags to append to logs.  It's also used
// by a handful of infrastructure-y packages like Mint or orgservice that have
// special needs.  Most services will never need to access it directly.
func Info() *Data {
	if appInfo.Data == nil {
		initInfoLocked()
	}
	return appInfo.Data
}

func initInfoLocked() {
	appInfo.mu.Lock()
	defer appInfo.mu.Unlock()

	appInfo.Data = info()
}

//nolint:funlen
func info() *Data {
	const unknown = "unknown"
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

	// There is no guarantee that this correlation between `app.Name` and
	// ServiceID will exist forever.  For example, in the future we could
	// have several apps sharing the same ServiceID.  But that's not
	// supportd by bootstrap yet and so this hard-coded assumption works
	// well enough for now.
	serviceID := fmt.Sprintf("%s@outreach.cloud", appName)

	if ab := os.Getenv("AZURE_BENTO"); ab != "" {
		bento = ab
	} else {
		parts := strings.Split(namespace, "--")
		if len(parts) == 2 {
			bento = parts[1]
		}
	}

	environment := unknown
	if env := os.Getenv("MY_ENVIRONMENT"); env != "" {
		environment = env
	}

	clusterName := unknown
	if cn := os.Getenv("MY_CLUSTER"); cn != "" {
		clusterName = cn
	}

	region := unknown
	// e.g. production.us-west-2
	if regionParts := strings.Split(clusterName, "."); len(regionParts) == 2 {
		region = regionParts[1]
	}

	podID := unknown
	if pi := os.Getenv("MY_POD_NAME"); pi != "" {
		podID = pi
	}

	nodeID := unknown
	if ni := os.Getenv("MY_NODE_NAME"); ni != "" {
		nodeID = ni
	}

	deployment := unknown
	if d := os.Getenv("MY_DEPLOYMENT"); d != "" {
		deployment = d
	}

	return &Data{
		Name:    appName,
		Version: Version,

		MainModule: mainModule,

		Environment:    environment,
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
		ClusterName:    clusterName,
		Region:         region,
		PodID:          podID,
		NodeID:         nodeID,
		Deployment:     deployment,

		ServiceID: serviceID,

		Bento: bento,
	}
}

// SetName sets the app name
//
// Should only be called from tests and app initialization
func SetName(name string) {
	appName = name
	initInfoLocked()
}

// Data provides the global app info
type Data struct {
	Name    string
	Version string

	MainModule string

	Environment    string
	Namespace      string
	ServiceAccount string
	ClusterName    string
	Region         string
	PodID          string
	NodeID         string
	Deployment     string

	// ServiceID is a unique identifier for the service used primarily
	// within the `authn` framework.
	ServiceID string

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
