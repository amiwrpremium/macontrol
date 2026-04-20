# Contributing to macontrol

Thanks for your interest. macontrol is a small Go project, so the process is
small too.

## Workflow

1. **Fork** the repo and create a branch off `master`.
2. **Commit** using [Conventional Commits](https://www.conventionalcommits.org). A `.gitmessage` template is provided — enable it with:
   ```bash
   git config commit.template .gitmessage
   ```
3. **Run checks** locally before pushing:
   ```bash
   make lint test
   ```
4. **Open a PR** against `master`. The PR **title** must be a valid
   Conventional Commit — a GitHub Action rejects non-conforming titles. We
   squash-merge, so only the PR title lands in the history.

## Commit conventions

- Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`,
  `chore`, `revert`.
- Scopes mirror the project tree: `sound`, `display`, `power`, `wifi`, `bt`,
  `battery`, `system`, `media`, `notify`, `tools`, `bot`, `flows`,
  `keyboards`, `callbacks`, `runner`, `config`, `ci`.
- Breaking changes: add `!` after the scope (`feat(bot)!: …`) and a
  `BREAKING CHANGE:` footer. release-please will bump the major version.
- Keep subjects in imperative mood, lower-case, under 72 chars, no period.

## Adding a new capability

Say you want to add `/spotify`:

1. `internal/domain/spotify/spotify.go` — pure control, no Telegram imports.
2. `internal/telegram/keyboards/spotify.go` — inline keyboard builder.
3. `internal/telegram/callbacks/spo.go` — callback_data dispatch.
4. `internal/telegram/handlers/spo.go` — glue between domain + keyboard.
5. Register in `internal/telegram/keyboards/home.go` (home button) and
   `internal/telegram/callbacks/registry.go` (route).
6. Write a domain test using the `runner.Fake` helper and a
   keyboard-layout test.

## Development setup

```bash
# Cross-compile for the target
make build                      # GOOS=darwin GOARCH=arm64

# Run against a dev bot token
TELEGRAM_BOT_TOKEN=... ALLOWED_USER_IDS=... go run ./cmd/macontrol
```

You can iterate on most of the tree from a Linux box — the domain layer is
mocked via the `runner` interface, and the Telegram layer tests don't call
any macOS commands.

## Releases

Maintainers don't tag manually. `release-please` opens a release PR whenever
`master` accumulates release-worthy commits; merging that PR tags the
version, and GoReleaser takes over from there (tarball + Homebrew formula).
