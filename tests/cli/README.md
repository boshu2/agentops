# CLI Tests

Validates that the `ao` CLI's `--json` flag produces valid JSON output across the committed contract set. Ensures flag consistency between documented behavior and actual output, treats non-JSON stdout as a failure, and verifies `--json` / `-o json` equivalence on a stable command. Requires the `ao` binary (auto-builds if missing).

## Running

```bash
bash tests/cli/test-json-flag-consistency.sh
bash tests/cli/test-json-flag-consistency-tempdir.sh
```
