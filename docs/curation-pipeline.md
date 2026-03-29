# The Curation Pipeline

> Each agent is an experiment. Each experiment produces data. The data gets curated. The curated data makes the next experiment smarter. The system learns. The agents don't.

## Purpose

The curation pipeline is the mechanism that transforms raw agent experience into reliable institutional knowledge. It is the primary lever for making the knowledge flywheel compound rather than accumulate noise.

Without curation, the flywheel stores everything and retrieves whatever fits the token budget. Stale learnings compete with current ones. Wrong learnings persist indefinitely. Contradictory learnings coexist without resolution. The system gets louder, not smarter.

The curation pipeline fixes this by applying six stages -- CATALOG, VERIFY, INDEX, SCORE, REJECT, CONSTRAIN -- that progressively filter and strengthen knowledge before it re-enters agent context windows.

> **Current Implementation (v1):** Only CATALOG and VERIFY stages are implemented as CLI commands (`ao curate catalog`, `ao curate verify`, `ao curate status`). Stages 3-6 are planned for future releases.

**This is NOT the knowledge flywheel.** The flywheel is the loop: extract, store, retrieve, apply, compound. The curation pipeline is what happens between "store" and "retrieve" -- the quality control that determines whether the flywheel compounds signal or noise.

---

## The Immune System Analogy

The curation pipeline works like an adaptive immune system:

- **CATALOG** and **VERIFY** are innate immunity -- basic structural filters that catch obvious problems immediately.
- **INDEX** and **SCORE** are antigen presentation -- making experience identifiable and measurable so the system can reason about it.
- **REJECT** is apoptosis -- the deliberate destruction of harmful or stale knowledge. This is the unlearning mechanism. Without it, the system develops autoimmune disease: old learnings that were once correct attack current work.
- **CONSTRAIN** is antibody production -- internally generated defenses from experience, accumulated over time, making the system more resilient to repeated failure modes.

Constraints are editable code, not immutable axioms. They can be modified or deleted as the system's understanding evolves. They are compiled from experience, not decreed from above.

---

## Incremental Build Plan

The pipeline ships incrementally. Each version is useful on its own. Later versions add power without invalidating earlier work.

```
v1 (ship first)           v2 (after v1 proves out)      v3 (after v2 proves out)
┌──────────┐              ┌──────────┐                   ┌──────────┐
│ CATALOG  │              │ CATALOG  │                   │ CATALOG  │
│ VERIFY   │              │ VERIFY   │                   │ VERIFY   │
└──────────┘              │ INDEX    │                   │ INDEX    │
                          │ SCORE    │                   │ SCORE    │
                          └──────────┘                   │ REJECT   │
                                                         │ CONSTRAIN│
                                                         └──────────┘
```

**v1 closes the biggest gap.** Today, `/forge` and `/retro` produce unstructured markdown files that go directly into `.agents/learnings/` with no verification. v1 adds structure (typed artifacts) and mechanical truth checks (did the tests pass?). This alone prevents the most damaging failure mode: confidently wrong learnings entering the knowledge base.

**v2 adds discoverability and quality measurement.** Once artifacts are structured and verified, they can be tagged for retrieval and scored for quality. This makes `ao search` and `ao lookup` return better results and enables the pool tiering system (gold/silver/bronze) to operate on verified data.

> `2026-03-09` status note: the repo now runs a finding-compiler prevention ratchet alongside the broader curation pipeline. In current implementation terms, reusable findings enter through `.agents/findings/registry.jsonl`, are deduplicated and promoted into `.agents/findings/<id>.md`, and then compile into user-facing advisory surfaces under `.agents/planning-rules/<id>.md` and `.agents/pre-mortem-checks/<id>.md`. When detector metadata exists and is valid, the same promoted finding can also compile into `.agents/constraints/index.json` plus `.agents/constraints/<id>.sh` for `task-validation-gate.sh`. The general `ao curate` stage family is still partial, but the prevention slice is no longer advisory-only.

**v3 adds the feedback loop.** Once artifacts are scored, the system can reject low-quality or stale knowledge and compile high-quality knowledge into permanent defenses. This is where the system starts to exhibit self-organization: it generates its own constraints from its own experience.

---

## Stage 1: CATALOG

**What it does:** Structures raw agent experience into typed artifacts with consistent metadata. Transforms free-form output from `/forge` and `/retro` into machine-parseable records.

### Input

Raw output from existing skills:

| Source Skill | Output Location | Content |
|-------------|-----------------|---------|
| `/forge` | `.agents/forge/YYYY-MM-DD-forge.md` | Decisions, learnings, failures, patterns extracted from transcripts |
| `/retro` | `.agents/learnings/YYYY-MM-DD-<topic>.md` | Lessons learned from completed work |
| `/retro` | `.agents/learnings/YYYY-MM-DD-<topic>.md` | Retrospective summaries with improvement agendas |
| `/post-mortem` | `.agents/learnings/`, `.agents/findings/registry.jsonl` | Council-validated learnings plus reusable structured findings |

### Output

Typed curation artifacts written to `.agents/pool/pending/`:

```json
{
  "candidate": {
    "id": "ao-cand-<hash>",
    "type": "learning|decision|failure|pattern",
    "content": "The specific insight or finding",
    "context": "What work this came from",
    "source": {
      "transcript_path": ".agents/forge/2026-02-24-forge.md",
      "message_index": 0,
      "session_id": "session-abc123",
      "timestamp": "2026-02-24T12:00:00Z"
    },
    "extracted_at": "2026-02-24T12:00:00Z",
    "metadata": {
      "schema_version": 1,
      "source_skill": "forge",
      "catalog_version": "v1"
    }
  },
  "status": "pending",
  "added_at": "2026-02-24T12:00:00Z",
  "updated_at": "2026-02-24T12:00:00Z"
}
```

All curation artifacts carry `schema_version: 1` in their metadata.

### Connection to Existing Infrastructure

- **Input:** Reuses existing `/forge` output format (Decision, Learning, Failure, Pattern types with confidence scores). Reuses `/retro` output format (ID, Category, Confidence).
- **Output:** Writes to `.agents/pool/pending/` using the existing `pool.Add()` Go API (`cli/internal/pool/pool.go`).
- **CLI:** `ao curate catalog` -- reads forge/retro output, produces typed pool entries.
- **Types:** Uses existing `types.KnowledgeType` (decision, solution, learning, failure, reference) and `types.Candidate` struct from `cli/internal/types/types.go`.

### Failure Mode Analysis

| Failure | Impact | Mitigation |
|---------|--------|------------|
| CATALOG missing | Raw markdown goes directly to `.agents/learnings/` -- no structure, no type, no provenance tracking. Search returns untyped results. This is today's status quo. | Graceful: system still works, just without structure. `/forge` and `/retro` continue writing to their existing locations. |
| Wrong type assigned | Learning classified as pattern or vice versa. Affects retrieval relevance but not correctness. | Low impact: type is a hint for retrieval, not a gate. Existing `types.KnowledgeType` has only 5 values. |
| Duplicate cataloging | Same learning cataloged twice from different sources (forge + retro). | Content hashing on `candidate.Content` prevents duplicates in pool. Existing `resolveArtifactPath()` handles filename collisions. |

### Dependencies

None. CATALOG is the entry point to the pipeline. It receives input from existing skills that already produce output independently.

---

## Stage 2: VERIFY

**What it does:** Applies mechanical verification to cataloged artifacts. Did the tests pass after this change? Did goal metrics improve? Binary signal only -- no subjective quality judgment.

### Input

Cataloged artifact from Stage 1 (pool entry in `.agents/pool/pending/`) plus verification context:

- Git state at the time of the learning (commit hash from provenance)
- Test results associated with that commit (`ao ratchet status`)
- Goal measurements before and after (`ao goals measure`)

### Output

Verification result appended to the pool entry:

```json
{
  "candidate": { "...": "..." },
  "verification": {
    "schema_version": 1,
    "verified_at": "2026-02-24T12:05:00Z",
    "tests_passed": true,
    "goals_improved": true,
    "commit_hash": "a7d4bac",
    "verification_type": "mechanical",
    "signals": [
      {"name": "go_test", "passed": true, "detail": "42/42 passed"},
      {"name": "goal_delta", "passed": true, "detail": "test_coverage: 78% -> 82%"}
    ]
  },
  "status": "pending",
  "added_at": "2026-02-24T12:00:00Z",
  "updated_at": "2026-02-24T12:05:00Z"
}
```

### Connection to Existing Infrastructure

- **Input:** Reads pool entries via `pool.Get()` or `pool.List()`.
- **Ratchet:** Uses `ao ratchet status` to check if the commit associated with this learning passed validation gates.
- **Goals:** Uses `ao goals measure` to check if goal metrics improved.
- **CLI:** `ao curate verify` -- runs mechanical checks against pool entries that lack verification.
- **Hooks:** Can be triggered by `session-end-maintenance.sh` to verify pending entries in background.

### Failure Mode Analysis

| Failure | Impact | Mitigation |
|---------|--------|------------|
| VERIFY missing | Unverified learnings enter the knowledge base. Some will be wrong. Agents apply incorrect learnings in future sessions, causing regressions. | Without VERIFY, the system operates as today -- no worse, but no better. v1 adds VERIFY specifically because this is the highest-impact gap. |
| Tests unavailable | No test results to verify against (e.g., documentation-only changes). | Mark `tests_passed: null` (not applicable). Verification is mechanical: if there's no signal, record that. Don't infer. |
| False verification | Tests pass but learning is still wrong (tests don't cover the relevant case). | VERIFY only checks mechanical signals. It does not claim "this learning is correct" -- it claims "the tests passed." SCORE (v2) adds quality judgment. |
| Commit hash missing | Learning extracted from a session where no commits were made. | Graceful degradation: record `commit_hash: null`, skip test verification, still catalog the artifact. Some verification is better than none. |

### Dependencies

Depends on **CATALOG** -- can only verify artifacts that have been cataloged with provenance metadata (source session, commit hash).

---

## Stage 3: INDEX

**What it does:** Tags cataloged, verified artifacts by topic, skill, and goal. Makes them findable through `ao search` and surfaceable through `ao lookup`.

### Input

Verified pool entry from Stage 2.

### Output

Index metadata appended to the pool entry and written to the search index:

```json
{
  "candidate": { "...": "..." },
  "verification": { "...": "..." },
  "index": {
    "schema_version": 1,
    "indexed_at": "2026-02-24T12:10:00Z",
    "topics": ["authentication", "middleware", "go"],
    "skills_used": ["forge", "implement"],
    "goals_related": ["test_coverage", "security"],
    "keywords": ["token", "expiry", "JWT", "refresh"]
  },
  "status": "pending"
}
```

Additionally, a JSONL entry is appended to `.agents/ao/search-index.jsonl`:

```json
{"file": ".agents/pool/pending/ao-cand-abc123.json", "title": "Token expiry pattern", "keywords": ["authentication", "JWT", "token", "expiry"], "topics": ["authentication", "middleware"], "timestamp": "2026-02-24T12:10:00Z"}
```

### Connection to Existing Infrastructure

- **Search:** Enriches `ao search` results. Currently, search scans markdown files and JSONL session logs. INDEX adds structured topic/keyword metadata that improves relevance ranking.
- **Lookup:** Improves `ao lookup --query "<topic>"` filtering. Currently, lookup uses substring matching against file content. INDEX adds explicit topic tags for faster, more accurate filtering.
- **Forge:** `ao forge markdown` already writes to `search-index.jsonl`. INDEX extends this with richer metadata.
- **CLI:** `ao curate index` -- reads cataloged entries, extracts topics/keywords, updates search index.

### Failure Mode Analysis

| Failure | Impact | Mitigation |
|---------|--------|------------|
| INDEX missing | Artifacts are stored but not findable. `ao search` returns results based only on content grep. `ao lookup` loads by recency only, not relevance. | This is today's status quo. INDEX improves retrieval quality but its absence doesn't break anything. |
| Wrong topics assigned | Artifact tagged with irrelevant topics. Returns as noise in search results. | INDEX runs after VERIFY, so at minimum the content is mechanically validated. Topic assignment can be re-run (`ao curate index --reindex`). |
| Index drift | Search index diverges from actual pool state (entries deleted but index not updated). | Index is append-only JSONL. `ao search` validates entries exist before returning results. Periodic `ao curate index --rebuild` reconciles. |

### Dependencies

Depends on **CATALOG** (needs typed artifacts to tag). Benefits from **VERIFY** (verified artifacts get priority in search ranking) but does not strictly require it.

---

## Stage 4: SCORE

**What it does:** Applies the 5-dimension quality gate to assign a tier (gold/silver/bronze/discard). Determines whether an artifact is worth keeping, worth reviewing, or should be discarded.

### Input

Indexed pool entry from Stage 3.

### Output

Scoring result written to the pool entry:

```json
{
  "candidate": { "...": "..." },
  "verification": { "...": "..." },
  "index": { "...": "..." },
  "scoring_result": {
    "schema_version": 1,
    "raw_score": 0.82,
    "tier_assignment": "gold",
    "rubric": {
      "specificity": 0.90,
      "actionability": 0.85,
      "novelty": 0.70,
      "context": 0.80,
      "confidence": 0.90
    },
    "gate_required": false,
    "scored_at": "2026-02-24T12:15:00Z"
  },
  "status": "pending"
}
```

### The 5-Dimension Rubric

| Dimension | Weight | What It Measures | Example: High Score | Example: Low Score |
|-----------|--------|-----------------|--------------------|--------------------|
| **Specificity** | 0.30 | Named entities, concrete values, exact versions | "Go 1.22 `slices.SortFunc` requires `cmp` return, not bool" | "Sort functions can be tricky" |
| **Actionability** | 0.25 | Imperative verbs, clear steps, copy-paste commands | "Always run `go vet ./...` before committing Go changes" | "Code quality is important" |
| **Novelty** | 0.20 | Uniqueness vs common knowledge | "macOS aliases `cp` to `cp -i`; use `/bin/cp` to bypass" | "Use `cp` to copy files" |
| **Context** | 0.15 | Quality of surrounding context, when/where this applies | "In Go HTTP handlers using `net/http`, always check `err != nil` after `json.Decode` because..." | "Check errors" |
| **Confidence** | 0.10 | Assertion strength, hedging vs certainty | "Empirically verified: `-s read-only` + `-o` works in Codex" | "This might work" |

### Tier Thresholds

| Tier | Score Range | Auto-Promotion | Human Review |
|------|-------------|----------------|--------------|
| Gold | 0.85 - 1.00 | Auto-promoted to `.agents/learnings/` after staging | Not required |
| Silver | 0.70 - 0.84 | Auto-promoted after 24h if no objection | Optional |
| Bronze | 0.50 - 0.69 | Requires human review | Required |
| Discard | < 0.50 | Not stored | N/A |

### Connection to Existing Infrastructure

- **Pool:** Uses existing `pool.Add()` with `types.Scoring` struct. The scoring dimensions (`types.RubricScores`) already exist in `cli/internal/types/types.go` with the exact weights above.
- **Pool flow:** Existing pool pipeline: pending -> staged -> promoted (or rejected). SCORE determines the entry point tier.
- **MemRL:** After promotion, artifacts enter the MemRL utility tracking system. Utility updates via `ao feedback` adjust the artifact's retrieval priority over time.
- **Maturity:** Scored artifacts get initial maturity of `provisional`. Positive feedback via `ao feedback --helpful` advances maturity through `candidate` -> `established`. Negative feedback can demote to `anti-pattern`.

### Failure Mode Analysis

| Failure | Impact | Mitigation |
|---------|--------|------------|
| SCORE missing | All artifacts treated equally. High-quality and low-quality learnings compete for the same context window space. Token budget wasted on noise. | Without SCORE, inject loads by recency. This means a vague learning from yesterday outranks a precise learning from last week. SCORE fixes this by weighting quality. |
| Scores too generous | Everything is gold. No filtering happens. Pool fills with mediocre artifacts. | The rubric is mechanical (keyword detection, not LLM judgment). Specificity counts named entities. Actionability counts imperative verbs. Hard to game with vague content. |
| Scores too strict | Nothing makes it to gold. Human review backlog grows. | Bronze threshold (0.50) is intentionally low. Most specific, actionable learnings clear it. If the threshold is wrong, it's a config change, not an architecture change. |

### Dependencies

Depends on **CATALOG** (needs typed content to score). Benefits from **VERIFY** (verified artifacts get a confidence boost) and **INDEX** (indexed artifacts have richer context for scoring).

---

## Stage 5: REJECT

**What it does:** Filters out learnings that score below threshold. Removes stale knowledge where decay exceeds usefulness. This is the unlearning mechanism.

### Input

Scored pool entries plus temporal signals:

- Current score from SCORE stage
- Age of the artifact (time since creation)
- Decay rate (default: 17%/week from Darr 1995, configurable via `types.DefaultDelta`)
- Citation count from `ao feedback` / citation tracker
- MemRL utility value (updated via EMA: `u_{t+1} = (1-alpha) * u_t + alpha * r`)

### Output

Rejected artifacts moved to `.agents/pool/rejected/` with rejection metadata:

```json
{
  "candidate": { "...": "..." },
  "scoring_result": { "...": "..." },
  "rejection": {
    "schema_version": 1,
    "rejected_at": "2026-02-24T12:20:00Z",
    "reason": "stale_decay",
    "detail": "Utility 0.18 below threshold 0.30 after 12 weeks without citation",
    "reviewer": "ao-curate-reject",
    "recoverable": true
  },
  "status": "rejected"
}
```

### Rejection Criteria

| Criterion | Threshold | Signal |
|-----------|-----------|--------|
| **Low score** | Raw score < 0.50 | Artifact is too vague, unactionable, or unoriginal to be useful |
| **Stale decay** | Utility < 0.30 after N weeks without citation | Knowledge has decayed past usefulness. No agent cited it, so it's not being applied |
| **Negative feedback** | MemRL utility < `MaturityDemotionThreshold` (0.30) with 3+ feedback events | Agents that used this learning reported it was unhelpful |
| **Superseded** | Newer learning explicitly replaces older one (`superseded_by` set) | The older learning is outdated. The newer one should be used instead |
| **Expired** | `valid_until` date has passed and `expiry_status` is `expired` | Time-sensitive learning (API version, library behavior) is no longer current |
| **Anti-pattern** | Maturity downgraded to `anti-pattern` with 5+ harmful feedback events | Learning is actively harmful. Kept in rejected pool for audit trail |

### Connection to Existing Infrastructure

- **Pool:** Uses existing `pool.Reject()` which moves entries to `.agents/pool/rejected/` and records a `ChainEvent` in `chain.jsonl`. Rejected entries remain on disk for audit.
- **MemRL:** Rejection criteria use existing utility values from `ao feedback` and maturity from CASS (`types.Maturity`). Thresholds use existing constants: `MaturityDemotionThreshold = 0.3`, `MinFeedbackForAntiPattern = 5`.
- **Decay:** Uses existing `ConfidenceDecayRate = 0.1` (10%/week) and `DefaultDelta = 0.17` (17%/week knowledge decay from Darr 1995).
- **Supersession:** Uses existing `types.Supersede()` and `candidate.IsSuperseded()` from `cli/internal/types/types.go`.
- **Expiry:** Uses existing `candidate.IsExpired()` and `candidate.UpdateExpiryStatus()`.
- **CLI:** `ao curate reject --stale --threshold=0.30` -- scans pool for artifacts meeting rejection criteria.

### Failure Mode Analysis

| Failure | Impact | Mitigation |
|---------|--------|------------|
| REJECT missing | Stale, wrong, and superseded learnings accumulate indefinitely. Context windows fill with noise. Agents apply outdated knowledge, causing regressions. This is the autoimmune disease scenario. | Without REJECT, the only cleanup is manual deletion. The flywheel equation `dK/dt = I(t) - delta*K + sigma*rho*K - B(K, K_crit)` has no delta term enforced -- knowledge grows without bound but quality degrades. |
| Over-rejection | Useful learnings removed too aggressively. Knowledge base shrinks below useful size. | Rejection is recoverable: rejected entries stay in `.agents/pool/rejected/` and can be restored. `ao pool list --status rejected` shows what was removed and why. |
| Under-rejection | Thresholds too lenient. Stale knowledge persists. | Thresholds are configurable. Start conservative (reject only below 0.30 utility), tighten based on flywheel metrics. `ao flywheel status` shows if velocity is negative (decay winning). |
| Feedback gaming | Agent always reports "helpful" to prevent rejection. | MemRL uses implicit signals (citation tracking) alongside explicit feedback. An artifact that is "helpful" but never cited still decays via time-based confidence decay. |

### Dependencies

Depends on **SCORE** (needs quality scores to evaluate). Uses **VERIFY** signals (unverified artifacts are rejection candidates). Benefits from **INDEX** (stale detection is per-topic, not global).

---

## Stage 6: CONSTRAIN

**What it does:** Compiles verified, high-scoring, repeatedly-cited learnings into permanent defenses: hook rules, test assertions, or validation gate checks. Converts experience into code.

### Input

Established learnings meeting all promotion criteria:

- Maturity: `established` (passed through `provisional` -> `candidate` -> `established`)
- Utility: >= `MaturityPromotionThreshold` (0.70)
- Feedback count: >= `MinFeedbackForPromotion` (3)
- Score: gold tier (>= 0.85)
- Verified: `tests_passed: true`
- Tagged as `constraint` or `anti-pattern` during INDEX

### Output

Constraint template written to `.agents/constraints/`:

```bash
#!/usr/bin/env bash
# Constraint: <learning title>
# Source: <learning ID>
# Generated: <timestamp>
# Lifecycle: draft
# Schema-Version: 1
#
# This constraint was compiled from learning <ID> which scored
# gold tier with utility 0.92 after 5 positive feedback events.
# Edit or delete this file to modify the constraint.

# Detection pattern (fill in or replace with actual check)
if false; then
  echo "CONSTRAINT VIOLATION: <description from learning>"
  echo "Source: .agents/pool/staged/<learning-id>.json"
  exit 1
fi
```

For the current finding-compiler slice, the path is registry-first and promotion is additive. Reusable findings are recorded in `.agents/findings/registry.jsonl`, deduplicated into promoted finding artifacts under `.agents/findings/<id>.md`, and then compiled into the advisory surfaces that planning and validation actually read: `.agents/planning-rules/<id>.md` and `.agents/pre-mortem-checks/<id>.md`. When detector metadata is present and valid, the same promoted artifact can also compile into `.agents/constraints/index.json` for mechanical enforcement. The narrow limit is important: not every registry row becomes an active constraint, and the registry remains the canonical intake ledger.

### Constraint Lifecycle

```
draft ──────> active ──────> retired
  │              │              │
  │  human/agent │   no longer  │
  │  fills in    │   relevant   │
  │  detection   │   or causes  │
  │  pattern     │   false      │
  │              │   positives  │
  v              v              v
 (template)   (enforced)    (archived)
```

| State | Behavior |
|-------|----------|
| `draft` | Generated from learning. Template with placeholder detection pattern. Not enforced. |
| `active` | Detection pattern filled in. Enforced by hooks during validation gates. |
| `retired` | No longer relevant. Kept for audit trail. Not enforced. |

### Connection to Existing Infrastructure

- **Hooks:** Active constraints are sourced by `task-validation-gate.sh` during validation. The hook reads `.agents/constraints/index.json`, evaluates supported detector kinds, and treats companion `.sh` files as review artifacts rather than executable runtime contracts.
- **Post-mortem / Retro:** After extracting learnings, if any score >= 4/5 on actionability and are tagged `constraint` or `anti-pattern`, the skill invokes `hooks/constraint-compiler.sh <learning-path>` to generate the template.
- **Pool:** Reads established learnings from `.agents/pool/staged/` or promoted artifacts in `.agents/learnings/`.
- **CLI:** `ao curate constrain` -- scans for established learnings meeting promotion criteria, generates constraint templates.

### Failure Mode Analysis

| Failure | Impact | Mitigation |
|---------|--------|------------|
| CONSTRAIN missing | Learnings stay as prose. Agents must re-read and re-interpret them every session. No mechanical enforcement of lessons learned. The system remembers but doesn't act on memory. | Without CONSTRAIN, the flywheel still works -- learnings are still retrieved and injected. But enforcement depends on the agent reading the learning AND acting on it, which is probabilistic, not guaranteed. |
| Bad constraint generated | Detection pattern has false positives. Blocks valid work. | Constraints start as `draft` (not enforced). Activation requires explicit human/agent review. `retired` lifecycle state handles constraints that develop false positives. |
| Constraint overload | Too many active constraints slow down validation gates. | Gate budget: maximum 50 active constraints (configurable). Oldest low-utility constraints auto-retire when budget exceeded. |
| Stale constraints | Codebase changes make constraint irrelevant. | Constraints carry source learning ID. If the source learning is rejected (Stage 5), the constraint is auto-retired. Constraint files are editable code -- delete to remove. |

### Dependencies

Depends on **SCORE** (needs gold-tier artifacts). Depends on **VERIFY** (only verified learnings become constraints). Benefits from **INDEX** (constraint tagging happens during indexing). Benefits from **REJECT** (rejected learnings don't become constraints).

---

## Quality Degradation

The pipeline degrades gracefully. Each version is strictly better than the previous state.

| State | What Works | What's Missing | Net Effect |
|-------|-----------|----------------|------------|
| **Registry-first intake + compiled prevention** (current slice) | `/pre-mortem`, `/vibe`, and `/post-mortem` read/write `.agents/findings/registry.jsonl`; the compiler promotes reusable entries into `.agents/findings/<id>.md`, then into `.agents/planning-rules/<id>.md` and `.agents/pre-mortem-checks/<id>.md`; mechanically detectable active findings also upsert `.agents/constraints/index.json` for runtime enforcement. | Registry intake still defaults to advisory promotion, and only mechanically detectable active findings can become enforced constraints. | Prevention is structured and partially automated: planning/review always see compiled outputs first, and task validation can enforce the mechanical subset. |
| **v1: CATALOG + VERIFY** | Artifacts are typed (learning/decision/failure/pattern). Mechanical verification (tests passed? goals improved?). | No topic tagging, no quality scoring, no rejection, no constraints. | Structured accumulation. Wrong learnings caught by test verification. Already a major improvement: the single biggest risk (confidently wrong learnings) is mitigated. |
| **v2: + INDEX + SCORE** | Artifacts are findable by topic. Quality-gated into tiers. Search returns ranked results. Lookup loads best-quality first. | No automated rejection, no constraint compilation. | Quality-filtered retrieval. Token budget spent on high-quality learnings first. Noise reduced proportional to scoring accuracy. |
| **v3: + REJECT + CONSTRAIN** | Stale/wrong learnings removed. High-quality learnings compiled into hooks. Full feedback loop. | Manual constraint activation. | Self-organizing system. Generates its own defenses from experience. Unlearns what no longer applies. The flywheel equation has all terms active. |

**v1 alone closes the biggest gap.** The jump from "no pipeline" to "v1" is larger than any subsequent jump. Structured, verified artifacts are dramatically better than unstructured, unverified markdown -- even without scoring or rejection.

---

## Pipeline Flow Diagram

```
  Agent Session
       │
       ▼
  ┌──────────┐     ┌─────────┐
  │ /forge   │     │ /retro  │    (existing skills, unchanged)
  └────┬─────┘     └────┬────┘
       │                │
       ▼                ▼
  ┌──────────────────────────┐
  │  STAGE 1: CATALOG        │  v1
  │  Structure into typed    │
  │  artifacts with metadata │
  │  → .agents/pool/pending/ │
  └────────────┬─────────────┘
               │
               ▼
  ┌──────────────────────────┐
  │  STAGE 2: VERIFY          │  v1
  │  Mechanical checks:      │
  │  tests pass? goals up?   │
  │  → verification metadata │
  └────────────┬─────────────┘
               │
               ▼
  ┌──────────────────────────┐
  │  STAGE 3: INDEX           │  v2
  │  Tag by topic, skill,    │
  │  goal. Update search idx │
  │  → ao search, ao lookup  │
  └────────────┬─────────────┘
               │
               ▼
  ┌──────────────────────────┐
  │  STAGE 4: SCORE           │  v2
  │  5-dim quality gate:     │
  │  specificity, action-    │
  │  ability, novelty,       │
  │  context, confidence     │
  │  → gold/silver/bronze    │
  └────────────┬─────────────┘
               │
          ┌────┴────┐
          │         │
          ▼         ▼
  ┌────────────┐  ┌──────────────┐
  │ STAGE 5:   │  │  Promoted to │  v3
  │ REJECT     │  │  .agents/    │
  │ stale,     │  │  learnings/  │
  │ wrong,     │  │  patterns/   │
  │ superseded │  └──────┬───────┘
  │ → rejected/│         │
  └────────────┘         ▼
                  ┌──────────────┐
                  │ STAGE 6:     │  v3
                  │ CONSTRAIN    │
                  │ Compile into │
                  │ hook rules   │
                  │→ constraints/│
                  └──────────────┘
                         │
                         ▼
                  Validation Gates
                  (enforced by hooks)
```

---

## Schema Versioning

All curation artifacts carry `schema_version: 1` in their metadata. This applies to:

- Pool entries (CATALOG output)
- Verification results (VERIFY output)
- Index metadata (INDEX output)
- Scoring results (SCORE output)
- Rejection records (REJECT output)
- Constraint templates (CONSTRAIN output)
- Promoted artifacts in `.agents/learnings/` and `.agents/patterns/`

When the schema evolves, the version increments. Readers check the version and apply appropriate parsing. Old-format artifacts without `schema_version` are treated as version 0 (pre-pipeline) and processed with best-effort compatibility.

---

## See Also

- [Knowledge Flywheel](knowledge-flywheel.md) -- The compounding loop that curation improves
- [The Science](the-science.md) -- Decay rates, escape velocity, formal model
- [Architecture](ARCHITECTURE.md) -- System design (Pillar 4: Knowledge Flywheel)
- `/forge` skill -- Transcript mining (CATALOG input)
- `/retro` skill -- Retrospective extraction (CATALOG input)
- `/post-mortem` skill -- Council-validated findings + finding-registry emission
- `ao pool list` -- View pool entries by tier and status
- `ao feedback` -- Record MemRL reward signals
- `ao search` -- Search knowledge base (improved by INDEX)
- `ao lookup` -- Query prior knowledge on demand (improved by INDEX + SCORE)
- `ao flywheel status` -- Knowledge growth rate and escape velocity
