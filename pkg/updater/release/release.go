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
	Fetch(ctx context.Context, token cfg.SecretData, opts *FetchOptions) (io.ReadCloser, error)
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

// Fetch fetches a release from a VCS provider and returns an asset
// from it as an io.ReadCloser. This must be closed to close the
// underlying HTTP request.
func Fetch(ctx context.Context, token cfg.SecretData, opts *FetchOptions) (io.ReadCloser, error) {
	if opts == nil {
		return nil, fmt.Errorf("opts is nil")
	}

	if opts.RepoURL == "" {
		return nil, fmt.Errorf("repo url is required")
	}

	if opts.Tag == "" {
		return nil, fmt.Errorf("tag is required")
	}

	if strings.Contains(opts.RepoURL, "github.com") {
		return (&github{}).Fetch(ctx, token, opts)
	}

	return nil, fmt.Errorf("unsupported fetch repo url: %s", opts.RepoURL)
}
