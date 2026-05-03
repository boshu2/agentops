---
id: plan-2026-05-02-ai-native-zero-trust-release-process
type: plan
date: 2026-05-02
epic: soc-owed
source: ".agents/research/2026-05-02-ai-native-zero-trust-release-process.md"
---

# Plan: AI-Native Zero-Trust Release Process

## Context

The user thesis is that normal CI/CD expects code to be in a different shape:
it trusts a pipeline's reported success too much. Agent-produced releases need a
zero-trust release process: simulate realistic environments and require SIL,
VIL, HIL, and digital-twin evidence before build/tag/release.

Existing closed epic `soc-h22t` added an 8/10 SIL/VIL/HIL readiness score. This
plan extends that base with digital-twin evidence, richer attestations, evidence
consumption instead of trusted status flags, and pre-publish workflow blocking.

Applied findings: `f-2026-05-01-021`, `f-2026-05-01-024`,
`f-2026-04-30-002`.

## Files To Modify

| File | Change |
| --- | --- |
| `docs/contracts/release-readiness.md` | Upgrade the contract with digital-twin and evidence-first semantics. |
| `schemas/release-readiness.v1.schema.json` or replacement v2 schema | Add digital-twin/evidence object references while preserving migration compatibility. |
| `scripts/check-release-readiness.sh` | Consume evidence files in official mode instead of trusting supplied statuses. |
| `scripts/check-release-digital-twin.sh` | New runner for disposable environment install/upgrade/operator workflow simulation. |
| `tests/scripts/release-digital-twin.bats` | New fixtures for digital-twin pass/fail/skipped behavior. |
| `scripts/check-release-hil.sh` | Strengthen target evidence beyond arbitrary `ao version`. |
| `.github/workflows/release.yml` | Block GoReleaser publish until readiness/security/evidence gates pass. |
| `scripts/resolve-release-artifacts.sh` | Require complete official evidence bundle. |
| `scripts/validate-release-audit-artifacts.sh` | Validate digital-twin/eval/security/readiness evidence before audit pass. |
| `tests/scripts/release-readiness.bats` | Cover evidence-file official mode. |
| `tests/scripts/release-hil.bats` | Cover stronger HIL/VIL workflow evidence and weak-evidence rejection. |
| `tests/scripts/release-artifacts.bats` | Cover complete proof bundle resolution/audit. |
| `tests/scripts/ci-local-release.bats` | Cover release workflow/readiness plumbing markers. |
| `docs/RELEASING.md`, `docs/CI-CD.md`, `docs/release-e2e-checklist.md` | Explain zero-trust release proof bundle and operational flow. |

## Boundaries

Always:

- Keep `soc-h22t` readiness score as the foundation; do not duplicate it.
- Require official release evidence to be file-backed and schema-validated.
- Treat HIL waivers as explicit evidence with reduced score, never silent pass.
- Keep public PR CI independent from private physical hosts.

Never:

- Tag or publish as part of this work.
- Make a passing GitHub workflow the only proof of release readiness.
- Treat `ao version` alone as sufficient HIL evidence for an official release.
- Generate post-publish readiness assets and call that a pre-publish gate.

Ask first:

- Which real hosts/benches are official HIL targets for `v2.40.0`.
- Whether the first digital twin should be local-only or include remote VM/container runners.

## Baseline Audit

| Check | Evidence |
| --- | --- |
| Release publisher gate | `.github/workflows/release.yml` publish depends on doc/security jobs but only requires doc success before GoReleaser. |
| Publish ordering | GoReleaser runs before SBOM/security/readiness assets are generated. |
| Current readiness contract | `docs/contracts/release-readiness.md` defines SIL/VIL/HIL/artifacts/security/evals; no digital twin. |
| Current readiness script | `scripts/check-release-readiness.sh` accepts status flags and only special-cases HIL file input. |
| Current HIL script | `scripts/check-release-hil.sh` runs arbitrary local/SSH target commands and records pass/fail. |
| Current artifact validation | `scripts/validate-release-audit-artifacts.sh` is the right place to fail missing proof bundle artifacts. |
| Existing issue history | `soc-h22t` and children are closed; this needs a new epic. |

## Issues

### `soc-owed.2` - Define zero-trust release evidence contract with digital twin dimension

Ownership: `docs/contracts/release-readiness.md`, readiness schema, docs index,
`docs/CI-CD.md`, `docs/RELEASING.md`.

Acceptance:

- Contract defines digital-twin evidence and evidence-rich SIL/VIL/HIL.
- Official release cannot pass on status strings alone.
- Contract compatibility and docs gates pass.

Validation:

- `bash scripts/check-contract-compatibility.sh`
- `rg -n 'digital twin|SIL|VIL|HIL|evidence' docs/contracts/release-readiness.md`

Test levels: L0, L1.

### `soc-owed.3` - Add digital twin release evidence runner

Ownership: `scripts/check-release-digital-twin.sh`,
`tests/scripts/release-digital-twin.bats`, `docs/release-e2e-checklist.md`.

Acceptance:

- Runner simulates realistic post-install operator workflows in disposable environments.
- Evidence records workflow results, release version, binary identity, logs, and target identity.
- Fixtures cover pass, fail, skipped/waived behavior.

Validation:

- `bash -n scripts/check-release-digital-twin.sh`
- `bats tests/scripts/release-digital-twin.bats`

Test levels: L1, L2, L3 when remote/real installed target is used.

### `soc-owed.4` - Make release readiness consume evidence artifacts instead of caller trust

Ownership: `scripts/check-release-readiness.sh`, readiness schema,
`tests/scripts/release-readiness.bats`, `scripts/ci-local-release.sh`.

Acceptance:

- Official mode derives readiness from evidence files.
- Missing, stale, or wrong-version evidence fails official readiness.
- Advisory/fast modes retain cheap feedback without pretending to be release proof.

Validation:

- `bash -n scripts/check-release-readiness.sh scripts/ci-local-release.sh`
- `bats tests/scripts/release-readiness.bats`

Test levels: L1, L2.

### `soc-owed.5` - Harden GitHub release publisher to block before GoReleaser publish

Ownership: `.github/workflows/release.yml`, workflow parity tests/docs,
`docs/RELEASING.md`.

Acceptance:

- GoReleaser publish is gated by pre-publish readiness/security evidence.
- Security failure cannot be ignored for official release publish.
- Tests or parity checks prove publish cannot run on doc-only success.

Validation:

- `bash scripts/validate-ci-policy-parity.sh`
- `rg -n 'release-readiness|security-gate|goreleaser' .github/workflows/release.yml`

Test levels: L1, L2.

### `soc-owed.6` - Strengthen VIL and HIL evidence beyond ao version smoke

Ownership: `scripts/check-release-hil.sh`, VIL/digital-twin runner integration,
`tests/scripts/release-hil.bats`, `docs/RELEASING.md`.

Acceptance:

- HIL/VIL evidence records meaningful install/upgrade/operator workflow checks.
- Evidence includes target identity, OS/arch/runtime identity, release version, and logs.
- Weak or mismatched target evidence is rejected in official mode.

Validation:

- `bash -n scripts/check-release-hil.sh`
- `bats tests/scripts/release-hil.bats`

Test levels: L1, L2, L3.

### `soc-owed.7` - Link eval, security, SBOM, and digital twin evidence into release audit artifacts

Ownership: `scripts/resolve-release-artifacts.sh`,
`scripts/validate-release-audit-artifacts.sh`, `tests/scripts/release-artifacts.bats`,
release docs/checklist.

Acceptance:

- `release-artifacts.json` links readiness, HIL, VIL/digital-twin, eval, security, and SBOM evidence.
- Release artifact resolution requires complete official proof bundles.
- Audit validation fails on missing eval/security/twin evidence.

Validation:

- `bash -n scripts/resolve-release-artifacts.sh scripts/validate-release-audit-artifacts.sh`
- `bats tests/scripts/release-artifacts.bats`

Test levels: L1, L2.

## Execution Order

Wave 1:

- `soc-owed.2` contract and schema direction.

Wave 2:

- `soc-owed.3` digital-twin runner.
- `soc-owed.6` HIL/VIL strengthening can begin after the evidence model is stable.

Wave 3:

- `soc-owed.4` readiness consumes evidence artifacts.

Wave 4:

- `soc-owed.5` pre-publish GitHub release gate.
- `soc-owed.7` proof bundle artifact/audit linkage.

## File Dependency Matrix

| File | Issues | Serialization |
| --- | --- | --- |
| `docs/contracts/release-readiness.md` | `.2`, `.4`, `.7` | Contract first, then consumer docs. |
| `schemas/release-readiness*.json` | `.2`, `.4` | Schema before readiness consumer. |
| `scripts/check-release-readiness.sh` | `.4`, `.7` | Consumer before artifact/audit finalization. |
| `scripts/check-release-digital-twin.sh` | `.3` | New file; no parallel writers. |
| `scripts/check-release-hil.sh` | `.6` | Coordinate with `.4` evidence schema. |
| `.github/workflows/release.yml` | `.5` | After readiness evidence shape stabilizes. |
| `scripts/validate-release-audit-artifacts.sh` | `.7` | After manifest fields are defined. |
| `docs/RELEASING.md` | `.2`, `.5`, `.6`, `.7` | Contract, workflow, target operation, final proof bundle. |

## File-Conflict Matrix

| Shared Surface | Risk | Handling |
| --- | --- | --- |
| Release readiness schema/script | High | Serialize `.2` then `.4`; do not patch both in parallel. |
| Release docs | Medium | Keep contract edits early and checklist edits late. |
| Release workflow | High | Patch after tests define pre-publish invariant. |
| Artifact validators | Medium | Add tests before validator hardening. |

## Planning Rules Compliance

- Mechanical verification: each issue has shell/BATS or parity checks.
- Self-assessment: official readiness must be evidence-file-backed.
- Propagation: scripts, schemas, docs, workflow, release audit, and tests are all owned.
- Rollback/rescue: private host absence remains an explicit waiver path.
- Four-surface closure: code/scripts, docs, tests, and release artifacts are included.

## Verification Commands

```bash
bash -n scripts/check-release-readiness.sh scripts/check-release-hil.sh scripts/ci-local-release.sh scripts/resolve-release-artifacts.sh scripts/validate-release-audit-artifacts.sh
bats tests/scripts/release-readiness.bats tests/scripts/release-hil.bats tests/scripts/release-artifacts.bats tests/scripts/ci-local-release.bats
bash scripts/check-contract-compatibility.sh
bash scripts/validate-ci-policy-parity.sh
bash scripts/generate-cli-reference.sh --check
scripts/eval-agentops.sh --fast --run-root /tmp/agentops-eval-release-zero-trust
scripts/ci-local-release.sh --release-version 2.40.0 --hil-waiver "temporary waiver until official HIL target inventory is approved"
```

## Next Steps

Run `$agentops:crank soc-owed` after resolving whether the first digital twin is
local-only or includes remote VM/container targets.
