# CLI Tests

Validates broad `ao` CLI contracts: JSON output for the committed machine-output set and executable help smoke coverage for every generated leaf command. The leaf smoke test parses `cli/docs/COMMANDS.md` and runs `ao <leaf> --help` for each leaf command, catching registration gaps that reference-only parity checks can miss.

## Running

```bash
bash tests/cli/test-json-flag-consistency.sh
bash tests/cli/test-json-flag-consistency-tempdir.sh
bash tests/cli/test-all-leaf-help-smoke.sh
```
