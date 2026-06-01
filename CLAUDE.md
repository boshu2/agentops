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
| `scripts/pre-push-gate.sh` | Smart pre-push validation (`--fast` for diff-based) |
| `scripts/ci-local-release.sh` | Local release validation gate (run before tagging) |
| `scripts/sync-skill-counts.sh` | Sync skill counts across docs after adding/removing skills |
| `scripts/generate-cli-reference.sh` | Regenerate CLI docs after changing commands/flags |
| `scripts/regen-codex-hashes.sh` | Regenerate hashes after changing skills-codex/ files |

## CI Validation

All pushes to `main` run `.github/workflows/validate.yml` (24 jobs). **Run checks locally before pushing.**

### Quick Local Validation

```bash
scripts/pre-push-gate.sh --fast          # Recommended: diff-based conditional checks
cd cli && make build && make test         # If you changed Go code
cd cli && make sync-hooks                 # If you changed hooks/ or lib/hook-helpers.sh
scripts/regen-codex-hashes.sh            # If you changed skills-codex/ files
scripts/pre-push-gate.sh                 # Full gate (all 33 checks, ~3min)
```

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

## AgentOps Workflow (RPI)

```
Research → Plan → Implement → Validate
    ↑                            │
    └──── Knowledge Flywheel ────┘
```

## Warmind (Team Knowledge Sharing)

Warmind is the team knowledge sharing system. Key files:

| File | Purpose |
|------|---------|
| `cli/cmd/ao/warmind.go` | CLI commands |
| `cli/internal/warmind/` | Core modules (pool, scoring, citations, maturity, contradict) |
| `cli/docs/warmind/POST-MORTEM-2026-05-27.md` | Gas City integration test findings |
| `cli/docs/warmind/RESEARCH-2026-05-27.md` | Cutting-edge research for V3 |

### Warmind Pipeline

```
.agents/learnings/  →  .warmind/pool/staged/  →  .warmind/learnings/
     (local)              (team staging)           (team canon)
```

### Key Commands

```bash
ao warmind sync              # Local → Pool (stages + scores)
ao warmind pool list         # Show staged candidates
ao warmind status            # Health metrics
ao warmind promote <id>      # Manual promotion (--force bypasses cites)
ao warmind close-loop        # Full flywheel: auto-promote + decay + contradict
ao inject "query"            # Uses learnings, records citations
```

### Promotion Tiers

| Tier | Score | Requirement |
|------|-------|-------------|
| Gold | ≥0.8 | Auto-promote after 24h |
| Silver | ≥0.5 | 1 citation from OTHER engineer |
| Bronze | <0.5 | 3 citations from OTHER engineers |

## Session Constraints

- **Multi-phase work:** Route through `ao rpi` (enforces timeouts and stall detection).
- **Before spawning workers:** Verify no file overlap across the wave. File collisions are the #1 swarm failure mode.
- **Before proposing new capability:** Check `ao rpi serve --help`, `hooks/hooks.json`, and `GOALS.md` first.
- **Gas City (gc) bridge:** `cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`. Do not write new tests or features for deprecated files (`rpi_loop_supervisor.go`, `rpi_c2_events.go`, `rpi_phased_tmux.go`, `rpi_workers.go`, `rpi_parallel.go`, `fire.go`).

## Gas City Dispatch (CRITICAL)

**NEVER manually perform work that should be dispatched to Gas City agents.** Use `gc` commands to dispatch work:

### Dispatching Work to Agents

```bash
# Send work to an agent (creates mail bead, agent picks up on next cycle)
gc mail send <agent> "<subject>" -m "<body>"
gc mail send mayor "Build is green"
gc mail send witness "Need investigation" -m "Check logs from last failed run"

# Nudge a live session directly (immediate delivery)
gc session nudge <agent> "<message>"
gc session nudge mayor "PR #42 needs review"

# Route work via sling (respects routing rules)
gc sling <agent> "<work description>"

# Handoff with session restart
gc handoff <agent> "<message>"
```

### Checking Agent Status

```bash
gc status                    # City-wide overview
gc session list              # All sessions
gc session peek <agent>      # View agent's recent transcript
gc mail check                # Check for pending mail
gc beads list                # List work items
```

### Common Patterns

| Task | Command | Notes |
|------|---------|-------|
| Request code review | `gc mail send witness "Review PR #X"` | Witness handles reviews |
| Trigger build/test | `gc mail send polecat "Run tests"` | Polecat handles CI tasks |
| Escalate to human | `gc mail send human "Decision needed"` | Routes to human inbox |
| Wake sleeping agent | `gc session wake <agent>` | Resume from sleep |
| Check agent health | `gc doctor` | Diagnose issues |

### Anti-Patterns (DO NOT DO)

```bash
# WRONG: Manually running tests that polecat should run
cd /path/to/repo && go test ./...

# RIGHT: Dispatch to polecat
gc mail send polecat "Run test suite for cli/"

# WRONG: Manually reviewing code
# [reading files and giving feedback inline]

# RIGHT: Dispatch to witness
gc mail send witness "Review changes in cli/cmd/ao/warmind.go"

# WRONG: Manually doing multi-step work
# [implementing feature step by step]

# RIGHT: Create a bead and let agents handle phases
gc sling mayor "Implement warmind embedding index"
```

### When to Dispatch vs Do Directly

| Scenario | Action |
|----------|--------|
| Quick file read/edit | Do directly |
| Single command | Do directly |
| Multi-phase implementation | Dispatch via `gc sling` or `ao rpi` |
| Code review | Dispatch to witness |
| Test suite run | Dispatch to polecat |
| Research task | Dispatch to researcher or do directly if simple |
| Build/deploy | Dispatch to appropriate agent |

### Execution Discipline

- **Verify before committing.** Go: `go test ./...` and `go vet ./...`. Python: run relevant tests. Never commit unverified code.
- **First-Edit Rule.** First Edit/Write/Bash must happen within your first 3 responses. Execute first, research second.
- **Intent Echo.** Before non-trivial tasks, state in ONE sentence what you understand. Wait for confirmation on multi-file changes.
- **Two-Correction Rule.** If corrected twice on the same task: STOP, re-read, state what you now understand differently, and confirm before trying again.

## AgentOps Knowledge Flywheel

Knowledge compounds automatically across sessions:

- **MEMORY.md** is auto-loaded by your AI coding tool every session
- **Session hooks** extract learnings, update MEMORY.md, and prune stale knowledge
- **Skills** invoke flywheel commands at the right moments (no manual ao commands needed)

Verify the flywheel any time:

```bash
ao flywheel status    # escape velocity check
ao status             # current knowledge inventory
```
