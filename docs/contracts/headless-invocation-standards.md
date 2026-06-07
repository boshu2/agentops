# Headless Invocation Standards

Standards for non-interactive worker execution in scripts, tests, and CI/CD.

## LAW 0: No Claude Print-Mode Workers

AgentOps scripts, tests, and CI must not invoke `claude -p` or
`claude --print` as a worker runtime. On Bo's machine that path bills the API
per token and has a known local process-spawn failure mode. Use one of these
lanes instead:

| Need | Runtime |
|------|---------|
| Headless local worker from a script | `codex exec` |
| Claude worker concurrency | NTM interactive panes or in-harness subagents |
| Deterministic local processing | Go or local shell without an LLM runtime |

`lib/scripts/team-runner.sh` is therefore Codex-only. A team spec with
`runtime: "claude"` must fail before spawning workers.

`lib/scripts/watch-claude-stream.sh` is retained only as a passive parser for
archived stream fixtures; it is not an approved executor path.

## Codex CLI Contract

Every headless Codex worker invocation should preserve these properties:

1. Run from the intended repo path with `-C`.
2. Use an explicit sandbox level: `-s read-only`, `-s workspace-write`,
   `-s danger-full-access`, or the existing `--full-auto` compatibility path.
3. Emit JSONL with `--json` so the watcher can detect completion and stalls.
4. Use `--output-schema` with `lib/schemas/worker-output.json`.
5. Write the final structured artifact with `-o`.
6. Wrap the process in shell `timeout`.

Reference shape:

```bash
sandbox_args=(-s read-only)
(
  AGENTOPS_INTENT_ECHO_DISABLED=1 timeout "$TIMEOUT_S" \
    codex exec "${sandbox_args[@]}" --json \
      -m "$CODEX_MODEL" \
      -C "$REPO_PATH" \
      --output-schema "$SCHEMA_PATH" \
      -o "$OUTPUT_FILE" \
      "$PROMPT"
  echo $? > "$EXIT_FILE"
) | CODEX_IDLE_TIMEOUT="$CODEX_IDLE_TIMEOUT" \
    bash lib/scripts/watch-codex-stream.sh "$STATUS_FILE"
```

## Sandbox Levels

`sandbox_level` in `lib/schemas/team-spec.json` is a Codex worker control:

| `sandbox_level` | Codex flag |
|-----------------|------------|
| `read-only` | `-s read-only` |
| `workspace-write` | `--full-auto` |
| `danger-full-access` | `-s danger-full-access` |

The `workspace-write` mapping is the historical team-runner behavior. Tighten it
only with a separate compatibility PR and fixture update.

## Timeout Strategy

Two layers prevent stalls:

1. **Shell `timeout`**: hard kill after N seconds.
2. **Watcher idle timeout**: fails when JSONL stops flowing.

Recommended defaults:

| Context | Shell timeout | Idle timeout |
|---------|---------------|--------------|
| Quick test | 45s | 15s |
| Skill test | 120s | 30s |
| Discovery phase | 600s | 60s |
| Implementation phase | 900s | 60s |
| Validation phase | 600s | 60s |

## Output Contract

Workers must write `lib/schemas/worker-output.json`:

```json
{
  "status": "done",
  "summary": "short result",
  "artifacts": ["path/to/artifact.md"],
  "errors": [],
  "token_usage": {"input": 0, "output": 0},
  "duration_ms": 0
}
```

The watcher writes a separate status file with process-level state:

```json
{
  "status": "completed",
  "token_usage": {"input": 0, "output": 0},
  "duration_ms": 0,
  "events_count": 0
}
```

## Team-Runner Contract

When a script uses `lib/scripts/team-runner.sh`, it must preserve:

1. `runtime` omitted or set to `codex`.
2. One output directory per agent under `.agents/teams/<team_id>/`.
3. `BEADS_NO_DAEMON=1` during the run to avoid tracker daemon conflicts.
4. Lead-owned validation and reporting after all workers finish.
5. Retry context capped and sanitized before being added to the next prompt.

## Session Chaining

For multi-phase workflows, use filesystem artifacts instead of session
resumption:

```bash
codex exec -C "$REPO_PATH" -s workspace-write \
  "Research X. Write findings to .agents/rpi/phase-1.md"

codex exec -C "$REPO_PATH" -s workspace-write \
  "Read .agents/rpi/phase-1.md for context. Implement the next slice."
```

Filesystem-based chaining is more reliable than session resumption because:

- Each phase gets a fresh context window.
- Artifacts survive auth expiration or process crashes.
- The lead can inspect and validate each phase boundary.

## Retry Logic

```bash
max_attempts=3
attempt=1
while [[ $attempt -le $max_attempts ]]; do
  if timeout 120 codex exec -C "$REPO_PATH" -s workspace-write "$PROMPT"; then
    break
  fi
  exit_code=$?
  if [[ $exit_code -eq 124 ]]; then
    echo "Timeout on attempt $attempt" >&2
  fi
  attempt=$((attempt + 1))
done
```

## Reference Implementations

| Script | Purpose |
|--------|---------|
| `lib/scripts/team-runner.sh` | Parallel Codex team orchestrator |
| `lib/scripts/watch-codex-stream.sh` | Codex JSONL watcher |
| `tests/team-runner/` | Static fixture tests for the runner contract |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `CODEX_MODEL` | `gpt-5.3-codex` | Codex model |
| `CODEX_IDLE_TIMEOUT` | `60` | Codex stream idle timeout |
| `TEAM_RUNNER_MAX_AGENTS` | `6` | Max concurrent agents |
| `TEAM_RUNNER_DRY_RUN` | unset | Print Codex commands without executing |
