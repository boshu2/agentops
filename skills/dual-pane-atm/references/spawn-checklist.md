# Dual-Pane ATM — Spawn Checklist

Copy and track per session.

## Pre-spawn

- [ ] **Route confirmed** — dual-pane fits; not shape 0 / swarm / full ATM queue
- [ ] **Work-split chosen** — pattern named in coordination ledger (see work-split-matrix.md)
- [ ] **Bead join-key** — `br create` or `br update --claim` if work is tracked
- [ ] **Reserve globs drafted** — disjoint per lane; never `"."` or whole repo
- [ ] **Packets written** — `packet-opus.md`, `packet-codex.md` (whole skill + scope)
- [ ] **Preflight tools** — `atm doctor`, `atm deps`, `which codex`, `which claude`

## Spawn

```bash
LABEL=<short-name>
RESERVE_OPUS="docs/contracts/ docs/architecture/foo.md"
RESERVE_CODEX="cli/internal/ cli/cmd/"

atm spawn agentops --label "$LABEL" --no-user \
  --cc=1:opus --cod=1:gpt-5.5 \
  --reserve $RESERVE_OPUS $RESERVE_CODEX \
  --no-cass-context --ready-timeout=2m --json
```

- [ ] Session name recorded: `agentops--$LABEL`
- [ ] `--reserve` paths match work-split matrix
- [ ] Optional: write `.agents/dual-pane/coordination.json` with `bead_id`, `pattern`, panes

## First lane (Claude / Opus — pane 2)

```bash
atm send agentops--"$LABEL" --pane=2 --file packet-opus.md \
  --no-cass-check --force-non-interactive --json
```

- [ ] Send acknowledged — capture shows input cleared or thinking indicator
- [ ] Artifact signal within one window: branch, file, or `br` note update

## Second lane (Codex — pane 3)

```bash
atm codex preflight --session agentops--"$LABEL" --pane 3 --json
# proceed only on codex-live or goal-completed

atm send agentops--"$LABEL" --pane=3 --codex-goal --file packet-codex.md \
  --no-cass-check --force-non-interactive --json

atm codex wait-goal-engaged --session agentops--"$LABEL" --pane 3 --json
```

- [ ] Preflight `proceed` — not `wait` / `respawn_required`
- [ ] `wait-goal-engaged` exit 0
- [ ] If unconfirmed: re-dispatch once; do not respawn until dump confirms wedge

## Post-spawn operator

- [ ] `am macros start-session` on each worker (if not born via `--reserve`)
- [ ] Per-lane `am file_reservations reserve` matches spawn globs
- [ ] Worktrees created if either lane edits tracked files
- [ ] Orchestrator notes actual resolved models if alias drift (e.g. opus → installed build)

## Teardown

- [ ] Capture: `atm save agentops--"$LABEL"` (+ codex palette-state if needed)
- [ ] Synthesis: `.agents/dual-pane/<session>-report.md`
- [ ] Release reservations; close or update bead with evidence
- [ ] `atm kill agentops--"$LABEL" --json`
