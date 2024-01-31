// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements a box configuration file format and provides helpers for working with it

// Package box implements the definitions of a box configuration file
// and tools to access it. This is used to configure the suite of tools
// that outreach provides, aka "startup in a box"
package box

import (
	"time"

	"gopkg.in/yaml.v3"
)

// SnapshotLockChannel is used to determine the quality of
// a given snapshot
type SnapshotLockChannel string

const (
	// SnapshotLockChannelStable is a stable channel
	SnapshotLockChannelStable SnapshotLockChannel = "stable"

	// SnapshotLockChannelRC is a release candidate (less stable) channel
	SnapshotLockChannelRC SnapshotLockChannel = "rc"

	// Version is the current version of the box spec.
	Version float32 = 3
)

// AWSConfig configures AWS access for tools that support it
type AWSConfig struct {
	// DefaultRole is the default role to assume when communicating
	// with AWS.
	DefaultRole string `yaml:"defaultRole"`

	// DefaultProfile is the default profile to use when communicating
	// with AWS.
	DefaultProfile string `yaml:"defaultProfile"`

	// AccountId is the default Account ID to use when communicating
	// with AWS.
	DefaultAccountID string `yaml:"defaultAccountID"`

	// RefreshMethod is the CLI used to refresh AWS credentials.
	// Known values:
	// * okta-aws-cli (default)
	RefreshMethod string `yaml:"refreshMethod"`
	// Okta contains configuration for using Okta authentication
	// with AWS.
	Okta OktaConfig `yaml:"okta"`
}

// OktaConfig is the Okta-specific config used for AWS access.
type OktaConfig struct {
	// FederationAppID is the Okta app ID for the AWS federation app.
	FederationAppID string `yaml:"federationAppID"`
	// OIDCClientID is the Okta app ID for the AWS OIDC app (not the
	// federation one).
	OIDCClientID string `yaml:"oidcClientID"`
	// OrgDomain is the hostname of the Okta instance.
	OrgDomain string `yaml:"orgDomain"`
	// SessionDuration is the TTL for the session in seconds.
	SessionDuration uint `yaml:"sessionDuration"`
}

type DeveloperEnvironmentConfig struct {
	// SnapshotConfig is the snapshot configuration for the devenv
	SnapshotConfig SnapshotConfig `yaml:"snapshots"`

	// VaultConfig denotes how to talk to Vault
	VaultConfig VaultConfig `yaml:"vault"`

	// ImagePullSecret is a path to credentials used to pull images with
	// currently the only supported value is a vault key path with
	// VaultEnabled being true
	ImagePullSecret string `yaml:"imagePullSecret"`

	// ImageRegistry is the registry to use for detecting your apps
	// e.g. gcr.io/outreach-docker
	ImageRegistry string `yaml:"imageRegistry"`

	// RuntimeConfig stores configuration specific to different devenv
	// runtimes.
	RuntimeConfig DeveloperEnvironmentRuntimeConfig `yaml:"runtimeConfig"`

	// VersionResolvers stores the configuration for version resolvers
	VersionResolvers DevenvVersionResolvers `yaml:"versionResolvers"`
}

// DeveloperEnvironmentRuntimeConfig stores configuration specific to
// different runtimes.
type DeveloperEnvironmentRuntimeConfig struct {
	// EnabledRuntimes dictates which runtimes are enabled, generally defaults to all.
	EnabledRuntimes []string `yaml:"enabledRuntimes"`

	// DevelopmentRegistries are image registries that should be used for
	// development docker images. These are only ever used for remote devenvs.
	DevelopmentRegistries DevelopmentRegistries `yaml:"developmentRegistries"`

	// Loft is configuration for the loft runtime in the devenv
	Loft LoftRuntimeConfig `yaml:"loft"`
}

// VaultConfig is the configuration for accessing Vault
type VaultConfig struct {
	// Enabled determines if we should setup vault or not
	Enabled bool `yaml:"enabled"`

	// AuthMethod is the method to talk to vault, e.g. oidc
	AuthMethod string `yaml:"authMethod"`

	// Address is the URL to talk to Vault
	Address string `yaml:"address"`

	// AddressCI is the URL to use to talk to Vault in CI
	// Defaults to Address
	AddressCI string `yaml:"addressCI"`
}

// DevenvVersionResolvers is the configurations used to get the latest version
type DevenvVersionResolvers struct {
	// Enabled is a list of image resolvers to use. If none are specified Maestro will be used
	// ordered based on priority. External customers should default to git
	Enabled []string `yaml:"enabled"`

	// Config is the configuration information for all version resolvers
	Config VersionResolverConfig `yaml:"config"`
}

// VersionResolverConfig contains configuration for version resolvers
type VersionResolverConfig struct {
	// Maestro configuration used by maestro image resolver
	Maestro MaestroConfig `yaml:"maestro"`
}

// MaestroConfig contains configuration used by the maestro version resolver
type MaestroConfig struct {
	// VaultSecretPath vault path that contains the auth secret to access maestro API
	VaultSecretPath string `yaml:"vaultSecretPath"`

	// VaultSecretKey the key within the VaultSecretPath that contains the API token
	VaultSecretKey string `yaml:"vaultSecretKey"`

	// Channels list of channels that maestro should retrieve version from
	Channels []string `yaml:"channels"`
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

	// ReadAWSRole is the role to use, if set, for RO access to AWS
	ReadAWSRole string `yaml:"readAWSRole"`

	// WriteAWSRole is the role to use, if set, for RW access to AWS
	WriteAWSRole string `yaml:"writeAWSRole"`
}

// Config is the basis of a box configuration
type Config struct {
	// RefreshInterval is the interval to use when refreshing a box configuration
	RefreshInterval time.Duration `yaml:"refreshInterval"`

	// Org is the Github org for this box, e.g. getoutreach
	Org string `yaml:"org"`

	// DeveloperEnvironmentConfig is the configuration for the developer environment for this box
	DeveloperEnvironmentConfig DeveloperEnvironmentConfig `yaml:"devenv"`

	// AWS is the configuration for communicating with AWS.
	AWS AWSConfig `yaml:"aws"`

	// CI is the configuration for the CI environment
	CI CI `yaml:"ci"`

	// CD is the configuration
	CD CD `yaml:"cd"`
}

// Storage is a wrapper type used for storing the box configuration
type Storage struct {
	// Config is the box configuration, see Config.
	// This is an yaml.Node because we can't guarantee that the
	// underlying type is a Config as we expect it to be.
	Config yaml.Node `yaml:"config"`

	// LastUpdated is the last time this file was checked for updates
	LastUpdated time.Time `yaml:"lastUpdated"`

	// Version is the version of this box spec.
	Version float32 `yaml:"version"`

	// StorageURL is the location that this came from
	StorageURL string `yaml:"storageURL"`
}

// NewConfig makes a full initialized Config
func NewConfig() *Config {
	return &Config{}
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

	// PostDeployApps is an array of applications to deploy via deploy-app
	// after running the Command specified.
	PostDeployApps []string `yaml:"post_deploy_apps"`

	// ReadyAddress is a URL to ping before marking the devenv as ready
	ReadyAddress string `yaml:"readyAddress"`

	// RestoreSteps defines the startup order of the resources within this snapshot.
	// If not specified, all snapshot resources are started at the same time
	RestoreSteps []RestoreStep `yaml:"restore_steps"`
}

// RestoreStep maps to a set of resources included in the snapshot.
//
// Examples:
//
// - a "databases" step specifies "IncludedNamespaces" of all database namespaces
//
// - a "all the rest" step only specifies "ExcludedNamespaces" used in previous steps
type RestoreStep struct {
	// Description describes what resources are restored in this step
	Description string

	// IncludedNamespaces defines what namespaces are restored in this step. It maps to Velero IncludedNamespaces restore parameter
	IncludedNamespaces []string `yaml:"included_namespaces"`

	// ExcludedNamespaces defines what namespaces are not restored in this step. It maps to Velero ExcludedNamespaces restore parameter
	ExcludedNamespaces []string `yaml:"excluded_namespaces"`
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
// separated list of snapshots
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
