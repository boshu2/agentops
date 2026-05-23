# AgentOps Skills Repository

## What this is

AgentOps compiles and compounds the context that feeds your software factory. It automates agent bookkeeping — attempts, decisions, citations, verdicts, handoffs, learnings — then encodes the DevSecOps CDLC and multi-agent operating practices into a portable corpus that compounds across sessions and runtimes, with humans in or on the loop at whatever rigor level fits.

## Zero-Context Startup (Read First)

If this is your first message in a fresh session, orient in this order:

1. `docs/newcomer-guide.md` for a practical repo orientation and learning path.
2. `docs/index.md` (MkDocs landing) and `docs/documentation-index.md` (full catalog) for navigation.
3. `README.md` for product-level framing.
4. Task-specific canonical surfaces:
   - CLI behavior: `cli/cmd/ao/`, `cli/internal/`, generated `cli/docs/COMMANDS.md`
   - Skills behavior: `skills/**/SKILL.md`
   - Hooks/gates: `hooks/hooks.json` and `hooks/*.sh`
   - Contracts/schemas: `schemas/**`, `lib/schemas/**`
5. `.agents/AGENTS.md` for knowledge store navigation (search on demand, don't pre-load).

## Source-of-Truth Precedence

When files disagree, trust in this order:

1. Executable implementation and generated outputs (`cli/**`, `hooks/**`, `scripts/**`, `cli/docs/COMMANDS.md`)
2. Declared contracts/manifests (`skills/**/SKILL.md`, `hooks/hooks.json`, `schemas/**`)
3. Narrative docs (`docs/**`, `README.md`)

Always report mismatches; do not silently pick a lower-precedence doc over executable behavior.

## Project Structure

```
cli/          Go CLI (ao binary) — cmd/ao, internal packages
skills/       Skill definitions (source of truth)
hooks/        Git/session hooks
lib/          Shared shell helpers
scripts/      Release, validation, and maintenance scripts
schemas/      JSON schemas for config/manifest
tests/        Integration and validation tests
bin/          Standalone shell tools
docs/         Documentation
```

## Critical: Skill File Locations

**Skills source of truth is `skills/` in THIS repo.**

When editing skills, ALWAYS edit the files under `skills/` in this repo. NEVER edit `~/.claude/skills/` directly — those are installed copies that get overwritten on `bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)`.

```
CORRECT:  skills/evolve/SKILL.md          (this repo — source of truth)
WRONG:    ~/.claude/skills/evolve/SKILL.md (installed copy — do not edit)
```

## Building the CLI

```bash
cd cli && make build        # Build ao binary to cli/bin/ao
cd cli && make test         # Run tests
cd cli && make lint         # Run linter
cd cli && make sync-hooks   # Sync embedded hooks/skills into cli/embedded/
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
cd cli && make sync-hooks                 # If you changed hooks/ or lib/hook-helpers.sh
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

**Embedded hooks must stay in sync.** After editing `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`: run `cd cli && make sync-hooks`.

**CLI docs must stay in sync.** After changing commands/flags: run `scripts/generate-cli-reference.sh`.

**Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/documentation-index.md`.

**Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15).

**No TODOs in SKILL.md.** Use `bd` issue tracking instead.

**No secrets in code.** CI greps for hardcoded passwords, API keys, tokens in non-test files.

## Testing Rules

See `.claude/rules/go.md` and `.claude/rules/python.md` for language-specific testing conventions. Key rules: L2 integration tests first, L1 unit tests always. No coverage-padding. No `cov*_test.go` naming.

## Release Pipeline

Tag triggers GoReleaser + GitHub Actions: `git tag v2.X.0 && git push origin v2.X.0`. **Always run `scripts/ci-local-release.sh` before tagging.** Retag with `scripts/retag-release.sh v2.X.0`.

## Agent Goals

GOALS.md is the strategic intent layer consumed by `/evolve` and `/goals`:
- `ao goals measure` — fitness gate checks
- `ao goals measure --directives` — list strategic directives as JSON
- `ao goals steer add/remove/prioritize` — manage directives
- `ao goals init` — bootstrap GOALS.md interactively
- `ao goals migrate --to-md` — convert GOALS.yaml → GOALS.md

## Workflow

**Every change to `main` is a PR. Every PR cites a bead. The unit of a PR is one *coherent arc* — a closable bead (or small-epic slice) with a single rollback semantic. Small epics (≤5 child beads, same surface) ship as one PR with N commits. Large epics (15+ child beads) ship as N PRs sliced by scenario or wave.** Direct pushes to `main` are rejected by branch protection. Derivation: `.agents/council/sdlc-shape-2026-05-17/DUEL.md` (local, gitignored — duel between Claude Opus 4.7 and Codex gpt-5.5, 2026-05-17). 2026-05-19 evolution from "1 scenario per PR" after the 8-PR merge-arc burned out the `gh-merge-chain` dance — `soc-1lp1`.

**Autonomous-session scope (sister rule to coherent-arc).** Coherent-arc governs the *shape* of a single PR; session-scope governs the *count* of consecutive PRs. **Default: 2-4 PRs per autonomous session.** At ≥5 PRs shipped or in-flight in one session, **stop and run a post-mortem before continuing** — diminishing returns and reactive-PR spirals (PR-fixes-fallout-from-prior-PR) are the dominant failure mode in the back-half of long sessions. Derivation: the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections; PRs #5–#6 each fixed fallout from #1–3. Visible reactivity by PR #5 but the loop kept nudging "keep going" without surfacing the post-mortem signal. Mechanical enforcement ships as the PreToolUse Bash hook at `hooks/session-pr-counter.sh` (soc-1aou, PR #362) — it fires at `count >= threshold-1` (default 5) and emits the post-mortem prompts to the agent via `additionalContext`, with optional hard-block via `AGENTOPS_SESSION_PR_BLOCK=1`. (soc-waxr)

### Phases

1. **Claim.** `bd ready` → pick a bead → `bd update <id> --claim`. **No bead, no PR.** If the work is genuinely new, `bd create` first.
2. **Scope.** Read the bead's acceptance: a `.feature` file (canonical when present) or an embedded `## Scenarios` block in the bead description. Free-text acceptance is invalid — promote it to scenarios before work begins. Default: **one PR per coherent arc** — bundle scenarios that ship-or-revert together; split scenarios with independent rollback. The PR is the *atomic-revert unit*. Carve-out: `type=chore` with `#trivial` label for tiny work.
3. **Ship.** `bd worktree create --branch <type>/<bead-id>-<scenario-token>-<short-slug>` — worktree-mandatory; do not edit in the shared checkout. Implement. Run per-tool sanity checks for the surfaces you touched (`cd cli && make test`, `bats tests/scripts/<file>.bats`, etc.); CI runs the omnibus validation on push.
4. **Close.** Open PR. CI validates the merge state. Squash-merge when green. The bead closes only when every scenario is merged (or explicitly cancelled in bead metadata).

### Branch + PR shape

| Element | Format |
|---|---|
| Branch | `<type>/<bead-id>-<scenario-token>-<short-slug>` · ≤80 chars · `<scenario-token>` = full slug if it fits, else `<slug-prefix>-<hash8>` |
| PR title | `<type>(<scope>): <subject> (<bead-id> #<scenario-slug>)` — full slug here |
| Required PR body trailers | `Closes-scenario: <bead-id>#<slug>` · `Bounded-context: BC<N>-<name>` · `Evidence: <path>` |
| Merge | Squash only · linear history · branch up-to-date · no force-push · no deletes |
| Reviews | 0 humans + required `claude-code-review` check (automation gate) |

### Multi-agent discipline (shared checkout)

The host `~/dev/agentops` is contended. **Agents do not edit it directly.** Use `bd worktree create --branch <name>` for every change. Cross-bead merge serialization: `bd merge-slot`. Foreign uncommitted files = quarantined; identify owner, attach to a bead, move into a worktree.

### Provenance

Source of truth: append-only JSONL at `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`). `bd update --metadata` is a derived projection — ledger wins on disagreement. Concurrent writes use `--set-metadata` / `--append-to` (never full-blob replacement) + dolt advisory locks. `claude-code-review` verdicts are first-class ledger events.

### Doctrine altitudes

- **North star:** [`docs/3.0.md`](docs/3.0.md) — what AgentOps 3.0 is (hookless-first CDLC, the SDLC↔CDLC loop, the four-practice waist). The single source of truth; everything below is consistent with it.
- **Spine:** [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — 7-move agent doctrine. **Primary navigation.**
- **One turn's executor:** `/rpi` skill. NOT primary.
- **Architecture:** 5 Bounded Contexts (BC1 Corpus → BC5 Runtime). Where code lives.
- **Consumer metaphor:** "CDLC" — the compounding Knowledge Flywheel framing (`Research → Plan → Implement → Validate → Knowledge Flywheel feedback`).

### Source layer — three axis owners, generated or schema-gated; **NEVER hand-edited inventory maps**

- **DDD (vocabulary):** `skills/domain/references/` — BC names + ubiquitous language.
- **Hex (structure):** `skills/*/SKILL.md` frontmatter (`hexagonal_role`, `consumes`, `produces`, `context_rel`) → generated to `docs/contracts/context-map.md`. CI gate: `validate-context-map-drift`.
- **Gherkin (acceptance):** `skills/*/references/*.feature` + bead-embedded `## Scenarios`. CI gate: `scenario-hash-stability`.

### CI tiers (no "advisory")

- **T0 (≤30s)** required gates · **T1 (≤5min)** verification · **T2 (≤15min)** quality — **all required**.
- **I0** informational; runs and reports artifact but does NOT appear as a PR check.

## Session Constraints

- **Multi-phase work:** Route through `ao rpi` (enforces timeouts and stall detection).
- **Before spawning workers:** Verify no file overlap across the wave. File collisions are the #1 swarm failure mode.
- **Before proposing new capability:** Check `ao rpi serve --help`, `hooks/hooks.json`, and `GOALS.md` first.
- **Gas City (gc) bridge:** `cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`. Do not write new tests or features for deprecated files (`rpi_loop_supervisor.go`, `rpi_c2_events.go`, `rpi_phased_tmux.go`, `rpi_workers.go`, `rpi_parallel.go`, `fire.go`).

### Execution Discipline

- **Verify before committing.** Go: `go test ./...` and `go vet ./...`. Python: run relevant tests. Never commit unverified code.
- **First-Edit Rule.** First Edit/Write/Bash must happen within your first 3 responses. Execute first, research second.
- **Intent Echo.** Before non-trivial tasks, state in ONE sentence what you understand. Wait for confirmation on multi-file changes.
- **Two-Correction Rule.** If corrected twice on the same task: STOP, re-read, state what you now understand differently, and confirm before trying again.
