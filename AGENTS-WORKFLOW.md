# AGENTS-WORKFLOW.md — How work flows from bead to merge

> Sibling of [`AGENTS.md`](AGENTS.md) (orientation), [`AGENTS-CI.md`](AGENTS-CI.md) (gate detail), [`AGENTS-CODEX.md`](AGENTS-CODEX.md) (parity rules), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) (runtime constraints). Split out of the monolithic AGENTS.md at 580 lines per soc-vuu6.3.

## Workflow

**Every change to `main` cites a bead and passes the cockpit gate before it lands. As of ag-qidx (2026-06-07) the model is PUSH-TO-MAIN: branch protection is OFF, and the pre-push gate is the pre-merge wall — `scripts/pre-push-gate.sh --fast` (default), or the Go orchestrator `ao gate check --fast` via `AGENTOPS_GATE_GO=1`. Run it before every push; rebase-on-reject (git serializes concurrent pushers); on a red `main`, fix forward. The unit of a change is still one *coherent arc* — a closable bead (or small-epic slice) with a single rollback semantic.** This SUPERSEDES the prior PR-per-change model **and** the `local-pre-push-gate-retirement.md` ADR (the old Actions-only authority decision is reversed — the local gate is now load-bearing). Rationale: `.agents/plans/2026-06-07-ao-gate-architecture.md` + the two pre-mortems — the GitHub PR serialization was self-inflicted and bought ~nothing for this solo+own-swarm repo, while the 20-slot free-plan CI was the bottleneck. Historical: the retired PR flow derived from `.agents/council/sdlc-shape-2026-05-17/DUEL.md`; the `gh-merge-chain` update-branch dance it required (`soc-1lp1`) is exactly what push-to-main removes.

**Autonomous-session scope (sister rule to coherent-arc).** Coherent-arc governs the *shape* of one shipped arc; session-scope governs the *count* of consecutive arcs. **Default: 2-4 arcs per autonomous session.** At >=5 shipped or in-flight arcs in one session, **stop and run a post-mortem before continuing**. The old PR-count signal is now interpreted as arc count because the repo no longer uses PRs as the normal landing path. Derivation: the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections; items #5-#6 each fixed fallout from #1-3. Mechanical enforcement is the mandatory `/evolve` post-mortem checkpoint (council-gated, cannot be bypassed; `skills/evolve/references/postmortem-checkpoint.md`). (soc-waxr, ag-o5xp)

### Phases

1. **Claim.** `bd ready` → pick a bead → `bd update <id> --claim`. **No bead, no push.** If the work is genuinely new, `bd create` first.
2. **Scope.** Read the bead's acceptance: a `.feature` file (canonical when present) or an embedded `## Scenarios` block in the bead description. Free-text acceptance is invalid — promote it to scenarios before work begins. Default: **one coherent arc per push** — bundle scenarios that ship-or-revert together; split scenarios with independent rollback. The direct-main commit range is the atomic revert unit. Carve-out: `type=chore` with `#trivial` label for tiny work.
3. **Ship.** `bd worktree create wt-<bead-id> --branch <type>/<bead-id>-<scenario-token>-<short-slug>` (the worktree dir name is the required positional arg) — worktree-mandatory; do not edit in the shared checkout. Implement. The pre-push gate runs automatically on push (the hook); run `scripts/pre-push-gate.sh --fast` manually first to fail fast.
4. **Land.** Push to `main` (the gate runs in the hook; rebase-on-reject). GitHub Actions are not part of the routine landing path; run them manually or through release tags only when explicitly needed. The bead closes when its arc is on `main` (or explicitly cancelled in bead metadata).

### Branch + Direct-Main Shape

| Element | Format |
|---|---|
| Branch | `<type>/<bead-id>-<scenario-token>-<short-slug>` · ≤80 chars · `<scenario-token>` = full slug if it fits, else `<slug-prefix>-<hash8>` |
| Commit title | `<type>(<scope>): <subject> (<bead-id>)` |
| Required evidence | bead id in commit message or close reason · local gate output path or summary · bounded context when relevant |
| Land | Push to `main` after the cockpit gate passes · rebase-on-reject (git serializes concurrent pushers) · no force-push · no deletes |
| Gate | cockpit pre-push gate (blocking, in the hook) + optional/manual Actions backstop. No PR review (PR flow retired — ag-qidx) |

### Multi-agent discipline (shared checkout)

The host `~/dev/agentops` is contended. **Agents do not edit it directly.** Use `bd worktree create <name> --branch <branch>` for every change. Cross-bead merge serialization: `bd merge-slot`. Foreign uncommitted files = quarantined; identify owner, attach to a bead, move into a worktree.

### Provenance

Source of truth: append-only JSONL at `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`). `bd update --metadata` is a derived projection — ledger wins on disagreement. Concurrent writes use `--set-metadata` / `--append-to` (never full-blob replacement) + dolt advisory locks. `claude-code-review` verdicts are first-class ledger events.

### Doctrine altitudes

- **Spine:** [`docs/architecture/operating-loop.md`](docs/architecture/operating-loop.md) — 7-move agent doctrine. **Primary navigation.**
- **One turn's executor:** `/rpi` skill. NOT primary.
- **Architecture:** 5 Bounded Contexts (BC1 Corpus → BC5 Runtime). Where code lives.
- **Consumer metaphor:** "CDLC" — the compounding Knowledge Flywheel framing.

### Source layer — three axis owners, generated or schema-gated; **NEVER hand-edited inventory maps**

- **DDD (vocabulary):** `skills/domain/references/` — BC names + ubiquitous language.
- **Hex (structure):** `skills/*/SKILL.md` frontmatter (`hexagonal_role`, `consumes`, `produces`, `context_rel`) → generated to `docs/contracts/context-map.md`. CI gate: `validate-context-map-drift`.
- **Gherkin (acceptance):** `skills/*/references/*.feature` + bead-embedded `## Scenarios`. CI gate: `scenario-hash-stability`.

### CI tiers (no "advisory")

- **T0 (≤30s)** required gates · **T1 (≤5min)** verification · **T2 (≤15min)** quality — **all required**.
- **I0** informational; runs and reports artifact but does NOT appear as a PR check.


## Local Pre-Push Checklist


Run `scripts/pre-push-gate.sh --fast` for a smart conditional gate that only checks what changed, or `ao gate check --full --workflow-coverage --require-workflow-parity` for full local release evidence. Or run individual checks below.

```bash
# Recommended: smart conditional gate
scripts/pre-push-gate.sh --fast

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

# Full gate (runs everything above and more):
scripts/ci-local-release.sh
```


## Releasing

Standard release flow:

1. Run `scripts/ci-local-release.sh` to validate
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
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - Git push is mandatory; bd push is conditional:
   ```bash
   git pull --rebase
   bd vc status
   bd dolt commit -m "tracker: <summary>"  # if tracker changes are pending
   bd dolt remote list
   bd dolt push  # only if a real Dolt remote is configured
   git push
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
- If `bd dolt push` says no remote is configured, do not treat that as a
  session failure. Record it as unavailable, then continue with the mandatory
  Git push. See [bd server-mode tracker closeout](docs/runbooks/bd-server-mode-closeout.md).

<!-- BEGIN BEADS INTEGRATION v:1 profile:full hash:f65d5d33 -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
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

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Quality
- Use `--acceptance` and `--design` fields when creating issues
- Use `--validate` to check description completeness

### Lifecycle
- `bd defer <id>` / `bd supersede <id>` for issue management
- `bd stale` / `bd orphans` / `bd lint` for hygiene
- `bd human <id>` to flag for human decisions
- `bd formula list` / `bd mol pour <name>` for structured workflows

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push   # only if a bd remote is configured (skip silently otherwise)
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

<!-- END BEADS INTEGRATION -->
