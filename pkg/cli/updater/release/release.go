// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment.

// Package release contains methods that interact with
// releases from VCS providers that do not exist natively in
// git. For example, Github Releases.
package release

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/getoutreach/gobox/pkg/cfg"
)

// fetcher implements the Fetch method for a VCS provider
type fetcher interface {
	// Fetch returns an asset as a io.ReadCloser
	Fetch(ctx context.Context, token cfg.SecretData, opts *FetchOptions) (io.ReadCloser, string, int64, error)

	// GetReleaseNotes returns the release notes of a release
	GetReleaseNotes(ctx context.Context, token cfg.SecretData, opts *GetReleaseNoteOptions) (string, error)
}

// FetchOptions is a set of options for Fetch
type FetchOptions struct {
	// RepoURL is the repository URL, it should be a valid
	// URL.
	RepoURL string

	// Tag is the tag of the release
	Tag string

	// AssetName is the name of the asset to fetch
	AssetName string

	// AssetNames is a list of asset names to fetch, the first
	// asset that matches will be returned.
	AssetNames []string
}

// GetReleaseNoteOptions is a set of options for GetReleaseNotes
type GetReleaseNoteOptions struct {
	// RepoURL is the repository URL, it should be a valid
	// URL.
	RepoURL string

	// Tag is the tag of the release
	Tag string
}

// Fetch fetches a release from a VCS provider and returns an asset
// from it as an io.ReadCloser. This must be closed to close the
// underlying HTTP request.
//
//nolint:gocritic // Why: rc, name, size, error
func Fetch(ctx context.Context, token cfg.SecretData, opts *FetchOptions) (io.ReadCloser, string, int64, error) {
	if opts == nil {
		return nil, "", 0, fmt.Errorf("opts is nil")
	}

	if opts.RepoURL == "" {
		return nil, "", 0, fmt.Errorf("repo url is required")
	}

	if opts.Tag == "" {
		return nil, "", 0, fmt.Errorf("tag is required")
	}

	if strings.Contains(opts.RepoURL, "github.com") {
		return (&github{}).Fetch(ctx, token, opts)
	}

	return nil, "", 0, fmt.Errorf("unsupported fetch repo url: %s", opts.RepoURL)
}

// GetReleaseNotes fetches the release notes of a release from a VCS provider.
func GetReleaseNotes(ctx context.Context, token cfg.SecretData, opts *GetReleaseNoteOptions) (string, error) {
	if opts == nil {
		return "", fmt.Errorf("opts is nil")
	}

	if opts.RepoURL == "" {
		return "", fmt.Errorf("repo url is required")
	}

	if opts.Tag == "" {
		return "", fmt.Errorf("tag is required")
	}

	if strings.Contains(opts.RepoURL, "github.com") {
		return (&github{}).GetReleaseNotes(ctx, token, opts)
	}

	return "", fmt.Errorf("unsupported get release notes repo url: %s", opts.RepoURL)
}
