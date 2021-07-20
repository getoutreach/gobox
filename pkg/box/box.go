// Package box implements the definitions of a box configuration file
// and tools to access it. This is used to configure the suite of tools
// that outreach provides, aka "startup in a box"
package box

import "time"

// SnapshotLockChannel is used to determine the quality of
// a given snapshot
type SnapshotLockChannel string

const (
	// SnapshotLockChannelStable is a stable channel
	SnapshotLockChannelStable SnapshotLockChannel = "stable"

	// SnapshotLockChannelRC is a release candidate (less stable) channel
	SnapshotLockChannelRC SnapshotLockChannel = "rc"
)

type DeveloperEnvironmentConfig struct {
	// SnapshotConfig is the snapshot configuration for the devenv
	SnapshotConfig *SnapshotConfig `yaml:"snapshots"`

	// VaultConfig denotes how to talk to Vault
	VaultConfig *VaultConfig `yaml:"vault"`

	// ImagePullSecret is a path to credentials used to pull images with
	// currently the only supported value is a vault key path with
	// VaultEnabled being true
	ImagePullSecret string `yaml:"imagePullSecret"`

	// ImageRegistry is the registry to use for detecting your apps
	// e.g. gcr.io/outreach-docker
	ImageRegistry string `yaml:"imageRegistry"`
}

// VaultConfig is the configuration for accessing Vault
type VaultConfig struct {
	// Enabled determines if we should setup vault or not
	Enabled bool `yaml:"enabled"`

	// AuthMethod is the method to talk to vault, e.g. oidc
	AuthMethod string `yaml:"authMethod"`

	// Address is the URL to talk to Vault
	Address string `yaml:"address"`
}

// SnapshotConfig stores configuration for generated and accessing
// snapshots
type SnapshotConfig struct {
	// Endpoint is the S3 compatible endpoint to fetch a snapshot from
	Endpoint string `yaml:"endpoint"`

	// Region is the region to use for this bucket
	Region string `yaml:"region"`

	// Bucket is the bucket that the snapshots are in
	Bucket string `yaml:"bucket"`

	// DefaultName is the default name (snapshot) to use, e.g. flagship
	DefaultName string `yaml:"defaultName"`

	// ReadAWSRole is the role to use, if set, for saml2aws for RO access
	ReadAWSRole string `yaml:"readAWSRole"`

	// WriteAWSRole is the role to use, if set, for saml2aws for RW access
	WriteAWSRole string `yaml:"writeAWSRole"`
}

// Config is the basis of a box configuration
type Config struct {
	// Org is the Github org for this box, e.g. getoutreach
	Org string `yaml:"org"`

	// DeveloperEnvironmentConfig is the configuration for the developer environment for this box
	DeveloperEnvironmentConfig *DeveloperEnvironmentConfig `yaml:"devenv"`
}

// Storage is a wrapper type used for storing the box configuration
type Storage struct {
	Config *Config `yaml:"config"`

	LastUpdated time.Time `yaml:"lastUpdated"`
	StorageURL  string    `yaml:"storageURL"`
}

// NewConfig makes a full initialized Config
func NewConfig() *Config {
	return &Config{
		DeveloperEnvironmentConfig: &DeveloperEnvironmentConfig{
			SnapshotConfig: &SnapshotConfig{},
		},
	}
}

// SnapshotTarget is the defn for a generated snapshot
type SnapshotTarget struct {
	// Command is the command to be run to generate this snapshot,
	// note that a devenv is already provisioned and accessible at this
	// stage of the generation process
	Command string `yaml:"command"`

	// PostRestore is a path to a yaml file that contains pre-rendered manifests
	// These manifests will be ran through a special go-template that allows
	// injecting information like the current user / git email.
	PostRestore string `yaml:"post_restore"`

	// DeployApps is an array of applications to deploy via deploy-app
	// before running the Command specified.
	DeployApps []string `yaml:"deploy_apps"`

	// ReadyAddress is a URL to ping before marking the devenv as ready
	ReadyAddress string `yaml:"readyAddress"`
}

// SnapshotGenerateConfig stores configuration for snapshots that should be generated
type SnapshotGenerateConfig struct {
	// Targets are all of the snapshots that can be generated. The key equates
	// the name of the generated snapshot
	Targets map[string]*SnapshotTarget `yaml:"targets"`
}

// SnapshotLockTarget is a generated snapshot and metadata on it.
// In general SnapshotLockListItem should be used instead.
type SnapshotLockTarget struct {
	// Digest is a MD5 base64 encoded digest of the archive
	Digest string `yaml:"digest"`

	// Key is the key that this snapshot is stored at, note that the bucket is
	// not set or determined here and instead come from the snapshotconfig
	URI string `yaml:"key"`

	// Config is the config used to generate this snapshot
	Config *SnapshotTarget

	// VeleroBackupName is the name of this snapshot. This is used to invoke velero
	// commands. It should not be used for uniqueness constraints.
	VeleroBackupName string `yaml:"veleroBackupName"`
}

// SnapshotLockListItem is a replacement for SnapshotLockTarget which is
// used by SnapshotLockList to provide details about a snapshot
type SnapshotLockListItem struct {
	// Digest is a MD5 base64 encoded digest of the archive
	Digest string `yaml:"digest"`

	// Key is the key that this snapshot is stored at, note that the bucket is
	// not set or determined here and instead come from the snapshotconfig
	URI string `yaml:"key"`

	// Config is the config used to generate this snapshot
	Config *SnapshotTarget

	// VeleroBackupName is the name of this snapshot. This is used to invoke velero
	// commands. It should not be used for uniqueness constraints.
	VeleroBackupName string `yaml:"veleroBackupName"`
}

// SnapshotLockList contains a channel (different releases of snapshots)
// seperated list of snapshots
type SnapshotLockList struct {
	// Snapshots is a channel separated list of snapshots for a given target
	Snapshots map[SnapshotLockChannel][]*SnapshotLockListItem `yaml:"snapshots"`
}

// SnapshotLock is an manifest of all the available snapshots
type SnapshotLock struct {
	// Version is the version of this configuration, used for breaking changes
	Version int `yaml:"version"`
	// GeneratedAt is when this lock was generated
	GeneratedAt time.Time `yaml:"generatedAt"`

	// Deprecated: Use TargetsV2 instead
	// Targets is a single snapshot for each target
	Targets map[string]*SnapshotLockTarget `yaml:"targets"`

	// TargetsV2 is a target -> lock list for snapshots
	TargetsV2 map[string]*SnapshotLockList `yaml:"targets_v2"`
}
