# Governance

A lightweight governance model for a small, actively-maintained
single-maintainer project. The bar is low for adding contributors
and the bar is high for breaking changes — the goal is to keep
macontrol's behavior predictable for the small but real set of
users who already rely on it.

## Project status

macontrol is **active**. The maintainer responds to issues and PRs
on a best-effort schedule. There is no funded SLA; nothing in this
document creates one.

## Decision-making

Most decisions — bug fixes, dependency bumps, doc edits, new tests
— are made by the maintainer at PR-merge time without formal
process.

For changes with user-visible behavior shifts (new bot command,
removed flag, breaking JSON schema, sudoers entry, TCC grant)
the convention is:

1. Open a GitHub **Discussion** describing the proposed change and
   the rationale, with at least one alternative considered.
2. Wait at least 72 hours for community input; longer if the
   change is contentious or large.
3. Land the implementation in a PR that links the Discussion.
4. Note the change in the next release's CHANGELOG entry; if
   the change is breaking, mark the release as a major bump per
   semver.

The maintainer holds final tie-break authority. The intent of the
Discussion step is to slow down breaking changes, not to
require consensus on every commit.

## Adding a maintainer

The maintainer set is open in principle. A new maintainer is
typically added after:

- Six or more substantive PRs merged over at least three months.
- Demonstrated review participation (helpful comments on other
  people's PRs).
- Alignment with the existing project conventions — testing,
  documentation, security posture, conventional-commit style.

The process: a current maintainer opens a PR adding the
candidate to [MAINTAINERS.md](./MAINTAINERS.md) and
[`.github/CODEOWNERS`](.github/CODEOWNERS), linking the
candidate's contributions. The candidate confirms in a comment.
Merging the PR makes the change effective.

There is no quorum requirement because the maintainer set is
currently size 1; the rule formalises so any future growth is
predictable.

## Removing a maintainer

A maintainer can step down at any time by moving themselves
from "Current" to "Emeritus" in [MAINTAINERS.md](./MAINTAINERS.md)
via PR. Emeritus maintainers retain their listing as a public
record of contribution but no longer have merge rights.

In the unlikely event of needing to revoke maintainership
non-voluntarily — repeated CoC violations, sustained malicious
activity — any current maintainer can open a PR moving the
person to Emeritus with rationale linked to the relevant
discussion. The PR follows the same 72-hour input window as
breaking changes above.

## Release process

Releases are managed by release-please:

1. Conventional-commit history on `master` accumulates feat /
   fix / perf / etc. entries.
2. release-please opens a PR titled `chore(release): X.Y.Z`
   summarising the changelog delta.
3. A maintainer reviews and merges the PR.
4. The merge tags `vX.Y.Z` and triggers goreleaser, which
   publishes a GitHub Release with cross-compiled binaries and
   pushes the formula update to [the homebrew tap](https://github.com/amiwrpremium/homebrew-tap).

Patch (`X.Y.Z` → `X.Y.Z+1`) and minor (`X.Y.Z` → `X.Y+1.0`)
releases land routinely. Major releases (`X.Y.Z` → `X+1.0.0`)
require an explicit user-visible justification (breaking flag,
config schema change, removed feature).

Hotfix process: a separate PR titled
`fix: <description>` cherry-picked from `master`, followed by an
out-of-cadence release-please cut.

## Security disclosures

See [SECURITY.md](./SECURITY.md) for the disclosure policy and
SLA. Vulnerability reports go through GitHub's private security
advisories — **never** open a public issue for a vulnerability.

The maintainer responds within 72 hours, ships a patch (or
provides a mitigation) within 30 days of confirming the report.

## License of contributions

macontrol is distributed under MIT (see [LICENSE](./LICENSE)).
Inbound contributions are licensed under the same terms — no
CLA is required. By opening a PR you affirm you have the right
to license the contribution under MIT and that you grant the
project that license.
