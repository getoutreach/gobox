// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment.

// Package release contains methods that interact with
// releases from VCS providers that do not exist natively in
// git. For example, Github Releases.
package release_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/cli/github"
	"github.com/getoutreach/gobox/pkg/cli/updater/release"
)

func TestFetch(t *testing.T) {
	token, err := github.GetToken()
	assert.NilError(t, err)

	type args struct {
		opts *release.FetchOptions
	}
	tests := []struct {
		name string
		args args
		// want is a hash of the expected output
		want     string
		wantName string
		wantErr  bool
	}{
		{
			name: "should fetch a GitHub release",
			args: args{
				opts: &release.FetchOptions{
					RepoURL:   "https://github.com/getoutreach/stencil",
					Tag:       "v1.25.1",
					AssetName: "stencil_1.25.1_linux_arm64.tar.gz",
				},
			},
			want:     "9a6847b048e8b2bcf5e720e25776da4b2766356b2d7cce1429903a3ed5170a07",
			wantName: "stencil_1.25.1_linux_arm64.tar.gz",
			wantErr:  false,
		},
		{
			name: "should fetch the correct asset when given a list",
			args: args{
				opts: &release.FetchOptions{
					RepoURL:    "https://github.com/getoutreach/stencil",
					Tag:        "v1.25.1",
					AssetNames: []string{"stencil_1.25.1_linux_arm64.tar.gz"},
				},
			},
			want:     "9a6847b048e8b2bcf5e720e25776da4b2766356b2d7cce1429903a3ed5170a07",
			wantName: "stencil_1.25.1_linux_arm64.tar.gz",
			wantErr:  false,
		},
		{
			name: "should fail when given an invalid tag",
			args: args{
				opts: &release.FetchOptions{
					RepoURL: "https://github.com/getoutreach/stencil",
					Tag:     "i-am-not-a-real-tag",
				},
			},
			wantErr: true,
		},
		{
			name: "should fail when given an invalid repo URL",
			args: args{
				opts: &release.FetchOptions{
					RepoURL: "not-a-real-repo-url",
					Tag:     "a-tag",
				},
			},
			wantErr: true,
		},
		{
			name: "should fail when no asset given",
			args: args{
				opts: &release.FetchOptions{
					RepoURL: "https://github.com/getoutreach/stencil",
					Tag:     "v1.25.1",
				},
			},
			wantErr: true,
		},
		{
			name: "should fail when no repo URL given",
			args: args{
				opts: &release.FetchOptions{},
			},
			wantErr: true,
		},
		{
			name: "should fail when no tag given",
			args: args{
				opts: &release.FetchOptions{
					RepoURL: "a-repo",
				},
			},
			wantErr: true,
		},
		{
			name:    "should fail when no opts given",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, name, _, err := release.Fetch(context.Background(), token, tt.args.opts)
			if tt.wantErr {
				assert.Assert(t, err != nil, "Fetch() expected err to not be nil, got nil")
			} else {
				assert.NilError(t, err)
				defer got.Close()

				b, err := io.ReadAll(got)
				assert.NilError(t, err)

				hashByt := sha256.Sum256(b)
				hash := hex.EncodeToString(hashByt[:])
				assert.Equal(t, hash, tt.want)
				assert.Equal(t, name, tt.wantName)
			}
		})
	}
}

func TestGetReleaseNotes(t *testing.T) {
	token, err := github.GetToken()
	assert.NilError(t, err)

	type args struct {
		opts *release.GetReleaseNoteOptions
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "should get release notes for a GitHub release",
			args: args{
				opts: &release.GetReleaseNoteOptions{
					RepoURL: "https://github.com/getoutreach/stencil",
					Tag:     "v1.25.1",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := release.GetReleaseNotes(context.Background(), token, tt.args.opts)
			if tt.wantErr {
				assert.Assert(t, err != nil, "GetReleaseNotes() expected err to not be nil, got nil")
				assert.Equal(t, got, "", "GetReleaseNotes() should return empty string")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
