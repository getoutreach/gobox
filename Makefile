# go option
GO                  ?= go
GOFMT               ?= gofmt
CLANG_FORMAT        ?= ./scripts/clang-format.sh
JSONNETFMT          ?= ./scripts/gobin.sh github.com/google/go-jsonnet/cmd/jsonnetfmt@v0.16.0
SHELL               := /usr/bin/env bash
GOOS                ?= $(shell go env GOOS)
GOARCH              ?= $(shell go env GOARCH)
PKG                 := $(GO) mod download -x
APP_VERSION         := $(shell git describe --match 'v[0-9]*' --tags --always HEAD)
LDFLAGS             := -w -s -X github.com/getoutreach/gobox/pkg/app.Version=$(APP_VERSION) -X main.HoneycombTracingKey=$(shell cat ~/.outreach/gobox/honeycomb/apiKey)
GOFLAGS             :=
LOG                 := "$(CURDIR)/scripts/lib/logging.sh"
GOPRIVATE           := github.com/getoutreach/*
GOPROXY             := https://proxy.golang.org
GO_EXTRA_FLAGS      := -v -tags=or_dev
TAGS                :=
BINDIR              := $(CURDIR)/bin
BIN_NAME            := gobox
PKGDIR              := github.com/getoutreach/gobox
CGO_ENABLED         ?= 1
TOOL_DEPS           := ${GO}
BENCH_FLAGS         := "-bench=Bench $(BENCH_FLAGS)"
TEST_TAGS           ?= or_test,or_int
SKIP_VALIDATE       ?=
FLY                 ?= $(shell ./scripts/gobin.sh -p github.com/concourse/concourse/fly@cfe7746ae74247743708be6c5b2f40215030a1f1)
E2E_ARGS            ?=  
E2E_NAMESPACE       ?= gobox--bento1a
E2E_SERVICE_ACCOUNT ?= gobox-e2e-client-svc
OUTREACH_DOMAIN     ?= outreach-dev.com
ACCOUNTS_URL        ?= https://accounts.$(OUTREACH_DOMAIN)
BASE_TEST_ENV       ?= GOPROXY=$(GOPROXY) GOPRIVATE=$(GOPRIVATE) OUTREACH_ACCOUNTS_BASE_URL=$(ACCOUNTS_URL) SKIP_VALIDATE=${SKIP_VALIDATE}


.PHONY: default
default: build

## help             show this help
.PHONY : help
help: Makefile
	@printf "\n[running make with no target runs make build]\n\n"
	@sed -n 's/^##[^#]//p' $<

## check-deps:      check for required dependencies
.PHONY: check-deps
check-deps:
	@[[ ! -z "${SKIP_CHECK_DEPS}" ]] || ./scripts/check_deps.sh ${TOOL_DEPS}

## pre-commit:      run housekeeping utilities before creating a commit
.PHONY: pre-commit
pre-commit: fmt

## build:           run codegen and build application binary
.PHONY: build
build: gobuild

## test:            run unit tests
.PHONY: test
test:
	$(BASE_TEST_ENV) ./scripts/test.sh

## coverage:        generate code coverage
.PHONY: coverage
coverage:
	 WITH_COVERAGE=true GOPROXY=$(GOPROXY) GOPRIVATE=$(GOPRIVATE) ./scripts/test.sh
	 go tool cover --html=/tmp/coverage.out

## integration:     run integration tests
.PHONY: integration
integration:
	TEST_TAGS=${TEST_TAGS} $(BASE_TEST_ENV) ./scripts/test.sh

## e2e:             run e2e tests for gobox
.PHONY: e2e
e2e:
	@devenv --skip-update status -q || \
		(echo "Starting developer environment"; set -x; devenv --skip-update provision ${E2E_ARGS})
	TEST_TAGS=or_test,or_e2e $(BASE_TEST_ENV) MY_NAMESPACE=$(E2E_NAMESPACE) MY_POD_SERVICE_ACCOUNT=$(E2E_SERVICE_ACCOUNT) OUTREACH_DOMAIN=$(OUTREACH_DOMAIN) ./scripts/test.sh

## benchmark:       run benchmarks
.PHONY: benchmark
benchmark:
	BENCH_FLAGS=${BENCH_FLAGS} TEST_TAGS=${TEST_TAGS} $(BASE_TEST_ENV) ./scripts/test.sh | tee /tmp/benchmark.txt
	@$(LOG) info "Results of benchmarks: "
	./scripts/gobin.sh golang.org/x/perf/cmd/benchstat /tmp/benchmark.txt

## dep:             download go dependencies
.PHONY: dep
dep:
	@$(LOG) info "Installing dependencies via '$(PKG)'"
	GOPROXY=$(GOPROXY) GOPRIVATE=$(GOPRIVATE) $(PKG)

## gogenerate:      run go codegen
.PHONY: gogenerate
gogenerate: check-deps
	@$(LOG) info "Running gogenerate"
	@GOPROXY=$(GOPROXY) GOPRIVATE=$(GOPRIVATE) $(GO) generate ./...

## gobuild:         build application binary
.PHONY: gobuild
gobuild: check-deps
	@$(LOG) info "Building binaries into ./bin/"
	mkdir -p $(BINDIR)
	GOPROXY=$(GOPROXY) GOPRIVATE=$(GOPRIVATE) CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(BINDIR)/ -ldflags "$(LDFLAGS)" $(GO_EXTRA_FLAGS) $(PKGDIR)/...

## fmt:             run source code formatters
.PHONY: fmt
fmt:
	@./scripts/fmt.sh

.PHONY: version
version:
	@echo "$(APP_VERSION)"

###Block(targets)
###EndBlock(targets)
