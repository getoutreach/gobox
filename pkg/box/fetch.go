package box

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/getoutreach/gobox/pkg/sshhelper"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	// BoxConfigPath is the $HOME/<BoxConfigPath> location of the box config storage
	BoxConfigPath = ".outreach/.config/box"
	// BoxConfigFile is the name of the box config storage file
	BoxConfigFile = "box.yaml"
)

func getBoxPath() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "failed to get user homedir")
	}

	return filepath.Join(homedir, BoxConfigPath, BoxConfigFile), nil
}

// LoadBox loads the default box or returns an error
func LoadBox() (*Config, error) {
	_, c, err := LoadBoxStorage()
	if err != nil {
		return nil, err
	}

	ApplyEnvOverrides(c)
	return c, nil
}

// ApplyEnvOverrides overrides a box configuration based on env vars.
func ApplyEnvOverrides(s *Config) {
	if vaultAddr := os.Getenv("VAULT_ADDR"); vaultAddr != "" {
		s.DeveloperEnvironmentConfig.VaultConfig.Address = vaultAddr
	}

	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		s.AWS.DefaultProfile = profile
	}

	if role := os.Getenv("AWS_ROLE"); role != "" {
		s.AWS.DefaultRole = role
	}

	// Set the CI address to the address if not set
	if s.DeveloperEnvironmentConfig.VaultConfig.AddressCI == "" {
		s.DeveloperEnvironmentConfig.VaultConfig.AddressCI = s.DeveloperEnvironmentConfig.VaultConfig.Address
	}
}

// LoadBoxStorage reads a serialized, storage wrapped
// box config from disk and returns it. In general LoadBox
// should be used over this function.
func LoadBoxStorage() (*Storage, *Config, error) {
	confPath, err := getBoxPath()
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(confPath)
	if err != nil {
		return nil, nil, err
	}

	var s Storage
	var c Config

	// Parse the storage layer
	if err := yaml.NewDecoder(f).Decode(&s); err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse box storage")
	}

	// Encode the config back to yaml so we can attempt to turn it
	// into a Config.
	b, err := yaml.Marshal(s.Config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal config")
	}

	// Parse the config out of the storage
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse config")
	}

	return &s, &c, nil
}

// EnsureBox loads a box if it already exists, or prompts the user for the box
// if not found. If it exists, remote is querired periodically for a new version.
// Deprecated: Use EnsureBoxWithOptions instead.
func EnsureBox(ctx context.Context, defaults []string, log logrus.FieldLogger) (*Config, error) {
	return EnsureBoxWithOptions(ctx, WithDefaults(defaults), WithLogger(log))
}

// EnsureBoxWithOptions loads a box if it already exists or returns an error.
// The box config is periodically refreshed based on the configured interval and
// based on a min version requirement, if set.
func EnsureBoxWithOptions(ctx context.Context, optFns ...LoadBoxOption) (*Config, error) {
	v := Version
	opts := &LoadBoxOptions{
		log: logrus.New(),

		// Always default to the min version being the version
		// in this package.
		MinVersion: &v,
	}

	for _, f := range optFns {
		f(opts)
	}

	s, c, err := LoadBoxStorage()
	if os.IsNotExist(err) {
		err = InitializeBox(ctx, []string{})
		if err != nil {
			return nil, err
		}

		return LoadBox()
	} else if err != nil {
		return nil, err
	}

	var reason string

	// Ensure that the min version is met if provided
	// this ensures that forwards compatibility is maintained
	if opts.MinVersion != nil {
		if s.Version < *opts.MinVersion {
			reason = "Minimum box spec version not met"
		}
	}

	if reason == "" {
		diff := time.Now().UTC().Sub(s.LastUpdated)
		if diff < c.RefreshInterval { // if last updated wasn't time interval, skip update
			return c, nil
		}
		reason = "Periodic refresh hit"
	}

	opts.log.WithField("reason", reason).Info("Refreshing box configuration")
	// past the time interval, refresh the config
	s.Config, err = downloadBox(ctx, s.StorageURL)
	if err != nil {
		return nil, err
	}
	return c, SaveBox(ctx, s)
}

// downloadBox downloads and parses a box config from a given repository
// URL.
func downloadBox(ctx context.Context, gitRepo string) (yaml.Node, error) {
	a := sshhelper.GetSSHAgent()

	//nolint:errcheck // Why: Best effort and not worth bringing logger here
	_, err := sshhelper.LoadDefaultKey("github.com", a, &logrus.Logger{Out: io.Discard})
	if err != nil {
		return yaml.Node{}, errors.Wrap(err, "failed to load Github SSH key into in-memory keyring")
	}

	fs := memfs.New()
	_, err = git.CloneContext(ctx, memory.NewStorage(), fs, &git.CloneOptions{
		URL:   gitRepo,
		Auth:  sshhelper.NewExistingSSHAgentCallback(a),
		Depth: 1,
	})
	if err != nil {
		return yaml.Node{}, err
	}

	f, err := fs.Open(BoxConfigFile)
	if err != nil {
		return yaml.Node{}, errors.Wrap(err, "failed to read box configuration file")
	}

	// Parse the config into a yaml.Node to keep comments
	var n yaml.Node
	if err := yaml.NewDecoder(f).Decode(&n); err != nil {
		return yaml.Node{}, errors.Wrap(err, "failed to decode box configuration file")
	}

	// We return the first node because we don't want the document start
	return *n.Content[0], nil
}

// SaveBox takes a Storage wrapped box configuration, serializes it
// and then saves it to the well-known config path on disk.
func SaveBox(_ context.Context, s *Storage) error {
	s.LastUpdated = time.Now().UTC()
	s.Version = Version

	b, err := yaml.Marshal(s)
	if err != nil {
		return errors.Wrap(err, "failed to marshal box storage")
	}

	confPath, err := getBoxPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(confPath, b, 0o600)
}

// InitializeBox prompts the user for a box config location,
// downloads it and then saves it to disk. In general EnsureBox
// should be used over this function.
func InitializeBox(ctx context.Context, defaults []string) error {
	gitRepo := ""

	err := survey.AskOne(&survey.Input{
		Message: "Please enter your box configuration git URL",
		Help:    "This is the repository that contains your box.yaml and will be used for outreach tooling",
	}, &gitRepo)
	if err != nil {
		return err
	}

	conf, err := downloadBox(ctx, gitRepo)
	if err != nil {
		return err
	}

	return SaveBox(ctx, &Storage{
		StorageURL: gitRepo,
		Config:     conf,
	})
}
