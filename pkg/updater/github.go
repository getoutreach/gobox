package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/getoutreach/gobox/pkg/cfg"
	olog "github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/google/go-github/v34/github"
	"github.com/inconshreveable/go-update"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
)

//nolint:gochecknoglobals
var (
	ErrNoNewRelease = errors.New("no new release")
	ErrNoAsset      = errors.New("no asset found")
	ErrMissingFile  = errors.New("file missing in archive")

	AssetSeperators = []string{"_", "-"}
	AssetExtensions = []string{".tar.gz", ""}
)

type Github struct {
	gc *github.Client

	org  string
	repo string

	// Configuration Options
	Silent bool
}

type githubRelease struct {
	*github.RepositoryRelease
	version semver.Version
}

// NewGithubUpdater creates a new updater powered by Github
func NewGithubUpdater(ctx context.Context, token cfg.SecretData, org, repo string) *Github {
	h := http.DefaultClient
	if token != "" {
		h = oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(token)},
		))
	}
	gc := github.NewClient(h)
	return &Github{gc, org, repo, false}
}

// NewGithubUpdaterWithClient creates a new updater with the provided Github Client
func NewGithubUpdaterWithClient(ctx context.Context, client *github.Client, org, repo string) *Github {
	return &Github{client, org, repo, false}
}

// Check checks if the credentials / repo are valid.
func (g *Github) Check(ctx context.Context) error {
	ctx = trace.StartCall(ctx, "github.Check")
	defer trace.EndCall(ctx)

	_, _, err := g.gc.Repositories.Get(ctx, g.org, g.repo)
	return err
}

// GetLatestVersion finds the latest release, based on semver, of a Github Repository
// (supplied when client was created). This is determined by the following algorithm:
//
// Finding new release:
//
//  Github releases are then streaming evaluated to find the currentVersion. All releases
//  that are not == to the current version end up being stored in memory as "candidates"
//  for being evaluated as a possible new version. If the current version is not found
//  then it is ignored.
//
// Including pre-releases:
//
//  If the current version is a pre-release:
//   - pre-releases are considered
//  If includePrereleases is true
//   - pre-releases are considered
//
// Selecting a new version:
//
//  Once the current releases has been found (or not found) then all versions found before
//  it are considered as candidates and checked to see if a newer release exists. Using the
//  aforementioned pre-release logic pre-releases are included based on that.
func (g *Github) GetLatestVersion(ctx context.Context, currentVersion string, includePrereleases bool) (*github.RepositoryRelease, error) {
	ctx = trace.StartCall(ctx, "github.GetLatestVersion", olog.F{"currentVersion": currentVersion, "prereleases": includePrereleases})
	defer trace.EndCall(ctx)

	// if we can't determine the version, fallback to empty (oldest) version
	version := semver.MustParse("0.0.0")
	if currentVersion != "" {
		parsedVersion, err := semver.ParseTolerant(currentVersion)
		if err == nil {
			version = parsedVersion
		}
	}

	// Skip pre versions that don't aren't rc (made by bootstrap)
	if len(version.Pre) > 0 && version.Pre[0].String() != "rc" {
		return nil, ErrNoNewRelease
	}

	// Note: ourRelease is nil if not found
	_, newRelease, err := g.getAllReleases(ctx, &version, includePrereleases)
	if err != nil {
		return nil, err
	}

	return newRelease, nil
}

// GetRelease finds a release with a given version (tag)
func (g *Github) GetRelease(ctx context.Context, version string) (*github.RepositoryRelease, error) {
	ctx = trace.StartCall(ctx, "github.GetRelease", olog.F{"version": version})
	defer trace.EndCall(ctx)

	rel, _, err := g.gc.Repositories.GetReleaseByTag(ctx, g.org, g.repo, version)
	return rel, err
}

//nolint:funlen,gocyclo // Not sure how to split this out currently.
func (g *Github) getAllReleases(ctx context.Context, currentVersion *semver.Version, includePrereleases bool) (curR, newR *github.RepositoryRelease, err error) {
	ctx = trace.StartCall(ctx, "github.getAllReleases")
	defer trace.EndCall(ctx)

	releases := make([]*githubRelease, 0)

	page := 0
	var currentRelease *github.RepositoryRelease

loop:
	for {
		rs, resp, err := g.gc.Repositories.ListReleases(ctx, g.org, g.repo, &github.ListOptions{
			Page:    page,
			PerPage: 10,
		})
		if err != nil {
			return nil, nil, err
		}

		for i, r := range rs {
			// skip releases without a tag
			if r.TagName == nil {
				continue
			}

			// Don't include drafts
			if r.GetDraft() {
				continue
			}

			version, err := semver.ParseTolerant(*r.TagName)
			if err != nil {
				// skip invalid semver tags
				continue
			}

			// check each version to find which github release we're equal to
			// we do this to ensure our version string is always calculated to
			// be the same.
			if currentVersion.EQ(version) {
				currentRelease = rs[i]

				// we found ourself, so stop processing at this point
				// since Github returns newest first.
				break loop
			}

			releases = append(releases, &githubRelease{
				rs[i],
				version,
			})
		}

		if resp.NextPage == 0 {
			break
		}

		page = resp.NextPage
	}

	trace.AddInfo(ctx, olog.F{"releases": len(releases), "pages": page})

	var newRelease *github.RepositoryRelease
	for i, r := range releases {
		if r.GetPrerelease() {
			// if we're not allowed to include pre-releases, and the release we're on
			// is not already a pre-release, then skip pre-release
			if !includePrereleases && !(currentRelease != nil && currentRelease.GetPrerelease()) {
				continue
			}
		}

		// if the release is newer than ours, use it
		// github returns newest releases first so this should be
		// generally safe
		if r.version.GT(*currentVersion) {
			newRelease = releases[i].RepositoryRelease
			break
		}
	}
	if newRelease == nil {
		return nil, nil, ErrNoNewRelease
	}

	return currentRelease, newRelease, nil
}

// DownloadRelease attempts to download a binary from a release.
//
// If the asset found is an archive, it'll be extracted and
// the value of `execName` will be used to pull a file out of
// the root of the archive. If `execName` is not provided it is
// inferred as the name of the currently running basename of the
// running executable. The downloaded file is returned as `downloadedBinary`
// with a cleanup function being returned to remove all leftover data.
//
// The cleanup function should be called even when an error occurs
//nolint:funlen
func (g *Github) DownloadRelease(ctx context.Context, r *github.RepositoryRelease, assetName, execName string) (downloadedBinary string, cleanup func(), err error) {
	ctx = trace.StartCall(ctx, "github.DownloadRelease", olog.F{"version": *r.TagName})
	defer trace.EndCall(ctx)

	// if we weren't given an executable name
	// look up the name of the currently running program
	if execName == "" {
		//nolint:govet // Why: We're OK shadowing error
		execPath, err := g.getExecPath()
		if err != nil {
			return "", func() {}, err
		}
		execName = filepath.Base(execPath)
	}

	url, asset, err := g.SelectAsset(ctx, r.Assets, assetName)
	if err != nil {
		return "", func() {}, err
	}

	// The url returned from SelectAsset has auth in it.
	// That endpoint doesn't allow double auth, and so we don't send the bearer token.
	// Instead, we use the default http client with no auth on every request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", func() {}, errors.Wrapf(err, "failed to create HTTP request to '%s'", url)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", func() {}, errors.Wrapf(err, "failed to send HTTP request to '%s'", url)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", func() {}, fmt.Errorf("got unexpected status code: %v", resp.StatusCode)
	}

	tmpDir := filepath.Join(os.TempDir(), "updater", strings.ReplaceAll(time.Now().Format(time.RFC3339Nano), ":", "_"))
	err = os.MkdirAll(tmpDir, 0755)
	if err != nil {
		return "", func() {}, errors.Wrap(err, "failed to make temp directory")
	}

	cleanupFn := func() {
		os.RemoveAll(tmpDir)
	} //nolint:funlen

	file := filepath.Join(tmpDir, *asset.Name)
	f, err := os.Create(file)
	if err != nil {
		return "", cleanupFn, errors.Wrap(err, "failed to create download file")
	}
	defer f.Close()

	traceCtx := trace.StartCall(ctx, "github.DownloadRelease.Download")
	if g.Silent {
		_, err = io.Copy(f, resp.Body)
	} else {
		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"downloading update",
		)
		_, err = io.Copy(io.MultiWriter(f, bar), resp.Body)
	}
	if err != nil && err != io.EOF {
		return "", cleanupFn, err
	}
	trace.EndCall(traceCtx)

	file, err = g.processArchive(ctx, file, tmpDir, execName, asset)
	return file, cleanupFn, err
}

// processArchive checks if the file is an archive, and extracts it looking for the same file
// as the current executable. If it is not an archive then the file given is returned
func (g *Github) processArchive(ctx context.Context, file, tmpDir, execName string, asset *github.ReleaseAsset) (string, error) {
	// TODO: better support for other archives in the future
	if strings.HasSuffix(asset.GetName(), ".tar.gz") {
		f, err := os.Open(file)
		if err != nil {
			return "", errors.Wrap(err, "failed to open downloaded archive")
		}
		defer f.Close()

		storageDir := filepath.Join(tmpDir, strings.TrimSuffix(asset.GetName(), ".tar.gz"))
		err = os.MkdirAll(storageDir, 0755)
		if err != nil {
			return "", err
		}

		// use the name of the executable here to allow for multiple clis in a given repository
		file, err = g.getFileFromArchive(ctx, f, storageDir, execName)
		if err != nil {
			return "", errors.Wrap(err, "failed to extract archive")
		}
	}

	return file, nil
}

func (g *Github) getExecPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", errors.Wrap(err, "failed to find running executable")
	} //nolint:funlen

	return filepath.EvalSymlinks(execPath)
}

// ReplaceRunning replaces the running executable with the specified
// path. This path is renamed to the current executable.
//
// The running process is replaced with a new invocation of the new binary.
func (g *Github) ReplaceRunning(ctx context.Context, newBinary string) error {
	execPath, err := g.getExecPath()
	if err != nil {
		return err
	}

	f, err := os.Open(newBinary)
	if err != nil {
		return err
	}

	err = update.Apply(f, update.Options{
		TargetPath: execPath,
	})
	return errors.Wrap(err, "failed to apply update")
}

//nolint:funlen
func (g *Github) getFileFromArchive(ctx context.Context, f *os.File, storageDir, filename string) (string, error) {
	ctx = trace.StartCall(ctx, "github.getFileFromArchive")
	defer trace.EndCall(ctx)

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", errors.Wrap(err, "failed to gzip decode stream")
	}

	tarReader := tar.NewReader(gzr)

	srcFile, srcSize, err := findTarFile(ctx, tarReader, filename)
	if err != nil {
		return "", err
	}

	file := filepath.Join(storageDir, filename)
	targetFile, err := os.Create(file)
	if err != nil {
		return "", err
	}
	defer targetFile.Close()

	if g.Silent {
		//nolint:gosec
		_, err = io.Copy(targetFile, srcFile)
	} else {
		bar := progressbar.DefaultBytes(
			srcSize,
			// extra space here to match the downloading update length
			"extracting update ",
		)
		_, err = io.Copy(io.MultiWriter(targetFile, bar), srcFile) //nolint:gosec // wtaf?
	}
	if err != nil && err != io.EOF {
		return "", err
	}

	return file, nil
}

// finds specified file in provided tar archive and returns io.Reader and its size for extraction progress
func findTarFile(ctx context.Context, archive *tar.Reader, filename string) (io.Reader, int64, error) {
	for ctx.Err() == nil {
		header, err := archive.Next()

		if err == io.EOF {
			return nil, 0, ErrMissingFile
		} else if err != nil {
			return nil, 0, err
		}

		if header.Typeflag == tar.TypeReg && header.Name == filename {
			return archive, header.Size, nil
		}
	}

	if ctx.Err() != nil {
		return nil, 0, ctx.Err()
	}

	return nil, 0, ErrMissingFile
}

// SelectAsset finds an asset on a Github Release.
// This looks up the following file patterns:
// - name_GOOS_GOARCH
// - name_version_GOOS_GOARCH
// - name_GOOS_GOARCH.tar.gz
// - name_version_GOOS_GOARCH.tar.gz
func (g *Github) SelectAsset(ctx context.Context, assets []*github.ReleaseAsset, name string) (string, *github.ReleaseAsset, error) {
	ctx = trace.StartCall(ctx, "github.SelectAsset")
	defer trace.EndCall(ctx)

	prefixes := make([]string, 0)
	suffixes := make([]string, 0)

	for _, sep := range AssetSeperators {
		prefixes = append(prefixes, name+sep)
	}

	for _, sep := range AssetSeperators {
		for _, ext := range AssetExtensions {
			// for example: bootstrap[_-]linux[_-]amd64[.tar.gz]
			suffixes = append(suffixes, sep+runtime.GOOS+sep+runtime.GOARCH+ext)
		}
	}

	var choosenAsset *github.ReleaseAsset
loop:
	for _, a := range assets {
		name := a.GetName()
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				for _, suffix := range suffixes {
					if strings.HasSuffix(name, suffix) {
						choosenAsset = a
						break loop
					}
				}
			}
		}
	}
	if choosenAsset == nil {
		return "", nil, ErrNoAsset
	}

	_, redirectURL, err := g.gc.Repositories.DownloadReleaseAsset(ctx, g.org, g.repo, choosenAsset.GetID(), nil)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to get asset url")
	}

	if redirectURL == "" {
		return "", nil, fmt.Errorf("failed to find asset url")
	}

	return redirectURL, choosenAsset, nil
}
