// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements the version resolver used
// by the updater for determining what the latest version is
// within a set of criteria.

// Package resolver contains a git tag aware version resolver that
// supports channels to determine the latest version.
package resolver

import (
	"context"
	"fmt"
	"regexp"

	"github.com/blang/semver/v4"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

// mutableTagRegex is the regex to use to consider a tag
// that is _not_ semver as a "mutable" tag, which is a tag
// that points to the current latest version of a channel.
// https://regex101.com/r/yWXH5i/1
var mutableTagRegex = regexp.MustCompile(`^[a-z-]+$`)

// StableChannel is the default channel used when a version
// doesn't contain a channel.
const StableChannel = "stable"

// Criteria is the criteria used to determine the latest version
type Criteria struct {
	// URL is the URL to the repository to check for updates.
	URL string

	// Channel is the channel to use for determining the latest version.
	// Leave empty to use the StableChannel.
	Channel string
}

// Version is a resolved version from a git repository
type Version struct {
	// mutable denotes if this version came from a mutable
	// tag
	mutable bool

	// sv is the semver version of the tag if it is a semver tag
	// note: if mutable == true, not semver
	sv semver.Version

	// Tag is the git tag that represents the version.
	Tag string `yaml:"version"`

	// Commit is the git commit that this tag refers to.
	Commit string `yaml:"commit"`

	// Channel is the channel that this version is associated with.
	Channel string `yaml:"channel"`
}

// String returns the user-friendly representation of a version,
// e.g. when using a version that came from a mutable tag use
// the commit hash+channel as opposed to just the tag.
func (v *Version) String() string {
	if v.mutable {
		return fmt.Sprintf("v0.0.0-%s+%s", v.Channel, v.Commit)
	}

	return v.Tag
}

// GetVersions returns all known channels and the versions that are available
// for each channel.
//
// Note: token is _optional_, if you do not wish to authenticate with your VCS
// provider you can pass an empty string.
var GetVersions = getVersions

// getVersions is documented above
func getVersions(ctx context.Context, token cfg.SecretData, url string) (map[string][]Version, error) {
	r := git.NewRemote(memory.NewStorage(), &gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	opts := &git.ListOptions{}

	// If we have a token, use it to authenticate
	if string(token) != "" {
		opts.Auth = &http.BasicAuth{
			Username: "x-oauth-token",
			Password: string(token),
		}
	}

	refs, err := r.ListContext(ctx, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to git ls-remote %q", url)
	}

	// create a map of channels to their respective
	// versions
	channels := map[string][]Version{}

	for _, ref := range refs {
		if !ref.Name().IsTag() {
			continue
		}

		tagName := ref.Name().Short()
		var v Version

		// Attempt to parse the tag as semver for a normal version
		// tag.
		//nolint:gocritic // Why: A switch statement doesn't read well.
		if semV, err := semver.ParseTolerant(tagName); err == nil {
			// Determine the channel from the tag
			// `v1.0.0-alpha.1` -> `alpha`
			// `v1.0.0-unstable+commit` -> `beta`
			channel := StableChannel
			if len(semV.Pre) > 0 {
				channel = semV.Pre[0].String()
			}

			v = Version{
				sv:      semV,
				Tag:     tagName,
				Commit:  ref.Hash().String(),
				Channel: channel,
			}
		} else if mutableTagRegex.MatchString(tagName) {
			// Matches mutable tag format, so handle it as one
			v = Version{
				mutable: true,
				Tag:     tagName,
				Commit:  ref.Hash().String(),
				Channel: tagName,
			}
		} else {
			// didn't match any of the above, so it is an invalid tag
			// skip.
			continue
		}

		// initialize the channels slice if it's nil
		if channels[v.Channel] == nil {
			channels[v.Channel] = []Version{}
		}

		channels[v.Channel] = append(channels[v.Channel], v)
	}

	// sort the versions
	for k := range channels {
		Sort(channels[k])
	}

	return channels, nil
}

// Resolve returns the latest version that satisfies the criteria
//
// Note: token is _optional_, if you do not wish to authenticate with your VCS
// provider you can pass an empty string.
func Resolve(ctx context.Context, token cfg.SecretData, c *Criteria) (*Version, error) {
	versions, err := GetVersions(ctx, token, c.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get versions for %q", c.URL)
	}

	// default to the stable channel if no channel is specified
	if c.Channel == "" {
		c.Channel = StableChannel
	}

	if _, ok := versions[c.Channel]; !ok {
		return nil, errors.Errorf("unknown channel %q", c.Channel)
	}

	v := getLatestVersion(versions[c.Channel])
	if v == nil {
		return nil, fmt.Errorf("no version found matching criteria")
	}

	// check if the stable version is greater than the latest version inside
	// of our requested version, if so, use that
	if c.Channel != StableChannel {
		stableV := getLatestVersion(versions[StableChannel])
		if stableV != nil {
			vers := []Version{*v, *stableV}
			Sort(vers)

			latestV := getLatestVersion(vers)
			if latestV != nil && latestV != v {
				v = latestV
			}
		}
	}

	return v, nil
}

// getLatestVersion returns the latest versions from a slice of versions.
// This does not mutate the provided slice, and it does not need to be sorted.
func getLatestVersion(argVersions []Version) *Version {
	if len(argVersions) == 0 {
		return nil
	}

	return &argVersions[0]
}
