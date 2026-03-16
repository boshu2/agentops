---
name: vibe
description: 'Comprehensive code validation. Runs cyclomatic complexity analysis, validates against constraints, then convenes a multi-model council to evaluate code quality and ship-readiness. Triggers: "vibe", "validate code", "check code", "review code", "code quality", "is this ready".'
skill_api_version: 1
metadata:
  tier: judgment
  dependencies:
    - council    # multi-model judgment
    - complexity # complexity analysis
    - bug-hunt   # proactive code audit
    - standards  # loaded for language-specific context
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: full
output_contract: skills/council/schemas/verdict.json
---

# Vibe Skill

> **Purpose:** Is this code ready to ship?

Three steps:
1. **Complexity analysis** — Find hotspots (radon, gocyclo)
2. **Bug hunt audit** — Systematic sweep for concrete bugs
3. **Council validation** — Multi-model judgment

---

## Quick Start

```bash
/vibe                                    # validates recent changes
/vibe recent                             # same as above
/vibe src/auth/                          # validates specific path
/vibe --quick recent                     # fast inline check, no agent spawning
/vibe --deep recent                      # 3 judges instead of 2
/vibe --sweep recent                     # deep audit: per-file explorers + council
/vibe --mixed recent                     # cross-vendor (Claude + Codex)
/vibe --preset=security-audit src/auth/  # security-focused review
/vibe --explorers=2 recent               # judges with explorer sub-agents
/vibe --debate recent                    # two-round adversarial review
```

---

## Execution Steps

### Crank Checkpoint Detection

Before scanning for changed files via git diff, check if a crank checkpoint exists:

```bash
if [ -f .agents/vibe-context/latest-crank-wave.json ]; then
    echo "Crank checkpoint found — using files_changed from checkpoint"
    FILES_CHANGED=$(jq -r '.files_changed[]' .agents/vibe-context/latest-crank-wave.json 2>/dev/null)
    WAVE_COUNT=$(jq -r '.wave' .agents/vibe-context/latest-crank-wave.json 2>/dev/null)
    echo "Wave $WAVE_COUNT checkpoint: $(echo "$FILES_CHANGED" | wc -l | tr -d ' ') files changed"
fi
```

When a crank checkpoint is available, use its `files_changed` list instead of re-detecting via `git diff`. This ensures vibe validates exactly the files that crank modified.

### Step 1: Determine Target

**If target provided:** Use it directly.

**If no target or "recent":** Auto-detect from git:
```bash
# Check recent commits
git diff --name-only HEAD~3 2>/dev/null | head -20
```

If nothing found, ask user.

**Pre-flight: If no files found:**
Return immediately with: "PASS (no changes to review) — no modified files detected."
Do NOT spawn agents for empty file lists.

### Step 1.5: Fast Path (--quick mode)

**`--quick` skips Steps 2.5, 2a–2g, 2h, and 3** (prior findings, constraint tests, metadata checks, OL validation, codex review, knowledge search, bug hunt, product context, spec loading) — jumping directly to Step 4 with inline council. These steps add 30–90s of pre-processing that aren't worth the cost for a single inline reviewer.

**Always run in `--quick`:** Step 2 (complexity — cheap and informative), Step 2.3 (domain checklists — cheap and high-value), Step 2.4 (finding registry — mandatory context), Step 2g (test pyramid — cheap file existence checks).

### Step 2: Run Complexity Analysis

**Detect language and run appropriate tool:**

**For Python:**
```bash
# Check if radon is available
mkdir -p .agents/council
echo "$(date -Iseconds) preflight: checking radon" >> .agents/council/preflight.log
if ! which radon >> .agents/council/preflight.log 2>&1; then
  echo "⚠️ COMPLEXITY SKIPPED: radon not installed (pip install radon)"
  # Record in report that complexity was skipped
else
  # Run cyclomatic complexity
  radon cc <path> -a -s 2>/dev/null | head -30
  # Run maintainability index
  radon mi <path> -s 2>/dev/null | head -30
fi
```

**For Go:**
```bash
# Check if gocyclo is available
echo "$(date -Iseconds) preflight: checking gocyclo" >> .agents/council/preflight.log
if ! which gocyclo >> .agents/council/preflight.log 2>&1; then
  echo "⚠️ COMPLEXITY SKIPPED: gocyclo not installed (go install github.com/fzipp/gocyclo/cmd/gocyclo@latest)"
  # Record in report that complexity was skipped
else
  # Run complexity analysis
  gocyclo -over 10 <path> 2>/dev/null | head -30
fi
```

**For other languages:** Skip complexity with explicit note: "⚠️ COMPLEXITY SKIPPED: No analyzer for <language>"

**Interpret results:**

| Score | Rating | Action |
|-------|--------|--------|
| A (1-5) | Simple | Good |
| B (6-10) | Moderate | OK |
| C (11-20) | Complex | Flag for council |
| D (21-30) | Very complex | Recommend refactor |
| F (31+) | Untestable | Must refactor |

**Include complexity findings in council context.**

### Step 2.3: Load Domain-Specific Checklists

Detect code patterns in the target files and load matching domain-specific checklists from `standards/references/`:

| Trigger | Checklist | Detection |
|---------|-----------|-----------|
| SQL/ORM code | `sql-safety-checklist.md` | Files contain SQL queries, ORM imports (`database/sql`, `sqlalchemy`, `prisma`, `activerecord`, `gorm`, `knex`), or migration files in changeset |
| LLM/AI code | `llm-trust-boundary-checklist.md` | Files import `anthropic`, `openai`, `google.generativeai`, or match `*llm*`, `*prompt*`, `*completion*` patterns |
| Concurrent code | `race-condition-checklist.md` | Files use goroutines, `threading`, `asyncio`, `multiprocessing`, `sync.Mutex`, `concurrent.futures`, or shared file I/O patterns |
| Codex skills | `codex-skill.md` | Files under `skills-codex/`, or files matching `*codex*SKILL.md`, `convert.sh`, `skills-codex-overrides/`, or converter scripts |

For each matched checklist, load it via the Read tool and include relevant items in the council packet as `context.domain_checklists`. Multiple checklists can be loaded simultaneously.

Skip silently if no patterns match. This step runs in both `--quick` and full modes (domain checklists are cheap to load and high-value).

### Step 2.4: Compiled Prevention Check

Before reading `.agents/rpi/next-work.jsonl`, load compiled prevention context from `.agents/pre-mortem-checks/*.md` and `.agents/planning-rules/*.md` when they exist. This is the primary reusable-prevention surface for review.

Use the tracked contracts in `docs/contracts/finding-compiler.md` and `docs/contracts/finding-registry.md`:

- prefer compiled pre-mortem checks and planning rules first
- rank by severity, `applicable_when` overlap, language overlap, changed-file overlap, and literal target-text overlap
- keep the ranking order consistent with `/plan` and `/pre-mortem`; do not invent a separate review-only heuristic
- cap at top 5 findings / compiled files
- if compiled outputs are missing, incomplete, or fewer than the matched finding set, fall back to `.agents/findings/registry.jsonl`
- fail open:
  - missing compiled directory or registry -> skip silently
  - empty compiled directory or registry -> skip silently
  - malformed line -> warn and ignore that line
  - unreadable file -> warn once and continue without findings

Include matched entries in the council packet as `known_risks` / checklist context with:
- `id`
- `pattern`
- `detection_question`
- `checklist_item`

### Step 2.5: Prior Findings Check

Read `.agents/rpi/next-work.jsonl` for unconsumed `severity=high` items matching the target area. Include them in the council packet as `context.prior_findings` so judges have carry-forward context. The review stage inherits prior findings context rather than restarting retrieval from scratch.

```bash
if [ -f .agents/rpi/next-work.jsonl ] && command -v jq &>/dev/null; then
  prior_count=$(jq -s '[.[] | select(.consumed == false) | .items[] | select(.severity == "high")] | length' \
    .agents/rpi/next-work.jsonl 2>/dev/null || echo 0)
  if [ "$prior_count" -gt 0 ]; then
    jq -s '[.[] | select(.consumed == false) | .items[] | select(.severity == "high")]' \
      .agents/rpi/next-work.jsonl 2>/dev/null
  fi
fi
```

Skip silently if the file is missing, `jq` is unavailable, or no unconsumed high-severity items exist.

### Step 2a: Run Constraint Tests

Run constraint tests before council if they exist (e.g., `internal/constraints/*_test.go`). Constraint tests catch mechanical violations (ghost references, TOCTOU races) that council judges miss. Failed constraint tests are CRITICAL findings that override council PASS verdict.

```bash
if [ -d "internal/constraints" ] && ls internal/constraints/*_test.go &>/dev/null; then
  go test ./internal/constraints/ -run TestConstraint -v 2>&1
fi
```

### Step 2b: Metadata Verification Checklist (MANDATORY)

Run mechanical checks BEFORE council (catches errors LLMs estimate instead of measure): file existence for all paths in `git diff --name-only HEAD~3`, line count verification, internal cross-reference resolution, and diagram label sanity. Include failures in council packet as `context.metadata_failures` (MECHANICAL findings).

### Step 2c: Deterministic Validation (Olympus)

**Guard:** Only run when `.ol/config.yaml` exists AND `which ol` succeeds. Skip silently otherwise.

Run `skills/vibe/scripts/ol-validate.sh`. Exit 0 = passed (include report in council context), Exit 1 = failed (auto-FAIL vibe, do NOT proceed to council), Exit 2 = skipped (note and continue).

### Step 2d: Codex Review (opt-in via `--mixed`)

**Requires `--mixed` flag.** Adds Codex-generated review as pre-existing context for council judges (augments, does not replace council judgment).

Run `codex review --uncommitted`, save to `.agents/council/codex-review-pre.md`, and include the first 2000 chars in the council packet as `codex_review` (raw output can be 50k+ — truncation prevents context bloat across N judges).

Skip silently if `--mixed` not passed, Codex CLI not on PATH, review fails, or no uncommitted changes.

### Step 2e: Search Knowledge Flywheel

Search for prior code review patterns via `ao search "code review findings <target>"` and include results in council packet context. Skip silently if `ao` is unavailable or returns no results.

### Step 2f: Bug Hunt or Deep Audit Sweep

Two paths (skip both if no source files in target or target is non-code):

**Path A — Deep Audit Sweep (`--deep` or `--sweep`):** Follow `references/deep-audit-protocol.md`. Chunk files into batches of 3–5, dispatch up to 8 Explore agents with 8-category checklists, merge findings into `.agents/council/sweep-manifest.md`, and include in council packet (judges shift to adjudication mode).

**Path B — Lightweight Bug Hunt (default):** Run `/bug-hunt --audit <target>` and include findings in council packet as `context.bug_hunt` (first 2000 chars). Catches concrete line-level bugs that holistic council review often misses.

### Step 2g: Test Pyramid Inventory (MANDATORY)

Assess test coverage against the test pyramid standard (the test pyramid standard (loaded via `/standards`)).

**Run even in `--quick` mode** — this is cheap (file existence checks) and high-signal.

1. **Identify changed modules** from git diff or target scope
2. **For each changed module, check coverage pyramid (L0–L3):**
   - L0: Does a contract/spec enforcement test cover this module?
   - L1: Does a unit test file exist for this module?
   - L2: If module crosses boundaries, does an integration test exist?
3. **For boundary-touching code, check bug-finding pyramid (BF1–BF5):**
   - BF4 (Chaos): Do external call sites have failure injection tests?
   - BF1 (Property): Do data transformations have property tests?
   - BF2 (Golden): Do output generators have golden file tests?
4. **Build coverage table** and include in council packet as `context.test_pyramid`:

```json
"test_pyramid": {
  "coverage": {
    "L0": {"status": "pass", "files": ["test_spec_enforcement.py"]},
    "L1": {"status": "pass", "files": ["test_module.py"]},
    "L2": {"status": "gap", "reason": "crosses subsystem boundary, no integration test"}
  },
  "bug_finding": {
    "BF4_chaos": {"status": "gap", "reason": "external API calls without failure injection"},
    "BF1_property": {"status": "na", "reason": "no data transformations in scope"}
  },
  "verdict": "WARN: L2 and BF4 gaps on boundary code"
}
```

**Verdict rules:**
- Missing L1 on feature code → **WARN** (include in council findings)
- Missing L0 on spec-changing code → **WARN**
- Missing BF4 on boundary code → **WARN** (advisory, not blocking)
- All levels covered → no mention needed

### Step 2h: Check for Product Context

When `PRODUCT.md` exists and no explicit `--preset` was passed, include it in the council packet via `context.files` and add a `developer-experience` perspective (DX judge covers api-clarity, error-experience, discoverability). With `--deep`, adds 1 more judge (4 total). If user passed explicit `--preset`, skip DX auto-include. If `PRODUCT.md` absent, proceed unchanged.

> **Tip:** Create `PRODUCT.md` from `docs/PRODUCT-TEMPLATE.md` to enable DX-aware code review.

### Step 3: Load the Spec

Before invoking council, try to find the relevant spec/bead: check if target is a bead ID (`bd show <id>`), search `.agents/plans/`, or check `git log`. If found, include in the council packet's `context.spec` field.

### Step 3.5: Load Suppressions

Before invoking council, load the default suppression list from `references/vibe-suppressions.md` and any project-level overrides from `.agents/vibe-suppressions.jsonl`. Suppressions are applied post-verdict to classify findings as CRITICAL vs INFORMATIONAL and to filter known false positives. See [references/vibe-suppressions.md](references/vibe-suppressions.md) for the full pattern list.

### Step 3.6: Load Pre-Mortem Predictions (Correlation)

When a pre-mortem report exists for the current epic, load prediction IDs for downstream correlation:

```bash
# Find the most recent pre-mortem report
PM_REPORT=$(ls -t .agents/council/*pre-mortem*.md 2>/dev/null | head -1)
if [ -n "$PM_REPORT" ]; then
  # Extract prediction IDs from frontmatter
  PREDICTION_IDS=$(sed -n '/^prediction_ids:/,/^[^ -]/p' "$PM_REPORT" | grep '^\s*-' | sed 's/^\s*- //')
fi
```

For each vibe finding, check if it matches a pre-mortem prediction:
- **Match found:** Tag finding with `predicted_by: pm-YYYYMMDD-NNN`
- **No match:** Tag finding with `predicted_by: none` (surprise issue)

Include the prediction correlation in the vibe report's findings table. This feeds the post-mortem's Prediction Accuracy section. Skip silently if no pre-mortem report exists.

### Step 4: Run Council Validation

**With spec found — use code-review preset:**
```
/council --preset=code-review validate <target>
```
- `error-paths`: Trace every error handling path. What's uncaught? What fails silently?
- `api-surface`: Review every public interface. Is the contract clear? Breaking changes?
- `spec-compliance`: Compare implementation against the spec. What's missing? What diverges?

The spec content is injected into the council packet context so the `spec-compliance` judge can compare implementation against it.

**Without spec — 2 independent judges (no perspectives):**
```
/council validate <target>
```
2 independent judges (no perspective labels). Use `--deep` for 3 judges on high-stakes reviews. Override with `--quick` (inline single-agent check) or `--mixed` (cross-vendor with Codex).

**Council receives:**
- Files to review
- Complexity hotspots (from Step 2)
- Git diff context
- Spec content (when found, in `context.spec`)
- Sweep manifest (when `--deep` or `--sweep`, in `context.sweep_manifest` — judges shift to adjudication mode, see `references/deep-audit-protocol.md`)

All council flags pass through: `--quick` (inline), `--mixed` (cross-vendor), `--preset=<name>` (override perspectives), `--explorers=N`, `--debate` (adversarial 2-round). See Quick Start examples and `/council` docs.

### Step 5: Council Checks

Each judge reviews for:

| Aspect | What to Look For |
|--------|------------------|
| **Correctness** | Does code do what it claims? |
| **Security** | Injection, auth issues, secrets |
| **Edge Cases** | Null handling, boundaries, errors |
| **Quality** | Dead code, duplication, clarity |
| **Complexity** | High cyclomatic scores, deep nesting |
| **Architecture** | Coupling, abstractions, patterns |

### Step 6: Interpret Verdict

| Council Verdict | Vibe Result | Action |
|-----------------|-------------|--------|
| PASS | Ready to ship | Merge/deploy |
| WARN | Review concerns | Address or accept risk |
| FAIL | Not ready | Fix issues |

### Step 7: Write Vibe Report

**Write to:** `.agents/council/YYYY-MM-DD-vibe-<target>.md` (use `date +%Y-%m-%d`)

```markdown
---
id: council-YYYY-MM-DD-vibe-<target-slug>
type: council
date: YYYY-MM-DD
---

# Vibe Report: <Target>

**Files Reviewed:** <count>

## Complexity Analysis

**Status:** ✅ Completed | ⚠️ Skipped (<reason>)

| File | Score | Rating | Notes |
|------|-------|--------|-------|
| src/auth.py | 15 | C | Consider breaking up |
| src/utils.py | 4 | A | Good |

**Hotspots:** <list files with C or worse>
**Skipped reason:** <if skipped, explain why - e.g., "radon not installed">

## Council Verdict: PASS / WARN / FAIL

| Judge | Verdict | Key Finding |
|-------|---------|-------------|
| Error-Paths | ... | ... (with spec — code-review preset) |
| API-Surface | ... | ... (with spec — code-review preset) |
| Spec-Compliance | ... | ... (with spec — code-review preset) |
| Judge 1 | ... | ... (no spec — 2 independent judges) |
| Judge 2 | ... | ... (no spec — 2 independent judges) |
| Judge 3 | ... | ... (no spec — 2 independent judges) |

## Shared Findings
- ...

## CRITICAL Findings (blocks ship)
- ... (findings that indicate correctness, security, or data-safety issues)

## INFORMATIONAL Findings (include in PR body)
- ... (style suggestions, minor improvements, suppressed/downgraded items)

## Concerns Raised
- ...

## All Findings

> Included when `--deep` or `--sweep` produces a sweep manifest. Lists ALL findings
> from explorer sweep + council adjudication. Grouped by category if >20 findings.

| # | File | Line | Category | Severity | Description | Source |
|---|------|------|----------|----------|-------------|--------|
| 1 | ... | ... | ... | ... | ... | sweep / council |

## Recommendation
<council recommendation>

## Decision

[ ] SHIP - Complexity acceptable, council passed
[ ] FIX - Address concerns before shipping
[ ] REFACTOR - High complexity, needs rework
```

### Step 8: Report to User

Tell the user:
1. Complexity hotspots (if any)
2. Council verdict (PASS/WARN/FAIL)
3. Key concerns
4. Location of vibe report

### Step 9: Record Ratchet Progress

After council verdict:
1. **PASS or WARN:** Run `ao ratchet record vibe --output "<report-path>" 2>/dev/null || true`. Suggest running `/post-mortem` to capture learnings.
2. **FAIL:** Do NOT record ratchet progress. Extract ALL findings from the council report as structured retry context (`FINDING: <desc> | FIX: <fix or recommendation> | REF: <ref or location>`, grouped by category if >20). Tell user to fix issues and re-run `/vibe`.

### Step 9.5: Feed Findings to Flywheel

**If verdict is WARN or FAIL**, persist reusable findings to `.agents/findings/registry.jsonl` (requires `dedup_key`, provenance, `pattern`, `detection_question`, `checklist_item`, `applicable_when` per finding-registry contract; append/merge by `dedup_key`; use temp-file-plus-rename atomic writes). Optionally write prose summary to `.agents/learnings/YYYY-MM-DD-vibe-<target>.md`. Skip if PASS.

After registry update, run `bash hooks/finding-compiler.sh --quiet 2>/dev/null || true` if the hook exists (keeps post-mortem path synchronized).

### Step 10: Test Bead Cleanup

After validation completes, clean up stale test beads (`bd list --status=open | grep -iE "test bead|test quest"`) via `bd close` to prevent bead pollution. Skip if `bd` unavailable.

---

## Integration with Workflow

```
/implement issue-123
    │
    ▼
(coding, quick lint/test as you go)
    │
    ▼
/vibe                      ← You are here
    │
    ├── Complexity analysis (find hotspots)
    ├── Bug hunt audit (find concrete bugs)
    └── Council validation (multi-model judgment)
    │
    ├── PASS → ship it
    ├── WARN → review, then ship or fix
    └── FAIL → fix, re-run /vibe
```

---

## Examples

See Quick Start above for common invocations. See `references/examples.md` for additional examples: security audit with spec compliance, developer-experience code review with PRODUCT.md, and fast inline checks.

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| "COMPLEXITY SKIPPED: radon not installed" | Python complexity analyzer missing | Install with `pip install radon` or skip complexity (council still runs). |
| "COMPLEXITY SKIPPED: gocyclo not installed" | Go complexity analyzer missing | Install with `go install github.com/fzipp/gocyclo/cmd/gocyclo@latest` or skip. |
| Vibe returns PASS but constraint tests fail | Council LLMs miss mechanical violations | Check `.agents/council/<timestamp>-vibe-*.md` for constraint test results. Failed constraints override council PASS. Fix violations and re-run. |
| Codex review skipped | `--mixed` not passed, Codex CLI not on PATH, or no uncommitted changes | Codex review is opt-in — pass `--mixed` to enable. Also requires Codex CLI on PATH and uncommitted changes. |
| "No modified files detected" | Clean working tree, no recent commits | Make changes or specify target path explicitly: `/vibe src/auth/`. |
| Spec-compliance judge not spawned | No spec found in beads/plans | Reference bead ID in commit message or create plan doc in `.agents/plans/`. Without spec, vibe uses 2 independent judges (3 with `--deep`). |

---

## See Also

- `skills/council/SKILL.md` — Multi-model validation council
- `skills/complexity/SKILL.md` — Standalone complexity analysis
- `skills/bug-hunt/SKILL.md` — Proactive code audit and bug investigation
- `.agents/specs/conflict-resolution-algorithm.md` — Conflict resolution between agent findings

## Reference Documents

- [references/deep-audit-protocol.md](references/deep-audit-protocol.md)
- [references/examples.md](references/examples.md)
- [references/go-patterns.md](references/go-patterns.md)
- [references/go-standards.md](references/go-standards.md)
- [references/json-standards.md](references/json-standards.md)
- [references/markdown-standards.md](references/markdown-standards.md)
- [references/patterns.md](references/patterns.md)
- [references/python-standards.md](references/python-standards.md)
- [references/report-format.md](references/report-format.md)
- [references/rust-standards.md](references/rust-standards.md)
- [references/shell-standards.md](references/shell-standards.md)
- [references/typescript-standards.md](references/typescript-standards.md)
- [references/vibe-coding.md](references/vibe-coding.md)
- [references/vibe-suppressions.md](references/vibe-suppressions.md)
- [references/yaml-standards.md](references/yaml-standards.md)
