# Alignment Matrix -- Scoring Rubric

The design gate evaluates proposed goals against five dimensions, each scored 0-3. This rubric defines what each score level means for each dimension.

## Gap Alignment

Does the proposed goal address a known product gap from PRODUCT.md?

| Score | Label | Criteria |
|-------|-------|----------|
| 0 | No gap match | Goal does not relate to any identified product gap |
| 1 | Tangential | Goal is loosely related to a gap but does not directly address it |
| 2 | Addresses gap | Goal clearly addresses an identified gap |
| 3 | Directly closes gap | Goal is the primary action needed to close a specific gap |

## Persona Fit

Does the proposed goal serve personas defined in PRODUCT.md?

| Score | Label | Criteria |
|-------|-------|----------|
| 0 | No persona served | No defined persona benefits from this goal |
| 1 | Edge case | A persona benefits only in rare or secondary workflows |
| 2 | Serves persona | A defined persona clearly benefits in their primary workflow |
| 3 | Primary need | This is a top-priority need for one or more core personas |

## Competitive Differentiation

Does the proposed goal strengthen the product's competitive position?

| Score | Label | Criteria |
|-------|-------|----------|
| 0 | Parity | Goal only achieves what competitors already offer |
| 1 | Minor edge | Goal provides a small advantage over competitors |
| 2 | Clear differentiator | Goal creates meaningful separation from competitors |
| 3 | Unique capability | Goal creates a capability no competitor offers |

## Precedent

Has similar work been done before? What can we learn from prior art?

| Score | Label | Criteria |
|-------|-------|----------|
| 0 | No prior art | No precedent exists in the codebase, knowledge base, or industry |
| 1 | Partial reference | Some related work exists but does not directly apply |
| 2 | Clear precedent | Similar work has been done and provides actionable patterns |
| 3 | Proven pattern | A well-established, battle-tested pattern exists that maps directly |

**Scoring note:** A score of 0 here is not inherently bad -- novel work scores low on precedent but may score high on competitive differentiation. The design gate considers all five dimensions together.

## Scope Fit

Is the proposed goal appropriately scoped for the current phase?

| Score | Label | Criteria |
|-------|-------|----------|
| 0 | Unbounded | Goal has no clear boundaries, deliverables, or exit criteria |
| 1 | Loose | Goal has vague boundaries; scope creep is likely |
| 2 | Well-scoped | Goal has clear boundaries, deliverables, and exit criteria |
| 3 | Surgical | Goal is precisely scoped with minimal surface area and clear done-state |

## Verdict Thresholds

| Condition | Verdict |
|-----------|---------|
| Average >= 2.0 AND no dimension at 0 | PASS |
| Average >= 1.5 OR one dimension at 0 with others compensating | WARN |
| Average < 1.5 OR multiple dimensions at 0 | FAIL |

With `--strict`: PASS threshold rises to average >= 2.5.
