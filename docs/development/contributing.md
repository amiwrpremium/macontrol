# Contributing

Thanks for considering contributing to macontrol. The project is small
and solo-maintained — clean PRs land fast, sprawling ones languish.

## The workflow

1. **Open or claim an issue** — for non-trivial changes. For typos
   and one-line fixes, just open the PR.
2. **Fork** the repo to your account.
3. **Create a branch** off `master`:
   ```bash
   git switch -c feat/my-thing
   ```
   Branch naming: `feat/`, `fix/`, `docs/`, `refactor/`, etc. — match
   the conventional-commit type.
4. **Make changes**. Keep diffs focused; one logical change per PR.
5. **Run checks** locally before pushing:
   ```bash
   make lint test
   make build       # cross-compile sanity check
   ```
6. **Commit** using
   [Conventional Commits](conventional-commits.md). The `.gitmessage`
   template helps:
   ```bash
   git config commit.template .gitmessage
   ```
7. **Push** and open a PR via GitHub UI or `gh pr create`.
8. **PR title must match Conventional Commits** — a CI workflow
   rejects titles that don't.

We squash-merge, so only the PR title appears in the master history.
Body content is preserved as the squash commit's body.

## What to put in the PR

- **Summary** — 1–3 bullets describing the change and *why*.
- **Test plan** — what you ran or tested manually.
- **Screenshots / GIFs** for any UI-visible change (new keyboard
  layout, new flow).
- **Breaking-change call-out** if applicable.

The repo's `.github/PULL_REQUEST_TEMPLATE.md` prompts for these.

## Scope rules

Things that get accepted quickly:

- **New buttons** on existing categories where the macOS backing CLI
  is straightforward.
- **New flows** for actions that need free-text input.
- **Bug fixes** with a regression test.
- **Docs improvements** — corrections, clearer explanations, better
  examples.
- **Test coverage** improvements — turning red lines green in
  `make cover`.
- **Dependency bumps** Dependabot opens (auto-merged after CI passes,
  for patch and minor updates).

Things that get pushback:

- **Adding `/sh` or arbitrary-shell escape hatch** — explicit non-goal.
  See [Architecture → Design decisions](../architecture/design-decisions.md).
- **Multi-tenant access controls** — out of scope; macontrol is
  single-owner by design.
- **Intel support** — explicit non-goal. The install script + CI both
  refuse non-arm64.
- **External-monitor brightness via DDC** — too many edge cases for
  too little gain. Open as opt-in if at all.
- **Adding heavy dependencies** — current dep tree is intentionally
  minimal. Ask before adding a new top-level dep.

When in doubt, open an issue first to discuss.

## Code style

- **gofumpt** for formatting — `make fmt` runs it.
- **golangci-lint** with the v2 config in `.golangci.yml` — `make lint`
  runs it.
- **Comments** for exported types and funcs (revive enforces this).
  Comment style is "Name does X" — start with the identifier, end
  with a period.
- **Tests** alongside source: `foo.go` → `foo_test.go`.
- **No global state** unless absolutely required. The flow registry
  and shortmap are exceptions; everything else is per-call.

Readability beats cleverness. A few extra lines of clear code is
better than one terse line.

## Adding a new capability

See [Adding a capability](adding-a-capability.md) for the 6-file
recipe. The TL;DR:

1. `internal/domain/<name>/<name>.go` — the macOS-side function.
2. `internal/domain/<name>/<name>_test.go` — test using `runner.Fake`.
3. `internal/telegram/keyboards/<ns>.go` — the inline keyboard.
4. `internal/telegram/callbacks/data.go` — register the namespace
   constant.
5. `internal/telegram/handlers/<ns>.go` — handle the callback.
6. `internal/telegram/handlers/callbacks.go` — wire the namespace to
   the new handler.

Plus a test in `internal/telegram/handlers/<ns>_test.go` and a doc
update in `docs/usage/categories/<name>.md` and
`docs/reference/macos-cli-mapping.md`.

## Reporting bugs

Use the issue templates at
<https://github.com/amiwrpremium/macontrol/issues/new/choose>. Bug
report templates auto-prompt for:

- macontrol version (`macontrol --version`)
- macOS version + chip
- Install method
- Doctor output
- Repro steps, expected behavior, actual behavior, logs

Don't paste tokens or your real Telegram user IDs into public issues.

## Security issues

Use the **private** disclosure channel:
<https://github.com/amiwrpremium/macontrol/security/advisories/new>

See [Security → Reporting vulnerabilities](../security/reporting-vulnerabilities.md).

## Communication

- **GitHub Issues** — bugs, feature requests.
- **GitHub Discussions** — questions, ideas, "how do I…?".
- **Pull requests** — code changes.
- **GitHub Security Advisories** — vulnerabilities.

There's no Discord, no Slack, no mailing list. The maintainer reads
GitHub.

## License

By contributing, you agree your contributions are licensed under
[MIT](../../LICENSE) — same as the rest of the project.

## Recognition

Every PR-merger lands their name in the squash commit's author field
and shows up on the contributor list. The `CHANGELOG.md` is
auto-generated from commits and shows your work in the release notes.

For larger contributions, you'll be added as a co-maintainer if you
want — open a discussion to talk about it.
