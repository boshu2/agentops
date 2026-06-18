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
  **Tri-vendor (+AGY):** add `--agy=1` and include `/tmp/dual-pane-agy/` in the same quoted
  `--reserve` value; panes 1/2/3 = Opus/Codex/AGY with `--no-user` (worker-only — not user+two-workers).
  Verify via spawn `--json` panes or `tmux list-panes` when `atm mapping` is empty; `atm activity` may omit AGY.
  After spawn, confirm pane numbers before any send; coordinate via Agent Mail, and `atm kill` on teardown.
  With `--no-user`, Opus is pane 1 and Codex is pane 2 (AGY pane 3 when `--agy=1`); with a user pane,
  Opus/Codex are panes 2/3. Opus: plain `atm send --pane=1 --file …`; AGY: `atm send --agy --file …` (or `--pane=3`; interactive TUI, not `agy -p`/`gemini -p`) (interactive TUI, not `agy -p`/`gemini -p`).
  Codex: poll `atm codex preflight` until `proceed` before `--codex-goal`; cold engage may need retry + `wait-goal-engaged`.
  Never use print-mode CLIs for the other-family pane.
- **In-session duel (no panes):** for a one-shot one-way-door decision, skip ATM
  panes — run >=3 perspective subagents (Agent `model:` override) plus the
  cross-family voice via `codex exec "<prompt>" </dev/null`, then winnow on the
  converged slice. This is what `/discovery`'s fanout gate runs.
- Acceptance scenarios for preflight and work-split gates live in
  `references/dual-pane-atm.feature`; mirror spawn checklist and work-split
  matrix from the source skill.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.
