# Codex Execution Profile — agy-sidecar-scheduled-tick

Read the base skill `../../skills/agy-sidecar-scheduled-tick/SKILL.md` first (the
eight Rules, the four-phase sidecar workflow, the per-fire evidence layout,
output spec, rubric). The base is the source of truth; this is the step-ordered
Codex path. One inviolable rule: **the cadence lives in the sidecar's `schedule`
builtin, not an external timer — and every fire must drop agentapi runtime
evidence.** The worker runtime is the AGY agentapi sidecar; never `claude -p`
(LAW 0). Door-9: `agy --version`/`--help`/`models` are allowed surface checks,
not the scheduled executor.

## Codex tool mapping

- **Declare the sidecar / write evidence** → `apply_patch` or shell redirection
  for `sidecar.json`, `schedule.txt`, `command.txt`, and the per-fire dir.
- **Probe + capture** → `shell_command` (`agy --version`, `curl …/status`,
  redirect the fire's stdout to `events.jsonl`, then `echo "$?" > exit-code`).
- **author != judge** → the scheduled author tick and the judge run in SEPARATE
  contexts; the orchestrator (not the tick) closes the bead. Mirror the verdict to
  a brain `userFacing` artifact so a different context consumes it.

## Steps

1. **Declare the sidecar + cadence (Phase 1).** Author `sidecar.json` with the
   `schedule` builtin (cron/interval) and the scoped per-fire command; record
   `schedule.txt` (sidecar name, builtin, cadence, runtime=agentapi, scopes). The
   cadence MUST be in `sidecar.json`, not host cron (Rule 1).
2. **Bring it up on agentapi (Phase 2).** Surface-check `agy --version`; probe
   `${AGENTAPI_URL:-http://127.0.0.1:3284}/status` → `agentapi-health.json`. Do
   not count any fire real until the runtime is reachable (Rule 2).
3. **Capture one fire (Phase 3).** Fresh timestamped `FIRE_DIR`; write
   `command.txt`; redirect the tick's stdout to `events.jsonl`, stderr to
   `stderr.log`; `echo "$?" > exit-code` on the very next line (Rules 3, 4). The
   tick claims one bead, works it scoped, writes evidence — does NOT close it.
4. **Mirror the verdict + validate (Phase 4).** Assert `schedule.txt`,
   `agentapi-health.json` (not `unreachable`), `events.jsonl`, `exit-code == 0`,
   `command.txt` all hold; mirror a `userFacing` brain artifact; reference the
   fire-dir in the bead / Agent Mail so the recurring evidence is discoverable.

## Guardrails

- **Cadence in the `schedule` builtin, not host cron (Rule 1)** — wrapping the
  print executor in external cron is the thing this skill replaces.
- **Every fire drops agentapi evidence (Rule 2); one fire, one timestamped dir
  (Rule 3); capture `$?` immediately (Rule 4).** Key the verdict off process
  reality, not self-report.
- **The tick does not close its own beads (Rule 5)** — author != judge; an
  independent context judges, the orchestrator closes.
- **Scope with `--add-dir`, sandbox by default (Rule 6); `dcg` BeforeTool guard
  stays on (Rule 7)** — a long-lived auto-driven process is exactly where a
  destructive command would slip through.
- **AGY lane only — never `claude -p` (Rule 8 / LAW 0).** Sidecar runtime is the
  agentapi server; other-vendor workers go through their native executor.
- **Backstage only.** Never surface this content in client-facing material.
