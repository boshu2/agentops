---
id: design-2026-05-02-ai-native-zero-trust-release-process
type: design
date: 2026-05-02
goal: "Make the AI-agent-native release process right: zero-trust CI/CD with SIL/VIL/HIL and digital-twin evidence before build/tag/release."
verdict: PASS
---

# Design: AI-Native Zero-Trust Release Process

## Goal

Make the release process match the product thesis: normal CI/CD assumes a passing
pipeline and code shape are enough; an AI-agent-native release must distrust both
until realistic environments have been exercised. The release gate should require
evidence from SIL, VIL, HIL, and a digital twin before build/tag/publish.

## Alignment Matrix

| Dimension | Score | Rationale |
| --- | ---: | --- |
| Gap Alignment | 3/3 | Directly addresses the Quality-First Maintainer gap: validation gates must block, not advise. |
| Persona Fit | 3/3 | Serves maintainers shipping fewer, higher-confidence releases and operators managing agent-produced change. |
| Competitive Diff | 3/3 | Strengthens AgentOps' core differentiation: operational discipline for indeterministic coding agents. |
| Precedent | 2/3 | The closed `soc-h22t` SIL/VIL/HIL readiness work is a strong base; digital twin is the missing release-specific lane. |
| Scope Fit | 2/3 | Broad but appropriate for a release-process epic; must be split into evidence contract, runners, workflow, and audit work. |

Average: 2.6/3.0

## Verdict

DESIGN VERDICT: PASS

Recommendation: proceed with discovery and planning. Treat this as a second-layer
release epic, not a rewrite of the just-closed SIL/VIL/HIL readiness score.

The product-fit constraint is that the release gate must remain usable. Waivers
can exist for unavailable physical targets, but they must be explicit evidence,
not a silent bypass.
