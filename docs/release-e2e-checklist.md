# Release E2E Checklist

Use this checklist before tagging a release to verify the local gate and release-smoke paths.

## 0) Pre-tag Triage

Use this before you tag. The release process in [RELEASING](RELEASING.md) is the source of truth; this checklist only narrows the local checks that prove the release is ready. If a gate fails after a fast pass, do not tag. Use [Incident Runbook](INCIDENT-RUNBOOK.md) when a failure needs cleanup or recovery after local validation.

| Question | Inspect |
|---|---|
| "Is the release process itself current?" | [RELEASING](RELEASING.md) and its pre-release checklist |
| "Did the local gate produce the expected evidence?" | `.agents/releases/local-ci/<timestamp>/` for SBOM, security report, readiness, HIL evidence, and related artifacts |
| "Did hooks and `ao rpi` smoke actually run?" | Fast gate markers: `Hook install smoke (minimal + full)` and `ao init --hooks + ao rpi smoke` |
| "Did the release smoke path fail after a fast pass?" | Re-run `bash scripts/ci-local-release.sh` and inspect the failing section before tagging |
| "Was the release already tagged or partially published?" | [RELEASING](RELEASING.md#failure-modes) and [Incident Runbook](INCIDENT-RUNBOOK.md) |

## 1) Fast local gate (quick confidence)

Run:

```bash
bash scripts/ci-local-release.sh --fast --jobs 4
```

Expect:

- Exit code `0`
- Output contains these markers:
  - `Codex runtime sections`
  - `Skill runtime parity`
  - `Hook install smoke (minimal + full)`
  - `ao init --hooks + ao rpi smoke`

Reference test: `tests/integration/test-release-e2e-validation.sh`.

## 2) Full local gate (pre-tag requirement)

Run:

```bash
bash scripts/ci-local-release.sh --release-version X.Y.Z --hil-target 'local:bushido:ao version'
```

Expect:

- Exit code `0`
- Final summary contains `LOCAL CI PASSED`
- `.agents/releases/local-ci/<timestamp>/release-readiness.json` has `release_status: pass` and `release_readiness_score >= 8`
- `.agents/releases/local-ci/<timestamp>/hil-evidence.json` has `status: pass` or `status: waived`

If the fast gate passes but this full gate fails, stop. The release is not ready to tag. Use the failing section in the full-gate output, rerun the official local gate until it passes, and if a tag or publish already happened, follow the failure-mode steps in [RELEASING](RELEASING.md#failure-modes) before retrying anything else.

## 3) Codex runtime lint (focused check)

Run:

```bash
bash scripts/validate-codex-runtime-sections.sh
```

Use this when editing Codex runtime guidance or AGENTS runtime sections.

## 4) Hook install + init/RPI smoke expectations

The local gate includes these release E2E smoke checks:

- `Hook install smoke (minimal + full)` validates:
  - `ao hooks install`
  - `ao hooks show`
  - `ao hooks install --full --source-dir <repo-root> --force`
  - Hook artifacts are written (`~/.claude/settings.json` and `~/.agentops/hooks/session-start.sh`)
- `ao init --hooks + ao rpi smoke` validates (in a fresh git repo):
  - `ao init --hooks`
  - `ao rpi status`
  - `ao rpi --help`
  - `ao rpi phased --help`

If the fast gate passes but one of these smoke paths fails, treat the release as blocked. Inspect the corresponding gate output, then rerun the full local gate so the fix is validated end to end before tagging.

## 5) Parity checks when workflow/docs contracts change

When changing CI workflow policy, hook/runtime docs, or required gate wording, run:

```bash
bash scripts/validate-ci-policy-parity.sh
bash scripts/validate-hooks-doc-parity.sh
bash scripts/validate-skill-runtime-parity.sh
bash scripts/validate-codex-runtime-sections.sh
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
bash scripts/validate-codex-backbone-prompts.sh
bash tests/docs/validate-doc-release.sh
```
