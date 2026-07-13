# Context Assembly Interface Contract

> **Status:** Draft
> **Covers:** Epic B (Adaptive Context Assembly) <-> Epic D (Mechanical Context Budget)
> **Source files:** `cli/cmd/ao/rpi_phased_handoff.go`, `cli/cmd/ao/rpi_phased_context.go`

This contract defines the interface between two future epics that will replace the current hardcoded context assembly in `buildHandoffContext` and `buildPromptForPhase`.

**Current behavior being replaced:**
- `buildHandoffContext` unconditionally emits all handoff fields for every phase.
- Narrative is capped at a hardcoded 1000 chars in `buildHandoffContext` (line 189) and 2000 chars in `buildPhaseHandoffFromState` (line 241).
- `phaseContextBudgets` are prose strings injected into prompts — no mechanical enforcement.

---

## 1. Phase Manifest Schema

A `phaseManifest` declares what context a phase needs and how much room it gets.

```go
type phaseManifest struct {
    Phase         int      `json:"phase"`
    HandoffFields []string `json:"handoff_fields"` // e.g., ["goal","epic_id","verdicts"]
    NarrativeCap  int      `json:"narrative_cap"`  // max chars for narrative; see NarrativeCap Semantics below
    MaxTokens     int      `json:"max_tokens"`     // total token budget for assembled context
}
```

### Field reference

`HandoffFields` values correspond to JSON keys on `phaseHandoff`:

| Field               | Type               | Notes                                    |
|---------------------|--------------------|------------------------------------------|
| `goal`              | `string`           | From latest handoff with non-empty Goal  |
| `epic_id`           | `string`           | Omitted when empty                       |
| `verdicts`          | `map[string]string`| Gate verdicts accumulated across phases  |
| `artifacts_produced`| `[]string`         | File paths discovered by phase           |
| `decisions_made`    | `[]string`         | Key decisions recorded during phase      |
| `open_risks`        | `[]string`         | Unresolved risks carried forward         |
| `narrative`         | `string`           | Free-text summary, subject to NarrativeCap |

### NarrativeCap Semantics

`NarrativeCap` controls narrative inclusion in assembled context. Its meaning depends on whether `HandoffFields` is set:

| HandoffFields | NarrativeCap | Behavior |
|---------------|-------------|----------|
| non-empty     | `0`         | **Omit** narrative entirely (least-privilege: phase didn't request it) |
| non-empty     | `> 0`       | Include narrative, truncated to `NarrativeCap` chars |
| empty/nil     | `0`         | **Default 1000** chars (backward compat: no manifest means "include everything") |
| empty/nil     | `> 0`       | Include narrative, truncated to `NarrativeCap` chars |

This overloading exists because `NarrativeCap=0` serves double duty: "omit" (when the manifest explicitly declares fields) vs "use default" (when no manifest exists). The `HandoffFields` array disambiguates.

### Default manifests

```go
var defaultManifests = map[int]phaseManifest{
    // Phase 1 (discovery): no prior context — first phase in the pipeline
    1: {
        Phase:         1,
        HandoffFields: nil,
        NarrativeCap:  0,
        MaxTokens:     0,
    },
    // Phase 2 (implementation): needs goal, plan decisions, and risk awareness
    2: {
        Phase:         2,
        HandoffFields: []string{"goal", "epic_id", "verdicts", "decisions_made", "open_risks"},
        NarrativeCap:  500,
        MaxTokens:     2500,
    },
    // Phase 3 (validation): needs goal, verdicts, and what was built
    3: {
        Phase:         3,
        HandoffFields: []string{"goal", "epic_id", "verdicts", "artifacts_produced"},
        NarrativeCap:  1000,
        MaxTokens:     2500,
    },
}
```

**Rationale:**
- Phase 1 has no predecessors; manifest is empty.
- Phase 2 carries forward decisions and risks so implementation addresses premortem findings. Narrative is short (500 chars) because structured fields convey most of the signal.
- Phase 3 needs to know what was produced (artifacts) to validate it. Longer narrative (1000 chars) gives validation richer context about implementation choices.
- MaxTokens of 2500 per phase keeps context injection under ~10% of a typical 32k context window.

---

## 2. Token Budget API

Epic D implements mechanical token estimation and truncation. These functions replace the current hardcoded char limits.

### Functions

```go
// estimateTokens returns an approximate token count using the char/4 heuristic.
// This is intentionally simple — a tiktoken-based estimator can be swapped in later
// without changing the interface.
func estimateTokens(text string) int

// truncateToTokenBudget truncates text to fit within a token budget.
// Truncation happens at sentence boundaries (". ", ".\n") to avoid mid-sentence cuts.
// If no sentence boundary exists within budget, truncates at the last word boundary.
// Appends "..." when truncation occurs.
func truncateToTokenBudget(text string, budget int) string

// applyContextBudget truncates assembled context to fit the manifest's MaxTokens budget.
// Returns the (possibly truncated) context and a result struct with metrics.
// If manifest.MaxTokens <= 0, returns the input unchanged with WasTruncated=false.
func applyContextBudget(context string, maxTokens int) (string, contextBudgetResult)
```

### Result struct

```go
type contextBudgetResult struct {
    OriginalTokens  int  `json:"original_tokens"`   // estimateTokens(input)
    BudgetTokens    int  `json:"budget_tokens"`      // manifest.MaxTokens
    TruncatedTokens int  `json:"truncated_tokens"`   // estimateTokens(output) — equals OriginalTokens when not truncated
    WasTruncated    bool `json:"was_truncated"`       // true if output != input
}
```

### Invariants

- `estimateTokens("")` returns 0.
- `truncateToTokenBudget(text, 0)` returns `text` unchanged (budget of 0 means unlimited).
- `TruncatedTokens <= BudgetTokens` when `WasTruncated` is true.
- `TruncatedTokens == OriginalTokens` when `WasTruncated` is false.
- Truncation never increases the length of the input.

---

## 3. Integration Point

Epic B and Epic D meet inside `buildHandoffContext`. The current signature:

```go
// Current (unconditional — emits all fields)
func buildHandoffContext(handoffs []*phaseHandoff) string
```

Changes to:

```go
// New (manifest-driven selection + token budget)
func buildHandoffContext(handoffs []*phaseHandoff, manifest phaseManifest) string
```

### Assembly pipeline

The order is **select -> assemble -> truncate**:

1. **Select** (Epic B): Iterate `manifest.HandoffFields`. For each field, extract matching data from the handoff structs. Skip fields not in the manifest. Apply `manifest.NarrativeCap` to the narrative field during selection (before assembly).

2. **Assemble** (existing logic, narrowed): Format selected fields into the text block. The current formatting in `buildHandoffContext` (lines 134-198 of `rpi_phased_handoff.go`) remains the template — it just operates on the filtered field set instead of all fields.

3. **Truncate** (Epic D): After assembly, if `manifest.MaxTokens > 0`, call `applyContextBudget(assembled, manifest)`. Log the `contextBudgetResult` for observability (write to `.agents/rpi/` alongside other phase artifacts).

### Call site change in `buildPromptForPhase`

```go
// Current (rpi_phased_context.go:442-447):
handoffs, _ := readAllHandoffs(cwd, phaseNum)
if len(handoffs) > 0 {
    ctx := buildHandoffContext(handoffs)
    // ...
}

// New:
handoffs, _ := readAllHandoffs(cwd, phaseNum)
if len(handoffs) > 0 {
    manifest := resolveManifest(phaseNum) // looks up defaultManifests, allows override
    ctx := buildHandoffContext(handoffs, manifest)
    // ...
}
```

### Backward compatibility

- When no manifest is configured (pre-migration runs), `resolveManifest` returns a permissive manifest with all fields and `MaxTokens: 0` (unlimited). This preserves current behavior.
- Legacy summary fallback (`buildPhaseContext`) is unaffected — it only activates when no structured handoffs exist.

### Observability

Each invocation of `applyContextBudget` should write a one-line JSON log entry to `.agents/rpi/context-budget-log.jsonl`:

```json
{"run_id":"abc123","phase":2,"original_tokens":3200,"budget_tokens":2500,"truncated_tokens":2480,"was_truncated":true,"ts":"2026-03-02T12:00:00Z"}
```

This enables postmortem analysis of whether budgets are too tight (frequent truncation) or too loose (consistent under-use).
