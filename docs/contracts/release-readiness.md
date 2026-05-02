# Release Readiness Contract

Release readiness is the machine-readable contract for deciding whether a
release has enough evidence to tag and publish. The authoritative artifact is:

```text
.agents/releases/local-ci/<run-id>/release-readiness.json
```

The JSON shape is versioned by `release-readiness.v1.schema.json`.

## Score

`scripts/check-release-readiness.sh` writes a 10 point score:

| Dimension | Weight | Release meaning |
|-----------|--------|-----------------|
| SIL | 2.0 | Deterministic local software-in-the-loop release gate passed |
| VIL | 2.0 | Validation-in-the-loop evidence passed, such as remote CI or release publisher parity |
| HIL | 2.0 | Hardware-in-the-loop evidence passed on a real target; explicit waiver earns 1.0 |
| Artifacts | 1.5 | SBOM, security report, readiness, and manifest artifacts exist |
| Security | 1.5 | Full release security gate produced a passing JSON report |
| Evals | 1.0 | Release smoke/eval checks passed |

Official release readiness requires both:

- `release_readiness_score >= 8`
- `release_status == "pass"`

In `official` mode, the score alone is not enough. SIL, VIL, artifact,
security, and eval dimensions must pass. HIL must pass or be explicitly waived.

## Modes

| Mode | Use | Exit behavior |
|------|-----|---------------|
| `official` | Pre-tag release audit with `--release-version` | Fails if the gate is not pass |
| `advisory` | Normal local full gate without a target release version | Writes JSON without blocking on missing HIL |
| `fast` | `ci-local-release.sh --fast` | Writes degraded JSON for quick feedback |

`scripts/ci-local-release.sh --release-version X.Y.Z` selects `official` mode
unless `--readiness-mode` overrides it.

## HIL Evidence

`scripts/check-release-hil.sh` captures the companion artifact:

```text
.agents/releases/local-ci/<run-id>/hil-evidence.json
```

Targets are supplied with repeated `--hil-target` flags on the local release
gate, with `AGENTOPS_RELEASE_HIL_TARGETS`, or by calling the HIL script
directly:

```bash
scripts/check-release-hil.sh --target 'local:bushido:ao version'
scripts/check-release-hil.sh --target 'ssh:bushido:bushido:ao version'
```

When no physical target is available for an official release, the release owner
must pass `--hil-waiver "reason"` so the waiver is visible in both the HIL and
readiness artifacts. A waiver is acceptable release evidence, but it scores only
half of the HIL dimension.

## Release Artifacts

`release-artifacts.json` records these fields when the local gate runs:

```json
{
  "release_readiness": "release-readiness.json",
  "hil_evidence": "hil-evidence.json"
}
```

`scripts/resolve-release-artifacts.sh` only resolves full release artifact sets
that include SBOM, security, readiness, and HIL evidence. `scripts/validate-release-audit-artifacts.sh`
validates readiness evidence for release audits generated on or after
2026-05-02, while still accepting older historical audits.
