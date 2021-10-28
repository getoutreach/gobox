package log_test

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/shuffler"
)

func TestAll(t *testing.T) {
	log.AllowContextFields("context.string", "context.number", "or.org.guid", "or.org.shortname")
	shuffler.Run(t, fatalSuite{}, withSuite{}, callerSuite{}, logContextSuite{})
}
