# Agent-Ergonomics Rubric — the 8 axes (12.5 pts each, /100)

Score each axis **0** (absent/harmful), **6** (partial), or **12.5** (full). Cite the evidence —
the actual command, the file:line, the observed exit code. Never score from memory of the docs.

## 1. Discoverability (`--help` as API)
- `--help` and `<cmd> help` both work and list every subcommand.
- Each subcommand has its own `--help` listing every flag with its accepted values.
- `--help` exits `0`; an unknown command exits with a usage code and a "did you mean" or the help text.
- **Full:** an agent that has only ever seen `--help` can construct any valid invocation.

## 2. Machine-readable output (the robot surface)
- A `--json` (single document) and/or `--ndjson` (one object per line) mode exists for every command
  that returns data.
- Machine output goes to **stdout**; logs, progress, and warnings go to **stderr**.
- Keys are stable and unlocalized; the schema does not change between a "0 results" and "N results" run.
- **Full:** output pipes through `jq .` (JSON) or per-line `jq` (NDJSON) with zero preprocessing.

## 3. Exit-code discipline
- Distinct, documented codes for success / usage error / not-found / precondition / partial.
- The same failure class always returns the same code.
- The code table appears in `--help`.
- **Full:** an agent can branch entirely on `$?` for the common decisions, before parsing output.

## 4. Predictable flags & naming
- Consistent conventions: `--long-flag`, short aliases only where conventional, `--no-x` negations.
- Same concept = same flag name across subcommands (`--output`, not `--out` here and `--format` there).
- Booleans are flags, not `--flag true`; repeatable options documented as repeatable.
- **Full:** flag names are guessable from one example; no subcommand contradicts another.

## 5. Non-interactive operability
- No command blocks on a TTY prompt when stdin is not a TTY.
- Confirmation comes from `--yes`/`--force`; destructive commands offer `--dry-run`.
- Auth/config can be supplied by flag or env var, never only by an interactive wizard.
- **Full:** the whole tool runs unattended in CI / an agent loop with no hangs.

## 6. Error message quality (for a parser, not just a reader)
- Errors are specific: what failed, which input, what to do next.
- The machine modes emit errors as structured data too (`{"error":{"code":"...","message":"..."}}`),
  not only as prose on stderr.
- No stack traces as the primary error surface in normal failures.
- **Full:** an agent can recover from a typical error using only the structured error fields.

## 7. Idempotency & safety affordances
- Re-running a "create" either succeeds idempotently or fails with a clear, distinct code.
- `--dry-run` previews without side effects; output names exactly what *would* change.
- Destructive operations are opt-in and reversible-or-warned.
- **Full:** an agent can safely retry on ambiguous failure without compounding damage.

## 8. Composability & streaming
- Output is line-oriented or NDJSON so it flows into `grep`/`jq`/`xargs`/CI without brittle parsing.
- Long-running commands stream incremental results (NDJSON, flushed) rather than buffering to the end.
- Reads from stdin where a pipe is natural (`-` convention or auto-detect).
- **Full:** the tool drops cleanly into a Unix/agent pipeline as both producer and consumer.

## Scoring template

```
| Axis                       | Score | Evidence (cmd / file:line / exit) | Fix |
|----------------------------|-------|-----------------------------------|-----|
| 1 Discoverability          |       |                                   |     |
| 2 Machine-readable output  |       |                                   |     |
| 3 Exit-code discipline     |       |                                   |     |
| 4 Predictable flags        |       |                                   |     |
| 5 Non-interactive          |       |                                   |     |
| 6 Error message quality    |       |                                   |     |
| 7 Idempotency & safety     |       |                                   |     |
| 8 Composability & streaming|       |                                   |     |
| TOTAL                      |  /100 |                                   |     |
```

Below 50: the tool is effectively unusable by an unattended agent — fix axes 2, 3, 5 first.
