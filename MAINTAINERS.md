# Maintainers

The list below is normative — [`.github/CODEOWNERS`](.github/CODEOWNERS)
references usernames in the same order. To add a maintainer, see
[GOVERNANCE.md](./GOVERNANCE.md#adding-a-maintainer).

## Current

| Maintainer | Areas | Contact |
|---|---|---|
| [@amiwrpremium](https://github.com/amiwrpremium) | all | private channel via [SECURITY.md](./SECURITY.md) |

## Emeritus

(none yet)

## Maintainer responsibilities

- Triage issues within ~7 days of opening.
- Review human-authored PRs within ~7 days of submission.
- Cut releases when conventional-commit history warrants one —
  release-please opens the PR; a maintainer reviews and merges.
- Respond to security advisories within 72 hours per
  [SECURITY.md](./SECURITY.md), ship a patch or mitigation
  within 30 days.
- Keep dependencies current — Renovate auto-merges patch and
  minor bumps per the policy in [`renovate.json`](./renovate.json);
  Dependabot still raises security-update PRs for CVEs that
  break out of the routine cadence.
- Keep CI green on `master`. Any drift (e.g. new lint rules in
  upstream golangci-lint, stdlib CVEs surfaced by govulncheck)
  is addressed in a small fix PR; tier work is not landed on a
  red master.
