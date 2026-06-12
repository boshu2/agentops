## Execution Steps

### Pre-Flight Checks

Before proceeding, verify:
1. **Git repo exists:** `git rev-parse --git-dir 2>/dev/null` — if not, error: "Not in a git repository"
2. **Work was done:** `git log --oneline -1 2>/dev/null` — if empty, error: "No commits found. Run /implement first."
3. **Epic context:** If epic ID provided, verify it has closed children. If 0 closed children, error: "No completed work to review."

**If `--process-only`:** Skip Pre-Flight Checks through Step 3. Jump directly to Phase 3: Process Backlog.

### Step 0.4: Load Reference Documents (MANDATORY)

Before Step 0.5 and Step 2.5, load required reference docs into context using the Read tool:

```
REQUIRED_REFS=(
  "skills/post-mortem/references/checkpoint-policy.md"
  "skills/post-mortem/references/metadata-verification.md"
  "skills/post-mortem/references/closure-integrity-audit.md"
  "skills/post-mortem/references/four-surface-closure.md"
)
```

For each reference file, use the **Read tool** to load its content and hold it in context for use in later steps. Do NOT just test file existence with `[ -f ]` -- actually read the content so it is available when Steps 0.5 and 2.5 need it.

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

Read `.agents/rpi/rpi-state.json` and extract session ID, phase, verdicts, and streak data. If absent or unparseable, skip silently. Prepend a tweetable summary to the report: `> RPI streak: N consecutive days | Sessions: N | Last verdict: PASS/WARN/FAIL`. See [streak-tracking.md](streak-tracking.md) for extraction logic and fallback behavior.

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
    # Read the summary with the Read tool for implementation context
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

If a closure is evidence-only, emit a proof artifact with `bash skills/post-mortem/scripts/write-evidence-only-closure.sh` and cite at `.agents/releases/evidence-only-closures/<target-id>.json`. Record `evidence_mode` plus repo-state detail for replayability. A valid durable packet is acceptable audit evidence even when the child intentionally has no scoped-file section.

### Step 2.5: Pre-Council Metadata Verification (MANDATORY)

Read `references/metadata-verification.md` for the full verification procedure. Mechanically checks: plan vs actual files, file existence in commits, cross-references in docs, and ASCII diagram integrity. Failures are included in the council packet as `context.metadata_failures`.

### Step 2.6: Pre-Council Deep Audit Sweep

**Skip if `--quick` or `--skip-sweep`.**

Before council runs, dispatch a deep audit sweep to systematically discover issues across all changed files. This uses the same protocol as `/validate --deep` — see the deep audit protocol in the vibe skill (`skills/validate/`) for the full specification.

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

## Council Verdict:

Run `/council` with the **retrospective** preset and always 3 judges:

```
/council --deep --preset=retrospective validate <epic-or-recent>
```

**Default (3 judges with retrospective perspectives):**
- `plan-compliance`: What was planned vs what was delivered? What's missing? What was added?
- `tech-debt`: What shortcuts were taken? What will bite us later? What needs cleanup?
- `learnings`: What patterns emerged? What should be extracted as reusable knowledge?

Post-mortem always uses 3 judges (`--deep`) because completed work deserves thorough review.

**Four-Surface Closure:** Validate all four surfaces -- Code, Documentation, Examples, and Proof. A PASS verdict requires all four surfaces addressed, not just code correctness. Read `skills/post-mortem/references/four-surface-closure.md` for the closure checklist and common gaps.

**Timeout:** Post-mortem inherits council timeout settings. If judges time out,
the council report will note partial results. Post-mortem treats a partial council
report the same as a full report — the verdict stands with available judges.

The plan/spec content is injected into the council packet context so the `plan-compliance` judge can compare planned vs delivered.

**With --quick (inline, no spawning):**
```
/council --quick validate <epic-or-recent>
```
Single-agent structured review. Fast wrap-up without spawning.

**With debate mode:**
```
/post-mortem --debate epic-123
```
Enables adversarial two-round review for post-implementation validation. Use for high-stakes shipped work where missed findings have production consequences. See `/council` docs for full --debate details.

**Advanced options (passed through to council):**
- `--mixed` — Cross-vendor (Claude + Codex) with retrospective perspectives
- `--preset=<name>` — Override with different personas (e.g., `--preset=ops` for production readiness)
- `--explorers=N` — Each judge spawns N explorers to investigate the implementation deeply before judging
- `--debate` — Two-round adversarial review (judges critique each other's findings before final verdict)

### Step 3.5: Prediction Accuracy (Pre-Mortem Correlation)

When a pre-mortem report exists for the current epic (`ls -t .agents/council/*pre-mortem*.md`), cross-reference its prediction IDs against actual vibe/implementation findings. Score each as HIT (prediction confirmed), MISS (did not materialize), or SURPRISE (unpredicted issue). Write a `## Prediction Accuracy` table in the report. Skip silently if no pre-mortem exists. See [prediction-tracking.md](prediction-tracking.md) for the full table format and scoring rules.
