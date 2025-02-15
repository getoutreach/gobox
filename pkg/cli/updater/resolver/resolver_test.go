// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements the version resolver used
// by the updater for determining what the latest version is
// within a set of criteria.

// Package resolver contains a git tag aware version resolver that
// supports channels to determine the latest version.
package resolver

import (
	"context"
	"reflect"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/github"
	"github.com/google/go-cmp/cmp"
)

// newTestingVersion is a helper function to create a testing version
func newTestingVersion(tag string) Version {
	return mustNewVersion(NewVersion(tag, false, "abcdefghijklmnopqrstuvwxyz"))
}

// mustNewVersion is a helper function to create a new version for tests
// that panics on errors
func mustNewVersion(v *Version, err error) Version {
	if err != nil {
		panic(err)
	}
	return *v
}

func TestResolve(t *testing.T) {
	type opts struct {
		UseRealResolver bool
	}

	tests := []struct {
		name     string
		o        *opts
		c        Criteria
		want     Version
		versions map[string][]Version
		wantErr  bool
	}{
		{
			name: "should return the stable version when no channel is specified",
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0"),
				},
				"unstable": {
					newTestingVersion("unstable"),
				},
			},
			want: newTestingVersion("v1.0.0"),
		},
		{
			name: "should support a channel",
			c:    Criteria{Channel: "rc"},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0"),
				},
				"rc": {
					newTestingVersion("v1.0.1-rc.1"),
				},
			},
			want: newTestingVersion("v1.0.1-rc.1"),
		},
		{
			name: "should always return a mutable channel as the latest version",
			c:    Criteria{Channel: "unstable"},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0"),
				},
				"unstable": {
					newTestingVersion("unstable"),
				},
			},
			want: newTestingVersion("unstable"),
		},
		{
			name: "should promote a channel to stable channel when stable is higher",
			c:    Criteria{Channel: "rc"},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0"),
				},
				"rc": {
					newTestingVersion("v0.9.0-rc.1"),
				},
			},
			want: newTestingVersion("v1.0.0"),
		},
		{
			name: "should support a constraint",
			c:    Criteria{Constraints: []string{"0.9.0"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0"),
					newTestingVersion("v0.9.0"),
					newTestingVersion("v0.8.0"),
				},
			},
			want: newTestingVersion("v0.9.0"),
		},
		{
			name: "should return a version between constraints",
			c:    Criteria{Constraints: []string{">0.9.0", "<1.0.0"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0"),
					newTestingVersion("v0.9.1"),
					newTestingVersion("v0.9.0"),
				},
			},
			want: newTestingVersion("v0.9.1"),
		},
		{
			name: "should satisfy a version constraint when there are multiple channels",
			c:    Criteria{Channel: "rc", Constraints: []string{">=0.9.0"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.1"),
				},
				"rc": {
					// this older than v0.9.1
					newTestingVersion("v0.9.1-rc.1"),
				},
			},
			want: newTestingVersion("v0.9.1"),
		},
		{
			name: "should satisfy a version constraint when there are multiple channels with a pre-release being higher",
			c:    Criteria{Channel: "rc", Constraints: []string{">=0.9.0"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"rc": {
					newTestingVersion("v0.9.1-rc.1"),
				},
			},
			want: newTestingVersion("v0.9.1-rc.1"),
		},
		{
			name: "should return a version outside of the channel when a constraint is provided asking for it",
			c:    Criteria{Constraints: []string{"~0.9.1-rc"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"rc": {
					newTestingVersion("v0.9.1-rc.1"),
				},
			},
			want: newTestingVersion("v0.9.1-rc.1"),
		},
		{
			name: "should only opt-into a channel when a constraint is provided or in the channel",
			c:    Criteria{Constraints: []string{">=0.9.1-rc.1"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"rc": {
					newTestingVersion("v0.9.1-rc.1"),
				},
				"beta": {
					// should be ignored
					newTestingVersion("v0.9.2-beta.1"),
				},
			},
			want: newTestingVersion("v0.9.1-rc.1"),
		},
		{
			name: "should support channel with constraint wanting another channel",
			c:    Criteria{Channel: "rc", Constraints: []string{">=0.9.1-beta"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"rc": {
					newTestingVersion("v0.9.1-rc.1"),
				},
				"beta": {
					newTestingVersion("v0.9.2-beta.1"),
				},
			},
			// should return beta because module asked for it
			want: newTestingVersion("v0.9.2-beta.1"),
		},
		{
			name: "should support channel being gtr with constraint wanting another channel",
			c:    Criteria{Channel: "beta", Constraints: []string{">=0.9.1-rc"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"rc": {
					newTestingVersion("v0.9.1-rc.1"),
				},
				"beta": {
					newTestingVersion("v0.9.2-beta.1"),
				},
			},
			// should return beta because channel asked for it
			want: newTestingVersion("v0.9.2-beta.1"),
		},
		{
			name: "should support comparing rc versions",
			c:    Criteria{Constraints: []string{">=0.9.1-rc.1"}},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"rc": {
					newTestingVersion("v0.9.1-rc.1"),
					newTestingVersion("v0.9.1-rc.2"),
				},
			},
			want: newTestingVersion("v0.9.1-rc.2"),
		},
		{
			name: "should support branches",
			c:    Criteria{Channel: "main", AllowBranches: true},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v0.9.0"),
				},
				"main": {
					mustNewVersion(NewVersion("main", true, "abcedfg")),
				},
			},
			want: mustNewVersion(NewVersion("main", true, "abcedfg")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var token cfg.SecretData
			if tt.o == nil || !tt.o.UseRealResolver {
				// mock the version resolver
				oldGetVersions := GetVersions
				GetVersions = func(ctx context.Context, token cfg.SecretData, url string, allowBranches bool) (map[string][]Version, error) {
					return tt.versions, nil
				}
				defer func() { GetVersions = oldGetVersions }()
			} else {
				var err error
				token, err = github.GetToken()
				if err != nil {
					t.Fatalf("failed to get github token: %v", err)
				}
			}

			gotPtr, err := Resolve(context.Background(), token, &tt.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr && err != nil {
				return
			}

			got := *gotPtr
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getVersions(t *testing.T) {
	type args struct {
		url string
	}

	githubVersionTest := newTestingVersion("v1.0.0")
	githubVersionTest.Commit = "398187b6edf742d4868b455754552b8b56f6abb0"
	unstableVersionText := newTestingVersion("unstable")
	unstableVersionText.Commit = "skip-validate"

	tests := []struct {
		name    string
		args    args
		want    map[string][]Version
		wantErr bool
	}{
		{
			name: "should return versions from Github",
			args: args{
				// This is the only repo with an `unstable` channel right now.
				url: "https://github.com/getoutreach/bootstrap",
			},
			want: map[string][]Version{
				StableChannel: {githubVersionTest},
				"unstable":    {unstableVersionText},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := github.GetToken()
			if err != nil {
				t.Errorf("failed to get github token, please ensure 'gh auth login' has been ran")
				return
			}

			got, err := getVersions(context.Background(), token, tt.args.url, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("getVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for channel, expectedVers := range tt.want {
				for _, expectedV := range expectedVers {
					if len(got[channel]) < len(expectedVers) {
						t.Errorf(
							"getVersions() channels[%s]len(%d)was less than min expected %d",
							channel, len(got[channel]), len(expectedVers),
						)
						return
					}

					gotV := got[channel][0]
					if gotV.Channel != expectedV.Channel ||
						(gotV.Commit != expectedV.Commit && expectedV.Commit != "skip-validate") ||
						gotV.Tag != expectedV.Tag ||
						gotV.Mutable != expectedV.Mutable ||
						!reflect.DeepEqual(gotV.sv, expectedV.sv) {
						t.Errorf("getVersions() = %v, want %v", gotV, expectedV)
						return
					}
				}
			}
		})
	}
}

func TestNewVersionFromVersionString(t *testing.T) {
	type args struct {
		ver string
	}
	tests := []struct {
		name    string
		args    args
		want    *Version
		wantErr bool
	}{
		{
			name: "should parse a version string",
			args: args{
				ver: "v1.0.0",
			},
			want: &Version{
				Tag:     "v1.0.0",
				Channel: StableChannel,
				sv:      semver.MustParse("1.0.0"),
			},
		},
		{
			name: "should parse a non-mutable channel",
			args: args{
				ver: "v1.0.0-rc.1",
			},
			want: &Version{
				Tag:     "v1.0.0-rc.1",
				Channel: "rc",
				sv:      semver.MustParse("1.0.0-rc.1"),
			},
		},
		{
			name: "should parse a version string with a channel",
			args: args{
				ver: "v0.0.0-unstable",
			},
			want: &Version{
				Mutable: true,
				Tag:     "unstable",
				Channel: "unstable",
				sv:      semver.MustParse("1.0.0"),
			},
		},
		{
			name: "should parse a version string with a channel and get commit info",
			args: args{
				ver: "v0.0.0-unstable+abcdefg",
			},
			want: &Version{
				Mutable: true,
				Tag:     "unstable",
				Channel: "unstable",
				Commit:  "abcdefg",
				sv:      semver.MustParse("1.0.0"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVersionFromVersionString(tt.args.ver)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVersionFromVersionString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(*tt.want, *got, cmp.FilterPath(func(p cmp.Path) bool {
				return p.String() == "sv" // ignore semver
			}, cmp.Ignore())); diff != "" {
				t.Fatal(diff)
			}

			if got.String() != tt.want.String() {
				t.Errorf("NewVersionFromVersionString() = %v, want %v", got, tt.want)
			}
		})
	}
}
