# Codex Tests

This directory contains static Codex artifact tests and three live CLI primitive
probes: `codex review --uncommitted`, read-only sandbox with final-message capture,
and `codex exec --output-schema` using a small test-owned JSON schema. These do
not prove AgentOps skill discovery, workflow execution or independent judgment.

```bash
bash tests/codex/integration/run-all.sh
```

A missing `codex` exits 77 (unavailable). A present CLI's nonzero exit, timeout,
missing output or invalid structured output fails; text containing `SKIPPED`
never changes that result. Each invocation runs once and retains `command.log`,
`command.status`, final output when available and its disposable fixture repo in
a printed unique diagnostics directory. The operator can remove that directory
after inspecting the evidence. No user configuration is changed.

Environment:

- `CODEX_MODEL`: optional invocation-local `model` and `review_model` override; otherwise use Codex's configured selections.
- `CODEX_TEST_TIMEOUT_SECONDS`: positive integer command budget, default `120`.
- `CODEX_TEST_LOG_DIR`: diagnostics parent directory, default system temporary directory.

For the aggregate runner, set `RUN_ALL_CODEX_MODEL` to limit a model override to
its live Codex lane without changing other suites' model fixtures.

Deterministic harness regression tests use a fake CLI, not live capability proof:

```bash
bats tests/scripts/codex-integration-harness.bats
```
