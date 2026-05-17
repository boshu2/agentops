# STEP 1.7.5 — Release-Readiness Gates

**Status:** MANDATORY when validation runs in a release context. Failure to execute or pass any gate below MUST emit `<promise>FAIL</promise>` with reason "release gates not run / failed". Validation MUST NOT recommend `/release` until all applicable gates pass.

## Release-context detection (`IS_RELEASE_CONTEXT`)

Set `IS_RELEASE_CONTEXT=1` when ANY of these conditions holds:

- The validation target branch matches `release/*`, `v*-prep`, `v*-evolve-run`, or `v[0-9]+.[0-9]+*` (any release-shaped branch name).
- The `--release-context` flag is set.
- The diff scope touches `cli/cmd/`, `cli/internal/`, `hooks/`, `schemas/`, or `skills/` AND the caller intends to recommend `/release` (validation is answering a "ready to tag?" question, not a routine cycle check).

When `IS_RELEASE_CONTEXT=0`, skip this step silently and record `Release-readiness gates: skipped (not release context)` in the phase summary.

## The gates

### a) CI-blocking local gate

```bash
bash scripts/ci-local-release.sh --ci-blocking
```

This is the canonical local pre-release truth check for the blocking GitHub
Validate job set. It intentionally excludes advisory jobs and release-only
artifact evidence. `--fast`, `scripts/pre-push-gate.sh`, and
`scripts/validate-local.sh` are weaker local hygiene gates and are
**INSUFFICIENT** for release readiness.

### b) Release artifact gate

```bash
bash scripts/ci-local-release.sh --release-version X.Y.Z
```

Run this when the target version is known so the release audit has SBOM,
security, HIL, and readiness artifacts for the final version. If
`scripts/ci-local-release.sh` does not exist in the repo, log `SKIP` and
continue. If it exists and either invocation fails, treat as `FAIL`.

### c) CLI reference docs regen (when CLI surface changed)

If the diff scope contains additions or removals to `cobra.Command{...}` definitions, `.Flags()` declarations, or any `cli/cmd/ao/*.go` files that look like command source, regenerate the reference and verify cleanliness:

```bash
# Detection (heuristic)
CLI_CHANGED=0
if git diff --name-only "${BASE:-HEAD~1}"...HEAD | grep -qE '^cli/cmd/.*\.go$'; then
    if git diff "${BASE:-HEAD~1}"...HEAD -- 'cli/cmd/**.go' \
       | grep -qE '^\+.*(cobra\.Command\{|\.Flags\(\)|Use:|Short:)'; then
        CLI_CHANGED=1
    fi
fi

if [[ "$CLI_CHANGED" == "1" ]]; then
    bash scripts/generate-cli-reference.sh
    if ! git diff --quiet cli/docs/COMMANDS.md; then
        echo "FAIL: cli/docs/COMMANDS.md is stale — CLI surface changed but reference was not regenerated before commit."
        echo "      Run scripts/generate-cli-reference.sh, commit the diff, retry."
        exit 1
    fi
fi
```

If git diff reports the file changed AFTER regen, the CLI reference was stale → FAIL.

## Phase-summary recording

All gates must report success (or `N/A` for c when no CLI surface change). Record verdicts as a checkbox row in the phase summary:

```
[✅] ci-local-release.sh --ci-blocking
[✅] ci-local-release.sh --release-version X.Y.Z
[✅] generate-cli-reference.sh (or [N/A] if no CLI surface change)
```

## Skip suppression

`--skip-release-gates` — operator-acknowledged risk acceptance (for non-release validation runs that incidentally hit the release-shaped branch heuristic). When used, record explicit reason in the phase summary.

## Why this step exists

The v2.41-evolve-run validation (May 2026) returned PASS on the fast pre-push gate alone, never ran the full gate or `ci-local-release.sh`, and missed that the branch had removed a CLI flag (`--oscillation-sweep`) without regenerating `cli/docs/COMMANDS.md`. The operator caught the gap; this step prevents the same recommendation pattern from recurring. Per-cycle `--fast` is a smoke test; release readiness is a different question with concrete gates.
