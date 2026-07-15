---
name: refactor
description: Execute one behavior-preserving structural
---
# Refactor — one structural experiment

Refactor changes structure while preserving observable behavior. It performs one
caller-selected transformation and reports the result.

## Procedure

1. Name the preserved behavior and the focused acceptance surface.
2. Record an honest baseline, including any reproducible ambient failures.
3. Apply one bounded transformation: extract, rename, inline, simplify,
   encapsulate, move, or delete dead code.
4. Run the focused check and the smallest package-level regression check justified
   by the changed surface.
5. Return the diff summary, commands, results, and behavior not checked.

Do not combine a newly discovered behavior fix with the structural change. A red
result is evidence for the caller; this skill does not revert, narrow, retry,
commit, validate, or route subsequent work automatically.

## References

- [Behavior-preserving simplification](references/behavior-preserving-simplification.md)
- [Behavior scenarios](references/refactor.feature)
