# Compiled Prevention Loading

> Extracted from premortem/SKILL.md on 2026-04-11.

## Step 1.4: Retrieve Prior Learnings (Mandatory)

Before review, retrieve learnings relevant to this plan's domain:

```bash
if command -v ao &>/dev/null; then
    # Decision-point pull: prefer the curated GOLD wiki with compact pointers
    # (no bodies) and a hard top-K cap — bookend-bounded (ADR-0002). Fall back to
    # the raw .agents/ corpus, but WARN loudly so a missing gold wiki is never a
    # silent zero-result.
    if [ -d .ao/wiki ]; then
        ao lookup --query "<plan goal or title>" --gold --pointers --limit 3 2>/dev/null | head -20
    else
        echo "WARN: gold wiki (.ao/wiki) absent — run 'ao wiki gold' to enable gold retrieval; falling back to raw .agents/ corpus" >&2
        ao lookup --query "<plan goal or title>" --limit 3 2>/dev/null | head -20
    fi
fi
```

If learnings are returned, include them as `known_context` in the review packet. Cite any learning by filename when it influences a prediction. The gold-absent path WARNs (not silent) and falls back to the raw corpus; skip silently only if ao is unavailable or returns no results.

## Step 1.4b: Load Compiled Prevention First (Mandatory)

Before quick or deep review, load compiled checks from `.agents/premortem-checks/*.md` when they exist. This is separate from flywheel search and does NOT get skipped by `--quick`.

Use the tracked contracts in `docs/contracts/finding-compiler.md` and `docs/contracts/finding-registry.md`:

- prefer compiled premortem checks first
- rank by severity, `applicable_when` overlap, language overlap, and literal plan-text overlap
- when the plan names files, rank changed-file overlap ahead of generic keyword matches
- cap at top 5 findings / check files
- if compiled checks are missing, incomplete, or fewer than the matched finding set, fall back to `.agents/findings/registry.jsonl`
- fail open:
  - missing compiled directory or registry -> skip silently
  - empty compiled directory or registry -> skip silently
  - malformed line -> warn and ignore that line
  - unreadable file -> warn once and continue without findings

Include matched entries in the council packet as `known_risks` with:

- `id`
- `pattern`
- `detection_question`
- `checklist_item`

Use the same ranked packet contract as `/plan`: compiled checks first, then active findings fallback, then matching high-severity next-work context when relevant. Avoid re-ranking with an unrelated heuristic inside premortem; the point is consistent carry-forward, not a fresh retrieval policy per phase.

### Record Citations for Applied Knowledge

After including matched entries as `known_risks`, record each citation so the flywheel feedback loop can track influence:

```bash
# Only use "applied" when the finding actually influenced the council packet.
# Use "retrieved" for items loaded but not referenced in the risk assessment.
ao metrics cite "<finding-path>" --type applied 2>/dev/null || true   # influenced risk assessment
ao metrics cite "<finding-path>" --type retrieved 2>/dev/null || true # loaded but not used
```

### Section Evidence

When lookup results include `section_heading`, `matched_snippet`, or `match_confidence` fields, prefer the matched section over the whole file — it pinpoints the relevant portion. Higher `match_confidence` (>0.7) means the section is a strong match; lower values (<0.4) are weaker signals. Use the `matched_snippet` as the primary context rather than reading the full file.

## Step 1a: Search Knowledge Flywheel (skip if `--quick`)

Only run this step for `--deep`, `--mixed`, or `--debate`.

```bash
if command -v ao &>/dev/null; then
    ao search "plan validation lessons <goal>" 2>/dev/null | head -10
fi
```

If ao returns prior plan review findings, include them as context for the council packet. Skip silently if ao is unavailable or returns no results.

## Step 1b: Check for Product Context

Quick mode does not create a separate product-review phase. Include product
context in the fresh judge's bounded packet only when the plan changes product
behavior or cites `PRODUCT.md`. Deep modes may add a dedicated perspective.

```bash
if [ -f PRODUCT.md ]; then
  # PRODUCT.md exists — include product perspectives alongside plan-review
fi
```

When `PRODUCT.md` is relevant to the plan and the user did not pass an explicit
`--preset` override:

1. Read `PRODUCT.md` content and include in the council packet via `context.files`
2. In `--quick` mode, the one fresh judge assesses only the product claims in
   the bounded packet.
3. In non-quick modes, add a single consolidated `product` perspective to the council invocation:
   ```
   /council --preset=plan-review --perspectives="product" validate <plan-path>
   ```
   This yields 3 judges total (2 plan-review + 1 product). The product judge covers user-value, adoption-barriers, and competitive-position in a single review.
4. With `--deep`: 5 judges (4 plan-review + 1 product).

When the user passed an explicit `--preset`, it takes precedence.

When product context is absent or irrelevant, proceed without it.

> **Tip:** Create `PRODUCT.md` from `docs/PRODUCT-TEMPLATE.md` to enable product-aware plan validation.
