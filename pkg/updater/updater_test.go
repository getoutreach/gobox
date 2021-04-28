package updater_test

import (
	"context"
	"os"

	"github.com/getoutreach/gobox/pkg/updater"
	"github.com/sirupsen/logrus"
)

func ExampleNeedsUpdate() {
	if updater.NeedsUpdate(context.Background(), logrus.New(), "", "v1.0.0", false, false, false, false) {
		// Stop to use the newer version
		os.Exit(0)
	}
}
