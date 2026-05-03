---
id: research-2026-05-02-ai-native-zero-trust-release-process
type: research
date: 2026-05-02
backend: codex-sub-agent + inline
goal: "AI-agent-native zero-trust release process with SIL/VIL/HIL and digital-twin evidence."
---

# Research: AI-Native Zero-Trust Release Process

## Summary

AgentOps already has a first-stage release readiness score from `soc-h22t`:
`release-readiness.json` scores SIL, VIL, HIL, artifacts, security, and evals.
That work is necessary but not sufficient for the user's thesis. The current
release process still has ordinary CI/CD assumptions:

- the GitHub release workflow publishes with GoReleaser before it generates
  readiness/security/SBOM assets;
- release readiness can be recorded from caller-supplied status strings rather
  than concrete evidence artifacts;
- HIL can be as weak as `ao version`;
- there is no release-specific digital-twin artifact, runner, schema, or gate.

The next release-process layer should become evidence-first: produce and verify
SIL, VIL, HIL, digital-twin, eval, security, SBOM, and build provenance before
build/tag/publish, or at minimum before GoReleaser publishes any artifact.

## Product Context Applied

`PRODUCT.md` frames AgentOps as operational discipline for indeterministic
workers. The Quality-First Maintainer persona wants fewer, higher-confidence
releases, and the core value prop says validation gates block rather than
advise. A zero-trust release gate is therefore product-aligned.

## Prior Knowledge Applied

- `.agents/plans/2026-05-02-release-readiness-eight-sil-vil-hil.md` and
  `.agents/research/2026-05-02-release-readiness-eight-sil-vil-hil.md` define
  the first-stage SIL/VIL/HIL readiness contract.
- `.agents/findings/f-2026-05-01-021.md` applies: cross-compile + scp + ssh to
  target is the known cross-host validation pattern.
- `.agents/findings/f-2026-05-01-024.md` applies: live daemon/service proof on
  production hosts should count as L3 system proof.
- `.agents/findings/f-2026-04-30-002.md` is relevant as a warning: gates should
  detect missing upstream inputs rather than repeatedly failing on empty state.

## Key Files

| File | Evidence |
| --- | --- |
| `.github/workflows/release.yml` | Publish job needs only doc gate success before GoReleaser; security is continue-on-error, and readiness is generated after publish. |
| `scripts/ci-local-release.sh` | Local release gate writes artifacts and currently has HIL/readiness phases after build/smoke/security. It also now stamps build version from `release_version()`. |
| `scripts/check-release-readiness.sh` | Scores statuses, but official mode still trusts supplied `--sil`, `--vil`, `--security`, and `--eval` values. |
| `scripts/check-release-hil.sh` | Captures local/SSH target evidence, but target commands are arbitrary and can be weak. |
| `docs/contracts/release-readiness.md` | Defines current 10-point score with SIL/VIL/HIL, but no digital-twin dimension. |
| `schemas/release-readiness.v1.schema.json` | Current schema has no evidence object references beyond HIL artifact/waiver. |
| `scripts/eval-agentops.sh` | Runs canaries and baseline audit, but release readiness does not link to eval run artifacts. |
| `scripts/resolve-release-artifacts.sh` and `scripts/validate-release-audit-artifacts.sh` | Right extension points for complete proof bundle validation. |

## Current Flow

1. Local operator runs `scripts/ci-local-release.sh --release-version X.Y.Z`
   to generate local artifacts and readiness evidence.
2. Operator tags and pushes.
3. GitHub `release.yml` verifies the tag and token, extracts notes, deletes any
   existing release, then publishes via GoReleaser.
4. After publish, the workflow generates SBOM, security summary, and advisory
   readiness assets.

This is not fully zero-trust because the release publisher does not consume the
authoritative evidence bundle before publish.

## Gap Analysis

1. **No digital twin lane.** Simulation exists in pre-mortem language, not as a
   release-environment execution artifact.
2. **Status strings are too trusting.** Official readiness should derive status
   from evidence files with schemas, timestamps, release version, target identity,
   binary digest, artifact digest, command logs, and pass criteria.
3. **Publisher can publish before evidence.** GoReleaser runs before readiness
   assets are generated in `.github/workflows/release.yml`.
4. **HIL/VIL evidence is shallow.** `ao version` proves command execution, not
   install/upgrade/operator workflow fidelity.
5. **Eval/security evidence is under-linked.** Eval fast/baseline audit and
   security reports should be referenced in `release-artifacts.json` and checked
   by audit validators.

## Test Levels

Required: L0, L1, L2.

Recommended: L3 when real HIL targets, remote VIL runners, or live daemon/runtime
hosts are available.

Rationale: this touches shell scripts, schemas, docs, GitHub Actions, release
artifact resolution, and external target execution. Digital twin evidence is a
system simulation lane and should be treated as L2 minimum, L3 when it runs
against real installed binaries or remote hosts.

## Quality Validation

Coverage checked: release docs, local release gate, GitHub release workflow,
current readiness/HIL scripts, schemas, eval wrapper, release artifact validators,
prior SIL/VIL/HIL research, and existing bd epic history.

Depth ratings:

| Area | Depth | Notes |
| --- | ---: | --- |
| Release publisher workflow | 3/4 | Publish-before-evidence path is clear. |
| Current readiness scoring | 3/4 | Status-score model is mapped. |
| Digital twin gap | 2/4 | Absence is clear; implementation shape needs decisions. |
| HIL/VIL strengthening | 2/4 | Target inventory remains operator-specific. |
| Eval/security linkage | 2/4 | Extension points are clear; exact artifact schema needs plan execution. |

Critical assumption: the first useful digital twin can be a deterministic
disposable local environment that exercises install/upgrade/operator workflows.
Remote or hardware-backed twins can be layered after the contract exists.
