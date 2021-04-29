package events_test

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/shuffler"
)

func TestAll(t *testing.T) {
	shuffler.Run(t, errorSuite{}, eventsSuite{})
}
