# Codex parity — gdb-for-debugging

This skill is portable across Claude / Gemini / Antigravity via the single `SKILL.md`
(install-path, not conversion). Codex uses the dual-file form: a slim `SKILL.md` plus a
`prompt.md`. The behavior contract below must match `SKILL.md` exactly across vendors.

## Invariant behavior (all vendors)

- The agent NEVER launches an interactive `gdb` session — there is no TTY to answer the
  `(gdb)` prompt. It always drives gdb head-less via `-batch` with `-ex` / `-x`.
- Every captured backtrace must come from a binary built with `-g -O0` (named frames +
  file:line), otherwise the agent rebuilds before continuing.
- `set pagination off` (or `-batch`, which implies it) is always set so gdb never blocks
  on `---Type <return>---`.
- A reproduce that yields a non-zero exit *with* a usable backtrace is success, not failure.

## The exact non-interactive command shapes (vendor-agnostic)

```bash
# Reproduce a crash + backtrace
gdb -batch -ex run -ex 'bt full' --args ./prog <args>

# Post-mortem a core dump
gdb -batch -ex 'bt full' -ex 'info registers' ./prog ./core

# Command file (long scripts)
gdb -batch -x cmds.gdb --args ./prog <args>
```

## Codex dual-file mapping

| Claude/Gemini/AGY (`SKILL.md`) | Codex |
|---|---|
| Frontmatter + full body in `SKILL.md` | slim `SKILL.md` (frontmatter + router) + `prompt.md` (procedure) |
| `## Critical Constraints` | mirrored verbatim into `prompt.md` |
| `## Robot Mode` batch commands | mirrored verbatim — identical command strings |

Parity check: the batch command strings, the constraints, and the Exit Codes table must be
byte-identical between the `SKILL.md` body and the Codex `prompt.md` so an agent behaves the
same regardless of which runtime loaded the skill.
