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
	"strings"

	"github.com/Masterminds/semver/v3"
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

// oprStripRegex is the regex to use to strip operators from a constraint
var oprStripRegex = regexp.MustCompile(`^\D+(\d)`)

// This block contains error types for the resolver package.
var (
	// ErrNoVersions is returned when no versions are found
	ErrNoVersions = errors.New("no version found matching criteria")
)

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

	// Constraints are the semver constraint(s) to use for determining the latest version.
	// See: https://pkg.go.dev/github.com/Masterminds/semver/v3#hdr-Checking_Version_Constraints_and_Comparing_Versions
	Constraints []string

	// AllowBranches is a flag to allow branches to be considered as versions, branches
	// will be represented as their own channel.
	AllowBranches bool
}

// Version is a resolved version from a git repository
type Version struct {
	// sv is the semver version of the tag if it is a semver tag
	// note: if mutable == true, not semver
	sv *semver.Version

	// Mutable denotes if this version is a reference pointing to a specific
	// sha that can't be compared as a semantic version. When encountering a
	// mutable version, it should always be used over a semantic version as
	// there is no way to compare the two.
	Mutable bool

	// Branch is the branch that the version is on, branches are special
	// versions that are always mutable.
	Branch string

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
	if v.Mutable {
		return fmt.Sprintf("v0.0.0-%s+%s", v.Channel, v.Commit)
	}

	return v.Tag
}

// GitRef returns the git ref for the version if there is one
func (v *Version) GitRef() string {
	if v.Branch != "" {
		return v.Branch
	}

	return v.Tag
}

// NewVersionFromVersionString creates a Version from a version string
// returned from v.String()
//
// Note: This is lossy, for non-mutable versions it will not have a commit hash
// and it cannot determine a branch vs. a mutable version.
func NewVersionFromVersionString(ver string) (*Version, error) {
	sv, err := semver.NewVersion(ver)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse version string as semver")
	}

	v := &Version{
		sv:      sv,
		Channel: StableChannel,
		Tag:     ver,
	}

	splPre := strings.Split(sv.Prerelease(), ".")

	// get the channel from the version string if set
	if len(splPre) > 0 && splPre[0] != "" {
		v.Channel = splPre[0]
	}

	if sv.Prerelease() != "" && mutableTagRegex.MatchString(sv.Prerelease()) {
		v.Mutable = true
		v.Commit = sv.Metadata()
		v.Tag = v.Channel
	}

	return v, nil
}

// NewVersion creates a new version from a tag and commit.
func NewVersion(ref string, isBranch bool, hash string) (*Version, error) {
	var v Version

	// Attempt to parse the tag as semver for a normal version
	// tag.
	//nolint:gocritic // Why: A switch statement doesn't read well.
	if semV, err := semver.NewVersion(ref); err == nil {
		// Determine the channel from the tag
		// `v1.0.0-alpha.1` -> `alpha`
		// `v1.0.0-unstable+commit` -> `unstable`
		channel := StableChannel
		splPre := strings.Split(semV.Prerelease(), ".")
		if len(splPre) > 0 && splPre[0] != "" {
			channel = splPre[0]
		}

		v = Version{
			sv:      semV,
			Tag:     ref,
			Commit:  hash,
			Channel: channel,
		}
	} else if mutableTagRegex.MatchString(ref) || isBranch {
		// Matches mutable tag format, so handle it as one
		v = Version{
			Mutable: true,
			Tag:     ref,
			Commit:  hash,
			Channel: ref,
		}
		if isBranch {
			v.Branch = ref
			v.Tag = ""
		}
	} else {
		return nil, errors.Errorf("reference %q is not a valid version", ref)
	}

	return &v, nil
}

// GetVersions returns all known channels and the versions that are available
// for each channel.
//
// Note: token is _optional_, if you do not wish to authenticate with your VCS
// provider you can pass an empty string.
var GetVersions = getVersions

// getVersions is documented above
func getVersions(ctx context.Context, token cfg.SecretData, url string, allowBranches bool) (map[string][]Version, error) {
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
		// skip references that aren't a tag
		if !ref.Name().IsTag() {
			// if we allow branches and this reference is a branch, we want to process it
			if !allowBranches || !ref.Name().IsBranch() {
				continue
			}
		}

		v, err := NewVersion(ref.Name().Short(), ref.Name().IsBranch(), ref.Hash().String())
		if err != nil {
			// IDEA(jaredallard): Some way to log this tag has been ignored?
			continue
		}

		// initialize the channels slice if it's nil
		if channels[v.Channel] == nil {
			channels[v.Channel] = []Version{}
		}

		channels[v.Channel] = append(channels[v.Channel], *v)
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
//
// Criteria:
//
// Constraints are supported as per the underlying semver library here:
// https://pkg.go.dev/github.com/Masterminds/semver/v3#hdr-Checking_Version_Constraints_and_Comparing_Versions
//
// If a channel is provided, without a constraint, the latest version of that channel will be returned if it's
// greater than the `stable` channel (if present). As a special case, if a channel's latest version is mutable,
// that version will always be returned over any constraint.
//
// Constraints, by default, do not include a pre-releases or allow selecting specific pre-releases tracks in the
// semver library used. In order to support this we parse the constraints to determine the allowed channels. By
// default `stable` is always considered, with whatever pre-release (e.g. `>=1.0.0-alpha` -> `alpha`) is specified
// in the pre-release constraint being added to the allowed channels. If a channel is specified, it is also added
// to the allowed channels. Due to this '&&' and '||' are not supported in constraints.
func Resolve(ctx context.Context, token cfg.SecretData, c *Criteria) (*Version, error) {
	versions, err := GetVersions(ctx, token, c.URL, c.AllowBranches)
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

	return getLatestVersion(versions, c)
}

// stringInSlice returns true if the string is in the slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// getLatestSatisfyingConstraint returns the latest version that satisfies the constraint
func getLatestSatisfyingConstraint(versions map[string][]Version, c *Criteria) (*Version, error) {
	allowedChannels := []string{c.Channel}
	if c.Channel != StableChannel {
		// always allow stable channel versions
		allowedChannels = append(allowedChannels, StableChannel)
	}

	// we have a constraint, so find the latest version that satisfies it
	constraints := make([]*semver.Constraints, len(c.Constraints))
	for i, constraintStr := range c.Constraints {
		// Limit the ability to use && and || in constraints so we can mutate the constraint
		// to allow pre-releases
		if strings.Contains(constraintStr, "&&") || strings.Contains(constraintStr, "||") {
			return nil, errors.Errorf("multiple constraints within a single constraint are not supported")
		}

		// strip space from the constraint
		constraintStr = strings.TrimSpace(constraintStr)
		if constraintStr == "*" {
			// for some reason * doesn't work with the prereleases hack we use below,
			// so convert * into >= 0.0.0, it's equivalent.
			constraintStr = ">=0.0.0"
		}

		// extract allowed channels from the constraint if there is one
		exampleVer := oprStripRegex.ReplaceAllString(constraintStr, "$1")
		if exampleVerSem, err := semver.NewVersion(exampleVer); err == nil {
			channel := strings.Split(exampleVerSem.Prerelease(), ".")[0]

			if channel != "" {
				alreadyHasChannel := stringInSlice(channel, allowedChannels)
				if !alreadyHasChannel {
					allowedChannels = append(allowedChannels, channel)
				}
			}
		}

		// If we're allowing channels other than the stable channel then we need to
		// mutate the constraint to allow pre-releases. The constraint matching doesn't
		// allow you to specify which pre-releases to allow, so we just allow all here
		// and filter it down later.
		if c.Channel != StableChannel && !strings.Contains(constraintStr, "-") {
			constraintStr += "-prereleases"
		}

		constraint, err := semver.NewConstraint(constraintStr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse constraint %q", constraintStr)
		}

		constraints[i] = constraint
	}

	// join all the versions into a single slice to consider for the constraint
	allVersions := make([]*Version, 0)
	for channel, vers := range versions {
		// skip channels that aren't allowed from the constraint / channel argument
		if !stringInSlice(channel, allowedChannels) {
			continue
		}

		for i := range vers {
			allVersions = append(allVersions, &vers[i])
		}
	}

	// find the latest version that satisfies the constraint
	var selectedVersion *Version
	for _, v := range allVersions {
		// if we find a mutable version, always skip it. They must be explicitly
		// selected.
		if v.Mutable {
			continue
		}

		// ensure this version meets all constraints
		matchesConstraints := true
		for _, constraint := range constraints {
			// if the version doesn't match the constraint, skip it
			if !constraint.Check(v.sv) {
				matchesConstraints = false
				break
			}
		}
		if matchesConstraints {
			// Select the version if we don't have one, otherwise if the version we found is
			// greater than the currently selected version, select it.
			if selectedVersion == nil || v.sv.GreaterThan(selectedVersion.sv) {
				selectedVersion = v
			}
		}
	}

	return selectedVersion, nil
}

// getLatestVersion returns the latest based on the provided criteria, if a constraint
// is provided it will be used to select the latest version that satisfies the constraint.
//
// See Resolve() comment for more details on the overall behaviour this implements.
func getLatestVersion(versions map[string][]Version, c *Criteria) (*Version, error) {
	// Note: selectedVersion may be nil at any point in time, until the end
	// of the function where the default is handled
	var selectedVersion *Version

	// get the latest version from the channel if we don't have a constraint
	if len(c.Constraints) == 0 {
		// if latest version for the channel is mutable, return that. We don't support
		// upgrading mutable (non-semver) versions to the stable channel or doing version
		// constraints.
		cv := getLatestVersionFromSlice(versions[c.Channel])
		if cv.Mutable {
			return cv, nil
		}

		// Consider all versions if we don't have a constraint
		c.Constraints = []string{">=0.0.0"}
	}

	// we have a constraint, so find the latest version that satisfies it
	var err error
	selectedVersion, err = getLatestSatisfyingConstraint(versions, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest version within constraints")
	}
	if selectedVersion == nil {
		return nil, ErrNoVersions
	}

	return selectedVersion, nil
}

// getLatestVersionFromSlice returns the latest versions from a slice of versions.
// This does not mutate the provided slice, and it does not need to be sorted.
func getLatestVersionFromSlice(argVersions []Version) *Version {
	vers := make([]Version, len(argVersions))
	copy(vers, argVersions)
	Sort(vers)
	return &vers[len(vers)-1]
}
