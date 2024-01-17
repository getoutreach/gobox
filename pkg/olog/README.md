# olog

```go
import "github/getoutreach/gobox/pkg/olog"
```

Package olog implements a lightweight logging library built around the [`slog`](https://pkg.go.dev/log/slog) package. It aims to never mask the core slog.Logger type by default. Provided is a global system for controlling logging levels based on the package and module that a logger was created in, with a system to update the logging level at runtime.

This package does not provide the ability to ship logs to a remote server, instead a logging collector should be used.

## Index

- [Usage](<#usage>)
- [func New() *slog.Logger](<#func-new>)
- [func NewWithHandler(h slog.Handler) *slog.Logger](<#func-newwithhandler>)
- [func SetDefaultHandler(ht DefaultHandlerType)](<#func-setdefaulthandler>)
- [func SetGlobalLevel(l slog.Level)](<#func-setgloballevel>)
- [type DefaultHandlerType](<#type-defaulthandlertype>)

## Usage

**Setup Package Level Logger** - *primary intended usage*

```go
import (
    "log/slog"

    "github.com/getoutreach/gobox/pkg/olog"
)

// logger - a package level singleton *slog.Logger instance.
//
// Uses a combination of built-in slog and olog functionality to
// provide a standard, structured logging interface and implementation.
// *slog.Logger instance are concurrency safe.
var logger *slog.Logger = getPackageLogger()

func getPackageLogger() *slog.Logger {
    // If package logger already exists, return it instead of creating
    // a new instance. This helps guard against unintentional or unnecessary
    // calls.
    if logger != nil {
        return logger
    }

    // Set logging level for entire logging package. This log level affects
    // only and all loggers created using the olog package. Refer to the
    // slog docs here https://pkg.go.dev/log/slog#Level for valid log levels.
    //
    // The default log level is `LevelInfo`.
    olog.SetGlobalLevel(slog.LevelInfo)

    // Return default logger provided by the olog package.
    return olog.New()
}

```

**Create Inline Logger Instance**

```go
import (
    "github.com/getoutreach/gobox/pkg/olog"
)

func helloWorld() {
    ...

    olog.New().Info("Hello World!")
}
```

**Creating Logger with Attributes**

```go
import (
    "context"
    "log/slog"

    "github.com/getoutreach/gobox/pkg/olog"
    "github.com/getoutreach/gobox/pkg/trace"
)

func doSomething(ctx context.Context) {
    logger := getLoggerWithContext(ctx)

    ...

    logger.Info("Do Something!")
}

func getLoggerWithContext(ctx context.Context) {
    traceId := trace.ID(ctx)
    // Extract any other variables from context

    // Create a logger instance which will always carry the provided
    // args as attributes on all log records created with this logger.
    return olog.New().With(slog.String("traceId", traceId))
}
```

*outputs*

```bash
>
{"msg": "Do Something!", "traceId": "<trace-id>", ...}
```

**Using the Logger**

```go
import (
    "context"
    "log/slog"
    "time"

    "github.com/getoutreach/gobox/pkg/olog"
)

// Create package level default logger.
var logger = olog.New()

// The olog package returns an instance of the standard
// golang slog.Logger struct and all usage is identical
// to what is found in that package. Please check the
// docs here to learn more: https://pkg.go.dev/log/slog
func logEveryLevel(ctx context.Context) {
    // The logger comes with methods for the 4 most common
    // log levels: debug, info, warn, and error.
    logger.Debug("This is a debug log") // Will not be output with the default log level configuration.
    logger.Info("This is an info log")
    logger.Warn("This is a warning log")
    logger.Error("This is an error log")

    // Each of these levels also have methods which accept
    // a context. This allows any data on the context to
    // be passed to the log record as well if used by the
    // logger's handler (not used by default).
    logger.InfoContext(ctx, "This is an info log with context")

    // You can also use the `Log` method to provide any
    // custom log level you wish. The slog.Level is just
    // an alias for an int, so any number will work. The
    // higher the number, the higher the logs priority
    // (debug = -1, info = 0, warn = 4, error = 8).
    traceLevel := slog.Level(-2)
    logger.Log(ctx, traceLevel, "This is a trace log")

    // Finally, in order to provide structured data to these
    // logs, you may provide any number of arguments to the
    // above methods (at the end of the call) and they will
    // be added to the log record as key:value's. See the
    // the docs here for more info: https://pkg.go.dev/log/slog

    // This is how to pass args using the alternating arg
    // approach. The first arg provided as part of the
    // variadic signature will be used as a key if it is
    // a string, and the following arg will be used as
    // it's value (if present).
    logger.Info("This log has data", "key", "value")
    logger.Debug("This log has data", "count", 5)

    // While the alternating arg approach may convenient in
    // some cases, most of the time it will be easier, or
    // more intuitive to use the Attr helper funcs in the
    // slog package found here: https://pkg.go.dev/log/slog#Attr
    logger.Info(
        "This log has many types of args",
        slog.Any("AnyValue", "This could be anything"),
        slog.Bool("BoolValue", true),
        slog.Duration("DurationValue", time.Second*5),
        slog.Float64("FloatValue", 3.14),
        slog.Int("IntValue", 42),
        slog.String("StringValue", "foobar"),
        slog.Time("TimeValue", time.Now()),
    )

    // Finally, you can pass any type as an attr value
    // as long as it implements the LogValuer interface
    // defined here: https://pkg.go.dev/log/slog#LogValuer
}

```

**Changing the Default Handler Type**

```go
import (
    "log/slog"

    "github.com/getoutreach/gobox/pkg/olog"
)

var logger *slog.Logger

func init() {
    // The olog package comes with 2 different handler
    // options out of the box: JSON and TEXT. By default,
    // the JSON handler is used for all loggers, unless
    // stdOut is a TTY (local execution environment).
    // But this can be manually changed using the following
    // func in the olog package:
    olog.SetDefaultHandler(olog.TextHandler)

    // Set the package level logger using the new default
    // handler type.
    logger := olog.New()
}

```

## Logger Hooks

To provide a mechanism with which to automatically add attributes to all logs (with access to context), a sub-package named `olog/hooks` has been provided. This package exposes a new `Logger` func which wraps the handler provided by the `olog` package and allows for hook functions to be provided by the caller which may return any number of `slog` attributes which will then be added to the final log record before it is written.

```go
import (
    "context"
    "log/slog"

    "github.com/getoutreach/gobox/pkg/olog/hooks"
    "github.com/getoutreach/gobox/pkg/trace"
)

var logger *slog.Logger

func init() {
    // Create custom hook func
    traceHook := hooks.LogHookFunc(func(ctx context.Context, r slog.Record) ([]slog.Attr, error) {
        return []slog.Attr{slog.String("traceId", trace.ID(ctx))}, nil
    })

    // Create a hooks logger with the provided AppInfo hook as well as
    // the custom traceHook, assigning to our package logger instance.
    logger = hooks.Logger(
        hooks.AppInfo,
        traceHook,
    )
}
```

## func [New](<https://github.com/getoutreach/gobox/blob/main/pkg/olog/olog.go#L39>)

```go
func New() *slog.Logger
```

New creates a new slog instance that can be used for logging. The provided logger use the global handler provided by this package. See the documentation on the 'handler' global for more information.

The logger will be automatically associated with the module and package that it was instantiated in. This is done by looking at the call stack.

Note: As mentioned above, this logger is associated with the module and package that created it. So, if you pass this logger to another module or package, the association will NOT be changed. This includes the caller metadata added to every log line as well as log\-level management. If a type has a common logging format that the other module or package should use, then a slog.LogValuer should be implemented on that type instead of passing a logger around. If trying to set attributes the be logged by default, this is not supported without retaining the original association.

## func [NewWithHandler](<https://github.com/getoutreach/gobox/blob/main/pkg/olog/olog.go#L114>)

```go
func NewWithHandler(h slog.Handler) *slog.Logger
```

NewWithHandler returns a new slog.Logger with the provided handler.

Note: A logger created with this function will not be controlled by the global log level and will not have any of the features provided by this package. This is primarily meant to be used only by tests or other special cases.

## func [SetDefaultHandler](<https://github.com/getoutreach/gobox/blob/main/pkg/olog/default_handler.go#L90>)

```go
func SetDefaultHandler(ht DefaultHandlerType)
```

SetDefaultHandler changes the default handler to be the provided type. This must be called before any loggers are created to have an effect on all loggers.

## func [SetGlobalLevel](<https://github.com/getoutreach/gobox/blob/main/pkg/olog/log_level.go#L65>)

```go
func SetGlobalLevel(l slog.Level)
```

SetGlobalLevel sets the global logging level used by all loggers by default that do not have a level set in the level registry. This impacts loggers that have previously been created as well as loggers that will be created in the future.

## type [DefaultHandlerType](<https://github.com/getoutreach/gobox/blob/main/pkg/olog/default_handler.go#L38>)

DefaultHandlerType denotes which handler should be used by default. This is calculated via the \`setDefaultHandler\` function on package init.

```go
type DefaultHandlerType int
```

```go
const (
    JSONHandler DefaultHandlerType = iota
    TextHandler
)
```
