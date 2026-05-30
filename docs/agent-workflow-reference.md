# Agent Workflow Reference (on-demand)

> **Not auto-loaded.** This is the deep-detail sidecar for `CLAUDE.md`. The root
> `CLAUDE.md` is a thin router; the CI-validation detail, release-pipeline
> detail, key-scripts table, and agent-goals command surface live here so they
> don't cost context on every session. Read this when you're actually touching
> CI, the release pipeline, or `GOALS.md` — not before.
>
> See also the tiered `AGENTS-*.md` split (`AGENTS-WORKFLOW.md`, `AGENTS-CI.md`,
> `AGENTS-CODEX.md`, `AGENTS-RUNTIME.md`) which owns the AGENTS-side scope detail.

## Building the CLI

```bash
cd cli && make build        # Build ao binary to cli/bin/ao
cd cli && make test         # Run tests
cd cli && make lint         # Run linter
cd cli && make sync-hooks   # Sync embedded lib/skills into cli/embedded/
```

## Key Scripts

| Script | Purpose |
|--------|---------|
| `scripts/ship.sh` | One-knob ship loop — detects inventory changes, runs regen sweep, opens PR |
| `scripts/ci-local-release.sh` | Local release validation gate (run before tagging) |
| `scripts/sync-skill-counts.sh` | Sync skill counts across docs after adding/removing skills |
| `scripts/generate-cli-reference.sh` | Regenerate CLI docs after changing commands/flags |
| `scripts/regen-codex-hashes.sh` | Regenerate hashes after changing skills-codex/ files |
| `scripts/verify-gate-claim.sh` | AP#7 mechanical enforcement — verify `Evidence:` claims against gate logs |

## CI Validation

All pushes to `main` run `.github/workflows/validate.yml` (65 jobs). **CI is the sole authoritative push gate** per `docs/contracts/local-pre-push-gate-retirement.md` (soc-g2r9). The previous `scripts/pre-push-gate.sh` local mirror was retired in PR #357 because it drifted from CI and cost a self-correction PR per drift incident.

### Quick Local Sanity Checks (per-tool, not omnibus)

```bash
cd cli && make build && make test         # If you changed Go code
cd cli && make sync-hooks                 # If you changed lib/ or skills/standards/references/
scripts/regen-codex-hashes.sh             # If you changed skills-codex/ files
bats tests/scripts/<script-you-touched>.bats   # Per-script regression suite

# If you touched docs/ and need the mkdocs strict check locally:
# (system mkdocs ≤1.1.2 cannot parse the modern mkdocs.yml — needs material plugins)
python3 -m venv .venv-mkdocs && .venv-mkdocs/bin/pip install -r requirements-docs.txt && .venv-mkdocs/bin/mkdocs build --strict
```

Run only the per-tool checks for the surfaces you actually touched. Push, let CI run, fix any failures. The 30-90s CI feedback loop replaced the 10-20s local omnibus gate intentionally — the per-incident drift cost dominates the per-push wait.

### Rules That Break CI

**No symlinks.** Ever. The plugin-load-test rejects all symlinks in the repo. If you need the same reference file in multiple skills, **copy** it.

**Skill counts must be synced.** Adding or removing a skill directory requires:

```bash
scripts/sync-skill-counts.sh
```

This updates SKILL-TIERS.md, PRODUCT.md, README.md, docs/SKILLS.md, docs/ARCHITECTURE.md, and using-agentops/SKILL.md. Forgetting this fails the doc-release-gate.

**Every `references/*.md` must be linked in SKILL.md.** If a file exists in `skills/<name>/references/`, the skill's SKILL.md must contain a markdown link to it or a `Read` instruction referencing it. Use `heal.sh --strict` to check.

**Codex skills are manually maintained.** Edit `skills-codex/<name>/SKILL.md` directly or add overrides in `skills-codex-overrides/<name>/`. Audit drift with `bash scripts/audit-codex-parity.sh --skill <name>`.

**Embedded lib/skills must stay in sync.** After editing `lib/` or `skills/standards/references/`: run `cd cli && make sync-hooks`.

**CLI docs must stay in sync.** After changing commands/flags: run `scripts/generate-cli-reference.sh`.

**Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/documentation-index.md`.

**Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15).

**No TODOs in SKILL.md.** Use `bd` issue tracking instead.

**No secrets in code.** CI greps for hardcoded passwords, API keys, tokens in non-test files.

## Testing Rules

See `.claude/rules/go.md` and `.claude/rules/python.md` for language-specific testing conventions. Key rules: L2 integration tests first, L1 unit tests always. No coverage-padding. No `cov*_test.go` naming.

## Release Pipeline

Tag triggers GoReleaser + GitHub Actions: `git tag v2.X.0 && git push origin v2.X.0`. **Always run `scripts/ci-local-release.sh` before tagging.** Retag with `scripts/retag-release.sh v2.X.0`.

For iterative pre-tag work, use `scripts/ci-local-release.sh --quick` (alias `--sanity`) — the fast code-correctness subset (current-platform build + test + version consistency + release smoke + cheap doc/snippet/shellcheck gates) that skips the slow release-rehearsal lane (SBOM, multi-platform cross-build, vuln scan, eval/HIL/readiness). Run the full gate (no flag) once before the actual tag.

## Agent Goals

GOALS.md is the strategic intent layer consumed by `/evolve` and `/goals`:
- `ao goals measure` — fitness gate checks
- `ao goals measure --directives` — list strategic directives as JSON
- `ao goals steer add/remove/prioritize` — manage directives
- `ao goals init` — bootstrap GOALS.md interactively
- `ao goals migrate --to-md` — convert GOALS.yaml → GOALS.md
