// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment.

// Package release contains methods that interact with
// releases from VCS providers that do not exist natively in
// git. For example, Github Releases.
package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestFetch(t *testing.T) {
	type args struct {
		opts *FetchOptions
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
			name: "should fetch a github release",
			args: args{
				opts: &FetchOptions{
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
				opts: &FetchOptions{
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
				opts: &FetchOptions{
					RepoURL: "https://github.com/getoutreach/stencil",
					Tag:     "i-am-not-a-real-tag",
				},
			},
			wantErr: true,
		},
		{
			name: "should fail when given an invalid repo URL",
			args: args{
				opts: &FetchOptions{
					RepoURL: "not-a-real-repo-url",
					Tag:     "a-tag",
				},
			},
			wantErr: true,
		},
		{
			name: "should fail when no asset given",
			args: args{
				opts: &FetchOptions{
					RepoURL: "https://github.com/getoutreach/stencil",
					Tag:     "v1.25.1",
				},
			},
			wantErr: true,
		},
		{
			name: "should fail when no repo URL given",
			args: args{
				opts: &FetchOptions{},
			},
			wantErr: true,
		},
		{
			name: "should fail when no tag given",
			args: args{
				opts: &FetchOptions{
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
			got, name, _, err := Fetch(context.Background(), "", tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			defer got.Close()

			b, err := io.ReadAll(got)
			if err != nil {
				t.Errorf("Fetch() error = %v", err)
				return
			}

			hashByt := sha256.Sum256(b)
			hash := hex.EncodeToString(hashByt[:])
			if hash != tt.want {
				t.Errorf("Fetch() hash = %v, want %v", hash, tt.want)
			}
			if name != tt.wantName {
				t.Errorf("Fetch() name = %v, wantName %v", name, tt.wantName)
			}
		})
	}
}

func TestGetReleaseNotes(t *testing.T) {
	type args struct {
		opts *GetReleaseNoteOptions
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "should get release notes for a github release",
			args: args{
				opts: &GetReleaseNoteOptions{
					RepoURL: "https://github.com/getoutreach/stencil",
					Tag:     "v1.25.1",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetReleaseNotes(context.Background(), "", tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleaseNotes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got == "" {
				t.Errorf("GetReleaseNotes() return empty string")
			}
		})
	}
}
