---
name: beads-br
description: Local-first issue tracker (beads_rust) for
---
<!-- TOC: Critical Rules | Quick Workflow | Essential Commands | bv Integration | Plan → Beads Conversion | Issue-Lifecycle Discipline | References -->

# beads-br — Beads Rust Issue Tracker

> **Non-invasive:** br NEVER runs git commands. Sync and commit are YOUR responsibility.

## ⚠️ Critical Constraints for Agents

| Rule | Why |
|------|-----|
| **ALWAYS use `--json`** | Structured output for parsing |
| **NEVER run bare `bv`** | Blocks session in TUI mode |
| **Sync is EXPLICIT** | `BEADS_DIR="$(ao beads dir)" br sync --flush-only` after changes |
| **Git is YOUR job** | br never runs git; `_beads/` sync is explicit |
| **No cycles allowed** | `br dep cycles` must return empty |

## Private-ledger repos — the persist_intent port contract

Some repos declare a **private bead ledger** separated from the public source
tree. The agentops repo is the canonical case; an agent that loads only this
skill (without the repo CLAUDE.md) must still honor these invariants:

| Invariant | Rule |
|---|---|
| **Indirection** | Resolve first with `ao beads dir`, then invoke as `BEADS_DIR="$(ao beads dir)" br <cmd>`. A worktree-local ledger path is valid only in the canonical checkout; linked worktrees use git's common dir to reach the canonical private ledger. |
| **Private repo** | `_beads/` is its **own git repository** (separate remote). Sync = `git -C "$(ao beads dir)" push`. |
| **Never leak** | never stage the private ledger from the host repo — bead bodies carry private context; the host repo is public and gitignores the ledger. |
| **bd is retired AS OUR TRACKER** | because the bd/Dolt remote-server lane was retired (2026-06-11) — do not run `bd` in a br repo; it appears only in explicitly-marked legacy notes. **Gas City carve-out (age-gc-integrate-8aom.2):** bd/dolt is gc's NATIVE city store and that is blessed — `bd`/`gc bd` inside a gc city dir (a dir with a `GC_HOME`/`.gc/` or a gc-managed `.beads/`, e.g. `~/gc/*`) is substrate operation, not a tracker violation. The seam: **br = agentops ledger** (git-JSONL); **bd/dolt = gc city-internal store**; outcomes cross one way via the read-only rollup (age-gc-adoption-u0he.6). Never point a gc city's bd at this repo's tracking, and never file agentops work in a city store. |
| **Prefix filter** | to prevent cross-project leakage in shared DBs, filter queries by the repo's issue prefix (e.g. `ag-`) before trusting `br ready` output. |
| **Writes fail closed** | because an empty/wrong `BEADS_DIR` makes br silently write a fallback tracker (age-gstf) — for `create`/`update`/`close`/`dep` use `BEADS_DIR="$(ao beads dir --require)" && export BEADS_DIR && br <write-cmd>`; `--require` refuses to print a path unless the directory holds a real ledger. |

This section is the `persist_intent` port contract: the skill that persists
intent owns the rules that keep that intent private and uncorrupted.

## Quick Workflow

```bash
# 1. Find work
BEADS_DIR="$(ao beads dir)" br ready --json

# 2. Claim it
BEADS_DIR="$(ao beads dir)" br update ag-abc123 --claim --json

# 3. Do work...

# 4. Complete (write commands: guarded, fail-closed resolution)
BEADS_DIR="$(ao beads dir --require)" && export BEADS_DIR && br close ag-abc123 --reason "Implemented X"

# 5. Sync to git (EXPLICIT!)
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" add -A && git -C "$(ao beads dir)" commit -m "tracker: close ag-abc123" && git -C "$(ao beads dir)" push
```

## Essential Commands

```bash
# Lifecycle
BEADS_DIR="$(ao beads dir)" br create "Title" -p 1 -t task --json
BEADS_DIR="$(ao beads dir)" br update <id> --claim --json
BEADS_DIR="$(ao beads dir)" br close <id> --reason "Done"
BEADS_DIR="$(ao beads dir)" br reopen <id>

# Querying (always use --json for agents)
BEADS_DIR="$(ao beads dir)" br ready --json
BEADS_DIR="$(ao beads dir)" br list --json
BEADS_DIR="$(ao beads dir)" br blocked --json
BEADS_DIR="$(ao beads dir)" br search "keyword"
BEADS_DIR="$(ao beads dir)" br show <id> --json

# Dependencies
BEADS_DIR="$(ao beads dir)" br dep add <child> <parent>
BEADS_DIR="$(ao beads dir)" br dep cycles
BEADS_DIR="$(ao beads dir)" br dep tree <id>

# Sync (EXPLICIT - never automatic)
BEADS_DIR="$(ao beads dir)" br sync --flush-only
BEADS_DIR="$(ao beads dir)" br sync --import-only

# System
BEADS_DIR="$(ao beads dir)" br doctor
BEADS_DIR="$(ao beads dir)" br config --list
```

## Priority Scale

| Priority | Meaning |
|----------|---------|
| 0 | Critical |
| 1 | High |
| 2 | Medium (default) |
| 3 | Low |
| 4 | Backlog |

## bv Integration

**CRITICAL:** Never run bare `bv` — it launches interactive TUI and blocks.

```bash
# Always use --robot-* flags:
bv --robot-next                      # Single top pick
bv --robot-triage                    # Full triage
bv --robot-plan                      # Parallel execution tracks
bv --robot-insights | jq '.Cycles'   # Check graph health
```

## Agent Mail Coordination

Use bead ID as thread_id for multi-agent coordination:

```python
file_reservation_paths(..., reason="ag-123")
send_message(..., thread_id="ag-123", subject="[ag-123] Starting...")
# Work...
BEADS_DIR="$(ao beads dir)" br close ag-123 --reason "Completed"
release_file_reservations(...)
```

## Session Ending Pattern

```bash
git pull --rebase
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" add -A && git -C "$(ao beads dir)" commit -m "tracker: update issues" && git -C "$(ao beads dir)" push
git push
git status  # Verify clean
```

## Plan → Beads Conversion (absorbed from /beads-workflow)

Absorbed from the retired `beads-workflow` skill (ag-ez7y6 consolidation) —
requests for `/beads-workflow`, "beads workflow", or "convert this markdown
plan into beads" route **here**.

> **Core Principle:** "Check your beads N times, implement once" — where N is
> as many as you can stomach. Beads should be so detailed and polished that you
> can mechanically unleash a swarm of agents to implement them, and it will
> come out just about perfectly.

### Conversion prompt and bead anatomy

Use the exact prompt for a comprehensive and granular set of beads, then the
polishing prompts in [PROMPTS.md](references/PROMPTS.md). Every resulting issue must satisfy
[BEAD-ANATOMY.md](references/BEAD-ANATOMY.md): self-contained intent, explicit
scope and dependencies, testable acceptance, and enough rationale that the
original plan is not needed during execution.

### Polishing Protocol

Run the standard prompt from [PROMPTS.md](references/PROMPTS.md), review, and
repeat until steady-state without losing features or test scope. If review
flatlines, restart from fresh context and finish with a cross-model pass.

## Output Specification

- **Artifact directory:** the private ledger returned by `ao beads dir
  --require`; never the host repo or a worktree-local fallback.
- **Filename convention:** `br` allocates `<configured-prefix><opaque-id>`;
  `issues.jsonl` is an export name, not the primary read surface.
- **Serialization/schema format:** `br show --json` returns a one-element issue
  array with nonempty identity, intent, type, priority, and legal status.
- **Validator command:** with the changed `$bead_id` set:

  ```bash
  set -euo pipefail
  BEADS_DIR="$(ao beads dir --require)"
  export BEADS_DIR
  issue="$(br show "$bead_id" --json)"
  jq -e --arg id "$bead_id" 'length == 1 and .[0].id == $id and
    ((.[0].title | type) == "string") and (.[0].title | length) > 0 and
    ((.[0].description | type) == "string") and (.[0].description | length) > 0 and
    ((.[0].issue_type | type) == "string") and (.[0].issue_type | length) > 0 and
    ((.[0].priority | type) == "number") and
    (.[0].status == "open" or .[0].status == "in_progress" or
     .[0].status == "blocked" or .[0].status == "closed")' <<<"$issue" >/dev/null
  br dep cycles --json | jq -e 'type == "object" and (.cycles | type) == "array" and (.cycles | length) == 0 and .count == 0 and .active_count == 0' >/dev/null
  ```
- **Downstream handoff:** pass the validated bead ID to its owning loop; after
  writes, explicitly export and commit/push the private ledger.

## Quality Rubric

Before implementation, verify each bead:

- [ ] **Self-contained** — Understandable without external context
- [ ] **Clear scope** — One coherent piece of work
- [ ] **Dependencies explicit** — Links to blocking/blocked beads
- [ ] **Testable** — Clear success criteria
- [ ] **Includes tests** — Unit and e2e tests in scope
- [ ] **Preserves features** — Nothing from plan was lost
- [ ] **Not oversimplified** — Complexity preserved where needed
- [ ] **No cycles** — `br dep cycles` returns empty

Beads are implementation-ready only after steady-state, cross-model review,
test coverage, clean dependencies, and a zero-cycle graph.

### bd → br migration (docs)

Replace legacy tracker commands with `br`; preserve prefixes and explicit
private-ledger sync. Gas City remains the documented `bd`/Dolt exception.

## Issue-Lifecycle Discipline

Keep the graph honest across sessions:

- Live `br show`/`ready`/`list` reads are authoritative; JSONL is an export.
- Run claim-verify before dispatch; coordinate instead of racing.
- Narrow vague umbrella work into an execution-ready child before implementation.
- Close only after scoped closure proof: the feature is merged to trunk and visible on `origin/main`, with touched
  files and validation commands; reconcile the parent in the reason.
- Route every residual to a successor in the same turn; never leave zombie parents.
- Append progress notes, normalize stale queue items, and capture fuzzy intent
  as a bead in the same turn. See [INTEGRATION.md](references/INTEGRATION.md)
  for multi-actor lifecycle detail.

## Anti-Patterns

- Never run ambiguous sync, bare `bv`, or assume auto-commit.
- Never close with cycles, unsynced state, or an unlanded feature commit.
- Never stage a private ledger into its public host repository.

## Troubleshooting

Run `br doctor`, `br dep cycles --json`, and `br config --list` against the
resolved ledger. See [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) for
worktree and sync recovery; storage layout is in [CONFIG.md](references/CONFIG.md).

## References

| Topic | File |
|-------|------|
| Full command reference | [COMMANDS.md](references/COMMANDS.md) |
| Configuration details | [CONFIG.md](references/CONFIG.md) |
| Troubleshooting guide | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
| Multi-agent patterns | [INTEGRATION.md](references/INTEGRATION.md) |
| Conversion/polish/fresh-session prompts | [PROMPTS.md](references/PROMPTS.md) |
| Bead structure (well-formed bead anatomy) | [BEAD-ANATOMY.md](references/BEAD-ANATOMY.md) |
