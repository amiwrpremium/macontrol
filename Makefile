SHELL := /bin/bash

GOFLAGS    ?= -trimpath
VERSION_PKG := github.com/amiwrpremium/macontrol/internal/version
LDFLAGS    ?= -s -w -X $(VERSION_PKG).Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none) -X $(VERSION_PKG).Date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BIN_NAME   := macontrol
BIN_DIR    := dist
TARGET_OS  := darwin
TARGET_ARCH:= arm64

.PHONY: all help build build-local run test test-race lint lint-extra lint-fix vet vuln fmt \
        tidy clean install-tools tools hooks hooks-uninstall hooks-run-pre-commit \
        hooks-run-pre-push snapshot release-dry doctor version

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

run: ## Run the daemon locally (reads token + whitelist from your Keychain)
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

lint-extra: ## Run the non-Go linters (markdownlint, yamllint, actionlint, editorconfig-checker, typos); skips any tool not installed
	@echo "==> markdownlint"
	@if command -v markdownlint-cli2 >/dev/null 2>&1; then \
	  markdownlint-cli2 '**/*.md' '#node_modules' '#vendor' '#.git'; \
	else \
	  echo "  (skip: install via 'npm i -g markdownlint-cli2')"; \
	fi
	@echo "==> yamllint"
	@if ! command -v yamllint >/dev/null 2>&1; then \
	  echo "  (skip: install via 'pip install yamllint' or 'brew install yamllint')"; \
	elif [ ! -f .yamllint.yml ]; then \
	  echo "  (skip: .yamllint.yml not present; tool defaults don't match this repo)"; \
	else \
	  yamllint -c .yamllint.yml -s .; \
	fi
	@echo "==> actionlint"
	@if command -v actionlint >/dev/null 2>&1; then \
	  actionlint; \
	else \
	  echo "  (skip: install via 'make tools')"; \
	fi
	@echo "==> editorconfig-checker"
	@if ! command -v editorconfig-checker >/dev/null 2>&1; then \
	  echo "  (skip: install via 'make tools')"; \
	elif [ ! -f .editorconfig-checker.json ]; then \
	  echo "  (skip: .editorconfig-checker.json not present; Go raw-string-literal contents trip the default rules)"; \
	else \
	  editorconfig-checker; \
	fi
	@echo "==> typos"
	@if ! command -v typos >/dev/null 2>&1; then \
	  echo "  (skip: install via 'cargo install typos-cli' or 'brew install typos-cli')"; \
	elif [ ! -f .typos.toml ]; then \
	  echo "  (skip: .typos.toml not present; tool defaults flag too many false positives without an allowlist)"; \
	else \
	  typos; \
	fi

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

tools: install-tools ## Install dev tools used by lint / hooks / lint-extra (superset of install-tools)
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/evilmartians/lefthook@latest
	go install github.com/rhysd/actionlint/cmd/actionlint@latest
	go install github.com/editorconfig-checker/editorconfig-checker/v3/cmd/editorconfig-checker@latest
	@command -v markdownlint-cli2 >/dev/null 2>&1 || echo "Note: markdownlint-cli2 needs Node — install via 'npm i -g markdownlint-cli2'"
	@command -v yamllint           >/dev/null 2>&1 || echo "Note: yamllint needs Python — install via 'pip install --user yamllint' or 'brew install yamllint'"
	@command -v typos              >/dev/null 2>&1 || echo "Note: typos needs Rust or brew — install via 'cargo install typos-cli' or 'brew install typos-cli'"

hooks: ## Install git hooks (pre-commit, commit-msg, pre-push) into .git/hooks from lefthook.yml
	@if ! command -v lefthook >/dev/null 2>&1; then echo "lefthook not on PATH. Run: make tools"; exit 1; fi
	lefthook install

hooks-uninstall: ## Remove git hooks installed by `make hooks`
	@command -v lefthook >/dev/null 2>&1 && lefthook uninstall || true

hooks-run-pre-commit: ## Run the pre-commit hook ad-hoc (without committing)
	lefthook run pre-commit

hooks-run-pre-push: ## Run the pre-push hook ad-hoc (without pushing)
	lefthook run pre-push

# --- misc ----------------------------------------------------------------

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.* dist/

version: ## Print the version that would be embedded
	@echo "$(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev) ($(shell git rev-parse --short HEAD 2>/dev/null || echo none))"
