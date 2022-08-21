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

	"github.com/blang/semver/v4"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/github"
)

// newTestingVersion is a helper function to create a testing version
func newTestingVersion(tag string, mutable bool) Version {
	v := Version{
		Tag:     tag,
		Commit:  "abcdef",
		Channel: StableChannel,
	}

	// IDEA(jaredallard): Move the version parsing logic out of
	// GetVersions so we can use this here.
	if mutable {
		v.mutable = true
		v.Channel = tag
	} else {
		var err error
		v.sv, err = semver.ParseTolerant(tag)
		if err != nil {
			panic(err)
		}

		if len(v.sv.Pre) > 0 {
			v.Channel = v.sv.Pre[0].String()
		}
	}

	return v
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		c        Criteria
		want     Version
		versions map[string][]Version
		wantErr  bool
	}{
		{
			name: "should return the stable version when no channel is specified",
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0", false),
				},
				"unstable": {
					newTestingVersion("unstable", true),
				},
			},
			want:    newTestingVersion("v1.0.0", false),
			wantErr: false,
		},
		{
			name: "should support a channel",
			c:    Criteria{Channel: "unstable"},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0", false),
				},
				"unstable": {
					newTestingVersion("unstable", true),
				},
			},
			want:    newTestingVersion("unstable", true),
			wantErr: false,
		},
		{
			name: "should promote a channel to stable channel when stable is higher",
			c:    Criteria{Channel: "rc"},
			versions: map[string][]Version{
				StableChannel: {
					newTestingVersion("v1.0.0", false),
				},
				"rc": {
					newTestingVersion("v0.9.0", true),
				},
			},
			want:    newTestingVersion("v0.9.0", true),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock the version resolver
			oldGetVersions := GetVersions
			GetVersions = func(ctx context.Context, token cfg.SecretData, url string) (map[string][]Version, error) {
				return tt.versions, nil
			}
			defer func() { GetVersions = oldGetVersions }()

			gotPtr, err := Resolve(context.Background(), "", &tt.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
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

	githubVersionTest := newTestingVersion("v1.0.0", false)
	githubVersionTest.Commit = "398187b6edf742d4868b455754552b8b56f6abb0"
	unstableVersionText := newTestingVersion("unstable", true)
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

			got, err := getVersions(context.Background(), token, tt.args.url)
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
						gotV.mutable != expectedV.mutable ||
						!reflect.DeepEqual(gotV.sv, expectedV.sv) {
						t.Errorf("getVersions() = %v, want %v", gotV, expectedV)
						return
					}
				}
			}
		})
	}
}
