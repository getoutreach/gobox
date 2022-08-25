package updater

import (
	"context"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gotest.tools/v3/assert"
)

func Test_generatePossibleAssetNames(t *testing.T) {
	type args struct {
		name    string
		version string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "should generate all possible asset names",
			args: args{
				name:    "test",
				version: "v1.0.0",
			},
			want: []string{
				// v prefixes
				"test_v1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.xz",
				"test_v1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz",
				"test_v1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.bz2",
				"test_v1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip",
				"test-v1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.xz",
				"test-v1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz",
				"test-v1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.bz2",
				"test-v1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip",

				// without v prefixes
				"test_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.xz",
				"test_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz",
				"test_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.bz2",
				"test_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip",
				"test-1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.xz",
				"test-1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz",
				"test-1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.bz2",
				"test-1.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generatePossibleAssetNames(tt.args.name, tt.args.version); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generatePossibleAssetNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updater_defaultOptions(t *testing.T) {
	defaultCheckInterval := 30 * time.Minute
	type fields struct {
		ghToken        cfg.SecretData
		disabled       bool
		channel        string
		forceCheck     bool
		repoURL        string
		version        string
		executableName string
		skipInstall    bool
		checkInterval  *time.Duration
		app            *cli.App
	}
	tests := []struct {
		name    string
		fields  fields
		want    fields
		wantErr bool
	}{
		{
			name: "should disable updater if version is from a local build",
			fields: fields{
				ghToken:        "1234",
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-rc.14-23-gfe7ad99",
				disabled:       false,
				executableName: "gobox",
			},
			want: fields{
				ghToken:        "1234",
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-rc.14-23-gfe7ad99",
				disabled:       true,
				executableName: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "rc",
			},
		},
		{
			name: "should get channel from version string",
			fields: fields{
				ghToken:        "1234",
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-unstable+adadafafafafafaf",
				disabled:       false,
				executableName: "gobox",
			},
			want: fields{
				ghToken:        "1234",
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-unstable+adadafafafafafaf",
				disabled:       false,
				executableName: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "unstable",
			},
		},
		{
			name: "should default to stable",
			fields: fields{
				ghToken:        "1234",
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0",
				disabled:       false,
				executableName: "gobox",
			},
			want: fields{
				ghToken:        "1234",
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0",
				disabled:       false,
				executableName: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "stable",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &updater{
				ghToken:        "1234",
				log:            logrus.New(),
				noProgressBar:  true,
				disabled:       tt.fields.disabled,
				channel:        tt.fields.channel,
				forceCheck:     tt.fields.forceCheck,
				repoURL:        tt.fields.repoURL,
				version:        tt.fields.version,
				executableName: tt.fields.executableName,
				skipInstall:    tt.fields.skipInstall,
				checkInterval:  tt.fields.checkInterval,
				app:            tt.fields.app,
			}
			if err := u.defaultOptions(); (err != nil) != tt.wantErr {
				t.Errorf("updater.defaultOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			modifiedFields := fields{
				ghToken:        u.ghToken,
				disabled:       u.disabled,
				channel:        u.channel,
				forceCheck:     u.forceCheck,
				repoURL:        u.repoURL,
				version:        u.version,
				executableName: u.executableName,
				skipInstall:    u.skipInstall,
				checkInterval:  u.checkInterval,
				app:            u.app,
			}

			if diff := cmp.Diff(modifiedFields, tt.want, cmp.AllowUnexported(fields{})); diff != "" {
				t.Errorf("updater.defaultOptions() %s", diff)
			}
		})
	}
}

func Test_updater_installVersion(t *testing.T) {
	type fields struct {
		repoURL        string
		executableName string
	}
	type args struct {
		tag string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "should fetch a version",
			fields: fields{
				repoURL:        "https://github.com/getoutreach/stencil",
				executableName: "stencil",
			},
			args: args{
				tag: "v1.25.1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &updater{
				repoURL:        tt.fields.repoURL,
				version:        "v0.0.0",
				executableName: tt.fields.executableName,
				skipInstall:    true,
				noProgressBar:  true,
			}
			if err := u.defaultOptions(); err != nil {
				t.Errorf("updater.defaultOptions() error = %v", err)
				return
			}

			if err := u.installVersion(context.Background(), tt.args.tag); (err != nil) != tt.wantErr {
				t.Errorf("updater.installVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestE2EUpdater(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	u, err := UseUpdater(context.Background(),
		WithRepoURL("https://github.com/getoutreach/stencil"),
		WithVersion("v0.0.0"),
		WithExecutableName("stencil"),
		WithForceCheck(true),
		WithSkipInstall(true),
		WithSkipMajorVersionPrompt(true),
		WithNoProgressBar(true),
	)
	if err != nil {
		t.Errorf("UseUpdater() error = %v", err)
		return
	}

	updated, err := u.check(context.Background())
	if err != nil {
		t.Errorf("updater.check() error = %v", err)
	}

	assert.Equal(t, true, updated, "expected updater to trigger")
}
