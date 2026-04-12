# Codex Tests

This directory contains static Codex artifact tests and live Codex CLI
integration tests.

Run the live integration suite:

```bash
bash tests/codex/integration/run-all.sh
```

When `codex` is absent from `PATH`, the integration suite exits successfully
with all child tests marked skipped. When `codex` is present, the tests invoke
the live CLI and may take several minutes.

Environment:

- `CODEX_MODEL`: model passed to `codex exec`; defaults to `gpt-5.3-codex`.
