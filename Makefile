SHELL := /bin/bash

GOFLAGS    ?= -trimpath
LDFLAGS    ?= -s -w -X main.version=$(shell cat version.txt 2>/dev/null || echo dev) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BIN_NAME   := macontrol
BIN_DIR    := dist
TARGET_OS  := darwin
TARGET_ARCH:= arm64

.PHONY: all help build build-local run test test-race lint lint-fix vet vuln fmt \
        tidy clean install-tools snapshot release-dry doctor version

all: lint test build ## Run the whole pipeline locally

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# --- build ---------------------------------------------------------------

build: ## Cross-compile for darwin/arm64 (from any host)
	@mkdir -p $(BIN_DIR)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/$(BIN_NAME) ./cmd/macontrol

build-local: ## Build for the host OS/arch (developer convenience)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/$(BIN_NAME) ./cmd/macontrol

run: ## Run the daemon locally with .env in CWD
	go run ./cmd/macontrol

# --- quality -------------------------------------------------------------

test: ## Run tests (no race, fast)
	go test ./...

test-race: ## Run tests with -race
	go test -race -coverprofile=coverage.out ./...

cover: test-race ## Generate HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "→ open coverage.html"

cover-floor: test-race ## Enforce the per-package coverage floor (.testcoverage.yml)
	go-test-coverage --config=./.testcoverage.yml

lint: ## Run golangci-lint
	golangci-lint run

lint-fix: ## Run golangci-lint with --fix
	golangci-lint run --fix

vet: ## go vet
	go vet ./...

vuln: ## govulncheck
	govulncheck ./...

fmt: ## gofumpt -l -w .
	gofumpt -l -w .

tidy: ## go mod tidy
	go mod tidy

# --- release / tooling ---------------------------------------------------

snapshot: ## GoReleaser snapshot build (no publishing)
	goreleaser release --snapshot --clean

release-dry: ## GoReleaser release --skip=publish (full pipeline, no upload)
	goreleaser release --clean --skip=publish

install-tools: ## Install dev tools (golangci-lint, gofumpt, govulncheck, goreleaser)
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install github.com/goreleaser/goreleaser/v2@latest

# --- misc ----------------------------------------------------------------

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.* dist/

version: ## Print the version that would be embedded
	@echo "$(shell cat version.txt 2>/dev/null || echo dev) ($(shell git rev-parse --short HEAD 2>/dev/null || echo none))"
