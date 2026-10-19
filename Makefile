# Recipes need bash (for pipefail) and grep, diff, cp and mv. On Windows
# that is Git's bash, and Git's usr/bin goes on PATH because Git bash does
# not put its coreutils there when make starts it from PowerShell. Set
# GIT_BASH if Git is installed elsewhere. Every other host uses /bin/bash.
ifeq ($(OS),Windows_NT)
  GIT_BASH ?= C:/PROGRA~1/Git/usr/bin/bash.exe
  SHELL := $(GIT_BASH)
  export PATH := $(dir $(GIT_BASH)):$(PATH)
else
  SHELL := /bin/bash
endif
.SHELLFLAGS := -eu -o pipefail -c

.PHONY: check-records markdownlint

check-records:
	python3 scripts/check_records.py --check-all

markdownlint:
	npx --yes markdownlint-cli2@0.23.2

ACP_VERSION := $(file < ./schema/version)
GOCACHE ?= $(CURDIR)/.gocache
MDSH ?= mdsh
GOPATH_BIN := $(shell go env GOPATH)/bin
GOLANGCI_LINT ?= $(GOPATH_BIN)/golangci-lint
GOVULNCHECK_PIN := $(shell grep -oE 'golang.org/x/vuln/cmd/govulncheck@v[0-9][0-9.]*' .github/workflows/ci.yml | head -1)
FLEET_LINT_CFG := .golangci.yml

# On Go 1.26 the root tests' leak guard needs the goroutineleak profile, an
# experiment there that Go 1.27 rejects, so the root's go test commands set
# it on 1.26 only. Tools built with Go 1.27, such as golangci-lint, reject it,
# so nothing else does. go env GOVERSION names the toolchain go.mod's
# toolchain line selects (0024-PLAN-go-1-26-9-floor.md, G5).
comma := ,
LEAKGUARD_ENV :=
ifneq ($(filter go1.26%,$(shell go env GOVERSION)),)
  ifeq ($(findstring goroutineleakprofile,$(GOEXPERIMENT)),)
    LEAKGUARD_ENV := GOEXPERIMENT=$(if $(GOEXPERIMENT),$(GOEXPERIMENT)$(comma))goroutineleakprofile
  endif
endif

.PHONY: all help test fmt vet lint tidy vuln pre-add-check push-guard

all: help

test: ## Runs all tests
	GOCACHE=$(GOCACHE) $(LEAKGUARD_ENV) go test ./...
	cd cmd/generate && GOCACHE=$(GOCACHE) go test ./...
	GOCACHE=$(GOCACHE) go build ./example/...

fmt: ## Formats all Go source files
	gofmt -w .
	gofmt -w ./cmd/generate

vet: ## Runs go vet
	go vet ./...
	cd cmd/generate && go vet ./...

# Lint the host and Windows. _windows.go files are invisible to a host-only run.
lint: ## Runs golangci-lint for the host and for GOOS=windows
	@if [ ! -x "$(GOLANGCI_LINT)" ]; then \
		echo "golangci-lint not found at $(GOLANGCI_LINT)"; \
		echo "Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"; \
		exit 1; \
	fi
	@status=0; \
	for os in "$$(go env GOOS)" windows; do \
		echo "golangci-lint (GOOS=$$os)"; \
		GOOS=$$os $(GOLANGCI_LINT) run -c $(FLEET_LINT_CFG) ./... || status=1; \
		echo "golangci-lint (GOOS=$$os cmd/generate)"; \
		( cd cmd/generate && GOOS=$$os $(GOLANGCI_LINT) run -c ../../$(FLEET_LINT_CFG) ./... ) || status=1; \
	done; \
	exit $$status

tidy: ## Runs go mod tidy
	go mod tidy
	cd cmd/generate && go mod tidy

vuln: ## Runs govulncheck at the version pinned in CI
	@if [ -z "$(GOVULNCHECK_PIN)" ]; then \
		echo "no govulncheck pin in .github/workflows/ci.yml"; \
		exit 1; \
	fi
	go run $(GOVULNCHECK_PIN) ./...
	cd cmd/generate && go run $(GOVULNCHECK_PIN) ./...

# A crasher is written under the package's testdata/fuzz/, and is committed
# with its fix (0011-PLAN-engine-tests.md, D4).
FUZZTIME ?= 60s
.PHONY: fuzz
fuzz: ## Runs each fuzz target for FUZZTIME (default 60s)
	$(LEAKGUARD_ENV) go test -run '^$$' -fuzz '^FuzzRoundTrip$$' -fuzztime $(FUZZTIME) .
	$(LEAKGUARD_ENV) go test -run '^$$' -fuzz '^FuzzCanonicalIDKey$$' -fuzztime $(FUZZTIME) .
	cd cmd/generate && go test -run '^$$' -fuzz '^FuzzBuild$$' -fuzztime $(FUZZTIME) .
	cd cmd/generate && go test -run '^$$' -fuzz '^FuzzExportedName$$' -fuzztime $(FUZZTIME) ./internal/model

# Benchmarks report and never gate; CI does not run them. A step that changes
# a hot path records the medians of these six runs before and after
# (0019-PLAN-idioms-performance.md, D13).
.PHONY: bench
bench: ## Runs the root package's benchmarks six times, with allocations
	$(LEAKGUARD_ENV) go test -run '^$$' -bench . -benchmem -count=6 .

# Mutation runs report and never gate (0011-PLAN-engine-tests.md, D5). MUTANTS
# samples that many mutants with SEED; empty runs every one. JOBS runs that many
# at once. The engine run skips TestGeneratedFilesUpToDate, so a mutant only
# that byte comparison kills survives (D6).
MUTANTS ?=
SEED ?= 1
JOBS ?= 1
MUTATE := go run ./internal/mutate -seed $(SEED) -j $(JOBS) $(if $(MUTANTS),-n $(MUTANTS))
.PHONY: mutation-engine mutation-generated
mutation-engine: ## Runs mutants of the generator's engine against its tests
	cd cmd/generate && $(MUTATE) -dir . -skip "^TestGeneratedFilesUpToDate$$" -files 'internal/load/*.go,internal/model/*.go,internal/emit/*.go,internal/output/*.go'

mutation-generated: ## Runs mutants of the generated SDK files against the root's tests
	cd cmd/generate && $(LEAKGUARD_ENV) $(MUTATE) -dir ../.. -pkgs . -files 'types_gen.go,agent_gen.go,client_gen.go,helpers_gen.go'

FILES ?=
pre-add-check: ## Runs the pre-add checks
	@./scripts/go-precheck.sh $(FILES)

push-guard: ## Runs the pre-push disclosure guard over the commits a push of main to origin would send
	python3 scripts/push_guard.py

help: ## Displays this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

version: README.md schema/meta.json schema/schema.json schema/meta.unstable.json schema/schema.unstable.json
	cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .
	gofmt -w .
	gofmt -w ./cmd/generate
	touch $@
	echo $(ACP_VERSION) > $@

schema/meta.json: schema/version
	curl -o $@ --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/schema-v$(ACP_VERSION)/meta.json

schema/schema.json: schema/version
	curl -o $@ --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/schema-v$(ACP_VERSION)/schema.json

schema/meta.unstable.json: schema/version
	@set -e; \
		url=https://github.com/agentclientprotocol/agent-client-protocol/releases/download/schema-v$(ACP_VERSION)/meta.unstable.json; \
		tmp=$@.tmp; \
		status=$$(curl -sS -L -o "$$tmp" -w '%{http_code}' "$$url") || { rm -f "$$tmp"; exit 1; }; \
		if [ "$$status" = "200" ]; then \
			mv "$$tmp" $@; \
		elif [ "$$status" = "404" ]; then \
			rm -f "$$tmp"; \
			printf '%s\n' '{"agentMethods":{},"clientMethods":{},"protocolMethods":{}}' > $@; \
		else \
			rm -f "$$tmp"; \
			echo "failed to download $$url (http $$status)" 1>&2; \
			exit 1; \
		fi

schema/schema.unstable.json: schema/version
	@set -e; \
		url=https://github.com/agentclientprotocol/agent-client-protocol/releases/download/schema-v$(ACP_VERSION)/schema.unstable.json; \
		tmp=$@.tmp; \
		status=$$(curl -sS -L -o "$$tmp" -w '%{http_code}' "$$url") || { rm -f "$$tmp"; exit 1; }; \
		if [ "$$status" = "200" ]; then \
			mv "$$tmp" $@; \
		elif [ "$$status" = "404" ]; then \
			rm -f "$$tmp"; \
			printf '%s\n' '{"$$defs":{}}' > $@; \
		else \
			rm -f "$$tmp"; \
			echo "failed to download $$url (http $$status)" 1>&2; \
			exit 1; \
		fi

README.md: schema/version
	@command -v $(MDSH) >/dev/null || { echo "mdsh not found" 1>&2; exit 1; }
	$(MDSH) --input README.md

.PHONY: clean
clean:
	rm -f schema/meta.json schema/schema.json schema/meta.unstable.json schema/schema.unstable.json version
	mdsh --clean --input README.md
	touch schema/version # Touching the schema version file ensures that the README.md is regenerated on next make.

.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (e.g. 'make release VERSION=0.4.4')" 1>&2; \
		exit 1; \
	fi
	@printf '%s\n' "$(VERSION)" > schema/version
	$(MAKE) version
	$(MAKE) fmt
	GOCACHE=$(GOCACHE) $(MAKE) test
	$(MAKE) pre-add-check
	@cmp -s schema/version version || (echo "schema/version and version differ; rerun 'make version'" 1>&2; exit 1)
	@echo
	@echo "Schema $(VERSION) is ready. Review the changes and commit; the module's tag follows RELEASING.md, not the schema version."
