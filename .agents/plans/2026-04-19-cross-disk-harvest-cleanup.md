---
title: Cross-Disk .agents/ Harvest Cleanup
date: 2026-04-19
goal: Execute all suggested next-work from the 2026-04-19 cross-disk harvest report
scope: Knowledge-store hygiene + one CLI packaging fix. No code features. No changes to rig source code.
Applied findings: f-2026-04-14-002 (durable citation paths), f-2026-04-14-001 (N/A — no Go command refactor), f-2026-04-10-001 (N/A — no CLI output-mode changes)
template: Standard
---

# Plan — Cross-Disk .agents/ Harvest Cleanup

## Context

On 2026-04-19, a cross-disk harvest walked 92 `.agents/` directories across
WSL home, `/mnt/c`, and `/mnt/d`. Source catalog:

- `.agents/harvest/latest.json` (committed machine-readable catalog)
- `.agents/harvest/latest.md` (human report)
- `.agents/harvest/cross-disk-2026-04-19T19-52-54.json` (timestamped snapshot)

The report surfaced four actionable cleanups and one CLI-packaging bug.

## Root-cause update (discovered during planning)

The installed `ao` binary at `~/.local/bin/ao` is **v2.26.1 (built 2026-03-20)**.
The source tree at `cli/cmd/ao/harvest.go` (321 LOC) plus
`cli/cmd/ao/harvest_test.go` (715 LOC) **already implements** the `harvest`
subcommand, wired at `harvest.go:38`. The skill-to-binary drift is a stale
install, not missing code. Rebuild unblocks the whole workflow.

## Boundaries

- **Must:** operate read-only except in the specific write paths below;
  preserve git history (`git mv` only, never delete-then-recreate); keep
  destructive actions behind explicit approval gates.
- **Must not:** touch `~/.agents/` authored content without dedup archiving;
  delete anything under `/mnt/d/archive/` beyond AppleDouble metadata; mutate
  git state of cloned external repos; clobber `~/.wslconfig`; touch `k3d`
  clusters `bb-prod` / `bb-staging`.
- **Scope floor:** exclude `/mnt/c/Users/Boful/AppData/Local/Temp/sync-gemini-home-*`
  temp dirs — garbage, not knowledge.

## Baseline Audit

Hard numbers from `.agents/harvest/latest.json`:

| Metric | Value |
|---|---|
| `.agents/` dirs scanned | 92 |
| Artifact `.md` files | 1,642 |
| Unique content hashes | 968 (59%) |
| Duplicate clusters | 324 |
| Files in duplicate clusters | 998 (41%) |
| Phantom AppleDouble files (top two clusters) | ~212 (165+47) |
| Cross-tier real clusters (wsl_live ↔ D_archive) | 22 |
| Platform-lab double-checkout identical files | 161 per checkout |
| wsl_live total files | 971 across 39 rigs |
| D_archive total files | 532 across 43 rigs |
| `~/.agents` hub (promotion target) | 123 files (96 learnings + 12 patterns + 15 research) |

CLI source vs. installed binary:
- Source: `cli/cmd/ao/harvest.go` — 321 LOC, wired at line 38
- Installed: `~/.local/bin/ao` v2.26.1, built 2026-03-20 (pre-harvest)

## Issues

### T1 — Rebuild and install `ao` binary from HEAD (unblocks T4/T5)

**Goal:** Replace the stale Mar 20 binary with a current-main build so
`ao harvest`, `ao dedup --merge`, `ao mine --sources agents` are available.

**Commands:**
```bash
cd /home/boful/dev/personal/agentops/cli
make build                      # produces cli/bin/ao
install -m 0755 bin/ao ~/.local/bin/ao
ao version                      # verify != v2.26.1
ao harvest --help               # verify subcommand present
```

**Acceptance criteria:**
- `ao version` reports a version string different from `v2.26.1-3-g7eb02a13`
- `ao harvest --help` exits 0 and lists `--roots`, `--promote-to`,
  `--min-confidence`, `--include`, `--output-dir`
- `ao dedup --help` still works (no regression)

**Files touched:** `~/.local/bin/ao` (write). No repo edits.

**Risk:** Low. `cli/bin/ao` is gitignored; installed binary is a personal
user file.

---

### T2 — Delete AppleDouble metadata under `/mnt/d/archive/mac-extract/`

**Goal:** Remove macOS resource-fork files (`._*.md`, `._*.json`, `._*`)
that Finder wrote during Mac-to-Windows transfer. They carry no content,
dominate dedup noise (top two duplicate clusters: 165-copy + 47-copy),
and inflate catalog size.

**Pre-check (read-only):**
```bash
find /mnt/d/archive/mac-extract -name "._*" 2>/dev/null | wc -l
find /mnt/d/archive/mac-extract -name "._*" -size +1k 2>/dev/null \
  | head -20   # surface any non-trivial ones before deletion
```

**Action:**
```bash
find /mnt/d/archive/mac-extract -name "._*" -type f -delete
```

**Acceptance criteria:**
- Post-delete: `find /mnt/d/archive/mac-extract -name "._*" | wc -l` returns 0
- Re-running the harvest walker (`python3 /tmp/harvest_all.py < /tmp/agents_dirs.txt`)
  shows the two phantom clusters (165-copy `._2025-12-27-agent-evolution...`
  and 47-copy `._2026-04-04-quick-operator-kernel...`) gone from
  `top_duplicate_clusters`.

**Files touched:** cold-archive metadata only. No wsl_live or `~/.agents`.

**Risk:** Low. AppleDouble files are platform metadata, safe to delete
on Windows; confirmed non-content by normalization output (all normalize
to empty body).

---

### T3 — Consolidate platform-lab double-checkout

**Goal:** Resolve the two inodes holding identical platform-lab `.agents/`
content:
- `/home/boful/dev/personal/platform-lab/.agents` (inode 840552)
- `/home/boful/gt/platform-lab/.agents` (inode 148757)

Both hold 161 files with identical distributions (73 learnings + 79 research
+ 9 findings). Not a symlink or bind mount — genuine duplicate checkout.

**Pre-check (read-only, per vault rule "Don't mutate git state of cloned
external repos"):**
```bash
cd /home/boful/dev/personal/platform-lab && git status && git remote -v && git log -1
cd /home/boful/gt/platform-lab             && git status && git remote -v && git log -1
diff -rq /home/boful/dev/personal/platform-lab/.agents \
         /home/boful/gt/platform-lab/.agents | head
```

**Decision gate (HUMAN):** Present to user:
- Which checkout is canonical? (expected: `dev/personal/platform-lab` per
  standard `dev/` layout; `gt/` houses worktrees/clones)
- Is either dirty (uncommitted work)?
- Is one a worktree of the other?

**Action (ONLY after approval; non-destructive):**
```bash
# Example if dev/personal/platform-lab is canonical:
mv /home/boful/gt/platform-lab /home/boful/gt/.archive/platform-lab-$(date +%Y%m%d)
# NOT rm -rf. Keep the archived copy for one month before deletion.
```

**Acceptance criteria:**
- Harvest walker reports only ONE `platform-lab/.agents` entry in wsl_live
- Canonical checkout's `git status` unchanged (no lost work)
- Archived copy retrievable from `~/gt/.archive/`

**Files touched:** `~/gt/platform-lab/` → `~/gt/.archive/platform-lab-<date>/`
(move, not delete). No edits to either checkout's content.

**Risk:** Medium. Conflates with "Don't mutate git state of cloned external
repos" rule — mitigated by moving to archive rather than deleting, and by
gating on `git status` clean.

---

### T4 — Run `ao harvest --dry-run` and reconcile with Python walker

**Goal:** Confirm the rebuilt `ao harvest` produces a catalog consistent
with the Python walker's output. This validates the CLI path and generates
the canonical `.agents/harvest/latest.json` that the skill expects.

**Depends on:** T1 (need rebuilt binary), T2 (cleaner catalog), T3
(de-duplicated platform-lab).

**Commands:**
```bash
cd /home/boful/dev/personal/agentops
ao harvest --dry-run --quiet \
  --roots /home/boful/,/mnt/c/Users/Boful/,/mnt/d/archive/mac-extract/ \
  --include learnings,patterns,research
# Compare rig count and duplicate cluster count to Python walker:
jq '.rig_count, .duplicate_clusters, .unique_content_hashes' \
   .agents/harvest/latest.json
```

**Acceptance criteria:**
- `ao harvest --dry-run` completes without error
- `.agents/harvest/latest.json` written by `ao` (overwrites Python
  walker's version — acceptable; Python output is archived in
  `cross-disk-2026-04-19T19-52-54.json`)
- Rig count within ±5 of 92 (ao may skip tool-cache dirs by default)
- Duplicate cluster count within ±10% of 324 after T2 cleanup
- If delta >10%, investigate; do NOT proceed to T5

**Files touched:** `.agents/harvest/latest.json` (overwrite, per skill
contract). Previous Python catalog preserved under its timestamped name.

**Risk:** Low. Dry-run only.

---

### T5 — Execute harvest promotion with approval gate

**Goal:** Promote high-confidence cross-rig learnings/patterns to
`~/.agents/learnings/` and archive duplicates. This is the actual
knowledge-flywheel outcome the skill was designed to produce.

**Depends on:** T4 (dry-run validated).

**Commands:**
```bash
# Preview what would be promoted:
ao harvest --dry-run --min-confidence 0.7 \
  --roots /home/boful/gt/,/home/boful/dev/personal/ \
  --promote-to /home/boful/.agents/learnings
# Human-review the promotion list in .agents/harvest/latest.json
# THEN execute (no --dry-run):
ao harvest --min-confidence 0.7 \
  --roots /home/boful/gt/,/home/boful/dev/personal/ \
  --promote-to /home/boful/.agents/learnings
# Post-clean dedup the promotion target:
ao dedup --merge </dev/null  # runs in CWD .agents/ — must cd to ~ first
cd /home/boful && ao dedup --merge
```

**Acceptance criteria:**
- `~/.agents/learnings/` file count grows by a bounded number (expected:
  10-30 promoted, based on 22 cross-tier real clusters)
- Every promoted file has traceable provenance in `.agents/harvest/latest.json`
- `ao dedup --merge` archives duplicates to `~/.agents/archive/dedup/`
  (per `ao dedup --help` behavior — not deleted)
- `ao doctor` still passes

**Files touched:**
- `~/.agents/learnings/` (new files written)
- `~/.agents/archive/dedup/` (duplicate archive)
- Per-rig `.agents/archive/dedup/` if `ao dedup --merge` runs cross-rig

**Risk:** Medium. Writes to authored content. Gated by explicit approval
and by archiving (not deleting) duplicates.

---

### T6 — Commit harvest catalog and plan; file skill-drift issue

**Goal:** Make this session's artifacts durable. Satisfy planning rule
f-2026-04-14-002 (plan evidence must live in git, not ephemeral
`.agents/` paths).

**Commands:**
```bash
cd /home/boful/dev/personal/agentops
git add .agents/harvest/ .agents/plans/2026-04-19-cross-disk-harvest-cleanup.md
git status
# Commit after T1-T5 complete OR if user requests an intermediate
# checkpoint after T1+T2+T3 (hygiene-only).
```

**Separate issue to file (via bd or markdown backlog):**

> **Skill drift: `/harvest` skill assumes rebuilt `ao` binary**
>
> The harvest skill at
> `~/.claude/plugins/cache/agentops-marketplace/agentops/2.37.2/skills/harvest/SKILL.md`
> invokes `ao harvest --roots ...`. The command exists in source
> (`cli/cmd/ao/harvest.go:38`) but installed binaries older than ~Mar 21
> 2026 lack it. Either:
> - Add a pre-flight check to the skill: `ao harvest --help` with
>   fallback messaging, OR
> - Bump the install script to hard-refuse old binaries, OR
> - Document the build requirement in the skill's Troubleshooting
>   table.

**Acceptance criteria:**
- Harvest catalog JSON + plan markdown committed to `main` (or a feature
  branch pending review)
- Skill-drift issue filed (bead if `bd init` run, else appended to
  `.agents/plans/` backlog)

**Files touched:** `.agents/harvest/*.json`, `.agents/harvest/*.md`,
`.agents/plans/2026-04-19-cross-disk-harvest-cleanup.md` (all staged for commit)

**Risk:** Low. Commit only.

## Files-to-Modify Matrix

| Task | File / Path | Access | Notes |
|---|---|---|---|
| T1 | `/home/boful/dev/personal/agentops/cli/` | read | `make build` reads source |
| T1 | `/home/boful/dev/personal/agentops/cli/bin/ao` | write | gitignored build output |
| T1 | `/home/boful/.local/bin/ao` | write | binary install |
| T2 | `/mnt/d/archive/mac-extract/**/._*` | delete | AppleDouble metadata only |
| T3 | `/home/boful/dev/personal/platform-lab/.git` | read | git status check |
| T3 | `/home/boful/gt/platform-lab/.git` | read | git status check |
| T3 | `/home/boful/gt/platform-lab/` | move | to `~/gt/.archive/platform-lab-<date>` |
| T4 | `/home/boful/.agents/` | read | cross-rig walk |
| T4 | `/home/boful/gt/**/.agents/` | read | cross-rig walk |
| T4 | `/home/boful/dev/personal/**/.agents/` | read | cross-rig walk |
| T4 | `/mnt/d/archive/mac-extract/**/.agents/` | read | cross-rig walk |
| T4 | `~/dev/personal/agentops/.agents/harvest/latest.json` | write | dry-run catalog |
| T5 | `/home/boful/.agents/learnings/` | write | promotion target |
| T5 | `/home/boful/.agents/archive/dedup/` | write | duplicate archive |
| T5 | `**/.agents/archive/dedup/` | write | per-rig dedup archive |
| T6 | `~/dev/personal/agentops/.agents/harvest/` | git add | commit catalog |
| T6 | `~/dev/personal/agentops/.agents/plans/2026-04-19-cross-disk-harvest-cleanup.md` | git add | commit plan |

No same-wave `write` collisions. T4's `write` to `latest.json` runs
in Wave 2 (alone). T5 writes to the hub, which no other wave touches.

## Execution Order (Waves)

```
Wave 1 (parallel, independent):
  ├─ T1: Rebuild + install ao binary
  ├─ T2: Delete AppleDouble metadata
  └─ T3: Consolidate platform-lab checkouts  (gated on user approval)

Wave 2 (after Wave 1):
  └─ T4: ao harvest --dry-run reconciliation  (needs T1 + cleaner tree)

Wave 3 (after Wave 2 user approval):
  └─ T5: Execute promotion + dedup           (writes to ~/.agents)

Wave 4 (final):
  └─ T6: Commit catalog + plan + file drift issue
```

## Planning Rules Compliance

| Rule | Status | Justification |
|---|---|---|
| PR-001 (mechanical enforcement) | PASS | Acceptance criteria are grep/diff/count commands, not judgment calls |
| PR-002 (external validation) | PASS | `ao version`, `ao harvest --help`, `ao doctor` are the external validators |
| PR-003 (feedback loops) | PASS | T4 (dry-run) gates T5 (destructive promotion); T5 output validates against T4 preview |
| PR-004 (separation) | PASS | Hygiene (T2, T3) separated from CLI fix (T1); reconciliation (T4) separated from promotion (T5) |
| PR-005 (process gates) | PASS | Two human-approval gates: T3 (platform-lab decision) and T5 (promotion preview) |
| PR-006 (cross-layer consistency) | PASS | Source `cli/cmd/ao/harvest.go` and skill `harvest/SKILL.md` reconciled by T1 + T6 |
| PR-007 (phased rollout) | PASS | Four waves; wave 1 is reversible, wave 2 is dry-run, wave 3 gated, wave 4 is commit-only |
| f-2026-04-14-001 (paired command/test diff) | N/A | No `cli/cmd/ao/*.go` source changes in this plan |
| f-2026-04-14-002 (durable citation paths) | PASS | T6 commits catalog and plan to git before any downstream bead closes on their evidence |
| f-2026-04-10-001 (json mode preserves writes) | N/A | No `--json` output-mode changes |

## Verification

After T6 commits:
```bash
# 1. Catalog is durable
git log -1 --stat .agents/harvest/ .agents/plans/2026-04-19-cross-disk-harvest-cleanup.md

# 2. CLI works end-to-end
ao harvest --dry-run --quiet --roots /home/boful/gt/ | head
ao dedup --help | grep -- --merge

# 3. Hub is consistent
ls -la /home/boful/.agents/learnings/ | wc -l
ls -la /home/boful/.agents/archive/dedup/ 2>/dev/null || echo "no dedup archive yet — OK"

# 4. AppleDouble gone
find /mnt/d/archive/mac-extract -name "._*" | wc -l   # expect 0

# 5. platform-lab single canonical
ls -d /home/boful/*/platform-lab/.agents 2>/dev/null
ls -d /home/boful/gt/platform-lab/.agents 2>/dev/null  # expect: missing
```

## Post-Merge Cleanup

- Delete `~/gt/.archive/platform-lab-<date>` after 30 days IF T3 produced
  no regret
- Regenerate `.agents/harvest/latest.json` via `ao harvest --dry-run` on
  a cadence (weekly?) to measure knowledge compounding
- If T5 promotes few items (<5), consider lowering `--min-confidence`
  to 0.5 for a follow-up pass
- Author or update the harvest skill's Troubleshooting table to point
  at the rebuild step when `ao harvest` returns "unknown command"

## Next Steps

1. Review this plan, approve (or revise).
2. Run `/pre-mortem` if desired (optional — hygiene work, low blast radius).
3. Start with Wave 1 (T1 + T2 in parallel; T3 after user confirms
   canonical checkout).
