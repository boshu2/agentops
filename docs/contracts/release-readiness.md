# Release Readiness Contract

Release readiness is the machine-readable contract for deciding whether a
release has enough evidence to tag and publish. The authoritative artifact is:

```text
.agents/releases/local-ci/<run-id>/release-readiness.json
```

The JSON shape is versioned by `release-readiness.v1.schema.json`.

## Zero-Trust Evidence Model

`release-readiness.json` has two related surfaces:

- `dimensions` preserves the 10 point score used by dashboards and release
  gates.
- `evidence` is the zero-trust contract. It declares the policy and the
  first-class lanes that produced the score.

The `evidence.policy` object records whether the artifact was produced in
official mode, whether caller-provided status flags were trusted, whether the
result is pre-publish blocking, and the threshold used for the decision. In
official mode, `status_flags_trusted` is always `false`; statuses are derived
from evidence artifacts, not from command-line `--sil pass` style inputs.

The `evidence.lanes` object has exactly these lanes:

| Lane | Required official artifact | Evidence identity |
|------|----------------------------|-------------------|
| `sil` | `sil-evidence.json` | `evidence_kind: software_in_loop` |
| `vil` | `digital-twin-evidence.json` | VIL status sourced from the digital twin |
| `digital_twin` | `digital-twin-evidence.json` | Disposable install/upgrade/operator workflow proof |
| `hil` | `hil-evidence.json` | Real target proof or explicit waiver |
| `artifacts` | `release-artifacts.json` bundle | Release artifact manifest |
| `security` | `security-gate-full.json` or accepted fallback | Security gate report |
| `evals` | `eval-agentops-fast.json` plus baseline audit | Eval and baseline-drift report |

Each lane records `status`, `artifact`, `required`, `blocking`,
`freshness_required`, and `release_version_required`. The digital-twin lane also
records workflow strength, the exercised `ao` binary digest, runtime identity,
check logs, and failure reasons. The HIL lane records waiver text when present,
target identity, command fingerprints, workflow checks, runtime identity, and
target logs.

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
Official mode derives those dimensions from evidence JSON files under
`--evidence-dir`; caller-provided status flags are only advisory/fast-mode
shortcuts. Evidence files must be fresh, parseable, and version-aligned when
`--release-version` is supplied. The digital-twin evidence must be full-strength
and must include an `ao` binary digest, runtime identity, and at least six
workflow checks. Passing HIL evidence must include at least one strong target
with runtime identity; otherwise HIL must be explicitly waived.

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
directly. Official targets must exercise more than `ao version`; evidence
records the command fingerprint, OS/architecture/runtime identity, workflow
checks, and optional release-version match.

```bash
scripts/check-release-hil.sh \
  --expected-version X.Y.Z \
  --target 'local:bushido:ao version && ao init --help && ao hooks show && ao rpi status'
scripts/check-release-hil.sh \
  --expected-version X.Y.Z \
  --target 'ssh:bushido:bushido:ao version && ao init --help && ao hooks show && ao rpi status'
```

When no physical target is available for an official release, the release owner
must pass `--hil-waiver "reason"` so the waiver is visible in both the HIL and
readiness artifacts. A waiver is acceptable release evidence, but it scores only
half of the HIL dimension.

## Digital-Twin / VIL Evidence

`scripts/check-release-digital-twin.sh` captures the disposable local workflow
artifact:

```text
.agents/releases/local-ci/<run-id>/digital-twin-evidence.json
```

Official mode requires a real `ao` binary and records install/upgrade smoke,
version parity, core command smoke, `ao init --hooks`, `ao hooks show`,
`ao rpi status`, and runtime/help surfaces in a temporary HOME and repository.
Fast mode runs a lightweight subset when the binary is available and records
`skipped` when it is not. The artifact includes `ao_bin_sha256`, runtime
identity, per-check output previews, and failure reasons so the release
readiness contract can prove which binary and workflow were exercised. A
digital-twin waiver is explicit evidence, not an implicit pass.

## Release Artifacts

`release-artifacts.json` records these fields when the local gate runs:

```json
{
  "sbom_cyclonedx": "sbom-vX.Y.Z.cyclonedx.json",
  "sbom_spdx": "sbom-vX.Y.Z.spdx.json",
  "security_report": "security-gate-full.json",
  "eval_fast_report": "eval-agentops-fast.json",
  "eval_baseline_audit": "eval-baseline-audit.json",
  "release_readiness": "release-readiness.json",
  "sil_evidence": "sil-evidence.json",
  "hil_evidence": "hil-evidence.json",
  "vil_evidence": "digital-twin-evidence.json",
  "digital_twin_evidence": "digital-twin-evidence.json"
}
```

`scripts/resolve-release-artifacts.sh` only resolves full release artifact sets
that include SBOMs, the security report, eval fast and baseline-audit outputs,
readiness, SIL evidence, HIL evidence, and digital-twin/VIL evidence.
`scripts/validate-release-audit-artifacts.sh` validates that proof bundle for
release audits generated on or after 2026-05-02, while still accepting older
historical audits.
