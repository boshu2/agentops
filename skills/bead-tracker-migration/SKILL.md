---
name: bead-tracker-migration
user-invocable: false
skill_api_version: 1
hexagonal_role: supporting
metadata:
  tier: execution
  stability: experimental
  dependencies:
    - beads
    - beads-br
description: |-
  Use when migrating an issue tracker workspace from bd to br with loss-free verification.
  Triggers:
context:
  window: inherit
output_contract: A br-backed .beads/ workspace with verified-equal issue counts plus a migration report.
---
<!-- TOC: Critical Constraints | Why This Exists | Quick Start | Workflow | Output | Exit Codes | Quality Rubric | Examples | Troubleshooting | See Also -->

# bead-tracker-migration — move a beads workspace from bd to br

> JSONL is the bridge. bd exports it, br imports it. The DB engines differ (bd
> can ride Dolt; br is always SQLite) but the issue records are interchange-compatible.

## ⚠️ Critical Constraints

- **Export from bd FIRST, on the source machine, before touching br.**
  **Why:** br `init` writes a fresh SQLite DB and will not see bd's Dolt rows. If
  you init br before exporting, the import has nothing authoritative to merge against.
  - WRONG: `br init --prefix bd && br sync --import-only` (DB empty, no JSONL yet)
  - CORRECT: `bd export --all > .beads/issues.jsonl` THEN `br init` THEN import.

- **Preserve the ID prefix exactly.** **Why:** issue IDs (`bd-abc123`) are embedded
  in commits, dependency links, and chat history. A new prefix orphans every reference.
  - WRONG: `br init --prefix br`
  - CORRECT: `br init --prefix bd` (match whatever bd used).

- **Never delete bd's Dolt data until verification passes.** **Why:** the JSONL
  export is issue-level only — it does NOT carry Dolt branches/commit history. If a
  count or content check fails, the Dolt DB is your only recovery source.

- **`.beads/metadata.json` is host-specific and stays gitignored.** **Why:** it
  encodes the source machine's Dolt server host/user; copying it to a br workspace
  re-points br at a server it never uses. Let br own its own SQLite metadata.

## Why This Exists

bd (Go beads) and br (beads_rust) are two implementations of the same model, but a
real migration is easy to get wrong: bd's canonical store is often Dolt (a remote
SQL server), while br is local SQLite that NEVER runs git. Operators reach for a
file copy or a same-DB switch and silently lose dependency links, comments, or
whole issues. This skill pins the one safe path (export → init → import → verify)
and the exact command-by-command translation so nothing is dropped and no IDs break.

## Quick Start

```bash
# From inside the bd-tracked repo (the one with .beads/)
bd export --all > /tmp/bd-migration.jsonl              # 1. full issue dump from bd
bd count --json                                         # 2. record source truth (save the number)

br init --prefix "$BD_PREFIX"                            # 3. fresh SQLite workspace, SAME prefix
cp /tmp/bd-migration.jsonl .beads/issues.jsonl          # 4. stage the JSONL for br
br sync --import-only                                    # 5. validate + load into SQLite
br count --json                                          # 6. verify count matches step 2
```

If step 6 equals step 2, the migration is data-complete. Then update docs (AGENTS.md)
and re-point automation from `bd …` to `br …` (see command map below).

## Migration Workflow

### Phase 1 — Capture source truth (bd side)
1. `bd export --all > /tmp/bd-migration.jsonl` — `--all` includes everything; the
   default excludes infrastructure beads (agents/rigs/roles/messages) and memories.
   Decide deliberately whether infra beads should travel.
2. Record invariants for the post-check: `bd count --json` (total), and counts by
   status: `bd count --by status --json` if available.
3. Note the prefix in use: it is the leading token of any ID, e.g. `bd-abc123` → `bd`.

**Checkpoint:** the JSONL file is non-empty and the recorded counts are saved. Do
not proceed until you have the source numbers written down.

### Phase 2 — Initialize br (br side)
4. `br init --prefix <prefix>` — `--backend` is ignored (br is always SQLite).
   This creates `.beads/<name>.db`. Do NOT carry over bd's `.beads/metadata.json`.
5. `br config list` to see the new br config surface; bd's config (`export.*`,
   `import.*`, `jira.*`, Dolt server settings) does NOT transfer — br has its own
   keys via `br config get|set`.

### Phase 3 — Import data (br side)
6. Place the export at `.beads/issues.jsonl` (br's default JSONL path).
7. `br sync --import-only` — import-only validates first and rejects malformed JSONL
   or git conflict markers (these guards cannot be bypassed). Fix the JSONL if it
   refuses; do not `--force` your way past a validation error.

**Checkpoint:** `br sync --status` shows the DB and JSONL in agreement.

### Phase 4 — Verify nothing was lost (the gate)
8. Run `scripts/verify-migration.sh <bd-count> <br-prefix>` — it re-counts on the br
   side and diffs against the source number, and spot-checks dependency + comment
   survival. Non-zero exit means STOP and investigate before deleting bd's data.

### Phase 5 — Cut over docs + automation
9. Update `AGENTS.md`/`CLAUDE.md` and any scripts using the command map below.
   Most notably: bd's optional auto-export becomes br's explicit
   `br sync --flush-only`; **git is now YOUR job** (br never commits).

## Output Specification

- **Workspace:** `.beads/` containing a br SQLite DB + `issues.jsonl`, same prefix.
- **Report file:** `bead-tracker-migration-report.md` at repo root, containing: source
  count, target count, delta (must be 0), prefix, and the command-map changes applied.

## Command Map (bd → br)

| Intent | bd | br |
|--------|----|----|
| Find ready work | `bd ready` | `br ready --json` |
| Create issue | `bd create` / `bd q` | `br create` / `br q` |
| Show issue | `bd show <id>` | `br show <id> --json` |
| Update / claim | `bd update <id>` | `br update <id> --status in_progress` |
| Close | `bd close <id>` | `br close <id> --reason "…"` |
| Dependencies | `bd dep` | `br dep` |
| List | `bd list` | `br list --json` |
| Search | `bd search <q>` | `br search <q>` |
| Lint templates | `bd lint` | `br lint` |
| Stats | `bd status` | `br stats` (alias `br status`) |
| Export to JSONL | `bd export` | `br sync --flush-only` |
| Import JSONL | (auto-import config) | `br sync --import-only` |
| Cross-machine sync | `bd dolt push/pull` | git commit `.beads/` (br never runs git) |
| Diagnostics | `bd doctor`* | `br doctor` (`--fix` to repair) |
| Agent docs | — | `br robot-docs guide` |

\* If bd lacks `doctor` in your build, use `bd lint` + `bd status`.

## Exit Codes (verify-migration.sh)

| Code | Meaning |
|------|---------|
| 0 | Counts equal; dependency + comment spot-checks pass. Safe to retire bd. |
| 1 | Count mismatch — issues lost or duplicated. Do NOT delete bd data. |
| 2 | Usage error (missing source count or prefix argument). |
| 3 | br workspace not found / br could not read the DB. |

## Quality Rubric

- **Loss-free:** `br count` equals the recorded `bd count` exactly (delta 0). Verified
  by `scripts/verify-migration.sh` exiting 0, not by eyeballing.
- **IDs preserved:** a known issue ID from bd resolves under br via `br show <id>`.
- **Relations survived:** at least one dependency link and one comment present pre-migration
  are still present (`br dep`, `br show … --json` carry them).
- **Reversible until verified:** bd's Dolt data is untouched until the gate passes.

## Examples

**Standard repo (bd prefix `ag`, Dolt-backed):**
```bash
bd export --all > /tmp/ag.jsonl
N=$(bd count --json | python3 -c 'import sys,json;print(json.load(sys.stdin)["count"])')
br init --prefix ag
cp /tmp/ag.jsonl .beads/issues.jsonl
br sync --import-only
bash scripts/verify-migration.sh "$N" ag; echo "exit=$?"
```

**Including infrastructure beads:** add `--all` to the export (already shown) — omit
it only if you intend to leave agents/rigs/roles/messages behind.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `br sync --import-only` rejects file | git conflict markers or malformed JSON in JSONL | Resolve markers / fix the bad line; import guards are not bypassable |
| br count lower than bd | bd export omitted infra beads/memories | Re-export with `bd export --all`; re-import |
| IDs changed to `br-…` | wrong `--prefix` at init | `br init --force --prefix <bd-prefix>` and re-import |
| br re-points at a remote server | copied bd's `.beads/metadata.json` | Delete it; let br manage its own SQLite metadata |
| `bd export` empty | running outside the bd repo | `cd` into the repo with `.beads/`; confirm `bd status` works |
| Want history, not just issues | JSONL is issue-level only | Keep the bd Dolt DB; JSONL migration does not carry Dolt branches/commits |

## See Also

- `beads-br` — day-to-day br operation (the destination tool).
- `beads` — bd workflow + Dolt remote (the source tool).
- Run `br robot-docs guide` and `br capabilities` for br's machine-readable contracts.
