# agentops-f6g Result

## Changed Paths

- `evals/agentops-core/security-toolchain-governance.json`
- `evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh`
- `.agents/evals/baselines/agentops-core.security-toolchain-governance.baseline.json`
- `docs/CI-CD.md`
- `.agents/crank/results/agentops-f6g.md`

## Validation

- `bash tests/scripts/test-toolchain-validate.sh` - PASS, `Results: 8 PASS, 0 FAIL`
- `bash tests/scripts/test-security-gate.sh` - PASS, `Results: 6 PASS, 0 FAIL`
- `scripts/eval-agentops.sh --suite evals/agentops-core/security-toolchain-governance.json --promote-baseline --promoted-by codex --rationale "Initial direct security toolchain governance baseline."` - PASS, promoted baseline with `failures=0`; initial missing-baseline warning was expected
- `scripts/eval-agentops.sh --suite evals/agentops-core/security-toolchain-governance.json` - PASS, latest rerun `failures=0 warnings=0`, baseline verdict `pass`, aggregate delta `0`
- `bash -n evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh && shellcheck --severity=error evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh` - PASS

## Discoveries

- `docs/CI-CD.md` still had the stale `scripts/security-toolchain-validate.sh` heading; it now names `scripts/toolchain-validate.sh`.
- The promoted baseline was created while other workers had unrelated dirty files in the shared branch worktree; those files were not edited for this task.
