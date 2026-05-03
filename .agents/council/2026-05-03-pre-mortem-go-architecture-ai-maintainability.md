# Pre-Mortem: Go Architecture AI Maintainability

## Council Verdict: WARN

The plan is implementable and mechanically verifiable, but it must avoid two failure modes:

1. Weakening containment while fixing macOS path aliases. The daemon helper must still reject traversal, absolute outside-root paths, and symlinks that resolve outside the repo.
2. Applying the state-path resolver to staging paths that are intentionally relative to temporary Dream checkpoint directories. Only live runtime state paths should be migrated in this slice.

## Required Hardening

- Add direct daemon unit coverage for symlink-spelled roots or paths where possible.
- Add override tests for overnight `AO_*` path variables so the resolver migration has proof.
- Keep gofmt as the final implementation step.

## Status

WARN is acceptable for implementation. No re-plan required.
