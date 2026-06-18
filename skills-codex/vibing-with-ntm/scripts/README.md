# Scripts — Operator Helpers

Zero-token helpers (executed, never loaded into Claude's context) for repetitive operator-loop operations. Most scripts only print and exit. `convergence-check.sh` maintains its own `/tmp/vibing-convergence-<session>.state` streak file so it can require two consecutive ticks before stopping. Per AGENTS.md RULE 1: none of these scripts ever delete files.

Run directly — they all have shebangs and are chmod +x.

| Script | One-line purpose |
| --- | --- |
| `orchestrator-tick.sh` | Terse one-screen snapshot for deciding a tick's action (source health → panes → oauth → stuck → triage → commits → backlog → suggested action) |
| `convergence-check.sh` | Test the 3 OC-016 convergence conditions; exits 0 only when streak ≥ 2 ticks |
| `pane-liveness-audit.sh` | Per-pane ground truth: `pane_current_command` + pid + flags bare-zsh panes (dead CLI) |
| `process-contention-sweep.sh` | Diagnose (don't kill) parasitic br/cargo/rsync/D-state processes holding cross-session locks |
| `disk-trajectory.sh` | Track per-tick disk delta; warn on trajectory >3pp/tick before absolute threshold fires |
| `depth-gate-evidence.sh` | Generate the 3-part evidence (grep counts + function signatures + test output) an OC-034 depth-gate requires |
| `stale-bead-audit.sh` | List in_progress beads whose git log shows shipped commits (safe-to-close candidates) |

## Composition — "Full Tick"

```bash
# One-tick diagnostic run, pipe to notebook / dispatch
SESSION=asupersync
REPO=/data/projects/asupersync

./scripts/orchestrator-tick.sh "$SESSION" "$REPO"
./scripts/pane-liveness-audit.sh "$SESSION" "$REPO"
./scripts/disk-trajectory.sh
./scripts/convergence-check.sh "$SESSION" "$REPO" && echo "STOP — swarm converged"
```

## Safety

- **None of these scripts modify tmux, beads, git, or project files.** `convergence-check.sh` writes only its own `/tmp/vibing-convergence-<session>.state` file.
- `process-contention-sweep.sh --include-kill-cmds` prints suggested `kill` commands for the operator to review — it never executes them.
- Mutations (bead closes, process kills, pane restarts) remain explicit operator actions per AGENTS.md.

## Usage With `/loop` When Available

To tick every 15 min:

```
/loop 15m bash .claude/skills/vibing-with-ntm/scripts/orchestrator-tick.sh asupersync /data/projects/asupersync
```

Convergence-detecting auto-exit:

```
/loop 15m "bash .claude/skills/vibing-with-ntm/scripts/convergence-check.sh asupersync /data/projects/asupersync || bash .claude/skills/vibing-with-ntm/scripts/orchestrator-tick.sh asupersync /data/projects/asupersync"
```

(Convergence-check exits 0 on converge, which `/loop` can honor as a stop condition when that slash tool is installed.)
