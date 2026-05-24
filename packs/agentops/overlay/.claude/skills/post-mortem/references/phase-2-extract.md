## Phase 2: Extract Learnings

Inline extraction of learnings from the completed work (formerly delegated to the retro skill).

### Step EX.1: Gather Context

```bash
# Recent commits
git log --oneline -20 --since="7 days ago"

# Epic children (if epic ID provided)
bd children <epic-id> 2>/dev/null | head -20

# Recent plans and research
ls -lt .agents/plans/ .agents/research/ 2>/dev/null | head -10
```

Read relevant artifacts: research documents, plan documents, commit messages, code changes. Use the Read tool and git commands to understand what was done.

**If retrospecting an epic:** Run the closure integrity quick-check from `references/context-gathering.md` (Phantom Bead Detection + Multi-Wave Regression Scan). Include any warnings in findings.

### Step EX.2: Classify Learnings

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

### Step EX.3: Write Learnings

**Write to:** `.agents/learnings/YYYY-MM-DD-<topic>.md`

```markdown
---
id: learning-YYYY-MM-DD-<slug>
type: learning
date: YYYY-MM-DD
category: <category>
confidence: <high|medium|low>
maturity: provisional
utility: 0.5
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

### Step EX.3.5: Test Pyramid Gap Analysis

Compare planned vs actual test levels per the test pyramid standard (`test-pyramid.md` in the standards skill). For each closed issue: check planned `test_levels` metadata against actual test files. Write a `## Test Pyramid Assessment` table (Issue | Planned | Actual | Gaps | Action). Gaps with severity >= moderate become `next-work.jsonl` items with type `tech-debt`.

### Step EX.4: Classify Learning Scope

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
- Local: "Compile's validate package needs O_CREATE|O_EXCL for atomic claims because Zeus spawns concurrent workers"
- Global: "Use O_CREATE|O_EXCL for atomic file creation when multiple processes may race on the same path"

### Step EX.5: Write Structured Findings to Registry

Before backlog processing, normalize reusable council findings into `.agents/findings/registry.jsonl`.

Use the tracked contract in `docs/contracts/finding-registry.md`:

- persist only reusable findings that should change future planning or review behavior
- require `dedup_key`, provenance, `pattern`, `detection_question`, `checklist_item`, `applicable_when`, and `confidence`
- `applicable_when` must use the controlled vocabulary from the contract
- append or merge by `dedup_key`
- use the contract's temp-file-plus-rename atomic write rule

This registry is the v1 advisory prevention surface. It complements learnings and next-work; it does not replace them.

### Step EX.6: Refresh Compiled Prevention Outputs

After the registry mutation, refresh compiled outputs immediately so the same session can benefit from the updated prevention set.

If `hooks/finding-compiler.sh` exists, run:

```bash
bash hooks/finding-compiler.sh --quiet 2>/dev/null || true
```

This promotes registry rows into `.agents/findings/*.md`, refreshes `.agents/planning-rules/*.md` and `.agents/pre-mortem-checks/*.md`, and rewrites draft constraint metadata under `.agents/constraints/`. Active enforcement still depends on the constraint index lifecycle and runtime hook support, but compilation itself is no longer deferred.

### Step ACT.3: Feed Next-Work

Actionable improvements identified during processing -> append one schema v1.4
batch entry to `.agents/rpi/next-work.jsonl` using the tracked contract in
[`../../../docs/contracts/next-work.schema.md`](../../../docs/contracts/next-work.schema.md)
and the write procedure in
[`harvest-next-work.md`](harvest-next-work.md).
Follow the claim/finalize lifecycle documented in `harvest-next-work.md`.

```bash
mkdir -p .agents/rpi
# Build VALID_ITEMS via the schema-validation flow in references/harvest-next-work.md
# Then append one entry per post-mortem / epic.
# If a harvested item already maps to a known proof surface, preserve it on the
# item as "proof_ref" instead of burying target IDs in free text. Example item:
# [{"title":"Verify the parity gate after proof propagation lands","type":"task","severity":"medium","source":"council-finding","description":"Re-run the targeted validator after the follow-up lands.","target_repo":"agentops","proof_ref":{"kind":"execution_packet","run_id":"6f36a5640805","path":".agents/rpi/runs/6f36a5640805/execution-packet.json"}}]
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

### Step ACT.4: Update Marker

```bash
date -Iseconds > .agents/ao/last-processed
```

This must be the LAST action in Phase 4.

**Phases 3-6 (Maintenance):** Read `maintenance-phases.md` for backlog processing, activation, retirement, and harvesting phases. Load when `--process-only` flag is set or when running full post-mortem.
