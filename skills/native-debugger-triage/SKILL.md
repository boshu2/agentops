---
name: native-debugger-triage
description: |-
  Use when debugging native programs or ELF binaries with gdb breakpoints, backtraces, and memory inspection.
  Triggers:
skill_api_version: 1
user-invocable: false
hexagonal_role: domain
practices:
- root-cause-analysis
- evidence-first-debugging
consumes: []
produces:
- backtrace
- core-dump-analysis
context:
  window: fork
metadata:
  tier: execution
  stability: stable
  dependencies: []
output_contract: backtrace + diagnosis (saved to .agents/research/ when investigating a real bug)
---
# native-debugger-triage — drive GNU gdb on native binaries, including head-less batch mode

> **Quick Ref:** `gdb -batch -ex run -ex bt --args ./prog ARGS` reproduces a crash and
> prints a backtrace in one non-interactive command. For a core file: `gdb -batch -ex bt PROG core`.

## ⚠️ Critical Constraints

- **Build with debug info or the backtrace is useless.** A stripped/`-O2` binary gives
  `??` frames and optimized-out locals.
  **Why:** gdb reads DWARF symbols; without them frames have no names or line numbers.
  - WRONG: `cc prog.c -o prog` then wonder why `bt` shows `0x4011 a6 in ?? ()`.
  - CORRECT: `cc -g -O0 prog.c -o prog` (add `-ggdb3` for macros; keep symbols un-stripped).

- **Never run an interactive gdb session inside an automated/agent context — it hangs.**
  An agent has no TTY to answer the `(gdb)` prompt.
  **Why:** gdb blocks waiting for stdin; the tool call times out with no output.
  - WRONG: `gdb ./prog` (drops to an interactive prompt and stalls).
  - CORRECT: `gdb -batch -ex run -ex bt --args ./prog` (runs, prints, exits non-zero on crash).

- **Enable core dumps before you can analyze one.** Default `ulimit -c` is often `0`.
  **Why:** the kernel writes no core file, so post-mortem analysis is impossible.
  - CORRECT: `ulimit -c unlimited` in the same shell, then re-run the program.
  - On Linux check `cat /proc/sys/kernel/core_pattern` — if it pipes to `systemd-coredump`,
    retrieve with `coredumpctl gdb PROG` instead of a local `core` file.

- **Watchpoints are slow without hardware support.** Software watchpoints single-step.
  **Why:** a software watchpoint can make execution 100–1000× slower.
  - CORRECT: scope to the narrowest expression (`watch obj.field`, not `watch *huge_array`)
    and `delete` it once the offending write is found.

## Why This Exists

A crash report ("Segmentation fault") with no backtrace is noise. gdb turns it into
evidence: the exact frame, line, argument values, and corrupted state at the moment of
failure. The skill exists because the *interactive* gdb everyone learns is the wrong tool
for an agent — the value is the **batch/`-batch -ex`** form that produces a captured
artifact (a backtrace, a watchpoint trace, a core analysis) without a human at a prompt.
Use it to move from "it crashed" to "it crashed because `parse()` dereferenced a NULL
`node->next` at line 84 when the list was empty."

## Quick Start

```bash
# 1. Build with symbols (precondition for everything below)
cc -g -O0 -o prog prog.c

# 2. Reproduce a crash and get a backtrace — one non-interactive command
gdb -batch -ex run -ex 'bt full' --args ./prog arg1 arg2

# 3. Post-mortem a core dump
ulimit -c unlimited; ./prog              # produces ./core (or coredumpctl on systemd)
gdb -batch -ex 'bt full' -ex 'info registers' ./prog ./core
```

## Workflow — investigate a native crash

**Phase 1 — Reproduce under gdb (batch).** Confirm you can trigger the failure head-less.
```bash
gdb -batch -ex run -ex bt --args ./prog <repro-args>
```
*Checkpoint:* you have a backtrace with named frames + line numbers. If frames are `??`,
STOP — rebuild with `-g -O0` before continuing (no other step works without symbols).

**Phase 2 — Localize.** Set a breakpoint at the crashing function, inspect state on entry.
```bash
gdb -batch \
  -ex 'break parse' \
  -ex run \
  -ex 'bt' \
  -ex 'info args' \
  -ex 'info locals' \
  -ex 'print node->next' \
  --args ./prog <repro-args>
```
*Checkpoint:* you can name the bad value (e.g. `node->next = 0x0`).

**Phase 3 — Catch the moment it goes bad.** Use a conditional breakpoint or a watchpoint
to stop only at the relevant iteration, not on every call.
```bash
# Conditional: stop only when the guard is already wrong
gdb -batch -ex 'break parse if node == 0' -ex run -ex 'bt full' --args ./prog <args>

# Watchpoint: stop on the write that corrupts a field
gdb -batch -ex 'break main' -ex run -ex 'watch g_state.count' -ex continue -ex bt --args ./prog
```
*Checkpoint:* you have the exact write/condition that produces the corrupt state.

**Phase 4 — Record the diagnosis.** Save the backtrace + the root-cause line to an artifact
(see Output Specification). Hand off to a fix (or the `bug-hunt` skill for the fix design).

## Core gdb command reference

| Goal | Command | Notes |
|---|---|---|
| Break at function/line | `break FUNC` · `break file.c:84` | `tbreak` = temporary (auto-deletes after first hit) |
| Conditional break | `break FUNC if x > 10` | condition is C-expression in target scope |
| Watchpoint (data) | `watch EXPR` · `rwatch` (read) · `awatch` (read/write) | hardware-backed when possible; scope it tight |
| Run / restart | `run [args]` · `start` (break at main first) | `set args ...` to set without running |
| Step | `next` (over) · `step` (into) · `finish` (out) · `continue` | `stepi`/`nexti` for instruction-level |
| Backtrace | `bt` · `bt full` (with locals) · `bt N` (top N) | `frame N` then `info locals` to inspect any frame |
| Inspect value | `print EXPR` · `print/x` (hex) · `p *ptr@count` (array) | `ptype VAR` for the type |
| Examine memory | `x/16xb ADDR` · `x/8gx $sp` · `x/i $pc` | `/<count><format><size>` |
| Registers | `info registers` · `print $pc` · `print $rsp` | |
| Threads | `info threads` · `thread N` · `thread apply all bt` | last is the go-to for deadlocks/hangs |
| List code | `list FUNC` · `list 80,90` | needs source available at the recorded path |
| Manage breakpoints | `info breakpoints` · `delete N` · `disable N` | |

## TUI mode (interactive only — never in batch)

For a human at a terminal: `gdb -tui ./prog`, or toggle in-session with `Ctrl-x a`.
`Ctrl-x 2` cycles source/asm/registers layouts; `Ctrl-l` redraws if the screen corrupts.
The TUI is a convenience for a person — an agent uses batch mode and never the TUI.

## Robot Mode (head-less / agent-driven)

The whole skill is designed to run without a TTY. Two equivalent ways to feed commands:

```bash
# Inline: each -ex is one gdb command, executed in order, then gdb exits (-batch)
gdb -batch -ex 'set pagination off' -ex run -ex 'bt full' --args ./prog <args>

# From a command file (cleaner for long scripts; -batch implies -nx -q and auto-quit)
printf 'set pagination off\nrun\nbt full\ninfo registers\nquit\n' > /tmp/gdb.cmds
gdb -batch -x /tmp/gdb.cmds --args ./prog <args>
```

- **Always include `set pagination off`** (or `-batch`, which sets it) — otherwise gdb stops
  at `---Type <return>---` waiting for a keypress that never comes.
- `-batch` implies `-nx` (skip `.gdbinit`) and `-q` (no banner) and quits at end.
- Attach to a running PID head-less: `gdb -batch -p <PID> -ex 'thread apply all bt'` (needs
  `ptrace` permission; on Linux may require `sudo` or `sysctl kernel.yama.ptrace_scope=0`).

## Output Specification

When investigating a real bug, save the evidence so it survives the session:
- **Path:** `.agents/research/YYYY-MM-DD-gdb-<short-slug>.md`
- **Contents:** the exact `gdb -batch` command run, the full `bt`/`bt full` output, the
  named root-cause value (e.g. `node->next == 0x0 at parse:84`), and the proposed fix.
For a one-off reproduce, the captured backtrace printed to stdout is the artifact.

## Exit Codes (what `gdb -batch` returns)

| Exit | Meaning | Agent action |
|---|---|---|
| 0 | Program ran to completion under gdb, no fatal signal | No crash reproduced — adjust repro args |
| non-0 | Program hit a fatal signal / gdb script ended after a stop | Expected when reproducing a crash; read the captured `bt` |
| 1 (gdb-level) | gdb itself errored (bad binary, missing file, bad `-ex`) | Fix the command, not the program |

Note: `gdb -batch` propagates the inferior's exit status; a crashing program yields a
non-zero exit *with* a usable backtrace — that is success for a reproduce, not a failure.

## Quality Rubric

- The backtrace shows **named frames with file:line** (not `??`) — proves symbols are present.
- The diagnosis names a **specific value + location** (`x == NULL at file.c:N`), not "it crashed somewhere."
- Every command in the captured artifact is **non-interactive** (`-batch`/`-ex`/`-x`) and reproducible.

## Examples

**Segfault, full backtrace with locals, one command:**
```bash
gdb -batch -ex run -ex 'bt full' --args ./parser bad-input.txt
```

**Find which write corrupts a counter:**
```bash
gdb -batch -ex 'break main' -ex run -ex 'watch g_count' -ex continue -ex bt --args ./server
```

**Post-mortem a systemd-managed core:**
```bash
coredumpctl gdb /usr/bin/myservice    # then: bt full, info registers, quit
```

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Backtrace shows `?? ()` / `<optimized out>` | No/insufficient debug info | Rebuild `-g -O0` (`-ggdb3` for macros); don't strip |
| gdb hangs in an agent context | Interactive prompt, no TTY | Use `-batch` + `-ex`/`-x`; add `set pagination off` |
| No `core` file after a crash | `ulimit -c 0` or piped core_pattern | `ulimit -c unlimited`; or `coredumpctl gdb PROG` |
| `ptrace: Operation not permitted` (attach) | Yama ptrace scope | `sudo sysctl kernel.yama.ptrace_scope=0` (or run as root) |
| Watchpoint never triggers / is super slow | Software watchpoint, or expr out of scope | Scope tighter; set it after the var exists; prefer `watch field` |
| `No symbol "x" in current context` | Wrong frame, or out of scope | `frame N` to the right frame; `info locals`/`info args` first |
| Source not shown by `list` | Build path differs from source path | `directory /path/to/src`, or `set substitute-path OLD NEW` |

## See Also

| I need to… | Use | Reference |
|---|---|---|
| Design the fix once root cause is known | `bug-hunt` skill | — |
| Run gdb on Codex (dual-file parity) | this skill, Codex form | [references/codex-parity.md](references/codex-parity.md) |
| Validate this skill's structure | Execute `scripts/validate.sh` | — |
