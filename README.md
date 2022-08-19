# gobox
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/getoutreach/gobox)
[![Generated via Bootstrap](https://img.shields.io/badge/Outreach-Bootstrap-%235951ff)](https://github.com/getoutreach/bootstrap)
[![Coverage Status](https://coveralls.io/repos/github/getoutreach/gobox/badge.svg?branch=main)](https://coveralls.io/github//getoutreach/gobox?branch=main)

A collection of libraries that are useful for implementing Go services, libraries, and more.

## Contributing

Please read the [CONTRIBUTING.md](CONTRIBUTING.md) document for guidelines on developing and contributing changes.

## High-level Overview

<!--- Block(overview) -->

Please see individual packages in the generated documentation for overviews on each.

## Go idioms

### Standard idioms

Please see [Code review
comments](https://github.com/golang/go/wiki/CodeReviewComments),
[go proverbs](https://go-proverbs.github.io/) and
[Idiomatic Go](https://dmitri.shuralyov.com/idiomatic-go).

### Log errors with events.NewErrorInfo

When logging errors, use `log.Debug(ctx, "some debug event", events.NewErrorInfo(err))` instead of using `log.F{"error": err}`. [NewErrorInfo](https://github.com/getoutreach/gobox/blob/master/docs/events.md) logs errors using outreach naming conventions and also takes care of logging stack traces.

### Do not use context.WithValue

Context is often abused for thread local state. There are very few legitimate uses for this ([tracing](https://github.com/getoutreach/gobox/blob/master/docs/trace.md) is one of those).

### Do not use fmt.PrintXXX or the standard log package

Prefer the [gobox log](https://github.com/getoutreach/gobox/blob/master/docs/log.md) package. This logs data in structured format suitable for outreach go services.

### Do not use non-literal messages with log

Do not use the following pattern:

```golang
   message := fmt.Sprintf("working on org: %s", model.Org.ShortName)
   log.Info(ctx, message, modelInfo)
```

The first arg of `log.XXX` calls should be a literal string so we can
quickly find out where a log message comes from. The rest of the args
can hold any structured data we want. The
[events](https://github.com/getoutreach/gobox/blob/master/docs/events.md)
package exposes a few common logging structures.

### Use code generation for stringifying enums

See [go generate](https://blog.golang.org/generate):

For example, given this snippet,

```golang
package painkiller

//go:generate ./scripts/gobin.sh golang.org/x/tools/cmd/stringer@v0.1.0 -type=Pill
type Pill int

const (
  Placebo Pill = iota
  Aspirin
  Ibuprofen
  Paracetamol
  Acetaminophen = Paracetamol
)
```

running `go generate ./...` from the root of the repo will create the
file pill_string.go, in package painkiller, containing a definition of
`func (Pill) String() string` which can be used to get the string
representation of the enum.

A suggested workflow is to run `go generate ./...` from the root of the repo before sending PRs out.

### Use events.Org for logging org tenancy info.

Org information can be logged with standard naming conventions using:

```golang
   orgInfo := events.Org{Bento: xyz, DatabaseHost: ...}
   log.Debug(ctx, "doing xyz", orgInfo)
```

In most cases, though you probably have some other model struct which
has this info. In those cases, the preferred route is to make those
model types themselves loggable:

```golang
type Model struct {...}

func (m *Model) MarshalLog(addField func(key string, value interface{}) {
     m.Org().MarshalLog(addField)
     ... add any custom fields you want: addField("myCustomField", m.CustomInfo)...
}

func (m *Model) Org() events.Org {
     return events.Org{...}
}
```

Now `Model` can be used in logs like so:

```golang
   var myModel m
   log.Debug(ctx, "doing xyz", myModel)
```

Better still is to [generate the MarshalLog function using struct tags](https://github.com/getoutreach/gobox/blob/main/tools/logger/generating.md)

<!--- EndBlock(overview) -->
