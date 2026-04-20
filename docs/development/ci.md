# CI

GitHub Actions workflows that run on every push and PR. Live under
`.github/workflows/`.

## Workflows

### `ci.yml` — lint, test, build, vuln scan

Triggers: push to `master`, pull requests against `master`.

Jobs:

| Job | Runs on | Does |
|---|---|---|
| `lint` | ubuntu-latest | `golangci-lint run` (v2 binary, latest) |
| `test` (matrix) | ubuntu-latest, macos-14 | `go test -race -coverprofile=coverage.out ./...` |
| `build` | ubuntu-latest | `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/macontrol` |
| `vuln` | ubuntu-latest | `govulncheck ./...` |

The `test` matrix runs the same suite on Linux and macOS-on-arm64.
Linux catches the bulk of regressions cheaply; macos-14 catches
darwin-specific issues (rarely; almost everything goes through
`runner.Fake`).

Coverage is uploaded as an artifact from the ubuntu-latest run for
inspection. A non-blocking `Enforce coverage floor` step runs
`go-test-coverage --config=.testcoverage.yml` — currently
`continue-on-error: true`, will be flipped to blocking once history is
stable.

### `pr-title.yml` — Conventional Commits enforcement

Trigger: PR opened, edited, reopened, or synchronized.

Validates the PR title against the type/scope allowlist defined in
the workflow file (see [Conventional Commits](conventional-commits.md)).

Fails the PR if:

- Title doesn't start with a valid type (`feat`, `fix`, `perf`,
  `refactor`, `docs`, `test`, `build`, `ci`, `chore`, `revert`).
- Subject starts with a capital letter.
- Format is otherwise broken.

### `codeql.yml` — security scanning

Triggers: push to `master`, PR against `master`, weekly cron (Mondays
07:00 UTC).

Runs GitHub's CodeQL static analyzer against the Go source. Findings
appear in the repo's Security tab.

Most issues CodeQL finds are false positives for a Go project (CodeQL
was originally built for Java/JavaScript and is most accurate there).
We still run it because it occasionally catches real issues, and it's
free for public repos.

### `release-please.yml` — version management

Trigger: push to `master`.

Runs `googleapis/release-please-action`. Reads conventional-commit
messages since the last tag and:

- Computes the next version (semver bump based on commit types).
- Opens (or updates) a "chore(release): vX.Y.Z" PR with the version
  bump and CHANGELOG entry.

When the release PR is merged, release-please tags the new version,
which fires `release.yml`.

See [Releasing](releasing.md) for the full flow.

### `release.yml` — GoReleaser

Trigger: tag push matching `v*`.

Runs GoReleaser:

- Cross-compiles `darwin/arm64` with `-trimpath` and stripped symbols.
- Packages the binary + LaunchAgent plist + sudoers sample into a
  tarball.
- Computes SHA-256 checksums.
- Creates the GitHub Release with the changelog as release notes.
- Pushes the formula update to `amiwrpremium/homebrew-tap` using the
  `HOMEBREW_TAP_TOKEN` secret.

Required secrets: `HOMEBREW_TAP_TOKEN` (a fine-grained PAT with
write on `amiwrpremium/homebrew-tap`).

## Status checks required by branch protection

The `master-protection` ruleset requires these checks to pass before a
PR can be merged:

- `Lint`
- `Test (ubuntu-latest)`
- `Test (macos-14)`
- `Build (darwin/arm64)`
- `Vulnerability scan`
- `Conventional Commits`

`Analyze (Go)` (CodeQL) is not required because the weekly cron run
can leave it in a "stale" state that blocks merges. It still runs;
it just doesn't gate.

## What runs when

| Event | ci.yml | pr-title.yml | codeql.yml | release-please.yml | release.yml |
|---|---|---|---|---|---|
| Push to `master` | ✅ | ❌ | ✅ | ✅ | ❌ |
| PR against `master` | ✅ | ✅ | ✅ | ❌ | ❌ |
| Tag push `v*` | ❌ | ❌ | ❌ | ❌ | ✅ |
| Weekly cron | ❌ | ❌ | ✅ | ❌ | ❌ |

## Caching

`actions/setup-go` with `cache: true` caches both the Go module cache
and the build cache by `go.sum` hash. Saves about 30 seconds per run
on cache hit.

## Concurrency

`ci.yml` uses:

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

Pushing a new commit to a PR cancels the previous CI run for that
PR. Saves CI minutes when iterating fast.

## Failure modes

### Lint fails

Usually a real lint issue. Either:

```bash
make lint        # see the issue locally
make lint-fix    # auto-fix where possible
make fmt         # gofumpt formatting
```

…then commit the fix.

If the fix isn't auto-applicable (revive comment requirements, gocritic
warnings about exitAfterDefer, etc.), see the linter's docs.

### Test (macos-14) fails but ubuntu passes

Some macOS-specific behavior that the test missed. Common causes:

- A test that depended on Linux-specific subprocess behavior (rare,
  since `runner.Fake` doesn't actually shell out).
- Timing-sensitive code where macOS's scheduler differs from Linux.

Reproduce locally on a Mac if you have one. If not, add `t.Skip`
under `runtime.GOOS != "linux"` if the test is fundamentally
Linux-specific.

### Build fails

Usually a Go version mismatch. The CI uses `go-version-file: go.mod`,
which reads the `go 1.25.9` directive. If you bumped go.mod to a
version that's not yet released (e.g. `go 1.26.0` before 1.26 is
out), CI can't fetch a matching toolchain.

Fix: stay on a released version in go.mod.

### Vulnerability scan fails

`govulncheck` found a CVE in a dep. Either:

- Bump the dep to a version with the fix: `go get
  golang.org/x/foo@latest && go mod tidy`.
- If no fix exists yet, document the issue in a comment and add
  `//gocritic:disable` (or accept the failure) until upstream
  releases.

### Conventional Commits fails

The PR title isn't valid. See
[Conventional Commits → Examples](conventional-commits.md#examples).

## Adding a new workflow

1. Create `.github/workflows/<name>.yml`.
2. Use `actions/setup-go@v5` with `go-version-file: go.mod`.
3. Use `actions/checkout@v4`.
4. Pin actions to major version (`@v5`), let Dependabot bump them.
5. If it should block PRs, add it to the branch-protection required
   checks (manual UI step).

## Dependabot

`.github/dependabot.yml` schedules weekly updates for Go modules and
GitHub Actions versions. PRs are auto-opened with `chore(deps):` or
`ci(deps):` titles.

We auto-merge patch and minor updates after CI passes. Major updates
need manual review (often involve API changes).

## Troubleshooting CI

- **"Resource not accessible by integration"** — the `GITHUB_TOKEN`
  doesn't have the permission needed. Check the workflow's `permissions:`
  block. release-please needs `contents: write` and `pull-requests:
  write`.
- **"context deadline exceeded"** during `actions/setup-go` — GitHub's
  cache infrastructure is having a moment. Re-run the workflow.
- **"checks_run was not found"** — branch protection is asking for a
  check that doesn't exist. Either rename the check in the workflow
  or remove it from required checks.

## Local CI emulation

```bash
# Run lint exactly the way CI does
golangci-lint run --timeout 5m

# Run tests with race detector
go test -race -coverprofile=coverage.out ./...

# Cross-compile sanity
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/macontrol ./cmd/macontrol

# Vuln scan
govulncheck ./...
```

If all four pass locally, CI will pass.
