# Updater

This package implements an updater in Golang that automatically updates binaries.

## Usage

There's a few different ways to use the updater. Our best integration is with the `urfave/cli` framework, which is a very popular framework for writing command line applications.

### `urfave/cli` integration

To use this, simply take your existing `cli.App` and create a new one with the `NewUpdater` function.

```go
func main() {
  a := &cli.App{}

  // Note: If you're using logrus (highly recommended) you'll also want to do
  // updater.WithLogger(log)
	if _, err := updater.UseUpdater(ctx, updater.WithApp(a)); err != nil {
    // Do something better than panic ;)
		panic(err)
	}
}
```

This will automatically update your application if there's an update available, as well as expose
commands on the `updater` sub-command, such as:

  * `updater set-channel <channel>` - set the current channel (release channel) this application is using
  * `updater get-channels` - list all available release channels for the current application
  * `updater use <version>` - replaces the current binary with a specific version of the application
  * `updater rollback` - rollback to the previous version of the application used before the last update
  * `updater status` - get information on the updater

### Other Places

You can use the `updater.UseUpdater` function in other places as well, but you will not get the tooling provided
by the `urfave/cli` framework version (such as the ability to set the current release channel).

Example:

```go
func main() {
  updated, err := updater.UseUpdater(context.Background())
  if err != nil {
    panic(err)
  }

  if updated {
    // tell user to restart their application
    fmt.Println("Updated. Please restart your application")
  }
}
```

## How the Updater Works / Requirements

The updater works by reading version information out of the current binary and using git tags on
the remote repository to source release versions / channels. The only hard requirement, at a minimum,
of the updater is that the binary must use the `pkg/app` package as well as use semantic versioning.

The updater derives the current version and release channel from a version string. The version string
comes from normal semantic versioning, e.g. `v1.0.0` -> `1.0.0`. A channel comes from the pre-release
field of a semantic-version, e.g. `v1.0.0-alpha.1` -> `alpha`. If there is no pre-release field, the
channel is `stable`.

### Mutable Tags

Another concept supported by the updater is a concept called "mutable tags", this allows you to release
code with binaries while not increase the version number. This is useful for HEAD based development where
you don't want to release a new version every time you make a change, but you do want it to still be testable.

For example, and this is how Outreach does CLI tooling releasing, you could use mutable tags to create a flow
like this:

  * Merges into `main` go to `unstable`
  * Merges into `rc` branch go to `rc`
  * Merges into `release` go to `stable`

This would be done by doing the following via CI:

  * main: Create a Github Release (or tag) named `unstable` on the latest commit
  * rc: Force-push `rc` with the contents of `main`, then create a Github Release (or tag) with a `rc` pre-release version (`v1.0.0-rc.1` for example)
  * release: Force-push `release` with the contents of `rc`, then create a Github Release (or tag) without a pre-release version (`v1.0.0` for example)

The updater would then consider the list of channels to be `stable`, `unstable`, and `rc` respectively

**Note**: This doesn't cover uploading release binaries to CI, which is also something you'd need to do. The updater supports
the following archive types: `.tar` `.tar.gz` `.tar.xz` `.tar.bz2` `.zip`. The name must be of a certain format, which is
documented in `updater_test.go` (`Test_generatePossibleAssetNames`) and must contain the binary with the same name as the
current executable.
