# gobox

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/getoutreach/gobox)
[![CircleCI](https://circleci.com/gh/getoutreach/gobox.svg?style=shield&circle-token=<YOUR_STATUS_API_TOKEN:READ:https://circleci.com/docs/2.0/status-badges/>)](https://circleci.com/gh/getoutreach/gobox)
[![Generated via Bootstrap](https://img.shields.io/badge/Outreach-Bootstrap-%235951ff)](https://github.com/getoutreach/bootstrap)

<!--- Block(description) -->
`gobox` is a collection of libraries that are useful for implementing Go services, libraries, and more.
<!--- EndBlock(description) -->

## Contents
1. [Go idioms](#go-idioms)
    1. [Standard idioms](#standard-idioms)
    2. [Log errors with events.NewErrorInfo](#log-errors-with-events-newerrorinfo)
    3. [Do not use context.WithValue](#do-not-use-context-withvalue)
    4. [Do not use fmt.PrintXXX or the standard log package](#do-not-use-fmt-printxxx-or-the-standard-log-package)
    5. [Do not use non-literal messages with log](#do-not-use-non-literal-messages-with-log)
    6. [Use code generation for stringifying enums](#use-code-generation-for-stringifying-enums)
    7. [Use events.Org for logging org tenancy info.](#use-events-org-for-logging-org-tenancy-info)
2. [Prerequisites](#prerequisites)
3. [Installing Go](#installing-go)
4. [Building and testing gobox](#building-and-testing-gobox)
5. [Documentation](#documentation)
6. [Examples](#examples)

## Go idioms

### Standard idioms

Please see [Code review
comments](https://github.com/golang/go/wiki/CodeReviewComments),
[go proverbs](https://go-proverbs.github.io/) and
[Idiomatic Go](https://dmitri.shuralyov.com/idiomatic-go).

### Log errors with events.NewErrorInfo

When logging errors, use `log.Debug(ctx, "some debug event", events.NewErrorInfo(err))` instead of using `log.F{"error": err}`.  [NewErrorInfo](https://github.com/getoutreach/gobox/blob/master/docs/events.md) logs errors using outreach naming conventions and also takes care of logging stack traces.

### Do not use context.WithValue

Context is often abused for thread local state.  There are very few legitimate uses for this ([tracing](https://github.com/getoutreach/gobox/blob/master/docs/trace.md) is one of those).

### Do not use fmt.PrintXXX or the standard log package

Prefer the [gobox log](https://github.com/getoutreach/gobox/blob/master/docs/log.md) package. This logs data in structured format suitable for outreach go services.

### Do not use non-literal messages with log

Do not use the following pattern:

```golang
   message := fmt.Sprintf("working on org: %s", model.Org.ShortName)
   log.Info(ctx, message, modelInfo)
```

The first arg of `log.XXX` calls should be a literal string so we can
quickly find out where a log message comes from.  The rest of the args
can hold any structured data we want.  The
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

## Prerequisites

* Golang >= 1.13

## Installing Go

This project uses [Golang Modules](https://blog.golang.org/using-go-modules)

To install Golang on Linux or OSX, I strongly recommend using the asdf package manager

Once you have installed asdf, make sure to install the Golang plugins with

```bash
asdf plugin-add golang
```

Then install the actual version

```bash
asdf install golang 1.13
```

Set your default version for Golang

```bash
asdf global golang 1.13
```

## Building and testing gobox

Client-side linting is via
[Golangci-lint](https://github.com/golangci/golangci-lint).  The
following script runs lint as well as all the tests:

```bash
$ ./scripts/test.sh
```

## Documentation

The authoritative documentation is best done by using `godoc -http=localhost:6060` and then browsing to [it locally](http://localhost:6060/pkg/github.com/getoutreach/gobox/)

The markdown version of it can be generated by running [godocdown](https://github.com/robertkrimen/godocdown)

```bash
$ scripts/docs.sh
```

Note that the markdown version does not show code examples.

## Examples

All the individual packages come with examples, so the local
documentation link is a good place to start.

This repository also contains an [example
service](https://github.com/getoutreach/gobox/tree/master/pkg/example)
handler as well the runnable
[cmd](https://github.com/getoutreach/gobox/tree/master/cmd/example)
version of it that are intended to showcase the core functionality.

.
