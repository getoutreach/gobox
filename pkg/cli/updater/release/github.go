// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the github release fetcher.

package release

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/getoutreach/gobox/pkg/cfg"
	gogithub "github.com/google/go-github/v57/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// _ ensures that the fetcher interface is implemented.
var _ fetcher = &github{}

// github is a fetcher for github releases
type github struct{}

// getOrgRepoFromURL returns the org and repo from a URL:
// expected format: https://github.com/getoutreach/stencil
func getOrgRepoFromURL(URL string) (string, string, error) { //nolint:gocritic // Why: This is in the function comment
	u, err := url.Parse(URL)
	if err != nil {
		return "", "", err
	}

	// /getoutreach/stencil -> ["", "getoutreach", "stencil"]
	spl := strings.Split(u.Path, "/")
	if len(spl) != 3 {
		return "", "", fmt.Errorf("invalid Github URL: %s", URL)
	}
	return spl[1], spl[2], nil
}

// createClient creates a Github client
func (g *github) createClient(ctx context.Context, token cfg.SecretData) *gogithub.Client {
	httpClient := http.DefaultClient
	if token != "" {
		httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: string(token)}))
	}
	return gogithub.NewClient(httpClient)
}

// GetReleaseNotes returns the release notes for a given tag
func (g *github) GetReleaseNotes(ctx context.Context, token cfg.SecretData, opts *GetReleaseNoteOptions) (string, error) {
	gh := g.createClient(ctx, token)

	org, repo, err := getOrgRepoFromURL(opts.RepoURL)
	if err != nil {
		return "", err
	}

	rel, _, err := gh.Repositories.GetReleaseByTag(ctx, org, repo, opts.Tag)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get release for %s/%s:%s", org, repo, opts.Tag)
	}

	return rel.GetBody(), nil
}

// Fetch fetches a release from a github repository and the underlying release
//
//nolint:gocritic // Why: rc, name, size, error
func (g *github) Fetch(ctx context.Context, token cfg.SecretData, opts *FetchOptions) (io.ReadCloser, string, int64, error) {
	gh := g.createClient(ctx, token)

	org, repo, err := getOrgRepoFromURL(opts.RepoURL)
	if err != nil {
		return nil, "", 0, err
	}

	rel, _, err := gh.Repositories.GetReleaseByTag(ctx, org, repo, opts.Tag)
	if err != nil {
		return nil, "", 0, errors.Wrapf(err, "failed to get release for %s/%s:%s", org, repo, opts.Tag)
	}

	// copy the assetNames slice, and append the assetName if it is not empty
	validAssets := append([]string{}, opts.AssetNames...)
	if opts.AssetName != "" {
		validAssets = append(validAssets, opts.AssetName)
	}

	// Find an asset that matches the provided asset names
	var a *gogithub.ReleaseAsset
	for _, asset := range rel.Assets {
		for _, assetName := range validAssets {
			matched := false

			// attempt to use glob first, if that errors then fall back to straight strings comparison
			if filePathMatched, err := filepath.Match(assetName, asset.GetName()); err == nil {
				matched = filePathMatched
			} else if err != nil && assetName == asset.GetName() {
				matched = true
			}

			if matched {
				a = asset
				break
			}
		}
	}
	if a == nil {
		return nil, "", 0, fmt.Errorf("failed to find asset %v in release %s/%s:%s", validAssets, org, repo, opts.Tag)
	}

	// The second return value is a redirectURL, but by passing http.DefaultClient we shouldn't have
	// to handle it. That being said, this didn't use to exist so we may need to handle the redirect
	// ourselves.
	rc, _, err := gh.Repositories.DownloadReleaseAsset(ctx, org, repo, a.GetID(), http.DefaultClient)
	if err != nil {
		return nil, "", 0, errors.Wrapf(err, "failed to download asset %s from release %s/%s:%s", a.GetName(), org, repo, opts.Tag)
	}
	return rc, a.GetName(), int64(a.GetSize()), nil
}
