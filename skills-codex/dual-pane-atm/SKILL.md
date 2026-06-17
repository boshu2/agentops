---
name: dual-pane-atm
description: "Run dual-pane ATM collaboration (Opus + Codex)."
---

# dual-pane-atm (Codex)

Codex-native entry point for the `dual-pane-atm` operator skill.

The AgentOps source skill `../../skills/dual-pane-atm/SKILL.md` is the source of truth
for domain behavior, commands, examples, references, and output expectations.
Read it first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant source references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Verify command syntax from local `--help` or checked-in references before acting.
- Treat peer gate requests (`ACTION NEEDED`, `Hey! Listen!`, merge-gate,
  unblock-condition, verdict/dry-run requests) as interrupts: answer the gate
  before broad watching, and surface the result where the peer can actually read
  it.
- For dual-pane sessions, load `$using-atm` and `$agent-mail`; spawn with ATM
  (`atm spawn ... --cc=1:opus --cod=1:gpt-5.5 --no-user --reserve "docs/contracts/ cli/internal/"` or
  smoke dirs like `/tmp/dual-pane-opus/ /tmp/dual-pane-codex/` as **one** quoted `--reserve` value).
  After spawn, `atm mapping --session="$SESSION"` before any send; coordinate via Agent Mail, and
  `atm kill` on teardown. With `--no-user`, Opus is pane 1 and Codex is pane 2; with a user pane,
  Opus/Codex are panes 2/3. Opus: prefer plain `atm send --pane=1 --file …`; if `--json` exits 1
  with empty stdout, retry without `--json` (advisory). Codex: `--codex-goal` may need a retry on
  cold engage — use `wait-goal-engaged`. Never use print-mode CLIs for the other-family pane.
- Acceptance scenarios live in `references/dual-pane-atm.feature`; mirror spawn checklist and work-split matrix from the source skill.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.
