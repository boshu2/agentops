# Dual-Pane ATM — Spawn Checklist

Copy and track per session.

## Pre-spawn

- [ ] **Instrument lane** — `ao orchestrate route --json` then `ao orchestrate preflight --profile tri-vendor --json` (or `dual-pane`)
- [ ] **Route confirmed** — dual-pane fits; not shape 0 / swarm / full ATM queue
- [ ] **Work-split chosen** — pattern named in coordination ledger (see work-split-matrix.md)
- [ ] **Bead join-key** — `br create` or `br update --claim` if work is tracked
- [ ] **Reserve globs drafted** — disjoint per lane; never `"."` or whole repo
- [ ] **Packets written** — `packet-opus.md`, `packet-codex.md` (whole skill + scope)
- [ ] **Preflight tools** — `atm doctor`, `atm deps`, `which codex`, `which claude`

## Pane map (pick before `atm send`)

| Session shape | Pane 1 | Pane 2 | Pane 3 |
|---|---|---|---|
| **`--no-user`** (two workers only) | Opus (Claude) | Codex | — |
| **`--no-user`** (tri-vendor: Opus + Codex + AGY) | Opus (Claude) | Codex | AGY (Antigravity) |
| **User pane present** (tri-pane) | User / orchestrator attach | Opus | Codex |

Use the pane numbers from the row that matches your spawn. Examples below assume **`--no-user`**.

## Spawn

```bash
LABEL=<short-name>
SESSION="agentops--$LABEL"
# Smoke / real splits: disjoint lane globs (optional: /tmp/dual-pane-opus/ /tmp/dual-pane-codex/ for dry runs)
RESERVE_OPUS="docs/contracts/"
RESERVE_CODEX="cli/internal/"
# One --reserve value: space-separated globs inside a single quoted string (not two flags).
RESERVE_GLOBS="$RESERVE_OPUS $RESERVE_CODEX"

atm spawn agentops --label "$LABEL" --no-user \
  --cc=1:opus --cod=1:gpt-5.5 \
  --reserve "$RESERVE_GLOBS" \
  --no-cass-context --ready-timeout=2m --json
```

Example (literal paths): `--reserve "/path/to/opus-globs/ /path/to/codex-globs/"` — **not** `--reserve path1 path2` as separate tokens after `--reserve`.

- [ ] Session name recorded: `agentops--$LABEL`
- [ ] `--reserve` is exactly **one** quoted argument whose interior lists all lane globs space-separated
- [ ] `--reserve` paths match work-split matrix
- [ ] Optional: write `.agents/dual-pane/coordination.json` with `bead_id`, `pattern`, panes (use pane numbers from table above)

## Verify panes (before first send)

```bash
atm mapping --session="$SESSION"
# Documented equivalents if your ATM build differs: session pane list / attach layout inspect (see using-atm)
```

- [ ] Mapping shows expected Opus + Codex panes at the numbers from the pane map table
- [ ] **Verify windshield** — `ao orchestrate verify --session "$SESSION" --profile tri-vendor --json` (use `dual-pane` profile for two-lane sessions)
- [ ] Session name matches `agentops--$LABEL`

## First lane (Claude / Opus — pane 1 with `--no-user`)

```bash
# Prefer plain send; add --json only when you need structured capture
atm send "$SESSION" --pane=1 --file packet-opus.md \
  --no-cass-check --force-non-interactive

# If you used --json and exit 1 with empty stdout, retry without --json (advisory)
```

(Tri-pane with user attach: Opus is **pane 2**.)

- [ ] Send acknowledged — capture shows input cleared or thinking indicator
- [ ] Artifact signal within one window: branch, file, or `br` note update

## Second lane (Codex — pane 2 with `--no-user`)

```bash
atm codex preflight --session "$SESSION" --pane 2 --json
# proceed only on codex-live or goal-completed

atm send "$SESSION" --pane=2 --codex-goal --file packet-codex.md \
  --no-cass-check --force-non-interactive --json

atm codex wait-goal-engaged --session "$SESSION" --pane 2 --json
```

(Tri-pane with user attach: Codex is **pane 3**.)

- [ ] Preflight `proceed` — not `wait` / `respawn_required`
- [ ] `wait-goal-engaged` exit 0 (retry once on cold engage if `--codex-goal` send times out — advisory, not a spawn blocker)
- [ ] If unconfirmed: re-dispatch once; do not respawn until dump confirms wedge

## Post-spawn operator

- [ ] `am macros start-session` on each worker (if not born via `--reserve`)
- [ ] Per-lane `am file_reservations reserve` matches spawn globs
- [ ] Worktrees created if either lane edits tracked files
- [ ] Orchestrator notes actual resolved models if alias drift (e.g. opus → installed build)

## Tri-vendor (+AGY) — optional

Smoke-tested tri-pane **worker-only** (`--no-user`). Include `/tmp/dual-pane-agy/` (or real lane glob) in `--reserve`.

```bash
LABEL=dual-pane-agy-smoke
SESSION="agentops--$LABEL"
RESERVE_GLOBS="/tmp/dual-pane-opus/ /tmp/dual-pane-codex/ /tmp/dual-pane-agy/"

atm spawn agentops --label "$LABEL" --no-user \
  --cc=1:opus --cod=1:gpt-5.5 --agy=1 \
  --reserve "$RESERVE_GLOBS" \
  --no-cass-context --ready-timeout=2m --json
```

**Verify:** spawn `--json` panes or `tmux list-panes -t "$SESSION:1"` when `atm mapping` is empty (Agent Mail unavailable).

```bash
# Opus — pane 1
atm send "$SESSION" --pane=1 --file packet-opus.md \
  --no-cass-check --force-non-interactive

# Codex — pane 2 (wait for preflight proceed before goal)
atm codex preflight --session "$SESSION" --pane 2 --json
atm send "$SESSION" --pane=2 --codex-goal --file packet-codex.md \
  --no-cass-check --force-non-interactive --json
atm codex wait-goal-engaged --session "$SESSION" --pane 2 --json

# AGY — pane 3 (interactive TUI; not agy -p / gemini -p)
atm send "$SESSION" --pane=3 --file packet-agy.md \
  --no-cass-check --force-non-interactive
```

- [ ] **Cycle 2 lesson:** poll `atm codex preflight` until `recommended_action` is `proceed` before `--codex-goal` — blind dismiss sends (e.g. `"3"`) while Codex is busy can wedge the goal lifecycle
- [ ] AGY pane 3 shows packet input / `AGY_SMOKE_OK` (or lane artifact) in tmux capture — `atm activity` may omit AGY


## Teardown

- [ ] Capture: `atm save "$SESSION"` (+ codex palette-state if needed)
- [ ] Synthesis: `.agents/dual-pane/<session>-report.md`
- [ ] Release reservations; close or update bead with evidence
- [ ] `atm kill "$SESSION" --json`
