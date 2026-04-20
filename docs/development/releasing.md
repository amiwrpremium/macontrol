# Releasing

The release flow is fully automated. You don't tag manually; you don't
build manually; you don't update CHANGELOG.md manually. You merge a
PR.

## The cycle

```text
Devs merge feature/fix PRs to master
        │
        ▼
release-please reads commits since last tag
        │
        ▼
Opens (or updates) a "chore(release): vX.Y.Z" PR
        │  - bumps version.txt
        │  - updates CHANGELOG.md from commits
        │  - .release-please-manifest.json bumped
        ▼
Maintainer merges the release PR
        │
        ▼
release-please tags vX.Y.Z and drafts a GitHub Release
        │
        ▼
release.yml fires on the tag push
        │  - GoReleaser builds darwin/arm64 binary
        │  - packages tarball + checksums
        │  - uploads to the GitHub Release
        │  - commits Formula/macontrol.rb to homebrew-tap
        ▼
Users: brew upgrade macontrol
```

No manual steps after merging the release PR. ~3 minutes from merge
to "available via brew upgrade".

## What triggers a release

`release-please` decides whether to open / update the release PR
based on what's been merged to `master` since the last tag:

| Commit type | Effect |
|---|---|
| `feat:` | Opens release PR with **minor** bump |
| `fix:`, `perf:` | Opens release PR with **patch** bump |
| `refactor:`, `revert:` | Opens release PR with **patch** bump (reverts can be major if reverting a feat) |
| `feat!:` (breaking change) | **Major** bump |
| `docs:`, `test:`, `build:`, `ci:`, `chore:` | No release PR opened (these are hidden from CHANGELOG) |

So merging only `docs:` and `chore:` PRs won't accumulate a release.
You need at least one `feat:`, `fix:`, or `perf:` to trigger a new
release.

## The release PR

Looks like:

```text
Title: chore(release): 0.2.0

Body:
🤖 I have created a release *beep* *boop*
---

## 0.2.0 (2026-05-15)

### Features

- (sound) add input volume control (#42)
- (display) DDC support for external monitors (#48)

### Bug Fixes

- (wifi) handle missing en0 on Macs without built-in Wi-Fi (#43)

### Performance

- (runner) reuse exec.Cmd buffers across calls (#46)

---

This PR was generated with Release Please.
```

Files changed in the release PR:

- `version.txt` — bumped (`0.1.0` → `0.2.0`)
- `.release-please-manifest.json` — bumped to match
- `CHANGELOG.md` — new section prepended

You can edit the PR's body if you want to add release notes beyond the
auto-generated CHANGELOG (e.g. upgrade notes, breaking-change
guidance). The body becomes the GitHub Release body when the PR is
merged.

## Merging the release PR

```bash
gh pr merge <release-pr-number> --squash --delete-branch
```

Or merge via the GitHub UI. **Squash-merge** specifically — that's
what triggers the tag push (the squashed commit's message has the
`Release-As: …` annotation that release-please uses to recognize the
release commit).

After merge:

1. release-please creates the tag `v0.2.0`.
2. release-please creates a draft GitHub Release.
3. The tag push triggers `release.yml`.
4. GoReleaser builds and uploads.
5. The formula in `homebrew-tap` updates.

You can watch progress in the Actions tab.

## What GoReleaser produces

Per release:

- `macontrol_<version>_darwin_arm64.tar.gz` — the binary plus the
  LaunchAgent plist template, sudoers sample, README, CHANGELOG,
  LICENSE.
- `checksums.txt` — SHA-256 of the tarball.
- A GitHub Release with the changelog as the body, install instructions
  in the footer, and both files attached.
- `Formula/macontrol.rb` committed to `amiwrpremium/homebrew-tap`
  with the new version + checksum.

After the homebrew-tap commit, `brew update && brew upgrade
macontrol` works for users.

## Required configuration

`HOMEBREW_TAP_TOKEN` secret in the macontrol repo settings. A
fine-grained PAT with write access to `amiwrpremium/homebrew-tap`
only. See [GitHub setup → Step 5](../../) for how to create it (the
walkthrough lives in `~/macontrol-github-setup.md` for the
maintainer).

If this token is missing or expired, `release.yml` fails the
homebrew-tap update step. The GitHub Release is still created with
the tarball; you'd just need to manually update the formula.

## Initial-version override

The first release was pinned to v0.1.0 (release-please's default
for Go is v1.0.0, which overclaims for a brand-new bot). The pin
lives in `release-please-config.json`:

```json
"packages": {
  ".": {
    "package-name": "macontrol",
    "initial-version": "0.1.0",
    "extra-files": ["version.txt"]
  }
}
```

`initial-version` is consulted only on the first release. After v0.1.0
is tagged, this field is ignored — subsequent releases follow the
standard semver bump rules.

When the project graduates to v1.0.0 (stable API, breaking changes
managed via SemVer):

- Either merge a PR with a `feat!:` commit (auto-bumps to 1.0.0).
- Or manually bump `version.txt` to `1.0.0` in a release PR.

## Hotfix releases

If a release ships with a critical bug:

1. Branch from the release tag:
   ```bash
   git switch -c hotfix/critical-thing v0.2.0
   ```
2. Apply the fix as a `fix:` commit.
3. Open a PR to master.
4. After it merges, release-please opens a `chore(release): 0.2.1`
   PR.
5. Merge that. v0.2.1 ships.

This works because release-please reads the full commit history, not
just the recent push.

## Release frequency

No fixed cadence. Releases ship when there's something worth
shipping. Typical pattern:

- Patch releases (v0.2.1) — within hours of a regression being
  reported and fixed.
- Minor releases (v0.3.0) — when 3–5 new features have accumulated.
- Major releases (v1.0.0+) — when API stability is committed.

Don't accumulate features for weeks before releasing. Small frequent
releases keep the CHANGELOG digestible.

## Verifying a release

After the release.yml run completes:

1. **GitHub Release exists**:
   ```
   https://github.com/amiwrpremium/macontrol/releases/tag/v0.2.0
   ```
   Should show the changelog, the tarball, and `checksums.txt`.

2. **Homebrew tap updated**:
   ```
   https://github.com/amiwrpremium/homebrew-tap/blob/master/Formula/macontrol.rb
   ```
   `version` and `sha256` should match the new release.

3. **Brew install works**:
   ```bash
   brew update
   brew info macontrol         # shows new version
   brew upgrade macontrol      # installs it
   macontrol --version         # confirms
   ```

4. **Manual install works**:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/amiwrpremium/macontrol/master/scripts/install.sh | sh
   ```
   Should download the new tarball and install.

If any of these are wrong, the `release.yml` log has details. Common
issues:

- **HOMEBREW_TAP_TOKEN expired** — generate a new one, update the
  secret, manually edit the formula or wait for next release.
- **Tarball missing files** — check `.goreleaser.yaml` `archives.files`
  list.
- **Checksum mismatch** — almost never; would be a GoReleaser bug.

## Skipping a release

To merge `feat:` commits without triggering a release (rare):

- Use type `chore:` instead. Loses CHANGELOG entry but no release PR
  opens.
- Or merge the feat normally, then close the release PR without
  merging — release-please will reopen it on the next push.

## Manually creating a release

If automation breaks and you need to ship now:

```bash
# Tag manually
git tag -a v0.2.1 -m "v0.2.1"
git push origin v0.2.1
```

The tag push triggers `release.yml`. GoReleaser handles the rest. The
release PR (if open) becomes stale — close it, release-please will
re-sync on the next push.

This bypasses release-please's CHANGELOG generation, so you'd need to
edit `CHANGELOG.md` by hand.

## Yanking a release

GitHub doesn't actually delete releases (they leave a "yanked" marker).
Procedure:

1. Edit the release on GitHub, mark it as `pre-release` (un-pins it
   from "latest").
2. Delete the tarball assets if they're harmful.
3. Update the homebrew-tap formula to point at the previous version
   (manually edit the .rb file in the tap repo).
4. Open a hotfix to master and ship the next patch release.

`brew upgrade macontrol` then naturally rolls users to the patched
version.

Tags themselves are protected by the `release-tags` ruleset and can't
be deleted. That's intentional — never let a published release tag
disappear.
