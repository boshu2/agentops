# Post-Mortem Output Templates

Canonical output shapes for `skills/post-mortem/SKILL.md`. When this file and the
skill disagree, follow the skill workflow and the executable/schema surfaces:

- `skills/post-mortem/SKILL.md`
- `skills/post-mortem/scripts/write-evidence-only-closure.sh`
- `schemas/evidence-only-closure.v1.schema.json`

## Post-Mortem Report Template

Write to `.agents/council/YYYY-MM-DD-post-mortem-<topic>.md`:

```markdown
---
id: post-mortem-YYYY-MM-DD-<topic-slug>
type: post-mortem
date: YYYY-MM-DD
source: "[[.agents/plans/YYYY-MM-DD-<plan-slug>]]"
---

# Post-Mortem: <Epic/Topic>

**Epic:** <epic-id or "recent">
**Duration:** <elapsed time from PM_START to now>
**Cycle-Time Trend:** <faster|slower|flat vs prior post-mortems>

## Council Verdict: PASS / WARN / FAIL

| Judge | Verdict | Key Finding |
|-------|---------|-------------|
| Plan-Compliance | ... | ... |
| Tech-Debt | ... | ... |
| Learnings | ... | ... |

### Implementation Assessment
<council summary>

### Concerns
<open issues or residual risks>

## Closure Integrity

| Check | Result | Details |
|-------|--------|---------|
| Evidence Precedence | PASS/WARN/FAIL | N children resolved by commit/staged/worktree, M without evidence |
| Phantom Beads | PASS/WARN | N phantom beads detected |
| Orphaned Children | PASS/WARN | N orphans found |
| Multi-Wave Regression | PASS/FAIL | N regressions detected |
| Stretch Goals | PASS/WARN | N stretch goals closed without rationale |

### Findings
- <specific closure-integrity finding>

## Learnings (from Phase 2)

### What Went Well
- ...

### What Was Hard
- ...

### Do Differently Next Time
- ...

### Patterns to Reuse
- ...

### Anti-Patterns to Avoid
- ...

### Footgun Entries (Required)

| Footgun | Trigger | Symptom | Fix |
|---------|---------|---------|-----|
| Human CLI parsing | Automation parses human-readable CLI output instead of a structured surface such as `--json` | Parser breaks on wording/order changes or mixed prose + JSON output | Use machine-readable output for automation and treat prose output as display-only |
| Hook git env leakage | Hook/helper shell inherits `GIT_DIR`, `GIT_WORK_TREE`, or `GIT_COMMON_DIR` from another repo/worktree | Git resolves the wrong repo or reports misleading dirty-state evidence | Unset git discovery env before repo resolution, then rerun git from the intended cwd |

## Knowledge Lifecycle

### Backlog Processing (Phase 3)
- Scanned: N learnings
- Merged: N duplicates
- Flagged stale: N

### Activation (Phase 4)
- Promoted to MEMORY.md: N
- Constraints compiled: N
- Next-work items fed: N

### Retirement (Phase 5)
- Archived: N learnings

## Proactive Improvement Agenda

| # | Area | Improvement | Priority | Horizon | Effort | Evidence |
|---|------|-------------|----------|---------|--------|----------|
| 1 | repo / execution / ci-automation | ... | P0/P1/P2 | now/next-cycle/later | S/M/L | ... |

## Prior Findings Resolution Tracking

| Metric | Value |
|---|---|
| Backlog entries analyzed | ... |
| Prior findings total | ... |
| Resolved findings | ... |
| Unresolved findings | ... |
| Resolution rate | ...% |

| Source Epic | Findings | Resolved | Unresolved | Resolution Rate |
|---|---:|---:|---:|---:|
| ... | ... | ... | ... | ...% |

## Command-Surface Parity Checklist

| Command File | Run-path Covered by Test? | Evidence (file:line or test name) | Intentionally Uncovered? | Reason |
|---|---|---|---|---|
| cli/cmd/ao/<command>.go | yes/no | ... | yes/no | ... |

## Next Work

| # | Title | Type | Severity | Source | Target Repo |
|---|-------|------|----------|--------|-------------|
| 1 | <title> | tech-debt / improvement / pattern-fix / process-improvement | high / medium / low | council-finding / retro-learning / retro-pattern | <repo-name or *> |

### Recommended Next /rpi
/rpi "<highest-value improvement>"

## Status

[ ] CLOSED - Work complete, learnings captured
[ ] FOLLOW-UP - Issues need addressing (create new beads)
```

Notes:

- Populate `## Next Work` before `### Recommended Next /rpi`. The suggestion must come from harvested items, not pre-harvest speculation.
- If no items are harvested, keep the report explicit: `Flywheel stable - no follow-up items identified.`
- Footgun entries are not optional flavor text. They are harvest inputs for `pattern-fix` items when the cycle discovered real operator/runtime gotchas.
- Evidence-only or pre-commit closure packets must preserve the durable tracked copy at `.agents/releases/evidence-only-closures/<target-id>.json`. The writer also emits a local council copy at `.agents/council/evidence-only-closures/<target-id>.json`.

Repo-state excerpt that every full evidence-only closure packet must include
(the full schema-shaped template follows):

```json
{
  "repo_state": {
    "repo_root": "<repo-root-or-.>"
  }
}
```

## Evidence-Only Closure Artifact

When a post-mortem closes an item on validation or policy evidence without a
code diff, write a proof artifact to
`.agents/council/evidence-only-closures/<target-id>.json`.

Example producer command:

```bash
bash skills/post-mortem/scripts/write-evidence-only-closure.sh \
  --target-id na-a7e.4 \
  --target-type issue \
  --producer post-mortem \
  --evidence-mode auto \
  --validation-command "bash tests/hooks/lib-hook-helpers.bats" \
  --evidence-summary "Structured proof artifact added and schema validation passed." \
  --artifact ".agents/council/2026-03-09-post-mortem-na-a7e.md"
```

Template shape:

```json
{
  "$schema": "../../../schemas/evidence-only-closure.v1.schema.json",
  "schema_version": 1,
  "artifact_id": "evidence-only-closure-<target-id>",
  "target_id": "<target-id>",
  "target_type": "issue",
  "created_at": "YYYY-MM-DDTHH:MM:SSZ",
  "producer": "post-mortem",
  "evidence_mode": "commit|staged|worktree",
  "validation_commands": ["bash <command>"],
  "repo_state": {
    "repo_root": "/abs/path/to/repo",
    "git_branch": "main",
    "git_dirty": true,
    "head_sha": "<sha>",
    "modified_files": [],
    "staged_files": [],
    "unstaged_files": [],
    "untracked_files": []
  },
  "evidence": {
    "summary": "<why evidence-only closure is valid>",
    "artifacts": ["<supporting-path>"],
    "notes": ["<optional note>"]
  }
}
```

Mode guidance:

- `commit`: commit-backed evidence exists and wins for the closed item
- `staged`: no qualifying commit evidence exists, but the scoped files are staged
- `worktree`: neither commit nor staged evidence exists, but qualifying unstaged or untracked files exist

`auto` is only the selection input to the writer script. The emitted artifact must
record the resolved mode.
