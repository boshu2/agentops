# Headless Invocation Standards

Standards for running Claude Code and Codex CLI non-interactively in scripts, tests, and CI/CD.

## Required Flags

Every headless Claude invocation MUST include:

| Flag | Purpose | Required? |
|------|---------|-----------|
| `-p` | Enable non-interactive (print) mode | Always |
| `--dangerously-skip-permissions` | Allow all tools without prompting | When skills chain skills |
| `--allowedTools "..."` | Scope tool access (least privilege) | When you control the exact prompt |
| `--max-turns N` | Prevent runaway turns | Always |
| `--no-session-persistence` | Don't save session to disk | Always for tests/CI |
| `--max-budget-usd N` | Cost guardrail | Always |

### When to use `--allowedTools` vs `--dangerously-skip-permissions`

**Default to `--dangerously-skip-permissions`** for any invocation that involves skills. Skills chain into sub-skills that use unpredictable tools (WebFetch, WebSearch, Agent sub-agents, council judges, etc.). `--allowedTools` is session-level — it constrains the entire session including tools used inside skill execution. Scoping too tightly causes silent tool failures deep in the skill chain.

Use `--allowedTools` only when **all of these are true**:
1. Your prompt does NOT invoke any skills (no `/research`, `/vibe`, etc.)
2. You know the exact set of tools needed
3. You want defense-in-depth beyond timeouts and budget limits

| Scenario | Permission strategy |
|----------|---------------------|
| RPI phases (skills chain skills) | `--dangerously-skip-permissions` |
| Smoke tests (invoke skills) | `--dangerously-skip-permissions` |
| Test helpers (generic skill testing) | `--dangerously-skip-permissions` |
| Simple query ("list available skills") | `--allowedTools "Skill,Read,Glob,Grep"` |
| Single-purpose script ("read X, write Y") | `--allowedTools "Read,Write"` |

## Tool Allowlists

Only applicable when using `--allowedTools` (no skill invocation):

| Context | Allowlist |
|---------|-----------|
| Read-only analysis | `Read,Grep,Glob` |
| Listing / querying | `Skill,Read,Glob,Grep` |
| Research (no skills) | `Read,Grep,Glob,Bash,Write,Agent` |
| Implementation (no skills) | `Read,Write,Edit,Grep,Glob,Bash,Agent` |

## Timeout Strategy

Three layers prevent stalls:

1. **Shell `timeout`** — Hard kill after N seconds (exit code 124)
2. **`--max-turns`** — Limit agentic conversation turns
3. **`--max-budget-usd`** — Cost ceiling per invocation

Recommended defaults:

| Context | Shell timeout | Max turns | Max budget |
|---------|---------------|-----------|------------|
| Quick test | 45s | 3 | $0.50 |
| Skill test | 120s | 5 | $1.00 |
| Discovery phase | 600s | 15 | $5.00 |
| Implementation phase | 900s | 30 | $5.00 |
| Validation phase | 600s | 15 | $5.00 |

## Output Format

| Use case | Flag | Notes |
|----------|------|-------|
| Human-readable | `--output-format text` (default) | Simple scripts |
| Structured processing | `--output-format json` | Parse with `jq -r '.result'` |
| Streaming / debugging | `--output-format stream-json --verbose` | JSONL events; final success event carries `structured_output` when `--json-schema` is used |
| Schema-validated | `--output-format json --json-schema '...'` | Typed output |

## Claude Team-Runner Contract

When Codex is orchestrating a headless Claude team through
`lib/scripts/team-runner.sh`, the Claude worker command must preserve four
properties:

1. Run from `repo_path` so the worker sees the intended worktree.
2. Use `--dangerously-skip-permissions` because worker prompts may invoke
   skills.
3. Use `--output-format stream-json --verbose` so the watcher can detect stalls
   and completion.
4. Use `--json-schema` with `lib/schemas/worker-output.json` so the final
   `structured_output` object matches the shared worker contract.

Reference shape:

```bash
(
  cd "$REPO_PATH" && timeout "$TIMEOUT_S" claude -p "$PROMPT" \
    --model "$CLAUDE_MODEL" \
    --plugin-dir "$REPO_PATH" \
    --dangerously-skip-permissions \
    --max-turns "$CLAUDE_MAX_TURNS" \
    --no-session-persistence \
    --max-budget-usd "$CLAUDE_MAX_BUDGET_USD" \
    --output-format stream-json \
    --verbose \
    --json-schema "$(jq -c . lib/schemas/worker-output.json)"
) | CLAUDE_IDLE_TIMEOUT="$CLAUDE_IDLE_TIMEOUT" \
    bash lib/scripts/watch-claude-stream.sh "$STATUS_FILE" "$OUTPUT_FILE"
```

`watch-claude-stream.sh` must treat the final `type=="result"` success event as
the completion signal and write `.structured_output` to `output.json`.

`sandbox_level` in `lib/schemas/team-spec.json` is a Codex worker control. Claude
team-runner workers ignore it and use the full-permission contract above.

## Session Chaining

For multi-phase workflows, use filesystem artifacts instead of session resumption:

```bash
# Phase 1 writes artifacts
claude -p "Research X. Write findings to .agents/rpi/phase-1.md" \
  --no-session-persistence ...

# Phase 2 reads those artifacts
claude -p "Read .agents/rpi/phase-1.md for context. Implement..." \
  --no-session-persistence ...
```

Filesystem-based chaining is more reliable than `--resume` because:
- Each phase gets a fresh context window
- No risk of context overflow from accumulated turns
- Artifacts survive auth expiration or process crashes

## Retry Logic

```bash
max_attempts=3
attempt=1
while [[ $attempt -le $max_attempts ]]; do
    if timeout 120 claude -p "..." --allowedTools "..." ...; then
        break
    fi
    exit_code=$?
    if [[ $exit_code -eq 124 ]]; then
        echo "Timeout on attempt $attempt" >&2
    fi
    attempt=$((attempt + 1))
done
```

## Codex CLI

Codex uses different flags but the same principles apply:

| Claude flag | Codex equivalent |
|-------------|-----------------|
| `-p` | `exec` subcommand |
| `--allowedTools` | `-s read-only` or `-s danger-full-access` or `--full-auto` |
| `--max-turns` | N/A (single turn) |
| `--output-format json` | `--json` |
| `--no-session-persistence` | Default (no sessions) |
| `--max-budget-usd` | N/A |

## Reference Implementations

| Script | Purpose |
|--------|---------|
| `tests/claude-code/test-helpers.sh` | Reusable test helpers with configurable tools |
| `tests/release-smoke-test.sh` | Release gate with scoped tools |
| `ao rpi` | Multi-phase RPI orchestrator (CLI command, not a script) |
| `lib/scripts/team-runner.sh` | Parallel Codex/Claude team orchestrator |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `ALLOWED_TOOLS` | empty | Comma-separated tool list for test helpers (empty = `--dangerously-skip-permissions`) |
| `MAX_TURNS` | 3 | Max agentic turns for test helpers |
| `MAX_BUDGET_USD` | 1.00 | Per-invocation cost guardrail |
| `DEFAULT_TIMEOUT` | 120 | Shell timeout in seconds |
| `CLAUDE_MODEL` | (default) | Model override |
| `CLAUDE_IDLE_TIMEOUT` | 60 | Claude stream idle timeout for `watch-claude-stream.sh` |
| `CLAUDE_MAX_TURNS` | 6 | Max turns per Claude team-runner worker |
| `CLAUDE_MAX_BUDGET_USD` | 5 | Max budget per Claude team-runner worker |
| `CODEX_MODEL` | gpt-5.3-codex | Codex model |
| `RPI_DRY_RUN` | unset | Print commands without executing |
| `RPI_VERBOSE` | unset | Enable verbose stream-json output |
