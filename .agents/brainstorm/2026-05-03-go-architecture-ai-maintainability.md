---
id: brainstorm-2026-05-03-go-architecture-ai-maintainability
type: brainstorm
date: 2026-05-03
---
# Brainstorm: Go Architecture AI Maintainability

## Problem Statement

The Go CLI surface has grown large enough that AI maintainers need mechanically enforced consistency: clean formatting, repo-root path handling that works on macOS symlinked temp roots, and runtime state paths that honor the canonical AgentOps path resolver.

## Approaches Considered

1. Full CLI architecture rewrite. This would move broad Cobra command logic into internal packages, but it touches too much behavior for a single autonomous overnight run.
2. Mechanical consistency pass. This fixes formatting drift, failing path-canonicalization tests, and high-signal state-path resolver violations without changing command contracts.
3. Documentation-only architecture map. This helps future agents but leaves the current failing Go suite and resolver drift in place.

## Selected Approach

Use the mechanical consistency pass. It is bounded, testable, aligned with the active state-path-resolver pattern, and directly improves the baseline for future AI work.

## Open Questions

Longer-term command-layer thinning remains open because prior research found many direct stdout/global-state patterns across `cli/cmd/ao`. This run will file or preserve follow-up work instead of attempting a package-wide rewrite.

## Next Step: $plan

Run `$plan --auto "fix Go architecture consistency issues for AI maintainability"` using epic `soc-lb4e` and child issues `soc-lb4e.2`, `soc-lb4e.3`, and `soc-lb4e.4`.
