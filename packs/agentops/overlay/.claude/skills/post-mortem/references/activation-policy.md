# Activation Policy (Phase 4)

Governs how high-scoring learnings from Phase 3 are promoted to MEMORY.md, compiled into constraints, and queued as improvements. Only learnings newer than the last-processed marker are candidates.

## 1. Promotion Threshold

A learning is promoted to MEMORY.md when its composite score reaches **6 or higher**.

**Score formula:**

```
score = confidence + citations + recency
```

| Component | Values |
|-----------|--------|
| **Confidence** | `high` = 3, `med` = 2, `low` = 1 |
| **Citations** | Base = 1; +1 per additional citation (e.g., 3 citations = 3) |
| **Recency** | < 7 days = 3, < 30 days = 2, otherwise = 1 |

**Maximum possible score:** 3 + N + 3 (unbounded on citations, but 9 is typical ceiling).

**Eligibility gate:** Only learnings processed in Phase 3 (newer than the `last-processed` marker in `.agents/ao/last-processed`) are scored. Older learnings are never re-evaluated unless the cursor is manually reset.

## 2. MEMORY.md Section Schema

Promoted entries are written into auto-managed sections of the project MEMORY.md at `~/.claude/projects/<project-path>/memory/MEMORY.md`.

| Section | Purpose | Example Entry |
|---------|---------|---------------|
| Last Session | Most recent session summary | Date, summary, key outcome |
| Architecture | Structural decisions and patterns | "Council is the core validation primitive -- no upstream deps except optional standards" |
| Process | Workflow and execution patterns | "Pre-mortem mandatory for 3+ issue epics (7 consecutive positive ROI)" |
| Debugging | Troubleshooting insights | "macOS cp alias prompts on overwrite -- use /bin/cp to bypass" |
| Patterns | Reusable patterns and anti-patterns | "Lead-Only Commit: workers write, never git commit" |
| Key Lessons | High-confidence cross-cutting lessons | "Skills source of truth is THIS REPO -- never edit installed copies" |

**Section ownership:** Phase 4 manages all sections except `Last Session`, which is updated by the session-close hook. If a learning doesn't clearly fit a section, default to `Key Lessons`.

## 3. Per-Section Cap

- **Maximum 15 entries per section.**
- **200-line hard limit** on total MEMORY.md content. Claude Code auto-loads project memory on every session start; content beyond 200 lines is truncated and invisible to the agent.
- When a section exceeds 15 entries after a proposed insertion: trigger eviction (see Section 5) before writing the new entry.
- The 200-line limit is a secondary backstop. Even if all sections are under 15 entries, if total line count would exceed 200, evict the globally lowest-scoring entry across all sections.

## 4. Entry Format

Each promoted entry is a **single line** for scannability:

```
- **<Short Title>**: <One-sentence insight> (source: `.agents/learnings/<file>`)
```

Rules:
- Title is 2-5 words, title-cased.
- Insight is one sentence, max 120 characters.
- Source path enables traceability back to the full learning artifact.
- No multi-line entries. If an insight can't fit in one sentence, it needs to be split or summarized more aggressively.

**Example:**

```
- **Lead-Only Commit**: Workers write files but never git commit; lead validates and commits per wave (source: `.agents/learnings/2026-02-08-crank-patterns.md`)
```

## 5. Eviction Policy

When adding a new entry would exceed the per-section cap (15 entries):

1. **Sort** existing entries in the target section by citation count (ascending), then by age (oldest first) as tiebreaker.
2. **Remove** the entry with lowest citations + oldest date.
3. **Log** the eviction by appending to `.agents/ao/evictions.log`:
   ```
   <ISO-8601-timestamp> EVICTED: <entry-title> from <section> (citations=N, age=Nd)
   ```
4. **Preserve source:** If the evicted entry's source learning still exists in `.agents/learnings/`, it remains there. Eviction removes the entry from MEMORY.md only -- the full learning artifact is never deleted by this process.

**Global eviction** (200-line backstop): When total line count would exceed 200, evict the globally lowest-scoring entry across all sections using the same sort order (citations ascending, then oldest first), regardless of which section it belongs to. Repeat until under 200 lines.

## 6. Constraint Activation Criteria

Learnings that meet the promotion threshold are additionally evaluated for constraint compilation when **ALL** of the following are true:

1. **Actionability score >= 4/5** -- the insight is immediately enforceable as a rule or check.
2. **Tagged as "constraint" or "anti-pattern"** in the learning's category field or body content.
3. **Not already compiled** -- no existing constraint in `.agents/constraints/` shares the same title (case-insensitive match).

When all criteria are met, trigger constraint compilation:

```bash
bash hooks/constraint-compiler.sh <learning-path>
```

The compiler reads the learning, extracts the enforceable rule, and writes a constraint file to `.agents/constraints/`. The resulting constraint is available to future pre-mortem and vibe sessions.

## 7. Skip Conditions

| Flag | Behavior |
|------|----------|
| `--skip-activate` | Run Phase 3 (process learnings) but skip Phase 4 entirely. No promotions, no evictions, no constraint compilation. |
| `--process-only` | Skip council + extraction (Phases 1-2), run Phase 3 through Phase 5 only. Phase 4 executes normally. |

**MEMORY.md bootstrap:** If the target MEMORY.md file does not exist when Phase 4 runs:

1. Create the file with section headers only:
   ```markdown
   # Project Memory

   ## Last Session

   ## Architecture

   ## Process

   ## Debugging

   ## Patterns

   ## Key Lessons
   ```
2. Proceed with promotion as normal -- the newly created file will receive its first entries.
