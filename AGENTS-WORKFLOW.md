# AGENTS-WORKFLOW.md — How work flows from bead to merge

> Sibling of [`AGENTS.md`](AGENTS.md) (orientation), [`AGENTS-CI.md`](AGENTS-CI.md) (gate detail), [`AGENTS-CODEX.md`](AGENTS-CODEX.md) (parity rules), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) (runtime constraints). Split out of the monolithic AGENTS.md at 580 lines per soc-vuu6.3.

## Workflow

**Every change to `main` cites a bead and passes the cockpit gate before it lands. As of ag-qidx (2026-06-07) the model is PUSH-TO-MAIN: branch protection is OFF, and the pre-push gate is the pre-merge wall. Current authority is the Go gate: the hook builds `ao` from source and runs `ao gate check --fast`; the legacy bash route is an escape hatch only via `AGENTOPS_GATE_BASH=1`. Run the Go gate before every push; rebase-on-reject (git serializes concurrent pushers); on a red `main`, fix forward. The unit of a change is still one *coherent arc* — a closable bead (or small-epic slice) with a single rollback semantic.** This SUPERSEDES the prior PR-per-change model **and** the `local-pre-push-gate-retirement.md` ADR (the old Actions-only authority decision is reversed — the local gate is now load-bearing). Rationale: `.agents/plans/2026-06-07-ao-gate-architecture.md` + the two pre-mortems — the GitHub PR serialization was self-inflicted and bought ~nothing for this solo+own-swarm repo, while the 20-slot free-plan CI was the bottleneck. Historical: the retired PR flow derived from `.agents/council/sdlc-shape-2026-05-17/DUEL.md`; the `gh-merge-chain` update-branch dance it required (`soc-1lp1`) is exactly what push-to-main removes.

**Autonomous-session scope (sister rule to coherent-arc).** Coherent-arc governs the *shape* of one shipped arc; session-scope governs the *count* of consecutive arcs. **Default: 2-4 arcs per autonomous session.** At >=5 shipped or in-flight arcs in one session, **stop and run a post-mortem before continuing**. The old PR-count signal is now interpreted as arc count because the repo no longer uses PRs as the normal landing path. Derivation: the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections; items #5-#6 each fixed fallout from #1-3. Mechanical enforcement is the mandatory `/evolve` post-mortem checkpoint (council-gated, cannot be bypassed; `skills/evolve/references/postmortem-checkpoint.md`). That checkpoint is a **re-plan point, not just stop/continue** — it may refactor, reorder, drop, or add to the *remaining* arcs from what the session taught (`/rpi`'s [Agile Re-Plan Loop](skills/rpi/references/agile-replan-loop.md); `--auto` pivots without an operator prompt). (soc-waxr, ag-o5xp)

**Tracker = br (beads_rust) + bv, as of 2026-06-11.** Issue tracking is **br** — offline, git-JSONL-backed (`_beads/issues.jsonl` + a local SQLite cache; `br sync` never runs git). The private ledger lives in the canonical checkout, not in every linked worktree. Resolve it with `ao beads dir` and invoke as `BEADS_DIR="$(ao beads dir)" br <cmd>`; do not rely on `$PWD/_beads` from linked worktrees. For **write commands** (`create`/`update`/`close`/`dep`), resolution must fail closed — an empty or wrong `BEADS_DIR` lets br silently write the wrong tracker (age-gstf). Guarded write shape: `BEADS_DIR="$(ao beads dir --require)" && export BEADS_DIR && br close <id> -r "Done"` (`--require` exits non-zero, printing nothing, unless the resolved directory holds a real ledger). Triage with **bv** (`bv --robot-insights`, `--robot-plan`, `--robot-priority`). **Two-store truth (age-gc-adoption-u0he):** `br` is **AgentOps' own repo tracker**; **bd/Dolt is the gascity SUBSTRATE store** — first-class and embraced, the native store a gas-city factory runs on. They are different layers, not competitors. AgentOps moved its OWN tracking to `br` for offline/local-first reasons (the earlier all-in bet on the agentic-flywheel stack coupled delivery to a remote single-host Dolt server — a SPOF with no offline lane; circuit breaker observed open in the 2026-06-11 recon, `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md`), so **do not run `bd` for this repo's tracking** — but bd/dolt is legitimate as the gascity substrate. The pre-br `.beads/` bd/Dolt data for this repo is preserved pending reconciliation; migration record: `.agents/swarm/results/br-migration.json`.

### Phases

1. **Claim.** `BEADS_DIR="$(ao beads dir)" br ready --json` → pick a bead → `BEADS_DIR="$(ao beads dir)" br update <id> --claim --json`. **No bead, no push.** If the work is genuinely new, `BEADS_DIR="$(ao beads dir)" br create "Title" -t task -p 2 --body "..." --json` first (deps: `--deps blocks:<id>` or `BEADS_DIR="$(ao beads dir)" br dep add <child> <parent>`).
2. **Scope.** Read the live bead body with `BEADS_DIR="$(ao beads dir)" br show <id> --json` before editing. Its acceptance is the contract: a `.feature` file (canonical when present) or an embedded `## Scenarios` block in the bead description. Free-text acceptance is invalid — promote it to scenarios before work begins. Default: **one coherent arc per push** — bundle scenarios that ship-or-revert together; split scenarios with independent rollback. The direct-main commit range is the atomic revert unit. Carve-out: `type=chore` with `#trivial` label for tiny work.
3. **Ship.** `git worktree add wt-<bead-id> -b <type>/<bead-id>-<scenario-token>-<short-slug>` — worktree-mandatory; do not edit in the shared checkout (canonical-root rules: [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md)). Implement, commit the bead's code as HEAD. Run `ao gate check --fast --scope head` to fail fast before you land.
4. **Land.** **`ao land <bead>` is the canonical land verb** — one command does the whole trusted land. It builds a fresh in-checkout `ao`, pins it for preflight, runs the commit-bound pawl review, and hands off to atomic land machinery. Reviewer-lane execution and evidence rules live in [`pawl-review`](skills/pawl-review/SKILL.md); `ao pawl` alone applies diversity and binds the verdict. On **REFUTED / NO-VERDICT it stops and exits non-zero — no land**. Push-to-main is refused without a CONFIRMED verdict. GitHub Actions are not part of the routine landing path. The bead closes when its arc is on `main` (or explicitly cancelled in bead metadata).

### Branch + Direct-Main Shape

| Element | Format |
|---|---|
| Branch | `<type>/<bead-id>-<scenario-token>-<short-slug>` · ≤80 chars · `<scenario-token>` = full slug if it fits, else `<slug-prefix>-<hash8>` |
| Commit title | `<type>(<scope>): <subject> (<bead-id>)` |
| Required evidence | bead id in commit message or close reason · local gate output path or summary · bounded context when relevant |
| Land | `ao land <bead>` (canonical) — fresh in-checkout binary → trusted pawl review (auto-bind on CONFIRM) → atomic `pawl-land.sh` (rebase-on-reject, single push) · no force-push · no deletes |
| Gate | cockpit pre-push gate (blocking, in the hook) + optional/manual Actions backstop. No PR review (PR flow retired — ag-qidx) |

### Multi-agent discipline (shared checkout)

The host `~/dev/agentops` is contended. **Agents do not edit it directly.** Use `git worktree add <name> -b <branch>` for every change. Cross-bead merge serialization: git itself (rebase-on-reject serializes concurrent pushers) plus Agent Mail coordination (`am` reservations / build slots) when multiple lanes are landing — `bd merge-slot` is retired with bd. Foreign uncommitted files = quarantined; identify owner, attach to a bead, move into a worktree.

### Provenance

Source of truth: append-only JSONL at `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`). Tracker state (`br` issue fields, notes, comments) is a derived projection — ledger wins on disagreement. The ledger is append-only: concurrent writers append events, never rewrite (the old `--set-metadata`/dolt-advisory-lock machinery is retired with bd). `claude-code-review` verdicts are first-class ledger events.

### Doctrine altitudes

- **Spine:** [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — 7-move agent doctrine. **Primary navigation.**
- **One turn's executor:** `/rpi` skill. NOT primary.
- **Architecture:** 6 Bounded Contexts (BC1 Corpus → BC6 Orchestration). Where code lives.
- **Consumer metaphor:** "CDLC" — the compounding Knowledge Flywheel framing.

### Source layer — three axis owners, generated or schema-gated; **NEVER hand-edited inventory maps**

- **DDD (vocabulary):** `skills/domain/references/` — BC names + ubiquitous language.
- **Hex (structure):** `skills/*/SKILL.md` frontmatter (`hexagonal_role`, `consumes`, `produces`, `context_rel`) → generated to `docs/contracts/context-map.md`. CI gate: `validate-context-map-drift`.
- **Gherkin (acceptance):** `skills/*/references/*.feature` + bead-embedded `## Scenarios`. CI gate: `scenario-hash-stability`.

### CI tiers (no "advisory")

- **T0 (≤30s)** required gates · **T1 (≤5min)** verification · **T2 (≤15min)** quality — **all required**.
- **I0** informational; runs and reports artifact but does NOT appear as a PR check.


## Local Pre-Push Checklist


Run `ao gate check --fast --scope head` for the smart conditional Go gate that only checks what changed, or `ao gate check --full --workflow-coverage --require-workflow-parity` for full local release evidence. Use `AGENTOPS_GATE_BASH=1` only as the documented legacy fallback. Or run individual checks below.

```bash
# Recommended: smart conditional gate
ao gate check --fast --scope head

# One-command local development bootstrap
bash scripts/install.sh --dev

# Or individual checks:

# 1. Skill integrity (most common failure)
bash skills/heal-skill/scripts/heal.sh --strict

# 2. Doc-release gate (skill counts, link validation)
./tests/docs/validate-doc-release.sh

# 3. ShellCheck
find . -name "*.sh" -type f -not -path "./.git/*" -print0 | xargs -0 shellcheck --severity=error

# 4. Markdownlint
git ls-files '*.md' | xargs markdownlint

# 5. Go build + tests (if cli/ changed)
cd cli && make build && make test

# 6. Contract compatibility
./scripts/check-contract-compatibility.sh

# 7. CI policy/docs parity
bash scripts/validate-ci-policy-parity.sh

# 8. Worktree disposition
bash scripts/check-worktree-disposition.sh

# 9. Plugin structure (symlinks, manifests)
./scripts/validate-manifests.sh --repo-root .
find skills -type l  # must be empty — zero symlinks allowed

 # 10. Headless runtime skill smoke (local Claude/Codex sessions; skips missing CLIs)
 bash scripts/validate-headless-runtime-skills.sh

 # 11. Codex-first override coverage (full skill catalog is classified and covered)
 bash scripts/validate-codex-override-coverage.sh

 # 12. Codex RPI contract and lifecycle guard checks
 bash scripts/validate-codex-rpi-contract.sh
 bash scripts/validate-codex-lifecycle-guards.sh

 # 13. Codex semantic parity audit (generated skills still match Codex-native tool/runtime semantics)
 bash scripts/audit-codex-parity.sh

 # 14. AgentOps contract canaries (official deterministic test gate)
 scripts/test-agentops-contract-canaries.sh

 # 15. Installed-binary smoke (USER-FACING CLI CHANGES ONLY): the `ao` on PATH
 #     must match the just-built binary, else UAT/closeout exercises the STALE
 #     product path (a same-version binary can still differ in content until
 #     `make install` refreshes it). Run before declaring the product path
 #     usable. Local-only — needs an installed ao, so it is not a CI gate. (age-6sg.3)
 cd cli && make build && cd .. && bash scripts/preflight-uat-binary.sh

# Full gate (runs everything above and more):
scripts/ci-local-release.sh
```


## Releasing

**Is a release due?** Run `scripts/check-release-due.sh` (also surfaced by `scripts/release-cadence-check.sh`) — a non-blocking nudge that reports commits + days since the last `vX.Y.Z` tag and flags when a release looks overdue (defaults: 50 commits / 14 days; override with `RELEASE_DUE_COMMITS` / `RELEASE_DUE_DAYS`). It only surfaces the signal — distribution stays pull-based; nothing auto-releases.

Standard release flow:

1. Validate. For a routine pre-tag sanity pass use the fast lane `scripts/ci-local-release.sh --quick` (<5min, code-correctness subset); run the full `scripts/ci-local-release.sh` (~78min) only for the actual tag.
2. Tag and push: `git tag v2.X.0 && git push origin v2.X.0`
3. GitHub Actions runs GoReleaser — builds binaries, creates release, updates Homebrew tap
4. Upgrade locally: `brew update && brew upgrade agentops`

For retagging (rolling post-tag commits into an existing release):

```bash
scripts/retag-release.sh v2.13.0
```

This moves the tag to HEAD, pushes, rebuilds the GitHub release, updates the Homebrew tap, and upgrades locally. One command, no manual steps.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds. For **user-facing CLI changes**, also run the installed-binary smoke (`cd cli && make build && cd .. && bash scripts/preflight-uat-binary.sh`) so the closeout proves the installed `ao` matches the build — not the stale product path — before declaring it usable.
3. **Update issue status** - Close finished work, update in-progress items
4. **LAND + PUSH TO REMOTE** - a reviewed bead lands with **`ao land <bead>`** (fresh in-checkout binary → trusted pawl review with auto-bound verdict → atomic `pawl-land.sh` push through the gate); that is the code-landing path, not a bare `git push`. Then sync the tracker (its own PRIVATE repo, never this public one) and push any residual:
   ```bash
   ao land <bead>                                     # land the reviewed bead's arc (verdict auto-bound, single push)
   git pull --rebase
   BEADS_DIR="$(ao beads dir)" br sync --flush-only   # export DB → _beads JSONL (br never runs git itself)
   git -C "$(ao beads dir)" add -A && git -C "$(ao beads dir)" commit -m "tracker: <summary>" && git -C "$(ao beads dir)" push  # if tracker changes are pending
   git push                                            # any residual non-bead commits (docs, tooling) not already landed by ao land
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches, and validate worktree disposition
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
- NEVER leave a foreign branch-attached worktree without a recorded disposition
- Keep the canonical root clean and attached to `main`.
- Run `bash scripts/check-worktree-disposition.sh` before push and session close.
- The tracker ledger is a PRIVATE nested git repo (`_beads/` → `boshu2/agentops-beads`),
  gitignored by this PUBLIC repo — bead bodies carry private fleet/client context and must
  NEVER land on the public remote. `git -C "$(ao beads dir)" push` IS the tracker sync. If
  `BEADS_DIR="$(ao beads dir)" br sync --flush-only` reports nothing to export, that is fine; continue with the
  mandatory git push.

<!-- BEGIN BEADS INTEGRATION v:1 profile:full (hand-converted bd→br 2026-06-11; no longer generator-managed) -->
## Issue Tracking with br (beads_rust)

**IMPORTANT**: This project tracks its OWN issues with **br (beads_rust)**. Do NOT use markdown TODOs, task lists, other tracking methods — or `bd` (which is the gascity substrate store, a different layer, not this repo's tracker).

Linked worktrees intentionally do not contain their own `_beads` directory. Run `ao beads dir` at session start and use that value for `BEADS_DIR` on every direct `br` invocation. `ao session bootstrap` prints the same path as `tracker: BEADS_DIR=...`.

### Why br?

- Dependency-aware: Track blockers and relationships between issues
- Git-native: SQLite cache + `_beads/issues.jsonl` ledger committed in its own private repo — offline, no server, no SPOF, no public leak
- Agent-optimized: JSON output, ready work detection, discovered-from links, `br robot-docs guide`
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
BEADS_DIR="$(ao beads dir)" br ready --json
```

**Create new issues:**

```bash
BEADS_DIR="$(ao beads dir)" br create "Issue title" --body "Detailed context" -t bug|feature|task -p 0-4 --json
BEADS_DIR="$(ao beads dir)" br create "Issue title" --body "What this issue is about" -p 1 --deps discovered-from:<parent-id> --json
```

**Claim and update:**

```bash
BEADS_DIR="$(ao beads dir)" br update <id> --claim --json
BEADS_DIR="$(ao beads dir)" br update <id> --priority 1 --json
```

**Complete work** (preferred: `ao done` stamps the close with the commit-bound verdict, refusing when none exists — "no verdict = not done" made mechanical):

```bash
ao done <id> -r "Completed"                    # verifies a bound verdict for HEAD (or --sha), then closes
BEADS_DIR="$(ao beads dir)" br close <id> --reason "Completed" --json   # raw close (no verdict check)
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `BEADS_DIR="$(ao beads dir)" br ready --json` shows unblocked issues (graph triage: `bv --robot-insights` / `bv --robot-plan`)
2. **Claim your task atomically**: `BEADS_DIR="$(ao beads dir)" br update <id> --claim --json`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `BEADS_DIR="$(ao beads dir)" br create "Found bug" --body "Details about what was found" -p 1 --deps discovered-from:<parent-id> --json`
5. **Complete**: `ao done <id> -r "Done"` (verdict-stamped close; falls back to `BEADS_DIR="$(ao beads dir)" br close <id> --reason "Done"` when no verdict applies)

### Quality
- Use `br update <id> --acceptance-criteria "..."` and `--design "..."` to fill structured fields
- Use `br lint` to check issues for missing template sections

### Lifecycle
- `br defer <id>` / `br undefer <id>` for scheduling
- `br stale` / `br orphans` / `br lint` for hygiene
- `br epic` for epic management, `br dep tree <id>` / `br dep cycles` for graph health

### Auto-Sync

br syncs through git, not a server:

- Each write auto-flushes the SQLite DB to `_beads/issues.jsonl` (disable with `--no-auto-flush`)
- `BEADS_DIR="$(ao beads dir)" br sync --flush-only` / `--import-only` / `--status` for explicit control; br NEVER runs git commands itself
- Remote sync = `git -C "$(ao beads dir)" add -A && git -C "$(ao beads dir)" commit && git -C "$(ao beads dir)" push` (private remote `boshu2/agentops-beads`)

### Important Rules

- ✅ Use br for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `BEADS_DIR="$(ao beads dir)" br ready --json` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems
- ❌ Do NOT run `bd` for this repo's tracking — br is AgentOps' own tracker (its Dolt server was a single-host SPOF; see the tracker note atop this file). bd/dolt itself is the gascity substrate store — legitimate there, just not this repo's tracker.

For more details, see README.md and [docs/newcomer-guide.md](docs/newcomer-guide.md).

## Session Completion

Covered in full by [Landing the Plane (Session Completion)](#landing-the-plane-session-completion) above — one canonical checklist; this section exists only so older links to it still resolve.

<!-- END BEADS INTEGRATION -->
