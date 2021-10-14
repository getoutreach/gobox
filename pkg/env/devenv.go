//go:build or_dev
// +build or_dev

package env

import (
	"github.com/getoutreach/gobox/pkg/cfg"
)

func ApplyOverrides() {
	cfg.SetDefaultReader(devReader(cfg.DefaultReader()))
}
