---
name: post-mortem
description: 'Wrap up completed work. Council validates the implementation, then extract and process learnings. Triggers: "post-mortem", "wrap up", "close epic", "what did we learn".'
---


# Post-Mortem Skill

> **Purpose:** Wrap up completed work — validate it shipped correctly, extract learnings, process the knowledge backlog, activate high-value insights, and retire stale knowledge.
>
> **Runtime note:** Hook-driven closeout is runtime-dependent. Claude/OpenCode can wire Phase 2-5 maintenance through lifecycle hooks. Codex does not expose that hook surface, so `$post-mortem` must own the Codex-native closeout once per thread through `ao codex ensure-stop --auto-extract`.

Six phases:
1. **Council** — Did we implement it correctly?
2. **Extract** — What did we learn?
3. **Process Backlog** — Score, deduplicate, and flag stale learnings
4. **Activate** — Promote high-value learnings to MEMORY.md and constraints
5. **Retire** — Archive stale and superseded learnings
6. **Harvest** — Surface next work for the flywheel

---

## Quick Start

```bash
$post-mortem                    # wraps up recent work
$post-mortem epic-123           # wraps up specific epic
$post-mortem --quick "insight"  # quick-capture single learning (no council)
$post-mortem --process-only     # skip council+extraction, run Phase 3-5 on backlog
$post-mortem --skip-activate    # extract + process but don't write MEMORY.md
$post-mortem --deep recent      # thorough council review
$post-mortem --mixed epic-123   # cross-vendor (Claude + Codex)
$post-mortem --skip-checkpoint-policy epic-123  # skip ratchet chain validation
```

### Codex Closeout

In Codex hookless mode, `$post-mortem` owns closeout during Step 6:

1. Run `ao codex ensure-stop --auto-extract`.
2. Rely on the CLI's same-thread dedupe instead of parsing lifecycle state in
   the skill layer.
3. Use `ao codex status` only when you need to report or verify lifecycle health.

`ao codex ensure-stop` uses the latest transcript or history fallback to queue/persist
learnings and run close-loop maintenance without runtime hooks. Do not stack the
generic `ao session close` + `ao flywheel close-loop` path on top of it in Codex.

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--quick "text"` | off | Quick-capture a single learning directly to `.agents/learnings/` without running a full post-mortem. Formerly handled by `$retro --quick`. |
| `--process-only` | off | Skip council and extraction (Phase 1-2). Run Phase 3-5 on the existing backlog only. |
| `--skip-activate` | off | Extract and process learnings but do not write to MEMORY.md (skip Phase 4 promotions). |
| `--deep` | off | 3 judges (default for post-mortem) |
| `--mixed` | off | Cross-vendor (Claude + Codex) judges |
| `--explorers=N` | off | Each judge spawns N explorers before judging |
| `--debate` | off | Two-round adversarial review |
| `--skip-checkpoint-policy` | off | Skip ratchet chain validation |
| `--skip-sweep` | off | Skip pre-council deep audit sweep |

---

## Quick Mode

Given `$post-mortem --quick "insight text"`:

### Quick Step 1: Generate Slug

Create a slug from the content: first meaningful words, lowercase, hyphens, max 50 chars.

### Quick Step 2: Write Learning Directly

**Write to:** `.agents/learnings/YYYY-MM-DD-quick-<slug>.md`

```markdown
---
type: learning
source: post-mortem-quick
date: YYYY-MM-DD
maturity: provisional
---

# Learning: <Short Title>

**Category**: <auto-classify: debugging|architecture|process|testing|security>
**Confidence**: medium

## What We Learned

<user's insight text>

## Source

Quick capture via `$post-mortem --quick`
```

This skips the full pipeline — writes directly to learnings, no council or backlog processing.

### Quick Step 3: Confirm

```
Learned: <one-line summary>
Saved to: .agents/learnings/YYYY-MM-DD-quick-<slug>.md

For deeper reflection, use `$post-mortem` without --quick.
```

**Done.** Return immediately after confirmation.

---

## Execution Steps

### Pre-Flight Checks

Before proceeding, verify:
1. **Git repo exists:** `git rev-parse --git-dir 2>/dev/null` — if not, error: "Not in a git repository"
2. **Work was done:** `git log --oneline -1 2>/dev/null` — if empty, error: "No commits found. Run $implement first."
3. **Epic context:** If epic ID provided, verify it has closed children. If 0 closed children, error: "No completed work to review."

**If `--process-only`:** Skip Pre-Flight Checks through Step 3. Jump directly to Phase 3: Process Backlog.

### Step 0.4: Load Reference Documents (MANDATORY)

Before Step 0.5 and Step 2.5, load required reference docs into context by reading them:

```
REQUIRED_REFS=(
  "references/checkpoint-policy.md"
  "references/metadata-verification.md"
  "references/closure-integrity-audit.md"
  "references/four-surface-closure.md"
)
```

For each reference file, **read its content** and hold it in context for use in later steps. Do NOT just test file existence with `[ -f ]` -- actually read the content so it is available when Steps 0.5 and 2.5 need it.

If a reference file does not exist (Read returns an error), log a warning and add it as a checkpoint warning in the council context. Proceed only if the missing reference is intentionally deferred.

### Step 0.5: Checkpoint-Policy Preflight (MANDATORY)

Read `references/checkpoint-policy.md` for the full checkpoint-policy preflight procedure. It validates the ratchet chain, checks artifact availability, and runs idempotency checks. BLOCK on prior FAIL verdicts; WARN on everything else.

### Step 1: Identify Completed Work and Record Timing

**Record the post-mortem start time for cycle-time tracking:**
```bash
PM_START=$(date +%s)
```

**If epic/issue ID provided:** Use it directly.

**If no ID:** Find recently completed work:
```bash
# Check for closed beads
bd list --status closed --since "7 days ago" 2>/dev/null | head -5

# Or check recent git activity
git log --oneline --since="7 days ago" | head -10
```

### Step 1.5: RPI Session Metrics

Read `.agents/rpi/rpi-state.json` and extract session ID, phase, verdicts, and streak data. If absent or unparseable, skip silently. Prepend a tweetable summary to the report: `> RPI streak: N consecutive days | Sessions: N | Last verdict: PASS/WARN/FAIL`. See [references/streak-tracking.md](references/streak-tracking.md) for extraction logic and fallback behavior.

### Step 2: Load the Original Plan/Spec

Before invoking council, load the original plan for comparison:

1. **If epic/issue ID provided:** `bd show <id>` to get the spec/description
2. **Search for plan doc:** `ls .agents/plans/ | grep <target-keyword>`
3. **Check git log:** `git log --oneline | head -10` to find the relevant bead reference

If a plan is found, include it in the council packet's `context.spec` field:
```json
{
  "spec": {
    "source": "bead na-0042",
    "content": "<the original plan/spec text>"
  }
}
```

### Step 2.1: Load Compiled Prevention Context

Before council and retro synthesis, load compiled prevention outputs when they exist:

- `.agents/planning-rules/*.md`
- `.agents/pre-mortem-checks/*.md`

Use these compiled artifacts first, then fall back to `.agents/findings/registry.jsonl` only when compiled outputs are missing or incomplete. Carry matched finding IDs into the retro as `Applied findings` / `Known risks applied` context so post-mortem can judge whether the flywheel actually prevented rediscovery.

### Step 2.2: Load Implementation Summary

Check for a crank-generated phase-2 summary:

```bash
PHASE2_SUMMARY=$(ls -t .agents/rpi/phase-2-summary-*-crank.md 2>/dev/null | head -1)
if [ -n "$PHASE2_SUMMARY" ]; then
    echo "Phase-2 summary found: $PHASE2_SUMMARY"
    # Read the summary for implementation context
fi
```

If available, use the phase-2 summary to understand what was implemented, how many waves ran, and which files were modified.

### Step 2.3: Reconcile Plan vs Delivered Scope

Compare the original plan scope against what was actually delivered:

1. Read the plan from `.agents/plans/` (most recent)
2. Compare planned issues against closed issues (`bd children <epic-id>`)
3. Note any scope additions, removals, or modifications
4. Include scope delta in the post-mortem findings

### Step 2.4: Closure Integrity Audit (MANDATORY)

Read `references/closure-integrity-audit.md` for the full procedure. Mechanically verifies:

1. **Evidence precedence per child** — every closed child resolves on the strongest available evidence in this order: `commit`, then `staged`, then `worktree`
2. **Phantom bead detection** — flags children with generic titles ("task") or empty descriptions
3. **Orphaned children** — beads in `bd list` but not linked to parent in `bd show`
4. **Multi-wave regression detection** — for crank epics, checks if a later wave removed code added by an earlier wave
5. **Stretch goal audit** — verifies deferred stretch goals have documented rationale

Include results in the council packet as `context.closure_integrity`. WARN on 1-2 findings, FAIL on 3+.

If a closure is evidence-only or closes before its proving commit exists, emit a proof artifact with `bash skills/post-mortem/scripts/write-evidence-only-closure.sh` and cite the durable tracked copy at `.agents/releases/evidence-only-closures/<target-id>.json` in the council packet. The writer also emits a local council copy at `.agents/council/evidence-only-closures/<target-id>.json`. The packet must record the selected `evidence_mode` plus repo-state detail that distinguishes staged files from broader worktree state so active-session audits stay mechanically replayable.

### Step 2.5: Pre-Council Metadata Verification (MANDATORY)

Read `references/metadata-verification.md` for the full verification procedure. Mechanically checks: plan vs actual files, file existence in commits, cross-references in docs, and ASCII diagram integrity. Failures are included in the council packet as `context.metadata_failures`.

### Step 2.6: Pre-Council Deep Audit Sweep

**Skip if `--quick` or `--skip-sweep`.**

Before council runs, dispatch a deep audit sweep to systematically discover issues across all changed files. This uses the same protocol as `$vibe --deep` — see the deep audit protocol in the vibe skill (`skills/vibe/`) for the full specification.

In summary:

1. Identify all files in scope (from epic commits or recent changes)
2. Chunk files into batches of 3-5 by line count (<=100 lines -> batch of 5, 101-300 -> batch of 3, >300 -> solo)
3. Dispatch up to 8 Explore agents in parallel, each with a mandatory 8-category checklist per file (resource leaks, string safety, dead code, hardcoded values, edge cases, concurrency, error handling, HTTP/web security)
4. Merge all explorer findings into a sweep manifest at `.agents/council/sweep-manifest.md`
5. Include sweep manifest in council packet — judges shift to adjudication mode (confirm/reject/reclassify sweep findings + add cross-cutting findings)

**Why:** Post-mortem council judges exhibit satisfaction bias when reviewing monolithic file sets — they stop at ~10 findings regardless of actual issue count. Per-file explorers with category checklists find 3x more issues, and the sweep manifest gives judges structured input to adjudicate rather than discover from scratch.

**Skip conditions:**
- `--quick` flag -> skip (fast inline path)
- `--skip-sweep` flag -> skip (old behavior: judges do pure discovery)
- No source files in scope -> skip (nothing to audit)

### Step 3: Council Validates the Work

Run `$council` with the **retrospective** preset and always 3 judges:

```
$council --deep --preset=retrospective validate <epic-or-recent>
```

**Default (3 judges with retrospective perspectives):**
- `plan-compliance`: What was planned vs what was delivered? What's missing? What was added?
- `tech-debt`: What shortcuts were taken? What will bite us later? What needs cleanup?
- `learnings`: What patterns emerged? What should be extracted as reusable knowledge?

Post-mortem always uses 3 judges (`--deep`) because completed work deserves thorough review.

**Four-Surface Closure:** Validate all four surfaces -- Code, Documentation, Examples, and Proof. A PASS verdict requires all four surfaces addressed, not just code correctness. Read `references/four-surface-closure.md` for the closure checklist and common gaps.

**Timeout:** Post-mortem inherits council timeout settings. If judges time out,
the council report will note partial results. Post-mortem treats a partial council
report the same as a full report — the verdict stands with available judges.

The plan/spec content is injected into the council packet context so the `plan-compliance` judge can compare planned vs delivered.

**With --quick (inline, no spawning):**
```
$council --quick validate <epic-or-recent>
```
Single-agent structured review. Fast wrap-up without spawning.

**With debate mode:**
```
$post-mortem --debate epic-123
```
Enables adversarial two-round review for post-implementation validation. Use for high-stakes shipped work where missed findings have production consequences. See `$council` docs for full --debate details.

**Advanced options (passed through to council):**
- `--mixed` — Cross-vendor (Claude + Codex) with retrospective perspectives
- `--preset=<name>` — Override with different personas (e.g., `--preset=ops` for production readiness)
- `--explorers=N` — Each judge spawns N explorers to investigate the implementation deeply before judging
- `--debate` — Two-round adversarial review (judges critique each other's findings before final verdict)

### Step 3.5: Prediction Accuracy (Pre-Mortem Correlation)

When a pre-mortem report exists for the current epic (`ls -t .agents/council/*pre-mortem*.md`), cross-reference its prediction IDs against actual vibe/implementation findings. Score each as HIT (prediction confirmed), MISS (did not materialize), or SURPRISE (unpredicted issue). Write a `## Prediction Accuracy` table in the report. Skip silently if no pre-mortem exists. See [references/prediction-tracking.md](references/prediction-tracking.md) for the full table format and scoring rules.

### Phase 2: Extract Learnings

Inline extraction of learnings from the completed work (formerly delegated to the retro skill).

#### Step EX.1: Gather Context

```bash
# Recent commits
git log --oneline -20 --since="7 days ago"

# Epic children (if epic ID provided)
bd children <epic-id> 2>/dev/null | head -20

# Recent plans and research
ls -lt .agents/plans/ .agents/research/ 2>/dev/null | head -10
```

Read relevant artifacts: research documents, plan documents, commit messages, code changes. Read relevant files and use git commands to understand what was done.

**If retrospecting an epic:** Run the closure integrity quick-check from `references/context-gathering.md` (Phantom Bead Detection + Multi-Wave Regression Scan). Include any warnings in findings.

#### Step EX.2: Classify Learnings

Ask these questions:

**What went well?**
- What approaches worked?
- What was faster than expected?
- What should we do again?

**What went wrong?**
- What failed?
- What took longer than expected?
- What would we do differently?

**What did we discover?**
- New patterns found
- Codebase quirks learned
- Tool tips discovered
- Debugging insights
- Test pyramid gaps found during implementation or review

For each learning, capture:
- **ID**: L1, L2, L3...
- **Category**: debugging, architecture, process, testing, security
- **What**: The specific insight
- **Why it matters**: Impact on future work
- **Confidence**: high, medium, low

#### Step EX.3: Write Learnings

**Write to:** `.agents/learnings/YYYY-MM-DD-<topic>.md`

```markdown
---
id: learning-YYYY-MM-DD-<slug>
type: learning
date: YYYY-MM-DD
category: <category>
confidence: <high|medium|low>
maturity: provisional
---

# Learning: <Short Title>

## What We Learned

<1-2 sentences describing the insight>

## Why It Matters

<1 sentence on impact/value>

## Source

<What work this came from>

---

# Learning: <Next Title>

**ID**: L2
...
```

#### Step EX.3.5: Test Pyramid Gap Analysis

Assess test coverage against the pyramid levels defined in the test pyramid standard (`test-pyramid.md` in the standards skill).

**For each closed issue in the epic:**

1. What `test_levels` were planned? (from issue metadata or plan)
2. What test levels actually exist? (grep test files for patterns)
3. What gaps remain?

**Write to post-mortem report** under `## Test Pyramid Assessment`:

```markdown
## Test Pyramid Assessment

| Issue | Planned Levels | Actual Levels | Gaps | Action |
|-------|---------------|---------------|------|--------|
| ag-001 | L0, L1, L2 | L1, L2 | L0 missing | Add contract tests (next-work item) |
| ag-002 | L1 | L1 | none | covered |

**Summary:** N/M issues have full pyramid coverage. K gaps identified.
```

Gaps with severity >= moderate become `next-work.jsonl` items with type `tech-debt` and title `"Add L{N} tests for {component}"`.

#### Step EX.4: Classify Learning Scope

For each learning extracted in Step EX.3, classify:

**Question:** "Does this learning reference specific files, packages, or architecture in THIS repo? Or is it a transferable pattern that helps any project?"

- **Repo-specific** -> Write to `.agents/learnings/` (existing behavior from Step EX.3). Use `git rev-parse --show-toplevel` to resolve repo root — never write relative to cwd.
- **Cross-cutting/transferable** -> Rewrite to remove repo-specific context (file paths, function names, package names), then:
  1. Write abstracted version to `~/.agents/learnings/YYYY-MM-DD-<slug>.md` (NOT local — one copy only)
  2. Run abstraction lint check:
     ```bash
     file="<path-to-written-global-file>"
     grep -iEn '(internal/|cmd/|\.go:|/pkg/|/src/|AGENTS\.md|CLAUDE\.md)' "$file" 2>/dev/null
     grep -En '[A-Z][a-z]+[A-Z][a-z]+\.(go|py|ts|rs)' "$file" 2>/dev/null
     grep -En '\./[a-z]+/' "$file" 2>/dev/null
     ```
     If matches: WARN user with matched lines, ask to proceed or revise. Never block the write.

**Note:** Each learning goes to ONE location (local or global). No `promoted_to` needed — there's no local copy to mark when writing directly to global.

**Example abstraction:**
- Local: "Athena's validate package needs O_CREATE|O_EXCL for atomic claims because Zeus spawns concurrent workers"
- Global: "Use O_CREATE|O_EXCL for atomic file creation when multiple processes may race on the same path"

#### Step EX.5: Write Structured Findings to Registry

Before backlog processing, normalize reusable council findings into `.agents/findings/registry.jsonl`.

Use the tracked contract in `docs/contracts/finding-registry.md`:

- persist only reusable findings that should change future planning or review behavior
- require `dedup_key`, provenance, `pattern`, `detection_question`, `checklist_item`, `applicable_when`, and `confidence`
- `applicable_when` must use the controlled vocabulary from the contract
- append or merge by `dedup_key`
- use the contract's temp-file-plus-rename atomic write rule

This registry is the v1 advisory prevention surface. It complements learnings and next-work; it does not replace them.

#### Step EX.6: Refresh Compiled Prevention Outputs

After the registry mutation, refresh compiled outputs immediately so the same session can benefit from the updated prevention set.

If `hooks/finding-compiler.sh` exists, run:

```bash
bash hooks/finding-compiler.sh --quiet 2>/dev/null || true
```

This promotes registry rows into `.agents/findings/*.md`, refreshes `.agents/planning-rules/*.md` and `.agents/pre-mortem-checks/*.md`, and rewrites draft constraint metadata under `.agents/constraints/`. Active enforcement still depends on the constraint index lifecycle and runtime hook support, but compilation itself is no longer deferred.

### Phase 3: Process Backlog

Score, deduplicate, and flag stale learnings across the full backlog. This phase runs on ALL learnings, not just those extracted in Phase 2.

Read `references/backlog-processing.md` for detailed scoring formulas, deduplication logic, and staleness criteria.

#### Step BP.1: Load Last-Processed Marker

```bash
MARKER=".agents/ao/last-processed"
mkdir -p .agents/ao
if [ ! -f "$MARKER" ]; then
  date -v-30d +%Y-%m-%dT%H:%M:%S 2>/dev/null || date -d "30 days ago" --iso-8601=seconds > "$MARKER"
fi
LAST_PROCESSED=$(cat "$MARKER")
```

#### Step BP.2: Scan Unprocessed Learnings

```bash
find .agents/learnings/ -name "*.md" -newer "$MARKER" -not -path "*/archive/*" -type f | sort
```

If zero files found: report "Backlog empty — no unprocessed learnings" and skip to Phase 4.

#### Step BP.3: Deduplicate

For each pair of unprocessed learnings:
1. Extract `# Learning:` title
2. Normalize: lowercase, strip punctuation, collapse whitespace
3. If two normalized titles share >= 80% word overlap, merge:
   - Keep the file with highest confidence (high > medium > low); if tied, keep most recent
   - Archive the duplicate with a `merged_into:` pointer

#### Step BP.4: Score Each Learning

Compute composite score for each learning:

| Factor | Values | Points |
|--------|--------|--------|
| Confidence | high=3, medium=2, low=1 | 1-3 |
| Citations | default=1, +1 per cite in `.agents/ao/citations.jsonl` | 1+ |
| Recency | <7d=3, <30d=2, else=1 | 1-3 |

**Score = confidence + citations + recency**

#### Step BP.5: Flag Stale

Learnings that are >30 days old AND have zero citations are flagged for retirement in Phase 5.

```bash
# Flag but do not archive yet — Phase 5 handles retirement
if [ "$DAYS_OLD" -gt 30 ] && [ "$CITE_COUNT" -eq 0 ]; then
  echo "STALE: $LEARNING_FILE (${DAYS_OLD}d old, 0 citations)"
fi
```

#### Step BP.6: Report

```
Phase 3 (Process Backlog) Summary:
- N learnings scanned
- N duplicates merged
- N scored (range: X-Y)
- N flagged stale
```

### Phase 4: Activate

Promote high-value learnings and feed downstream systems. Read `references/activation-policy.md` for detailed promotion thresholds and procedures.

**If `--skip-activate` is set:** Skip this phase entirely. Report "Phase 4 skipped (--skip-activate)."

#### Step ACT.1: Promote to MEMORY.md

Learnings with score >= 6 are promoted:
1. Read the learning file
2. Extract title and core insight
3. Check MEMORY.md for duplicate entries (grep for key phrases)
4. If no duplicate: append to `## Key Lessons` in MEMORY.md

```markdown
## Key Lessons
- **<Title>** — <one-line insight> (source: `.agents/learnings/<filename>`)
```

**Important:** Append only. Never overwrite MEMORY.md.

#### Step ACT.2: Re-Run the Finding Compiler Idempotently

If registry rows changed during this post-mortem, rerun the compiler before feeding next-work so downstream sessions read the freshest compiled prevention outputs:

```bash
bash hooks/finding-compiler.sh --quiet 2>/dev/null || true
```

#### Step ACT.3: Feed Next-Work

Actionable improvements identified during processing -> append one schema v1.3
batch entry to `.agents/rpi/next-work.jsonl` using the tracked contract in
[`../../.agents/rpi/next-work.schema.md`](../../.agents/rpi/next-work.schema.md)
and the write procedure in
[`references/harvest-next-work.md`](references/harvest-next-work.md).
Follow the claim/finalize lifecycle documented in `references/harvest-next-work.md`.

```bash
mkdir -p .agents/rpi
# Build VALID_ITEMS via the schema-validation flow in references/harvest-next-work.md
# Then append one entry per post-mortem / epic.
ENTRY_TIMESTAMP="$(date -Iseconds)"
SOURCE_EPIC="${EPIC_ID:-recent}"
VALID_ITEMS_JSON="${VALID_ITEMS_JSON:-[]}"

printf '%s\n' "$(jq -cn \
  --arg source_epic "$SOURCE_EPIC" \
  --arg timestamp "$ENTRY_TIMESTAMP" \
  --argjson items "$VALID_ITEMS_JSON" \
  '{
    source_epic: $source_epic,
    timestamp: $timestamp,
    items: $items,
    consumed: false,
    claim_status: "available",
    claimed_by: null,
    claimed_at: null,
    consumed_by: null,
    consumed_at: null
  }'
)" >> .agents/rpi/next-work.jsonl
```

#### Step ACT.4: Update Marker

```bash
date -Iseconds > .agents/ao/last-processed
```

This must be the LAST action in Phase 4.

#### Step ACT.5: Report

```
Phase 4 (Activate) Summary:
- N promoted to MEMORY.md
- N duplicates merged
- N flagged for retirement
- N constraints compiled
- N improvements fed to next-work.jsonl
```

### Phase 5: Retire Stale

Archive learnings that are no longer earning their keep.

#### Step RET.1: Archive Stale Learnings

Learnings flagged in Phase 3 (>30d old, zero citations):

```bash
mkdir -p .agents/learnings/archive
for f in <stale-files>; do
  mv "$f" .agents/learnings/archive/
  echo "Archived: $f (stale: >30d, 0 citations)"
done
```

#### Step RET.2: Archive Superseded Learnings

Learnings merged during Phase 3 deduplication were already archived with `merged_into:` pointers. Verify the pointers are valid:

```bash
for f in .agents/learnings/archive/*.md; do
  [ -f "$f" ] || continue
  MERGED_INTO=$(grep "^merged_into:" "$f" 2>/dev/null | awk '{print $2}')
  if [ -n "$MERGED_INTO" ] && [ ! -f "$MERGED_INTO" ]; then
    echo "WARN: $f points to missing file: $MERGED_INTO"
  fi
done
```

#### Step RET.3: Clean MEMORY.md References

If any archived learning was previously promoted to MEMORY.md, remove those entries:

```bash
for f in <archived-files>; do
  BASENAME=$(basename "$f")
  # Check if MEMORY.md references this file
  if grep -q "$BASENAME" MEMORY.md 2>/dev/null; then
    echo "WARN: MEMORY.md references archived learning: $BASENAME — consider removing"
  fi
done
```

**Note:** Do not auto-delete MEMORY.md entries. WARN the user and let them decide.

#### Step RET.4: Report

```
Phase 5 (Retire) Summary:
- N stale learnings archived
- N superseded learnings archived
- N MEMORY.md references to review
```

### Step 4: Write Post-Mortem Report

**Write to:** `.agents/council/YYYY-MM-DD-post-mortem-<topic>.md`

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
**Cycle-Time Trend:** <compare against prior post-mortems — is this faster or slower? Check .agents/council/ for prior post-mortem Duration values>

## Council Verdict: PASS / WARN / FAIL

| Judge | Verdict | Key Finding |
|-------|---------|-------------|
| Plan-Compliance | ... | ... |
| Tech-Debt | ... | ... |
| Learnings | ... | ... |

### Implementation Assessment
<council summary>

### Concerns
<any issues found>

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

List discovered footguns — common mistakes or surprising behaviors that cost time:

| Footgun | Impact | Prevention |
|---------|--------|-----------|
| description | how it wasted time | how to prevent |

These entries are promoted to `.agents/learnings/` and injected into future worker prompts to prevent recurrence. Zero-cycle lag between discovery and prevention.

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

## BF Assessment

| Level | Exist? | Bugs | Action |
|-------|--------|------|--------|
| BF1 | y/n | N | property tests |
| BF4 | y/n | N | chaos tests |

## Next Work

| # | Title | Type | Severity | Source | Target Repo |
|---|-------|------|----------|--------|-------------|
| 1 | <title> | tech-debt / improvement / pattern-fix / process-improvement | high / medium / low | council-finding / retro-learning / retro-pattern | <repo-name or *> |

### Recommended Next $rpi
$rpi "<highest-value improvement>"

## Status

[ ] CLOSED - Work complete, learnings captured
[ ] FOLLOW-UP - Issues need addressing (create new beads)
```

### Step 4.5: Synthesize Proactive Improvement Agenda (MANDATORY)

**After writing the post-mortem report, analyze extraction + council context and proactively propose improvements to repo quality and execution quality.**

Read the extraction output (from Phase 2) and the council report (from Step 3). For each learning, ask:
1. **What process does this improve?** (build, test, review, deploy, documentation, automation, etc.)
2. **What's the concrete change?** (new check, new automation, workflow change, tooling improvement)
3. **Is it actionable in one RPI cycle?** (if not, split into smaller pieces)

Coverage requirements:
- Include **ALL** improvements found (no cap).
- Cover all three surfaces:
  - `repo` (code/contracts/docs quality)
  - `execution` (planning/implementation/review workflow)
  - `ci-automation` (validation/tooling reliability)
- Include at least **1 quick win** (small, low-risk, same-session viable).

Write process improvement items with type `process-improvement` (distinct from `tech-debt` or `improvement`). Each item must have:
- `title`: imperative form, e.g. "Add pre-commit lint check"
- `area`: which part of the development process to improve
- `description`: 2-3 sentences describing the change and why retro evidence supports it
- `evidence`: which retro finding or council finding motivates this
- `priority`: P0 / P1 / P2
- `horizon`: now / next-cycle / later
- `effort`: S / M / L

**These items feed directly into Step 5 (Harvest Next Work) alongside council findings. They are the flywheel's growth vector — each cycle makes the system smarter.**

Write this into the post-mortem report under `## Proactive Improvement Agenda`.

Example output:
```markdown
## Proactive Improvement Agenda

| # | Area | Improvement | Priority | Horizon | Effort | Evidence |
|---|------|-------------|----------|---------|--------|----------|
| 1 | ci-automation | Add validation metadata requirement for Go tasks | P0 | now | S | Workers shipped untested code when metadata didn't require `go test` |
| 2 | execution | Add consistency-check finding category in review | P1 | next-cycle | M | Partial refactoring left stale references undetected |
```

### Step 4.6: Prior-Findings Resolution Tracking (MANDATORY)

After Step 4.5, compute and include prior-findings resolution tracking from `.agents/rpi/next-work.jsonl`. Read `references/harvest-next-work.md` for the jq queries that compute totals and per-source resolution rates. Write results into `## Prior Findings Resolution Tracking` in the post-mortem report.

### Step 4.7: Command-Surface Parity Gate (MANDATORY)

Before marking post-mortem complete, enforce command-surface parity for modified CLI commands:

1. Identify modified command files under `cli/cmd/ao/` from the reviewed scope.
2. For each file, record at least one tested run-path (unit/integration/e2e) in `## Command-Surface Parity Checklist`.
3. Any intentionally uncovered command family must be explicitly listed with a reason and follow-up item.

If any modified command file is missing both coverage evidence and an intentional-uncovered rationale, post-mortem cannot be marked complete.

### Step 4.8: Persist Retro History (Trend Tracking)

After writing the post-mortem report, persist a structured summary JSON to `.agents/retro/YYYY-MM-DD-<epic-slug>.json` and append an index line to `.agents/retro/index.jsonl`. When 2+ prior retros exist, compute verdict streak, average cycle time, and learnings-per-retro trend for the report. See [references/retro-history.md](references/retro-history.md) for the full JSON schema, write rules, and trend queries.

### Step 5: Harvest Next Work

Scan the council report and extracted learnings for actionable follow-up items:

1. **Council findings:** Extract tech debt, warnings, and improvement suggestions from the council report (items with severity "significant" or "critical" that weren't addressed in this epic)
2. **Retro patterns:** Extract recurring patterns from learnings that warrant dedicated RPIs (items from "Do Differently Next Time" and "Anti-Patterns to Avoid")
3. **Process improvements:** Include all items from Step 4.5 (type: `process-improvement`). These are the flywheel's growth vector — each cycle makes development more effective.
4. **Footgun entries (REQUIRED):** Extract platform-specific gotchas, surprising API behaviors, or silent-failure modes discovered during implementation. Each must include: trigger condition, observable symptom, and fix. Write as type `pattern-fix` with source `retro-learning`. If a footgun was discovered this cycle, it must appear in this harvest — do not defer.
5. **Write `## Next Work` section** to the post-mortem report:

```markdown
## Next Work

| # | Title | Type | Severity | Source | Target Repo |
|---|-------|------|----------|--------|-------------|
| 1 | <title> | tech-debt / improvement / pattern-fix / process-improvement | high / medium / low | council-finding / retro-learning / retro-pattern | <repo-name or *> |
```

6. **SCHEMA VALIDATION (MANDATORY):** Before writing, validate each harvested item against the tracked contract in [`.agents/rpi/next-work.schema.md`](../../.agents/rpi/next-work.schema.md). Read `references/harvest-next-work.md` for the validation function and write procedure. Drop invalid items; do NOT block the entire harvest.

7. **Write to next-work.jsonl** (canonical path: `.agents/rpi/next-work.jsonl`). Read `references/harvest-next-work.md` for the write procedure (target_repo assignment, claim/finalize lifecycle, JSONL format, required fields).

8. **Do NOT auto-create bd issues.** Report the items and suggest: "Run `$rpi --spawn-next` to create an epic from these items."

If no actionable items found, write: "No follow-up items identified. Flywheel stable."

### Step 6: Feed the Knowledge Flywheel

Post-mortem automatically feeds learnings into the flywheel:

```bash
if command -v ao &>/dev/null; then
  ao forge markdown .agents/learnings/*.md 2>/dev/null
  echo "Learnings indexed in knowledge flywheel"

  # Validate and lock artifacts that passed council review
  ao temper validate --min-feedback 0 .agents/learnings/YYYY-MM-DD-*.md 2>/dev/null || true
  echo "Artifacts validated for tempering"

  if [ -n "${CODEX_THREAD_ID:-}" ] || [ "${CODEX_INTERNAL_ORIGINATOR_OVERRIDE:-}" = "Codex Desktop" ]; then
    # Codex owns closeout through ao codex ensure-stop, not the generic session-close path.
    # The CLI itself is idempotent for the same thread, so no jq/state parsing is required here.
    ao codex ensure-stop --auto-extract 2>/dev/null || true
    echo "Codex closeout ensured for the current thread"
  else
    # Close session and trigger full flywheel close-loop (includes adaptive feedback)
    ao session close 2>/dev/null || true
    ao flywheel close-loop --quiet 2>/dev/null || true
    echo "Session closed, flywheel loop triggered"
  fi
else
  # Learnings are already in .agents/learnings/ from Phase 2.
  # Without ao CLI, grep-based search in $research and $inject
  # will find them directly — no copy to pending needed.

  # Feedback-loop fallback: update confidence for cited learnings
  mkdir -p .agents/ao
  if [ -f .agents/ao/citations.jsonl ]; then
    echo "Processing citation feedback (ao-free fallback)..."
    # Read cited learning files and boost confidence notation
    while IFS= read -r line; do
      CITED_FILE=$(echo "$line" | grep -o '"learning_file":"[^"]*"' | cut -d'"' -f4)
      if [ -f "$CITED_FILE" ]; then
        # Note: confidence boost tracked via citation count, not file modification
        echo "Cited: $CITED_FILE"
      fi
    done < .agents/ao/citations.jsonl
  fi

  # Session-outcome fallback: record this session's outcome
  EPIC_ID="<epic-id>"
  echo "{\"epic\": \"$EPIC_ID\", \"verdict\": \"<council-verdict>\", \"cycle_time_minutes\": 0, \"timestamp\": \"$(date -Iseconds)\"}" >> .agents/ao/outcomes.jsonl

  # Skip ao temper validate (no fallback needed — tempering is an optimization)
  echo "Flywheel fed locally (ao CLI not available — learnings searchable via grep)"
fi
```

### Step 7: Report to User

Tell the user:
1. Council verdict on implementation
2. Key learnings
3. Any follow-up items
4. Location of post-mortem report
5. Knowledge flywheel status
6. **Suggested next `$rpi` command** from the harvested `## Next Work` section (ALWAYS — this is how the flywheel spins itself)
7. ALL proactive improvements, organized by priority (highlight one quick win)
8. Knowledge lifecycle summary (Phase 3-5 stats)

**The next `$rpi` suggestion is MANDATORY, not opt-in.** After every post-mortem, present the highest-severity harvested item as a ready-to-copy command:

```markdown
## Flywheel: Next Cycle

Based on this post-mortem, the highest-priority follow-up is:

> **<title>** (<type>, <severity>)
> <1-line description>

Ready to run:
```
$rpi "<title>"
```

Or see all N harvested items in `.agents/rpi/next-work.jsonl`.
```

If no items were harvested, write: "Flywheel stable — no follow-up items identified."

---

## Integration with Workflow

```
$plan epic-123
    |
    v
$pre-mortem (council on plan)
    |
    v
$implement
    |
    v
$vibe (council on code)
    |
    v
Ship it
    |
    v
$post-mortem              <-- You are here
    |
    |-- Phase 1: Council validates implementation
    |-- Phase 2: Extract learnings (inline)
    |-- Phase 3: Process backlog (score, dedup, flag stale)
    |-- Phase 4: Activate (promote to MEMORY.md, compile constraints)
    |-- Phase 5: Retire stale learnings
    |-- Phase 6: Harvest next work
    |-- Suggest next $rpi --------------------+
                                              |
    +----------------------------------------+
    |  (flywheel: learnings become next work)
    v
$rpi "<highest-priority enhancement>"
```

---

## Examples

### Wrap Up Recent Work

**User says:** `$post-mortem`

**What happens:**
1. Agent scans recent commits.
2. Runs `$council --deep validate recent`.
3. Extracts learnings, processes backlog, and promotes items.
4. Harvests next-work to `.agents/rpi/next-work.jsonl`.

**Result:** Report with learnings, stats, and a suggested `$rpi` command.

### Other Modes

- **Epic-specific:** `$post-mortem ag-5k2` — review against the target plan
- **Quick capture:** `$post-mortem --quick "insight"` — write a learning without council
- **Process-only:** `$post-mortem --process-only` — run backlog processing only
- **Cross-vendor:** `$post-mortem --mixed ag-3b7` — broaden judgment coverage

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Council times out | Epic too large or too many files changed | Split post-mortem into smaller reviews or increase timeout |
| No next-work items harvested | Council found no tech debt or improvements | Flywheel stable — write entry with empty items array to next-work.jsonl |
| Checkpoint-policy preflight blocks | Prior FAIL verdict in ratchet chain without fix | Resolve prior failure (fix + re-vibe) or skip checkpoint-policy via `--skip-checkpoint-policy` |
| Metadata verification fails | Plan vs actual files mismatch or missing cross-references | Include failures in council packet as `context.metadata_failures` — judges assess severity |

---

## See Also

- `../council/SKILL.md` — Multi-model validation council
- `../vibe/SKILL.md` — Council validates code (`$vibe` after coding)
- `../pre-mortem/SKILL.md` — Council validates plans (before implementation)


## Reference Documents

- [references/harvest-next-work.md](references/harvest-next-work.md)
- [references/learning-templates.md](references/learning-templates.md)
- [references/plan-compliance-checklist.md](references/plan-compliance-checklist.md)
- [references/closure-integrity-audit.md](references/closure-integrity-audit.md)
- [references/security-patterns.md](references/security-patterns.md)
- [references/checkpoint-policy.md](references/checkpoint-policy.md)
- [references/metadata-verification.md](references/metadata-verification.md)
- [references/context-gathering.md](references/context-gathering.md)
- [references/output-templates.md](references/output-templates.md)
- [references/backlog-processing.md](references/backlog-processing.md)
- [references/activation-policy.md](references/activation-policy.md)
- [references/prediction-tracking.md](references/prediction-tracking.md)
- [references/retro-history.md](references/retro-history.md)
- [references/streak-tracking.md](references/streak-tracking.md)
- [references/four-surface-closure.md](references/four-surface-closure.md)

## Local Resources

### references/

- [references/activation-policy.md](references/activation-policy.md)
- [references/backlog-processing.md](references/backlog-processing.md)
- [references/checkpoint-policy.md](references/checkpoint-policy.md)
- [references/closure-integrity-audit.md](references/closure-integrity-audit.md)
- [references/context-gathering.md](references/context-gathering.md)
- [references/harvest-next-work.md](references/harvest-next-work.md)
- [references/learning-templates.md](references/learning-templates.md)
- [references/metadata-verification.md](references/metadata-verification.md)
- [references/output-templates.md](references/output-templates.md)
- [references/plan-compliance-checklist.md](references/plan-compliance-checklist.md)
- [references/prediction-tracking.md](references/prediction-tracking.md)
- [references/retro-history.md](references/retro-history.md)
- [references/security-patterns.md](references/security-patterns.md)
- [references/streak-tracking.md](references/streak-tracking.md)

### scripts/

- `scripts/closure-integrity-audit.sh`
- `scripts/preflight-refs.sh`
- `scripts/validate.sh`
- `scripts/write-evidence-only-closure.sh`
