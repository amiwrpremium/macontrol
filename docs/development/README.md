# Development

For contributors. Covers workflow, testing, CI, and the recipe for
adding a new capability.

## What's here

- **[Contributing](contributing.md)** — workflow, PR process, scope
  rules.
- **[Conventional Commits](conventional-commits.md)** — types,
  scopes, examples.
- **[Adding a capability](adding-a-capability.md)** — the 6-file
  recipe for a new feature.
- **[Testing](testing.md)** — `make test`, `make cover`, harness
  patterns.
- **[CI](ci.md)** — workflows and what each job does.
- **[Releasing](releasing.md)** — release-please + GoReleaser + tap
  flow.
- **[Roadmap](roadmap.md)** — planned features and explicit non-goals.

## At a glance

```bash
git clone https://github.com/amiwrpremium/macontrol.git
cd macontrol
make lint test            # check before committing
git switch -c feat/my-thing
# … make changes …
git commit -am "feat(scope): one-line summary"
git push -u origin feat/my-thing
gh pr create
```

## The shape of contributions

| Type | Example | Where it goes |
|---|---|---|
| Bug fix in domain layer | wifi parser misses an SSID format | `internal/domain/<pkg>/` + test |
| New button on existing category | `bt:rename` to rename a paired device | keyboard + handler + (maybe) flow + test |
| New category | `🎵 Music` for `/music` controls | full 6-file recipe |
| New CLI subcommand | `macontrol export-config` | `cmd/macontrol/<file>.go` |
| Docs improvement | typo, clarification | edit `docs/**/*.md` |
| Build/CI tweak | new lint rule, new workflow | `.github/`, `.golangci.yml`, etc. |

The rule of thumb: small, focused PRs land faster. A 50-line PR that
does one thing is easier to review than a 500-line PR that does five.

## Required reading before contributing

1. **[Contributing](contributing.md)** — read first.
2. **[Conventional Commits](conventional-commits.md)** — required for
   PR titles.
3. **[Architecture → Project layout](../architecture/project-layout.md)**
   — find your way around.
4. **[Architecture → Design decisions](../architecture/design-decisions.md)**
   — understand the constraints before proposing changes.

For specific feature work:

- **Adding a button** → [Adding a capability](adding-a-capability.md).
- **Adding tests** → [Testing](testing.md).
- **Fixing CI** → [CI](ci.md).
- **Cutting a release** → [Releasing](releasing.md).
