# Context Packet Specification

> **The context packet is the product.** Context quality is the primary lever for agent output quality. Every skill, hook, and flywheel component exists to ensure the right context is in the right window at the right time.

```
schema_version: 1
```

**Compatibility policy:** v1 fields are immutable once shipped. Future versions (v2, v3) add new optional fields only. Consumers MUST ignore unknown fields. Producers MUST NOT remove or rename v1 fields.

---

## Overview

A context packet is the structured payload assembled by `ao lookup` and delivered into an agent's context window on demand. It replaces the current raw knowledge dump with a purpose-built artifact containing exactly what an agent needs to do its work — no more, no less.

The packet has five sections, each with a defined character budget, content source, and eviction priority. The total budget is ~28K characters (~7K tokens at `InjectCharsPerToken = 4`), which leaves 90%+ of the context window available for actual work.

```
┌──────────────────────────────────────────────────────────┐
│                    CONTEXT PACKET                        │
│                                                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                   │
│  │  GOALS  │  │ HISTORY │  │  INTEL  │                   │
│  │  ~2K ch │  │  ~8K ch │  │ ~12K ch │                   │
│  └─────────┘  └─────────┘  └─────────┘                   │
│  ┌─────────┐  ┌──────────┐                               │
│  │  TASK   │  │ PROTOCOL │                               │
│  │  ~4K ch │  │  ~2K ch  │                               │
│  └─────────┘  └──────────┘                               │
│                                                           │
│  Total: ~28K chars / ~7K tokens                           │
│  Window utilization: <10% (leaves 90%+ for work)          │
└──────────────────────────────────────────────────────────┘
```

---

## Section 1: GOALS (~2K chars / ~500 tokens)

**Purpose:** Define what "good" looks like for this repository. The fitness specification that tells the agent what success means.

**Source:** `GOALS.md` (version 4, preferred) or `GOALS.yaml` (versions 1-3), loaded via `goals.LoadGoals()`. Current pass/fail status from the most recent fitness snapshot in `.agents/ao/goals/baselines/`.

**Content:**

```
## GOALS

Fitness spec for this repository. Your work must not regress these gates.

### Passing (14/16)
- test-pass-rate: PASS (98% coverage, threshold 95%)
- lint-clean: PASS (0 warnings)
- build-success: PASS
  ...

### Failing (2/16)
- doc-coverage: FAIL (72%, threshold 80%) ← priority target
- api-response-time: FAIL (320ms, threshold 200ms)

### Directives
- "Prioritize test reliability over coverage percentage"
- "All new public APIs require OpenAPI spec"
```

**Truncation behavior:** When the GOALS section exceeds its 2K character budget, include only failing gates and their current values. Passing gates are summarized as a count: "14/16 gates passing (omitted for brevity)." Directives are always included — they are the strategic intent layer.

**Graceful degradation:** If no `GOALS.md` or `GOALS.yaml` exists, or no baseline snapshot is available:

```
## GOALS

No fitness goals defined for this repository.
Run `ao goals init` to bootstrap GOALS.md, or this is the first cycle.
```

---

## Section 2: HISTORY (~8K chars / ~2K tokens)

**Purpose:** What previous agents did and what happened. Establishes continuity across sessions so the agent does not repeat work or re-discover known failures.

**Source:** `cycle-history.jsonl` (from `.agents/evolve/cycle-history.jsonl`), recent session summaries (from `.agents/ao/sessions/`), and the ratchet chain (from `.agents/ao/chain.jsonl`).

**Content:**

```
## HISTORY

### Recent Cycles (last 10)
- Cycle 42: target=doc-coverage, result=improved, sha=abc1234 (2026-02-23)
- Cycle 41: target=test-pass-rate, result=unchanged (2026-02-23)
- Cycle 40: target=api-response-time, result=regressed → reverted (2026-02-22)
  ...

### Recent Sessions (last 5)
- [2026-02-23] Refactored auth middleware, extracted JWT validation
- [2026-02-22] Added rate limiting to /login endpoint
  ...

### Ratchet Chain (last 5 entries)
- vibe PASS on commit def5678 (2026-02-23)
- pre-mortem PASS for epic ag-xyz (2026-02-22)
  ...
```

**Assembly rules:**
1. Cycle history entries are sorted newest-first, limited to the 10 most recent.
2. Session summaries are loaded via `collectRecentSessions()`, limited to 5.
3. Ratchet chain entries are loaded via `ratchet.LoadChain()`, limited to the 5 most recent.
4. Each sub-section is independently truncatable. If the combined content exceeds 8K characters, apply sub-section limits: cycles to 5, sessions to 3, chain entries to 3.

**Graceful degradation:** If no history sources exist (new repo, first session):

```
## HISTORY

No prior session data — this is the first cycle.
Future sessions will see what you did and how it went.
```

---

## Section 3: INTEL (~12K chars / ~3K tokens)

**Purpose:** What worked and what did not, filtered for THIS agent's specific task. The highest-value section — directly improves output quality by front-loading relevant knowledge.

**Source:** Learnings from `.agents/learnings/` (via `collectLearnings()` with Two-Phase MemRL retrieval), patterns from `.agents/patterns/` (via `collectPatterns()`), and task-relevant knowledge from the flywheel.

**Content:**

```
## INTEL

### Relevant Learnings (freshness-weighted, task-filtered)
- **LRN-047**: JWT refresh tokens must use separate signing key (gold, 0.92 freshness)
- **LRN-031**: Rate limiting at middleware layer, not per-handler (silver, 0.78 freshness)
- **LRN-019**: Redis connection pool needs explicit MaxIdle setting (bronze, 0.61 freshness)
  ...

### Active Patterns
- **error-handling-go**: Wrap errors with %w, never bare string errors
- **test-table-driven**: All Go tests use table-driven pattern
  ...
```

**Assembly rules:**
1. The `--query` argument (to `ao lookup`) filters learnings and patterns by substring match against the agent's task description.
2. Learnings are ranked by composite score (freshness * utility, Two-Phase MemRL retrieval). Maximum 10 learnings.
3. Patterns are ranked by composite score. Maximum 5 patterns.
4. When `--apply-decay` is set, confidence decay is applied before ranking (Darr 1995, delta=0.17/week).
5. CASS maturity weighting is applied when available.

**Truncation behavior:** When INTEL exceeds its 12K character budget, evict oldest learnings first (lowest freshness score), then oldest patterns.

**Graceful degradation:** If no learnings or patterns exist:

```
## INTEL

No prior knowledge found — this is the first cycle.
As you work, use `/retro` or `/post-mortem` to extract learnings for future sessions.
```

---

## Section 4: TASK (~4K chars / ~1K tokens)

**Purpose:** What this agent specifically needs to do right now. The immediate work assignment with acceptance criteria.

**Source:** Bead description (from `bd show <id>`), epic context, or the task assignment from `/crank` wave planning. For `/evolve`, this is the current goal target and its failing check command.

**Content:**

```
## TASK

### Assignment
Bead: ag-poz.2 — "Create context-packet.md specification"
Epic: ag-poz — "The Seed"
Priority: P1
Type: docs

### Description
Define exactly what each agent receives — the product's primary engineering artifact.
Must include 5 sections with char budgets, redaction contract, assembly algorithm.

### Acceptance Criteria
- [ ] File exists at docs/context-packet.md
- [ ] Contains all 5 sections (GOALS, HISTORY, INTEL, TASK, PROTOCOL)
- [ ] Contains schema_version reference
- [ ] Contains redaction contract
- [ ] Contains char budgets
```

**Assembly rules:**
1. Task content is injected by the orchestrating skill (`/crank`, `/implement`, `/evolve`) or by the session-start hook when a bead is assigned.
2. If a bead ID is available, its full description and acceptance criteria are included.
3. Epic context (parent bead title, sibling issues) is summarized in one line.
4. Cross-cutting constraints relevant to the task are appended from the epic's constraint list.

**Truncation behavior:** TASK is never truncated. If the raw bead description exceeds 4K characters, the orchestrator must compress it before assembly. The assembler does not truncate TASK — it is the agent's primary directive.

**Graceful degradation:** If no task is assigned (e.g., ad-hoc session without bead tracking):

```
## TASK

No specific task assigned. This is a free-form session.
Use `/plan` to decompose work into tracked issues, or work ad-hoc.
```

---

## Section 5: PROTOCOL (~2K chars / ~500 tokens)

**Purpose:** How to save work so the next agent benefits. The operational contract for artifact persistence, learning extraction, and ratchet advancement.

**Source:** Static content, generated from the repo's configuration. Version-controlled in the context packet assembler, not dynamically gathered.

**Content:**

```
## PROTOCOL

### Saving Work
1. Commit early and atomically. One logical change per commit.
2. If you are a worker (not the lead), write files but do NOT commit.
   The lead validates and commits.
3. Run `/vibe` before pushing. The push gate will block unvalidated pushes.

### Recording Learnings
1. After completing work, run `/retro` to extract session learnings.
2. Learnings are saved to `.agents/learnings/` and scored for quality.
3. High-quality learnings are injected into future sessions automatically.

### Updating the Ratchet
1. Gate passes are recorded via `ao ratchet record`.
2. The ratchet chain in `.agents/ao/chain.jsonl` is append-only.
3. Never manually edit chain.jsonl — use `ao ratchet` subcommands.

### Handoff
1. If your session ends before work is complete, the precompact-snapshot
   hook saves context to `.agents/handoff/`.
2. The next session's session-start hook reads handoff context automatically.
3. Write a brief summary of where you stopped and what remains.
```

**Assembly rules:** PROTOCOL is assembled from a static template that varies only by repo configuration (e.g., whether hooks are installed, whether `ao` CLI is available). It is generated once at `ao init` time and updated when hooks change.

**Truncation behavior:** PROTOCOL is never truncated. It is the smallest section and contains operational safety instructions. Removing any part risks agents not persisting their work correctly.

**Graceful degradation:** If `ao` CLI is not installed, the PROTOCOL section omits ratchet and hook references and provides minimal instructions:

```
## PROTOCOL

Commit your work with clear messages. Write a session summary to
`.agents/handoff/` before ending. Run `/retro` if available.
```

---

## Character Budgets and Eviction Order

### Budget Table

| Section | Min (chars) | Target (chars) | Max (chars) | Tokens (~) |
|---------|-------------|----------------|-------------|------------|
| GOALS | 200 | 2,000 | 3,000 | ~500 |
| HISTORY | 200 | 8,000 | 10,000 | ~2,000 |
| INTEL | 200 | 12,000 | 15,000 | ~3,000 |
| TASK | 200 | 4,000 | 5,000 | ~1,000 |
| PROTOCOL | 200 | 2,000 | 2,500 | ~500 |
| **Total** | **1,000** | **28,000** | **35,500** | **~7,000** |

Token estimates use `InjectCharsPerToken = 4` (conservative, from `cli/cmd/ao/lookup.go`).

### Overflow Eviction Order

When the assembled packet exceeds the total budget, sections are trimmed in this priority order (last trimmed = highest priority):

```
Eviction order (first to trim → last to trim):

  1. HISTORY  — trim oldest entries first
  2. INTEL    — trim lowest-scored learnings first
  3. GOALS    — reduce to failing gates only
  4. TASK     — NEVER trimmed
  5. PROTOCOL — NEVER trimmed (static, smallest)
```

**Eviction algorithm:**

1. Assemble all five sections at full content.
2. Measure total character count.
3. If total exceeds budget (`--max-tokens * InjectCharsPerToken`):
   a. Trim HISTORY: reduce cycles to 5, then sessions to 3, then chain to 3. Re-measure.
   b. If still over: trim INTEL: remove lowest-freshness learnings one at a time. Re-measure.
   c. If still over: trim GOALS: keep only failing gates + directives. Re-measure.
   d. TASK and PROTOCOL are never trimmed.
4. If the packet is STILL over budget after all trimming (pathological case), truncate HISTORY to a single summary line.

---

## Redaction Contract

Before assembly, each section's raw content passes through a redaction gate. The gate is fail-closed: if a high-confidence match is detected, the source line is omitted entirely and replaced with a marker.

### Denylist Patterns

| Pattern | Regex | Example |
|---------|-------|---------|
| Environment variable assignments | `[A-Z_]{2,}=\S+` | `DATABASE_URL=postgres://...` |
| JWT tokens | `eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}` | `eyJhbGciOi...` |
| API keys / high-entropy strings | `[A-Za-z0-9+/=]{20,}` preceded by key-like context (`key`, `token`, `secret`, `password`, `credential`, `auth`) | `AKIAIOSFODNN7EXAMPLE` |
| PII: email addresses | `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}` | `user@example.com` |
| PII: IP addresses in private ranges | `(10\.\d+\.\d+\.\d+\|172\.(1[6-9]\|2\d\|3[01])\.\d+\.\d+\|192\.168\.\d+\.\d+)` | `192.168.1.42` |
| Connection strings | `(postgres\|mysql\|mongodb\|redis)://[^\s]+` | `redis://admin:pass@host:6379` |

### Redaction Behavior

1. **Scan each source line** before it enters the section assembler.
2. **Match against denylist** patterns in order. First match wins.
3. **On match:** omit the entire source line. Insert `[REDACTED: <pattern-type>]` in its place. Log the redaction event to `.agents/ao/redaction.log` with timestamp, pattern type, source file, and line number.
4. **Fail-closed:** if the regex engine errors or times out (>100ms per line), treat the line as a match and redact it.
5. **No false-negative tolerance:** it is acceptable to over-redact (false positives). It is not acceptable to leak secrets (false negatives).

### Redaction Log Format

```jsonl
{"ts":"2026-02-24T10:00:00Z","pattern":"jwt_token","source":".agents/learnings/auth-session.md","line":47,"section":"INTEL"}
{"ts":"2026-02-24T10:00:00Z","pattern":"env_var_assignment","source":".agents/ao/sessions/2026-02-23.jsonl","line":12,"section":"HISTORY"}
```

---

## Assembly Algorithm

The complete assembly flow from invocation to output:

```
ao lookup [--query="<query>"] [--max-tokens=N]
  │
  ├─ 1. Resolve query (--query flag)
  │
  ├─ 2. Gather raw content for each section:
  │     ├─ GOALS:    goals.LoadGoals() + latest snapshot
  │     ├─ HISTORY:  cycle-history.jsonl + sessions + chain.jsonl
  │     ├─ INTEL:    collectLearnings(query) + collectPatterns(query)
  │     │            + collectOLConstraints(query)
  │     ├─ TASK:     bead description (if assigned) or empty
  │     └─ PROTOCOL: static template (repo-config-dependent)
  │
  ├─ 3. Redaction gate: scan each section's raw lines
  │     ├─ Match against denylist patterns
  │     ├─ Replace matches with [REDACTED: <type>]
  │     └─ Log redactions to .agents/ao/redaction.log
  │
  ├─ 4. Render sections into formatted output (markdown or JSON)
  │
  ├─ 5. Measure total character count
  │
  ├─ 6. If over budget: apply eviction (HISTORY → INTEL → GOALS)
  │
  ├─ 7. Record provenance (what was injected, section sizes, query)
  │     └─ Append to .agents/ao/injection-log.jsonl
  │
  └─ 8. Output to stdout
```

### Step 2 Detail: Section Gathering

Each section uses existing infrastructure where possible:

| Section | Existing Function | New Behavior |
|---------|-------------------|--------------|
| GOALS | `goals.LoadGoals()`, `goals.Measure()` | Format pass/fail summary; include directives |
| HISTORY | `collectRecentSessions()`, `ratchet.LoadChain()` | Add cycle-history.jsonl parsing; merge three sources |
| INTEL | `collectLearnings()`, `collectPatterns()`, `collectOLConstraints()` | Already implemented — wrap with section header and char budget |
| TASK | (new) | Read from bead assignment or orchestrator injection |
| PROTOCOL | (new) | Static template, selected by repo config detection |

### Step 7 Detail: Provenance Tracking

Every injection event is logged to `.agents/ao/injection-log.jsonl`:

```jsonl
{
  "schema_version": 1,
  "ts": "2026-02-24T10:00:00Z",
  "session_id": "ses-abc123",
  "query": "authentication",
  "sections": {
    "goals": {"chars": 1842, "items": 16},
    "history": {"chars": 7200, "items": 18},
    "intel": {"chars": 11500, "items": 14},
    "task": {"chars": 2100, "items": 1},
    "protocol": {"chars": 1800, "items": 0}
  },
  "total_chars": 24442,
  "total_tokens_est": 6110,
  "budget_chars": 28000,
  "redactions": 2,
  "truncated_sections": []
}
```

This provenance record enables:
- **Injection-to-outcome correlation:** link what was injected (session_id + sections) to session results (via ratchet chain + session close events).
- **Budget utilization tracking:** identify if budgets are consistently under/over-utilized and tune.
- **Redaction auditing:** count of redactions per session for security review.
- **Quality feedback loop:** if sessions with higher INTEL scores produce better outcomes, the MemRL utility weights are reinforced.

---

## Evolution of `ao lookup`

The deprecated `ao inject` output a flat knowledge dump: learnings, patterns, and sessions rendered as markdown or JSON. The context packet evolves this through an on-demand retrieval pattern:

### Phase 1: Structured Sections (non-breaking)

`ao lookup` organizes output into the five sections defined above instead of the legacy flat format.

```bash
# On-demand query:
ao lookup --query "authentication"
ao lookup --query "authentication" --max-tokens 7000
```

`ao lookup` activates:
- Section-based assembly instead of flat rendering
- Per-section char budgets and overflow eviction
- Redaction gate before assembly
- Provenance logging to injection-log.jsonl

### Phase 2: GOALS and TASK Integration

Wire the GOALS and TASK sections into the packet assembler:
- GOALS: call `goals.LoadGoals()` and format the pass/fail summary.
- TASK: accept a `--task` flag or `--bead` flag that pulls the bead description.

```bash
ao lookup --query "authentication" --bead ag-poz.2
```

### Phase 3: On-Demand Default

The on-demand pattern (`ao lookup`) replaces the session-start injection model. Agents consult `.agents/AGENTS.md` for orientation and use `ao lookup --query "topic"` to retrieve context when needed.

### Backward Compatibility

- Phase 1 is fully additive. No existing behavior changes.
- Phase 2 adds new flags. No existing flags are removed or renamed.
- Phase 3 changes the default but provides `--legacy` escape hatch.
- All phases carry `schema_version: 1`. The version field is present in provenance logs and, when JSON format is used, in the output envelope.

---

## Graceful Degradation Matrix

The context packet degrades gracefully when sources are missing. No section produces an error — empty sources result in a human-readable note explaining the gap.

| Condition | Affected Sections | Behavior |
|-----------|-------------------|----------|
| New repo, no `.agents/` | ALL | Each section shows "No prior data" note. PROTOCOL uses minimal template. |
| No `GOALS.md` or `GOALS.yaml` | GOALS | "No fitness goals defined. Run `ao goals init`." |
| No `cycle-history.jsonl` | HISTORY (cycles) | Cycles sub-section omitted. Sessions and chain still shown if available. |
| No `.agents/ao/sessions/` | HISTORY (sessions) | Sessions sub-section omitted. |
| No `.agents/ao/chain.jsonl` | HISTORY (chain) | Chain sub-section omitted. |
| No learnings or patterns | INTEL | "No prior knowledge found." |
| No bead assigned | TASK | "No specific task assigned. Free-form session." |
| `ao` CLI not installed | PROTOCOL | Minimal template without ratchet/hook references. |
| All sources empty | Entire packet | Valid packet with five sections, each containing a degradation note. Still useful — PROTOCOL tells the agent how to start building the knowledge base. |

---

## Relationship to Existing Components

The context packet unifies and structures what multiple components already provide:

| Component | Current Role | Context Packet Role |
|-----------|-------------|---------------------|
| `ao lookup` (`lookup.go`) | On-demand knowledge retrieval | The packet assembler |
| `goals.LoadGoals()` | Fitness measurement | Feeds GOALS section |
| `collectLearnings()` | MemRL retrieval | Feeds INTEL section (learnings) |
| `collectPatterns()` | Pattern retrieval | Feeds INTEL section (patterns) |
| `collectRecentSessions()` | Session history | Feeds HISTORY section (sessions) |
| `ratchet.LoadChain()` | Provenance chain | Feeds HISTORY section (chain) |
| `recordCitations()` | Citation tracking | Provenance tracking (injection-log.jsonl) |
| `hooks/session-start.sh` | Session initialization | Points agent to `.agents/AGENTS.md` for on-demand lookup |
| Memory packets (`memory-packet.v1.schema.json`) | Boundary-memory for handoff | Orthogonal — handoff packets are emitted at session END; context packets are assembled at session START |

---

## See Also

- [Architecture](ARCHITECTURE.md) — System design and the five pillars
- [Knowledge Flywheel](knowledge-flywheel.md) — How learnings compound across sessions
- [How It Works](how-it-works.md) — Context windowing, Brownian Ratchet, Ralph Wiggum
- [The Science](the-science.md) — Freshness decay model, MemRL two-phase retrieval
- [CLI Reference](cli/commands.md) — `ao lookup` command documentation
