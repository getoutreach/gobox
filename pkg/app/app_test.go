package app_test

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/app"
)

func TestAppInfo(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("appname")

	appInfo := app.Info()
	assert.Equal(t, appInfo.Name, "appname")
	assert.Equal(t, appInfo.ServiceID, "appname@outreach.cloud")
	assert.Equal(t, appInfo.Domain, "outreach.io")
}

func TestAppInfoRegion(t *testing.T) {
	defer func() {
		os.Unsetenv("MY_CLUSTER")
		os.Unsetenv("MY_REGION")
		app.SetName(app.Info().Name)
	}()
	os.Setenv("MY_CLUSTER", "test.r1")
	app.SetName("appname")
	assert.Equal(t, app.Info().Region, "r1")

	os.Setenv("MY_REGION", "r2")
	app.SetName("appname")
	assert.Equal(t, app.Info().Region, "r2")
}
