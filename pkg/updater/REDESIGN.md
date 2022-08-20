# Updater Redesign

 * Use the metadata field e.g. `1.0.0-<here>` to indicate a release "channel". Also support the ability
   to have a non-semver version be a mutable tag for a repository. e.g. `unstable`. Note: ignore commitguard.
  * e.g. `1.0.0-beta.0` -> `beta`
  * e.g. `unstable` tag would be the `unstable` channel
 * Pull down all git tags by cloning the repository, using git tags for the versions. This speeds up the
   version calculation logic
 * Support multiple different binary storage locations, through an interface. Initial implementation should just
   be through Github Releases. 
 * Support multiple different CLI frameworks entry points, e.g. `urfave/cli` and cobra.


## Version Logic

 * If on a channel (non-release version) we should automatically consider versions from that channel for updating.
 * Release versions should be ignored if the user is on the `rc` or `unstable`, or X other channel UNLESS the release
   version is greater than the current version.
 * Release is defined as a version without any build metadata, e.g. `1.0.0`
