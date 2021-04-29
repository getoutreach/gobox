package app_test

import (
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
}
