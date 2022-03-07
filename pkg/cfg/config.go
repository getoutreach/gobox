// Package cfg manages config for outreach go services
//
// Every go app or package that needs config should define a strongly
// typed struct for it
//
// Example
//
//     type HoneycombConfig struct {
//        Disable    bool       `yaml:"Disable"`
//        Dataset    string     `yaml:"Dataset"`
//        APIHost    string     `yaml:"APIHost"`
//        SampleRate float64    `yaml:"SampleRate"`
//        Key        cfg.Secret `yaml:"Key"`
//     }
//
//     func (x someComponent) someFunc(ctx context.Context) error {
//          var hcConfig HoneycombConfig
//          if err := cfg.Load("honeycomb.yaml", &hcConfig); err != nil {
//              return err
//          }
//
//          ... now use the config...
//     }
//
//
// All config structs should typically implement their own `Load()`
// method so that the config location is specified in one spot:
//
//     func (c *HoneycombConfig) Load() error {
//         return cfg.Load("honeycomb.yaml", &c)
//     }
//
//
// Dev environment overrides
//
// The default directory prefix for config will be chosen to be
// compatible with our k8s deployment strategy.
//
// For dev environments, the preferred path is ~/.outreach/ and this
// can be configured by app init using the following override:
//
//      import env "github.com/getoutreach/gobox/pkg/env"
//      func init() {
//          env.ApplyOverrides()
//      }
//
// To build with this override, the build tag or_dev should be used.
//
// Dev environments may also need command line or environment
// overrides.  The suggested mechanism is to add the override as part
// of the individual `Load()` method on the struct.  At some point, we
// will add code generators for config (based on struct tags such as
// `env:OUTREACH_HONEYCOMB_KEY` or some such mechanism) and the
// `Load()` methods will be automatically generated with the specified
// overrides.
//
// Secrets
//
// While secrets can be accessed in an adhoc way using the secrets
// package, the recommended way is to fetch fixed secrets (i.e. not
// things like oauth tokens) via the `cfg.Secret` type.  For the
// example above, the API key can be accessed via:
//
//      secretData, err := hcConfig.Key.Data(ctx)
//
// Note that SecretData cannot be serialized to JSON or YAML.  While
// it can be converted to string using an explicit conversion, the
// converted value should not be cached or passed to internal
// functions.
package cfg

import (
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v2"
)

// the default read is a prod reader which looks for
// config files in /run/config/outreach.io/<filename>
// nolint:gochecknoglobals
var defaultReader = Reader(func(fileName string) ([]byte, error) {
	name := "/run/config/outreach.io/" + fileName
	if runtime.GOOS == "windows" {
		name = "C:" + filepath.FromSlash(name)
	}

	return os.ReadFile(name)
})

// Reader reads the config from the provided file
type Reader func(fileName string) ([]byte, error)

// Load reads the config.
//
// Usage:
//
//     var appConfig MyConfig
//     err := cfg.Load("myapp.json", &appConfig)
//
//
// This parses the config using JSON.  If a config has special needs,
// it can implement its own UnmarshalJSON (such as implementing
// environment overrides)
func (r Reader) Load(fileName string, ptr interface{}) error {
	data, err := r(fileName)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, ptr)
}

// Load uses the default config reader to load config
func Load(fileName string, ptr interface{}) error {
	return defaultReader.Load(fileName, ptr)
}

// SetDefaultReader sets the default reader.  Only meant for tests and
// dev environment overrides
func SetDefaultReader(f Reader) {
	defaultReader = f
}

// DefaultReader returns the current default reader. Only meant for
// tests and dev environment overrides
func DefaultReader() Reader {
	return defaultReader
}
