# AGENTS-WORKFLOW.md — How work flows from bead to merge

> Sibling of [`AGENTS.md`](AGENTS.md) (orientation), [`AGENTS-CI.md`](AGENTS-CI.md) (gate detail), [`AGENTS-CODEX.md`](AGENTS-CODEX.md) (parity rules), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md) (runtime constraints). Split out of the monolithic AGENTS.md at 580 lines per soc-vuu6.3.

## Workflow

**Every change to `main` is a PR. Every PR cites a bead. The unit of a PR is one *coherent arc* — a closable bead (or small-epic slice) with a single rollback semantic. Small epics (≤5 child beads, same surface) ship as one PR with N commits. Large epics (15+ child beads) ship as N PRs sliced by scenario or wave.** Direct pushes to `main` are rejected by branch protection. Derivation: `.agents/council/sdlc-shape-2026-05-17/DUEL.md` (local, gitignored — duel between Claude Opus 4.7 and Codex gpt-5.5, 2026-05-17). 2026-05-19 evolution from "1 scenario per PR" after the 8-PR merge-arc burned out the `gh-merge-chain` dance — `soc-1lp1`.

**Autonomous-session scope (sister rule to coherent-arc).** Coherent-arc governs the *shape* of a single PR; session-scope governs the *count* of consecutive PRs. **Default: 2-4 PRs per autonomous session.** At ≥5 PRs shipped or in-flight in one session, **stop and run a post-mortem before continuing** — diminishing returns and reactive-PR spirals (PR-fixes-fallout-from-prior-PR) are the dominant failure mode in the back-half of long sessions. Derivation: the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections; PRs #5–#6 each fixed fallout from #1–3. Visible reactivity by PR #5 but the loop kept nudging "keep going" without surfacing the post-mortem signal. Mechanical enforcement ships as the PreToolUse Bash hook at `hooks/session-pr-counter.sh` (soc-1aou, PR #362) — it fires at `count >= threshold-1` (default 5) and emits the post-mortem prompts to the agent via `additionalContext`, with optional hard-block via `AGENTOPS_SESSION_PR_BLOCK=1`. (soc-waxr)

### Phases

1. **Claim.** `bd ready` → pick a bead → `bd update <id> --claim`. **No bead, no PR.** If the work is genuinely new, `bd create` first.
2. **Scope.** Read the bead's acceptance: a `.feature` file (canonical when present) or an embedded `## Scenarios` block in the bead description. Free-text acceptance is invalid — promote it to scenarios before work begins. Default: **one PR per coherent arc** — bundle scenarios that ship-or-revert together; split scenarios with independent rollback. The PR is the *atomic-revert unit*. Carve-out: `type=chore` with `#trivial` label for tiny work.
3. **Ship.** `bd worktree create --branch <type>/<bead-id>-<scenario-token>-<short-slug>` — worktree-mandatory; do not edit in the shared checkout. Implement. Run `scripts/pre-push-gate.sh --fast` before push.
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


Run `scripts/pre-push-gate.sh --fast` for a smart conditional gate that only checks what changed. Or run individual checks below. If any fail, CI will fail too.

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

# 7. Hook/docs parity
bash scripts/validate-hooks-doc-parity.sh

# 8. CI policy/docs parity
bash scripts/validate-ci-policy-parity.sh

# 9. Worktree disposition
bash scripts/check-worktree-disposition.sh

# 10. Plugin structure (symlinks, manifests)
./scripts/validate-manifests.sh --repo-root .
find skills -type l  # must be empty — zero symlinks allowed

 # 11. Headless runtime skill smoke (local Claude/Codex sessions; skips missing CLIs)
 bash scripts/validate-headless-runtime-skills.sh

 # 12. Codex-first override coverage (full skill catalog is classified and covered)
 bash scripts/validate-codex-override-coverage.sh

 # 13. Codex RPI contract and lifecycle guard checks
 bash scripts/validate-codex-rpi-contract.sh
 bash scripts/validate-codex-lifecycle-guards.sh

 # 14. Codex semantic parity audit (generated skills still match Codex-native tool/runtime semantics)
 bash scripts/audit-codex-parity.sh

 # 15. AgentOps contract canaries (official deterministic test gate)
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
