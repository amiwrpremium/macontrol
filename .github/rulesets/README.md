# GitHub rulesets

Declarative rulesets for the `master` branch and the `v*` release tags.
GitHub's newer rulesets system (2023+) is more powerful than classic
branch protection: it supports tag protection, commit-message patterns,
and JSON import/export.

## Files

| File | Target | Purpose |
|---|---|---|
| `branch-master.json` | `refs/heads/master` | required PR + status checks + linear history |
| `tags-v.json` | `refs/tags/v*` | only the maintainer (admin) can create/move/delete release tags |

## One-time bootstrap

Rulesets are repo-scoped — no GitHub App to install:

```bash
gh api repos/amiwrpremium/macontrol/rulesets \
  --method POST \
  --input .github/rulesets/branch-master.json

gh api repos/amiwrpremium/macontrol/rulesets \
  --method POST \
  --input .github/rulesets/tags-v.json
```

The `actor_id: 5` bypass entry is GitHub's built-in repo-admin role; no
lookup required.

## Reconciling drift

```bash
# List existing rulesets and grab the IDs.
gh api repos/amiwrpremium/macontrol/rulesets

# Update in place (replaces the entire ruleset).
gh api repos/amiwrpremium/macontrol/rulesets/<ruleset-id> \
  --method PUT \
  --input .github/rulesets/branch-master.json
```

The ruleset on the live repo is authoritative; if it drifts from the
JSON in this directory, re-run the `PUT` to bring it back in line.

## Why both rulesets *and* `settings.yml`?

`settings.yml`'s `branches:` block is the legacy branch-protection API.
It's kept as a fallback so a freshly forked / freshly recreated repo is
sane out of the box even before the rulesets above are imported. Once
the rulesets are imported, evaluation is additive (both must pass), so
they harmonise rather than conflict.

## Notable design choices (see the JSON files' `_comment` blocks for the full text)

- `enforce SHA pinning` (from `pin-check.yml`) is **not** in
  `branch-master.json`'s required checks. pin-check has a paths filter
  and only fires on PRs that touch `.github/workflows/**` — making it
  required would block every release PR.
- `required_signatures` is **not** set on either ruleset. release-please
  and Renovate don't sign their commits; requiring signatures on PR tips
  would permanently block bot-driven PRs. The substantive control is
  GitHub's auto-signed squash-merge commit on `master`.
- `required_approving_review_count: 0` on `branch-master.json` reflects
  the solo-maintained status. When/if the project grows past one
  maintainer, raise this number and re-import the ruleset.
