package cli

import (
	"context"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestCommonProps(t *testing.T) {
	lm := commonProps()

	props := make(map[string]interface{})
	lm.MarshalLog(func(key string, v interface{}) {
		props[key] = v
	})

	if props["os.name"] != runtime.GOOS {
		t.Errorf("expected '%s', got '%s'", runtime.GOOS, props["os.name"])
	}
	if props["os.arch"] != runtime.GOARCH {
		t.Errorf("expected '%s', got '%s'", runtime.GOARCH, props["os.arch"])
	}
}

func TestSetupTracer(t *testing.T) {
	t.Log(`Verify that we don't panic when calling setupTracer.

This covers a regression where we didn't provide enough OpenTelemetry setup
in overrideConfigLoaders which caused setupTracer to panic.

Typically we should try to test the public interfaces (i.e. HookInUrfaveCLI),
but that causes the actual CLI to be executed (it ends up calling app.RunContext),
which is trickier to test.

Hence, we are calling private functions in the test, 
which are more prone to change over time. 
Since it's a simple test, the tradeoff seems reasonable.`)
	overrideConfigLoaders("", "", false)
	ctx := context.Background()
	setupTracer(ctx, t.Name())
}

func TestGenerateShellCompletion(t *testing.T) {
	// urfave requires arguments be set in `os.Args`. Capture the current value
	// and restore at the end of the test.
	startingArgs := os.Args
	defer func() {
		os.Args = startingArgs
	}()

	for _, fixture := range []struct {
		args                []string
		expectsErr          bool
		expectedOutputRegex string
	}{
		// Ensure we get the --skip-update flag suggested for both flag
		// generations.
		{[]string{"-", "--generate-bash-completion"}, false, "--skip-update"},
		{[]string{"--generate-fish-completion"}, false, "-l skip-update"},
		// Ensure we get the boolean flag we created.
		{[]string{"--t", "--generate-bash-completion"}, false, "--test-flag"},
		{[]string{"--generate-fish-completion"}, false, "-l test-flag"},
		// This should return an error, since the last flag isn't a completion request flag.
		{[]string{"--test-flag"}, true, ""},
	} {
		var sb strings.Builder
		app := cli.NewApp()
		app.Name = "test-app"
		app.Flags = []cli.Flag{
			&cli.BoolFlag{Name: "test-flag", Usage: "Flips the flag"},
		}
		app.EnableBashCompletion = true
		app.Writer = &sb
		ctx := context.Background()

		fullArgs := []string{"test-app"}
		fullArgs = append(fullArgs, fixture.args...)
		lastArg := fixture.args[len(fixture.args)-1]
		os.Args = fullArgs

		err := generateShellCompletion(ctx, app, fullArgs)
		if err != nil != fixture.expectsErr {
			t.Errorf(
				"expected err != nil to be %t, got %t (err=%s) for lastArg=%s", fixture.expectsErr, err == nil, err, lastArg,
			)
		}
		if matched, err := regexp.MatchString(fixture.expectedOutputRegex, sb.String()); err != nil {
			t.Errorf("bad regular expression %s", fixture.expectedOutputRegex)
		} else if !matched {
			t.Errorf("expected string %s; got '%s' for lastArg=%s", fixture.expectedOutputRegex, sb.String(), lastArg)
		}
	}
}
