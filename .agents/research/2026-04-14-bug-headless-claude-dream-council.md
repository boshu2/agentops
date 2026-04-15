# Bug Hunt: Dream Claude Headless Council Spawn

Date: 2026-04-14
Issue: `na-jox1`
Symptom: Dream council runs from Codex completed for `codex` but `claude` consistently timed out after 90s.

## Root Cause

The Dream Claude runner in `cli/cmd/ao/overnight_council.go` was using the
wrong headless Claude contract:

- it passed `--json-schema <path>` even though Claude expects inline schema JSON
- it expected raw structured JSON on stdout, but Claude `--output-format json`
  returns a result envelope with the actual payload under `structured_output`

That mismatch caused the direct Claude invocation to hang until the outer
timeout instead of returning a structured council artifact.

## Reproduction

Local Claude CLI (`2.1.108`) reproduced the mismatch directly:

- `claude -p --json-schema /tmp/schema.json ...` timed out
- `claude -p --output-format json --json-schema "$(jq -c . /tmp/schema.json)" ...`
  completed quickly and returned a `type=="result"` envelope with
  `structured_output`

## Fix

Updated the Dream Claude runner to:

1. read the schema file and pass inline schema JSON
2. request explicit JSON output with `--output-format json`
3. disable session persistence for faster deterministic headless runs
4. normalize Claude result envelopes by extracting `structured_output` before
   validating the council report

## Validation

- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao -run 'TestRunDreamCouncilRunner_|TestDreamRunClaudeCouncil_'`
- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/overnight -timeout 2m`
- Real Dream replay:
  `/tmp/ao-headless-claude-fix overnight start --goal 'validate claude headless council spawn' --runner claude --max-iterations 1 --run-timeout 4m --output-dir /tmp/ao-dream-claude-headless-fix --json`
  which finished with `council-claude: done` and wrote
  `/tmp/ao-dream-claude-headless-fix/council/claude.json`
