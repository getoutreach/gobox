package updater

import (
	"context"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/getoutreach/gobox/pkg/cli/updater/resolver"
	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gotest.tools/v3/assert"
)

func Test_updater_defaultOptions(t *testing.T) {
	defaultCheckInterval := 30 * time.Minute
	type fields struct {
		disabled       bool
		channel        string
		forceCheck     bool
		repoURL        string
		version        string
		executablePath string
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
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-rc.14-23-gfe7ad99",
				disabled:       false,
				executablePath: "gobox",
			},
			want: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-rc.14-23-gfe7ad99",
				disabled:       true,
				executablePath: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "rc",
			},
		},
		{
			name: "should get channel from version string",
			fields: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-unstable+adadafafafafafaf",
				disabled:       false,
				executablePath: "gobox",
			},
			want: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-unstable+adadafafafafafaf",
				disabled:       false,
				executablePath: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "unstable",
			},
		},
		{
			name: "should default to stable",
			fields: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0",
				disabled:       false,
				executablePath: "gobox",
			},
			want: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0",
				disabled:       false,
				executablePath: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "stable",
			},
		},
		{
			name: "shouldn't disable rc release",
			fields: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-rc.15",
				executablePath: "gobox",
			},
			want: fields{
				repoURL:        "https://github.com/getoutreach/gobox",
				version:        "v10.3.0-rc.15",
				disabled:       false,
				executablePath: "gobox",
				skipInstall:    false,
				checkInterval:  &defaultCheckInterval,
				channel:        "rc",
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
				executablePath: tt.fields.executablePath,
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
				disabled:       u.disabled,
				channel:        u.channel,
				forceCheck:     u.forceCheck,
				repoURL:        u.repoURL,
				version:        u.version,
				executablePath: u.executablePath,
				skipInstall:    u.skipInstall,
				checkInterval:  u.checkInterval,
				app:            u.app,
			}

			if diff := cmp.Diff(tt.want, modifiedFields, cmp.AllowUnexported(fields{})); diff != "" {
				t.Errorf("updater.defaultOptions() %s", diff)
			}
		})
	}
}

func Test_updater_installVersion(t *testing.T) {
	type fields struct {
		repoURL        string
		executablePath string
	}
	type args struct {
		version *resolver.Version
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
				executablePath: "stencil",
			},
			args: args{
				version: &resolver.Version{
					Tag: "v1.25.1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &updater{
				repoURL:        tt.fields.repoURL,
				version:        "v0.0.0",
				executablePath: tt.fields.executablePath,
				skipInstall:    true,
				noProgressBar:  true,
			}
			if err := u.defaultOptions(); err != nil {
				t.Errorf("updater.defaultOptions() error = %v", err)
				return
			}

			if err := u.installVersion(context.Background(), tt.args.version); (err != nil) != tt.wantErr {
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

func Test_updater_getVersionInfo(t *testing.T) {
	type fields struct{}
	type args struct {
		v *semver.Version
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantChannel      string
		wantLocallyBuilt bool
	}{
		{
			name: "should return basic channel from string",
			args: args{
				v: semver.MustParse("v1.2.3-rc.1"),
			},
			wantChannel: "rc",
		},
		{
			name: "should return locally built when locally built",
			args: args{
				v: semver.MustParse("v1.2.3-rc.1-2-g1234"),
			},
			wantChannel:      "rc",
			wantLocallyBuilt: true,
		},
		{
			name: "should return locally built when locally built from no channel",
			args: args{
				v: semver.MustParse("v1.2.3-2-g1234"),
			},
			wantChannel:      resolver.StableChannel,
			wantLocallyBuilt: true,
		},
		{
			name: "should return channel and no local build with build metadata",
			args: args{
				v: semver.MustParse("v1.2.3-aNotherChannel.1+build.1"),
			},
			wantChannel: "aNotherChannel",
		},
		{
			name: "should not fail on basic version",
			args: args{
				v: semver.MustParse("v1.2.3"),
			},
			wantChannel: resolver.StableChannel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &updater{}
			gotChannel, gotLocallyBuilt := u.getVersionInfo(tt.args.v)
			if gotChannel != tt.wantChannel {
				t.Errorf("updater.getVersionInfo() gotChannel = %v, want %v", gotChannel, tt.wantChannel)
			}
			if gotLocallyBuilt != tt.wantLocallyBuilt {
				t.Errorf("updater.getVersionInfo() gotLocallyBuilt = %v, want %v", gotLocallyBuilt, tt.wantLocallyBuilt)
			}
		})
	}
}
