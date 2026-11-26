// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment.

// Package release contains methods that interact with
// releases from VCS providers that do not exist natively in
// git. For example, Github Releases.
package release

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/getoutreach/gobox/pkg/cfg"
)

// fetcher implements the Fetch method for a VCS provider.
type fetcher interface {
	// Fetch returns an asset as a io.ReadCloser
	Fetch(ctx context.Context, token cfg.SecretData, opts *FetchOptions) (io.ReadCloser, string, int64, error)

	// GetReleaseNotes returns the release notes of a release
	GetReleaseNotes(ctx context.Context, token cfg.SecretData, opts *GetReleaseNoteOptions) (string, error)
}

// FetchOptions is a set of options for Fetch.
type FetchOptions struct {
	// RepoURL is the repository URL, it should be a valid
	// URL.
	RepoURL string

	// Tag is the tag of the release
	Tag string

	// AssetName is the name of the asset to fetch, globs are
	// supported.
	AssetName string

	// AssetNames is a list of asset names to fetch, the first
	// asset that matches will be returned. Globs are supported.
	AssetNames []string
}

// GetReleaseNoteOptions is a set of options for GetReleaseNotes.
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
		return nil, "", 0, errOptsNil
	}

	if opts.RepoURL == "" {
		return nil, "", 0, errRepoURLRequired
	}

	if opts.Tag == "" {
		return nil, "", 0, errTagRequired
	}

	if strings.Contains(opts.RepoURL, "github.com") {
		return (&github{}).Fetch(ctx, token, opts)
	}

	return nil, "", 0, fmt.Errorf("%w: %s", errUnsupportedFetchURL, opts.RepoURL)
}

// GetReleaseNotes fetches the release notes of a release from a VCS provider.
func GetReleaseNotes(ctx context.Context, token cfg.SecretData, opts *GetReleaseNoteOptions) (string, error) {
	if opts == nil {
		return "", errOptsNil
	}

	if opts.RepoURL == "" {
		return "", errRepoURLRequired
	}

	if opts.Tag == "" {
		return "", errTagRequired
	}

	if strings.Contains(opts.RepoURL, "github.com") {
		return (&github{}).GetReleaseNotes(ctx, token, opts)
	}

	return "", fmt.Errorf("%w: %s", errUnsupportedNotesURL, opts.RepoURL)
}

var (
	errOptsNil             = errors.New("opts is nil")
	errRepoURLRequired     = errors.New("repo url is required")
	errTagRequired         = errors.New("tag is required")
	errUnsupportedFetchURL = errors.New("unsupported fetch repo url")
	errUnsupportedNotesURL = errors.New("unsupported get release notes repo url")
)
