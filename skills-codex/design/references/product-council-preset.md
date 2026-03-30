# Product Council Preset

The `--preset=product` council configuration uses three product-strategy perspectives. Used by `$design` to validate goal alignment with PRODUCT.md.

## Judge Perspectives

### 1. User Value Judge

**Focus:** Does this goal deliver meaningful value to defined personas?

Evaluates:
- Which personas benefit and how directly
- Whether the goal solves a real user problem or is internally motivated
- Impact on user workflows and adoption friction
- Whether the value is immediate or requires additional work to realize

### 2. Adoption Barriers Judge

**Focus:** What prevents this goal from succeeding in practice?

Evaluates:
- Implementation complexity relative to the team's current capacity
- Dependencies on external systems or uncommitted work
- Migration or breaking-change risk for existing users
- Documentation and discoverability requirements
- Whether the goal introduces new concepts users must learn

### 3. Competitive Position Judge

**Focus:** Does this goal strengthen or weaken competitive standing?

Evaluates:
- Whether the goal creates differentiation or merely achieves parity
- How competitors have approached similar problems
- Whether the goal locks in advantages or creates switching costs
- Alignment with the product's stated competitive strategy in PRODUCT.md

## Verdict Combination

Each perspective produces an independent assessment with:
- **Verdict:** PASS, WARN, or FAIL
- **Confidence:** high, medium, or low
- **Key concern:** one-sentence summary of the primary risk identified

The combined verdict follows standard consensus rules:
- **All PASS:** PASS
- **Any FAIL:** FAIL (with explanation from the failing perspective)
- **Mixed PASS/WARN:** WARN (with concerns aggregated)

The verdict is reported alongside the alignment matrix scores in the design artifact. Both inform the final `$design` verdict, but the alignment matrix scores are the primary decision mechanism.

## Invocation (Codex)

In Codex, the three perspectives are evaluated inline rather than via agent spawning. The lead agent cycles through each perspective sequentially, writing findings for each before synthesizing the combined verdict.

Pass the alignment matrix (with scores and rationale) as context so each perspective evaluation has full information rather than re-deriving scores independently.
