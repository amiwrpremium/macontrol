# Conventional Commits

PR titles (and therefore squash-merge commit subjects) must follow
[Conventional Commits v1.0](https://www.conventionalcommits.org). A
GitHub Action enforces this on every PR.

## Format

```text
<type>(<scope>): <subject>

[<body>]

[<footer>]
```

- **type** — what kind of change.
- **scope** (optional but encouraged) — what part of the codebase.
- **subject** — imperative, lowercase, no trailing period, ≤ 72 chars.
- **body** (optional) — bullet list, wrap at 72 cols.
- **footer** (optional) — references, breaking-change notes.

The repo's `.gitmessage` template encodes all this.

## Types

| Type | Use for | Bumps version? | Shows in CHANGELOG? |
|---|---|---|---|
| `feat` | New feature | minor | yes (Features) |
| `fix` | Bug fix | patch | yes (Bug Fixes) |
| `perf` | Performance improvement (no behavior change) | patch | yes (Performance) |
| `refactor` | Code restructure (no behavior change) | none | yes (Refactors) |
| `docs` | Docs only | none | no (hidden) |
| `test` | Tests only | none | no (hidden) |
| `build` | Build system, deps | none | no (hidden) |
| `ci` | CI config | none | no (hidden) |
| `chore` | Anything else | none | no (hidden) |
| `revert` | Reverting a previous commit | varies | yes (Reverts) |

The version-bump rules are how `release-please` decides whether your
merge triggers a minor or patch version bump in the next release PR.

## Breaking changes

Add a `!` after the type/scope and a `BREAKING CHANGE:` footer:

```text
feat(bot)!: switch from numeric user IDs to OAuth tokens

BREAKING CHANGE: ALLOWED_USER_IDS is no longer used. Existing config
files require re-running `macontrol setup` to migrate.
```

Triggers a major version bump.

## Scopes

Allowed scopes are listed in `.github/workflows/pr-title.yml`:

```text
sound        display      power        wifi         bt
battery      system       media        notify       tools
bot          flows        keyboards    callbacks    runner
config       capability   setup        service      doctor
launchd      brew         tap          ci           release
deps         docs
```

Scopes mirror the project layout. For:

- **Domain changes** — use the package name (`sound`, `display`, etc.).
- **Telegram-layer changes** — use `bot`, `flows`, `keyboards`, or
  `callbacks` depending on which sub-package.
- **CLI subcommand changes** — use `setup`, `service`, `doctor`.
- **CI/release tooling** — use `ci`, `release`, or `deps`.
- **Docs** — use `docs`, or omit the scope for general doc changes.

Scope is optional — `feat: add foo` is valid. Including a scope helps
narrow what changed at a glance, so use one when sensible.

## Examples

### Feature with scope

```text
feat(sound): add inline keyboard for volume ± and mute toggle

- Per-tap delta of 1 or 5 with explicit MAX shortcut
- Mute / Unmute button label flips based on current state
- Refresh button re-reads osascript output
- All actions edit the dashboard in place
```

### Bug fix with scope

```text
fix(wifi): handle missing en0 on Macs without built-in Wi-Fi

The interface-discovery loop assumed every Mac has en0. On Mac mini
configurations without Wi-Fi, the absence wasn't surfaced as a clear
error. Now returns "no Wi-Fi hardware port found" so the user knows.
```

### Multiple scopes

When a change touches multiple scopes, omit it:

```text
feat: add macontrol CLI with setup wizard and service management
```

…rather than `feat(setup,service,doctor): …`.

### Docs change

```text
docs: clarify that Wi-Fi scanning is unsupported

Apple removed `airport -s` in macOS 14.4. Update the troubleshooting
table and the architecture note to reflect this.
```

### CI change

```text
ci: bump golangci-lint-action from v6 to v9

v9 supports the v2 config schema natively, which removes the need
for an explicit version pin.
```

### Dependency bump (Dependabot opens these)

```text
chore(deps): bump github.com/caarlos0/env/v11 from 11.3.1 to 11.4.0

…
```

(Dependabot uses `chore` for module updates; `ci` for Actions updates.)

### Revert

```text
revert: switch from REST polling to WebSocket

Reverts commit abc123. The WebSocket approach added too much state
machinery for our use case.
```

## What gets rejected

The PR-title check rejects:

- Title that doesn't start with a known type.
- Type immediately followed by `:` instead of `(scope):` or `:`. (Both
  are valid; `feat (scope):` with a space is not.)
- Subject starting with a capital letter — must be lowercase.
- Empty subject after the colon.

The check runs on every push to a PR, so iteration is fast.

## Commit message body

Bodies are encouraged but optional. When present, use **bullet points**:

```text
feat(bot): add destructive-action confirm sub-keyboard

- First tap on Restart/Shutdown/Logout shows ✅ Confirm / ✖ Cancel
- Second tap fires the action; cancel edits back to the inline home
- Reduces accidental power-offs from a single misclick
```

Avoid prose paragraphs in commit bodies — they're harder to scan in
`git log --oneline` style readouts and CHANGELOG entries.

## Why we enforce this

- **Auto-generated CHANGELOG** — release-please reads commit subjects
  to build the release notes. Without conventional commits, the
  changelog is gibberish.
- **Auto-generated version bumps** — `feat:` → minor, `fix:` →
  patch, `feat(x)!:` → major. No manual version-file edits.
- **Searchable history** — `git log --grep="^fix(wifi)"` finds every
  Wi-Fi bug fix. Useful for retrospectives.

The trade-off is one extra line of thought when writing PR titles.
Worth it.

## Local helpers

```bash
git config commit.template .gitmessage
# now `git commit` opens the template in your editor
```

The template includes a checklist of types, scopes, and example
subjects.

If you use `lefthook` (`lefthook install`), the `commit-msg` hook
validates messages locally before they're committed:

```bash
git commit -m "wrong format"
# commit message must follow Conventional Commits (see .gitmessage)
```
