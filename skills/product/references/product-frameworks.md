# Product-sense lenses (framework reference)

> **Provenance:** This lens table was **moved verbatim** out of `skills/product/SKILL.md`
> §3g (bead `age-skills-audit-fable-l6ic.10`, generic-craft trim). It is a nine-framework
> name-drop — a capable model already knows these frameworks and applies them without the
> table. The skill's durable value is the AgentOps council-autoload wiring (PRODUCT.md →
> `/pre-mortem` + `/validate` product/DX perspectives) and the interview flow, both of which
> stay in `SKILL.md`. This file is the reference for the per-lens questions; consult it
> during §3g, but do **not** name-drop these frameworks in the generated PRODUCT.md —
> translate each into a concrete product decision.

For each lens, gather or infer the answer:

| Lens | Question to answer | Output it should shape |
|------|--------------------|------------------------|
| **Chesky 10/11-star experience** | What would make the first meaningful use feel unexpectedly great, not merely functional? | `10-Star Experience` section and first-value path. |
| **Rahul Vohra / Superhuman PMF** | Which narrow segment would be very disappointed if this disappeared? Who should we ignore for now? | `PMF Wedge`, target personas, and anti-personas. |
| **April Dunford positioning** | What is the real alternative, where does it win, and what context makes this product obviously better? | Competitive positioning and strategic bet. |
| **Teresa Torres discovery** | What recurring customer touchpoints or experiments will keep this honest? | Evidence and discovery metrics. |
| **Marty Cagan outcomes** | What user/business outcome matters beyond shipped features? | Core value propositions and known gaps. |
| **Gibson Biddle DHM** | How does the product delight users in ways that are hard to copy and sustainable to keep improving? | Product strategy and moat. |
| **Elena Verna PLG** | Can the user reach value without human glue or heavy setup? Where is friction too high? | 10-star experience and onboarding gaps. |
| **Melissa Perri build-trap guardrail** | Are we listing features or making strategic choices tied to target conditions? | Product strategy and prioritization. |
| **Shreyas Doshi product sense** | What motivation, friction, satisfaction, and nudges decide whether usage repeats? | Value props, activation, and retention loop. |

## Canonical PRODUCT.md Template

Use this section order. The bracketed text describes required content; the
Validated Principles block is the only optional section.

```markdown
---
last_reviewed: YYYY-MM-DD
---

# PRODUCT.md

## Mission
[One sentence: what the product does and for whom.]

## Vision
[What the world looks like if the product succeeds.]

## Target Personas
### Persona: [role]
- **Goal:** [desired outcome]
- **Pain point:** [current obstacle]
- **Gap exposure:** [which known gap this persona feels]

## PMF Wedge
[Narrow segment to optimize for, who would be disappointed without it, and anti-personas intentionally out of scope.]

## 10-Star Experience
[First 30–60 minutes, first evidence of value, and what makes the next use better.]

## What the Product Actually Is
[Concrete architectural layers and the gap each closes; not a feature-list slogan.]

## Core Value Propositions
- [Outcome mapped to a concrete gap.]

## Product Strategy
- **Delight:** [source of user love]
- **Hard to copy:** [compounding differentiation]
- **Sustainable:** [how it improves without manual-service drag]
- **Outcome:** [target condition over feature count]
- **Retention loop:** [why repeat use becomes more valuable]

## Design Principles
[Optional: validated principles with counts and source links. Otherwise use interview-derived operational principles.]

## Competitive Positioning
| Alternative | Where They Win | Where We Win |
|-------------|----------------|--------------|
| [alternative] | [honest strength] | [context-specific advantage] |

## Product Sense Review
| Lens | Decision |
|------|----------|
| 10-star experience | [first-use delight] |
| PMF wedge | [narrow segment] |
| Positioning | [category/context] |
| Continuous discovery | [evidence-refresh loop] |
| Outcome over output | [target condition] |
| PLG friction | [self-serve/onboarding choice] |
| Build-trap guardrail | [what not to build or claim] |

## Strategic Bet
[Contrarian market thesis.]

## Evidence
**Traction:**
- [metric]: [value and time period]

**Measured Impact:**
- [outcome]: [evidence]

[If pre-traction: `Pre-traction — tracking: ...`.]

## Known Product Gaps
| Gap | Impact | Status |
|-----|--------|--------|
| [gap] | [who it affects and how] | [open/in-progress/planned] |

## Usage
- `/pre-mortem`: loads product context; deeper modes add a product perspective.
- `/validate`: loads developer-experience context; deeper modes add a DX perspective.
- `/discovery`: turns the wedge and journey into acceptance behaviors.
- `/council --preset=product`: product review on demand.
- `/council --preset=developer-experience`: DX review on demand.

An explicit user preset overrides automatic context inclusion.
```
