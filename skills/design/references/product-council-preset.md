# Product Council Preset

The `--preset=product` council configuration spawns judges with product-strategy perspectives. Used by `/design` to validate goal alignment with PRODUCT.md.

## Judge Perspectives

When council runs with `--preset=product`, it assigns three specialized perspectives:

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

Each judge produces an independent assessment with:
- **Verdict:** PASS, WARN, or FAIL
- **Confidence:** high, medium, or low
- **Key concern:** one-sentence summary of the primary risk identified

The combined council verdict follows standard council consensus rules:
- **All PASS:** Council PASS
- **Any FAIL:** Council FAIL (with explanation from the failing judge)
- **Mixed PASS/WARN:** Council WARN (with concerns aggregated)

The council verdict is reported alongside the alignment matrix scores in the design artifact. Both inform the final `/design` verdict, but the alignment matrix scores are the primary decision mechanism.

## Invocation

```
Skill(skill="council", args="--preset=product validate design alignment for: <goal>")
```

Pass the alignment matrix (with scores and rationale) as context so judges can evaluate with full information rather than re-deriving scores independently.
