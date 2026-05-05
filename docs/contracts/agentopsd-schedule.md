# AgentOps Daemon Scheduling Contract

`agentopsd` gains native scheduling. Define recurring jobs in
`.agents/schedule.yaml`; the daemon ticks them on cron cadence, submits jobs into
the daemon Queue, applies backpressure, and records every fire/skip/delete in the
ledger. The substrate is platform-independent — the same schedule file works on
macOS (under launchd-spawned `agentopsd`), Linux (systemd user services), and
container hosts.

This contract is the operator-facing reference for the scheduling primitives
landed in epic `soc-8inr` (recurrence supervisor + `llmwiki.loop` executor +
schedule config). For the broader daemon architecture, see
[AgentOps Daemon Contract](agentops-daemon.md).

The product story: install AgentOps, point `agentopsd` at your repo, get
continuously-compounding knowledge wiki + bd flywheel without writing a systemd
unit or a launchd plist for every recurring job.

## Scope

This contract covers:

- the `.agents/schedule.yaml` schema and parse-time validation
- cron expression syntax accepted by the supervisor
- backpressure semantics (`skip_if_running`, `max_queue_depth`)
- mutation auth model for the schedule HTTP routes
- ledger event vocabulary for scheduling (`schedule.created/fired/skipped/deleted`)
- the `llmwiki.loop` executor's per-stage idempotency contract
- migration from the `submit-with-window.sh` wrapper pattern
- troubleshooting recipes and rollback steps

This contract does not cover:

- internal scheduler implementation details beyond the operator-visible contract
- per-job-type payload schemas (each executor owns its own contract)
- distributed scheduling across multiple `agentopsd` instances (single-node only)

## Source Of Truth

The ledger is the source of truth for schedule state. `ListSchedules` is a
ledger replay: a schedule is present iff its most recent `schedule.created`
event has not been followed by a matching `schedule.deleted`. There is no
separate schedule table; the `.agents/schedule.yaml` file is the boot-time seed
that materializes into ledger events on `ao schedule add` or `ao daemon run
--schedule-file`.

Authoritative state:

- registered schedules (derived from `schedule.created` − `schedule.deleted`)
- per-tick fire / skip outcomes
- materialized job submissions (carry `schedule_name` + `submission_id` in payload)

Derived projections:

- the response of `GET /v1/schedules`
- `ao schedule list` table output
- `ao daemon status` schedule counters (when emitted)

## `.agents/schedule.yaml` Schema Reference

The canonical JSON-schema is at
[`schemas/schedule.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/schedule.schema.json).
Strict YAML decoding (yaml.v3 `KnownFields(true)`) is enforced at every level —
**typos in field names produce loud errors at parse time, not silent ignore.**

Full example showing every field:

```yaml
schedules:
  - name: nightly-llmwiki-loop
    cron: "0 3 * * *"
    job_type: llmwiki.loop
    timeout: 30m
    payload:
      vault: ~/wiki
      stages: ["ingest", "query", "lint", "promote"]
      lint_interval_hours: 24
    backpressure:
      skip_if_running: true
      max_queue_depth: 3

  - name: hourly-dream-run
    cron: "0 * * * *"
    job_type: dream.run
    timeout: 45m
    payload:
      goal: "compound today's surfaces"
      output_dir: .agents/overnight/latest
    backpressure:
      skip_if_running: true
      max_queue_depth: 2

  - name: every-5min-tick
    cron: "*/5 * * * *"
    job_type: wiki.forge
    payload:
      tier: entity-extract
```

### Field reference

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique schedule identifier. Pattern: `^[a-z][a-z0-9-]*$`. Duplicate names are rejected at parse time. |
| `cron` | string | yes | 5-field cron expression with descriptors. See [Cron Expression Cheat-Sheet](#cron-expression-cheat-sheet). |
| `job_type` | string | yes | Job type to materialize. Pattern: `^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$` (e.g., `llmwiki.loop`, `dream.run`, `factory.local-pilot`). |
| `payload` | any | no | Free-form payload passed to the executor. Re-encoded through JSON to canonical bytes by the parser. |
| `timeout` | string | no | Per-job timeout (Go duration format: `30m`, `1h30m`). Threaded into the materialized job's payload as `timeout`. |
| `backpressure.skip_if_running` | bool | no | Skip the tick if a prior job from this schedule is still in-flight. Default `false`. |
| `backpressure.max_queue_depth` | integer | no | Skip the tick if non-terminal jobs from this schedule ≥ N. Capped by `AGENTOPS_SCHEDULE_MAX_QUEUE_DEPTH_CEILING` (default 1000). |

### Schema strictness

`additionalProperties: false` is enforced at every level of both the JSON
schema and the YAML decoder:

- top-level: `schedules` is the only accepted key
- per-entry: `name`, `cron`, `job_type`, `payload`, `timeout`, `backpressure`
- per-entry `backpressure`: `skip_if_running`, `max_queue_depth`

A typo like `back_pressure` or `cronspec` fails the load with a `LoadError`
naming the offending field. The daemon **fails closed** on malformed
`schedule.yaml` (see [CLI Reference](#cli-reference)).

### Parse-time validation

The parser (`cli/internal/schedule/parser.go`, `Load(path)`) enforces:

1. **Strict YAML decoding** — unknown fields rejected.
2. **Duplicate name rejection** — second occurrence of a name fails the load.
3. **Cron validation** — delegates to `daemon.ParseCron`; rejects 6-field
   sub-minute schedules, returns `*CronParseError` preserving the original
   input.
4. **`job_type` pattern** — must match `^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$`.
5. **Effective-period floor** — measures two consecutive cron ticks; gap must
   be ≥ `AGENTOPS_SCHEDULE_MIN_PERIOD_SECONDS` (default 60s).
6. **`max_queue_depth` ceiling** — must be ≤ `AGENTOPS_SCHEDULE_MAX_QUEUE_DEPTH_CEILING`
   (default 1000).
7. **`timeout` parsing** — Go `time.ParseDuration`; invalid string fails the
   load.
8. **Payload re-encoding** — YAML payload is decoded to `any` and re-marshalled
   as JSON so the daemon receives canonical bytes.
9. **Payload contract validation** — the parser materializes the same defaults
   the recurrence supervisor applies at fire time, then validates the resulting
   job payload for job types with typed daemon specs. Bad schedule payloads fail
   at load time instead of first cron fire.

### Payload Defaults And Requirements

Before submission, the recurrence supervisor injects `schedule_name`,
`submission_id`, `tick_at`, and `timeout` (when configured). It also fills
job-type defaults so schedule YAML can stay operator-sized while the queue sees
a typed daemon payload.

| `job_type` | Defaults injected | Required in `payload` |
|---|---|---|
| `rpi.run` | `schema_version`, `job_type`, `run_id`, `goal`, `start_phase`, `max_phase`, `test_first`, `backend` | none; override defaults when the run needs a specific goal, run id, backend, or phase range |
| `rpi.phase` | `schema_version`, `job_type`, `run_id`, `goal`, `phase`, `phase_name`, `backend` | none; set `phase` and `goal` explicitly for production phase schedules |
| `dream.run` | `schema_version`, `job_type`, `dream_run_id`, `mode` | `output_dir` |
| `dream.stage` | `schema_version`, `job_type`, `dream_run_id`, `stage`, `mode` | `output_dir` |
| `wiki.forge` | `schema_version`, `job_type`, `output_dir`, `worker_kind`, `provider`, `max_attempts` | `source_paths` |
| `plans.projection` | `schema_version`, `job_type`, `refresh_trigger` | `project_id`, `output_dir` |
| `factory.admission` | `schema_version`, `job_type`, `run_id`, `mode` | `work_order` |
| `factory.local-pilot` | `schema_version`, `job_type`, `run_id`, `mode` | `work_order` |
| `llmwiki.loop`, `wiki.build`, `openclaw.snapshot` | schedule metadata only | none at the schedule layer |

Typed validation rejects malformed values, for example `rpi.run` with
`start_phase: "two"` or `wiki.forge` without any `source_paths`.

### Environment overrides

| Env var | Default | Effect |
|---|---|---|
| `AGENTOPS_SCHEDULE_MIN_PERIOD_SECONDS` | `60` | Minimum gap between consecutive cron ticks. Operator-tightenable, not loosenable below the cron-package floor. |
| `AGENTOPS_SCHEDULE_MAX_QUEUE_DEPTH_CEILING` | `1000` | Hard cap on `backpressure.max_queue_depth` per schedule. |

Both env vars must parse as positive integers; an invalid value fails the load.

## Cron Expression Cheat-Sheet

`agentopsd` accepts the **5-field standard with descriptors** via the
`robfig/cron/v3` parser. **Six-field expressions with seconds are intentionally
rejected** — sub-minute scheduling enables denial-of-service patterns and is out
of scope.

```
┌───────────── minute       (0 - 59)
│ ┌─────────── hour         (0 - 23)
│ │ ┌───────── day of month (1 - 31)
│ │ │ ┌─────── month        (1 - 12)
│ │ │ │ ┌───── day of week  (0 - 6, Sunday=0)
│ │ │ │ │
* * * * *
```

Common patterns:

| Cron | Meaning |
|---|---|
| `0 3 * * *` | Daily at 03:00 |
| `0 * * * *` | Hourly on the hour |
| `*/5 * * * *` | Every 5 minutes |
| `*/15 * * * *` | Every 15 minutes |
| `0 9-17 * * 1-5` | Hourly 09:00–17:00 weekdays |
| `*/10 7-22 * * *` | Every 10 minutes 07:00–22:59 |
| `@daily` | Equivalent to `0 0 * * *` |
| `@hourly` | Equivalent to `0 * * * *` |
| `@weekly` | Equivalent to `0 0 * * 0` |
| `@monthly` | Equivalent to `0 0 1 * *` |

The minimum effective tick interval is **60 seconds**. The supervisor's poll
loop runs at one-minute cadence by default (matching the cron-package's
five-field minimum granularity). Operators who need a tighter floor can lower
the supervisor poll via internal API; loosening below 60 seconds is
deliberately not supported.

## Backpressure Semantics

Two knobs gate whether a due tick fires. They are evaluated by the pure
function `shouldFire` (`cli/internal/daemon/recurrence.go`) against the live
queue snapshot.

### `skip_if_running: true`

The supervisor counts non-terminal jobs whose `payload.schedule_name` matches
this schedule. If any are in `running` state, the tick is skipped. A
`schedule.skipped` event is recorded with reason `skip_if_running:in-flight`.

Use this for long-running jobs (Dream runs, full wiki forge passes) where
overlapping invocations would thrash shared state.

### `max_queue_depth: N`

The supervisor counts non-terminal jobs whose `payload.schedule_name` matches
this schedule. If the count is ≥ N, the tick is skipped with reason
`max_queue_depth:N`.

Both knobs may be combined; `skip_if_running` is checked first. If neither is
set, every tick fires unconditionally (subject to crash-resume dedup).

### Recovery model

When backpressure clears, the supervisor fires on the **next** cron tick — there
is no event-driven wake-up. This is a deliberate design choice: it keeps the
supervisor's invariants tractable (one decision per tick) and prevents
storm-on-resume failure modes when a long-running job finally drains the queue.

If you need event-driven recovery, drop the schedule and submit jobs directly
via `POST /v1/jobs`.

### Skipped tick observability

Every skipped tick emits a `schedule.skipped` ledger event:

```json
{
  "event_type": "schedule.skipped",
  "schedule_name": "nightly-llmwiki-loop",
  "reason": "skip_if_running:in-flight",
  "tick_at": "2026-05-01T03:00:00Z"
}
```

Tail recent skips with the daemon ledger query routes or `ao daemon status`
(when the schedule projection exposes them).

## Mutation Auth Model

Schedule mutations are privileged operator surface. They require the
**admin** mutation capability (see `cli/internal/daemon/auth.go`).

### Header

The canonical token header is **`X-AgentOps-Daemon-Token`**. This is the
spelling shipped in `DefaultMutationTokenHeader`; common drift points in
operator docs use `X-Agentops-Mutation-Token` or similar — those are wrong.

`Authorization: Bearer <token>` is also accepted as a fallback.

### Token resolution

`ao schedule add | run | remove` resolves the mutation token via:

1. `--token <value>` flag (highest priority)
2. `--token-file <path>` flag
3. `AGENTOPSD_TOKEN`
4. `AGENTOPS_DAEMON_TOKEN` legacy environment fallback
5. token metadata from `.agents/daemon/activation.json`

The token file must have mode ≤ `0600` (owner-only); permissive modes fail
with `ErrUnsafeTokenFileMode`.

### Mutation routes

| Route | Method | Auth | Capability |
|---|---|---|---|
| `/v1/schedules` | POST | required | `admin` |
| `/v1/schedules/{name}` | DELETE | required | `admin` |
| `/v1/schedules` | GET | not required | n/a (read-only) |

Routes are registered through the `registerMutationRoute` helper. The
build-time guard at
[`scripts/check-mutation-route-coverage.sh`](https://github.com/boshu2/agentops/blob/main/scripts/check-mutation-route-coverage.sh)
asserts every mutation `mux.HandleFunc` call site goes through the helper —
this closes the loophole where a developer could register a path directly and
bypass auth. See [AgentOps Daemon Contract — Mutation Auth](agentops-daemon.md)
for the broader auth model.

Mutation routes are loopback-only by default
(`MutationPolicy.RequireLocalRemote = true`); cross-host calls are denied
before token validation.

## Examples

### Nightly llmwiki.loop

```yaml
schedules:
  - name: nightly-llmwiki-loop
    cron: "0 3 * * *"
    job_type: llmwiki.loop
    timeout: 30m
    payload:
      vault: ~/wiki
    backpressure:
      skip_if_running: true
      max_queue_depth: 3
```

```bash
ao schedule add --file .agents/schedule.yaml --token-file ~/.agents/daemon-token
ao schedule list
```

### Hourly dream.run

```yaml
schedules:
  - name: hourly-dream-run
    cron: "0 * * * *"
    job_type: dream.run
    timeout: 45m
    payload:
      goal: "compound today's open work"
      output_dir: .agents/overnight/latest
    backpressure:
      skip_if_running: true
      max_queue_depth: 2
```

### One-shot test fire

```bash
ao schedule run nightly-llmwiki-loop --token-file ~/.agents/daemon-token
# fired: nightly-llmwiki-loop  job_id=...  status=queued
```

`ao schedule run` resolves the schedule (via `GET /v1/schedules`), then submits
a one-shot job to `/v1/jobs` with idempotency key
`schedule-run:<name>:<unix-nanos>`. The prefix intentionally differs from the
recurrence supervisor's `submission_id` so a manual fire never collides with a
scheduled tick.

## CLI Reference

All schedule subcommands live under `ao schedule` (see
`cli/cmd/ao/schedule.go`).

### `ao schedule add`

```
ao schedule add --file <path> [--token <value>] [--token-file <path>]
```

Parses the YAML file, then `POST /v1/schedules` once per template. Token
required. Adds are not transactional — if the third of five templates fails
the loop returns immediately and the first two are already saved. Re-run after
fixing the offending entry; existing schedules conflict with `409 Conflict`,
which is detectable in the response body.

### `ao schedule list`

```
ao schedule list [--json]
```

`GET /v1/schedules`. Tab-separated table by default
(`NAME  CRON  JOB_TYPE  BACKPRESSURE`); `--json` emits the full
`ListSchedulesResponse` for machine consumers. **No mutation token required.**

### `ao schedule run`

```
ao schedule run <name> [--token <value>] [--token-file <path>]
```

One-shot test fire. Client-side flow:

1. `GET /v1/schedules` → find `<name>` in the list
2. Build a `SubmitJobRequest` from the template's `job_type` + `payload`
3. `POST /v1/jobs` with `IdempotencyKey = schedule-run:<name>:<unix-nanos>`

Distinct idempotency-key prefix means manual fires never collide with the
supervisor's deterministic `submission_id` keys for cron ticks.

### `ao schedule remove`

```
ao schedule remove <name> [--token <value>] [--token-file <path>]
```

`DELETE /v1/schedules/{name}`. Idempotent at the store layer — removing a
non-existent schedule is a no-op (logs a warning, returns success). Token
required.

### `ao daemon run --schedule-file`

```
ao daemon run [--schedule-file <path>] [--workers N]
```

Loads schedules at startup. Resolution order:

1. `--schedule-file <path>` (explicit)
2. `.agents/schedule.yaml` (auto-detected at cwd)
3. neither present → daemon starts without schedules

The daemon **fails closed** on malformed YAML. The error message names the
file path and the underlying parse error and refuses to start. Repair the file
or remove the flag.

```
schedule load failed at .agents/schedule.yaml: schedule: ... (daemon refuses
to start with malformed schedule.yaml; fix the file or remove --schedule-file)
```

To validate a `schedule.yaml` without restarting the daemon, run
`ao schedule add --file <path>` against a running daemon — the parser is the
same as the boot-time loader.

## Ledger Event Reference

Schedule operations emit four ledger event types (declared in
`cli/internal/daemon/store.go`). They share the daemon's `LedgerSchemaVersion`
(currently `1`); newer event types are additive — older daemons replaying
newer ledgers **skip-and-log unknown event types** rather than failing.

### `schedule.created`

Emitted by `Store.SaveSchedule` when a new schedule is registered (via
`POST /v1/schedules` or `ao daemon run --schedule-file` boot replay). The
payload carries the full `RecurringJobTemplate` so replay can reconstruct the
schedule list:

```json
{
  "event_type": "schedule.created",
  "name": "nightly-llmwiki-loop",
  "template": {
    "name": "nightly-llmwiki-loop",
    "cron": "0 3 * * *",
    "job_type": "llmwiki.loop",
    "payload": "{...}",
    "timeout": "30m",
    "backpressure": {"skip_if_running": true, "max_queue_depth": 3}
  },
  "created_at": "2026-05-01T15:42:00Z"
}
```

The actor field is `agentopsd-scheduler` for boot-replay-driven creates and the
mutation `TokenName` for HTTP-driven creates.

### `schedule.fired`

Emitted by the recurrence supervisor **before** the materialized job is
submitted to the queue. This write-order rule (per pre-mortem amendment A4) is
load-bearing: a crash between ledger append and queue submit is recovered on
the next tick via the deterministic `submission_id` idempotency key.

```json
{
  "event_type": "schedule.fired",
  "name": "nightly-llmwiki-loop",
  "submission_id": "a3f2c81d4e5b6f70",
  "tick_at": "2026-05-01T03:00:00Z"
}
```

`submission_id` is `sha256("<schedule_name>:<tick_unix_seconds>")[:16]` — first
16 hex chars. Deterministic per (name, tick) so retries collapse on the queue's
idempotency layer.

For operator visibility, each new `schedule.fired` ledger event also emits a
single stderr log line:

```text
[recurrence] fired <name> submission_id=<id> tick_at=<RFC3339Nano>
```

### `schedule.skipped`

Emitted when backpressure causes the supervisor to skip a tick.

Each skip also emits:

```text
[recurrence] skipped <name> reason=<reason> tick_at=<RFC3339Nano>
```

```json
{
  "event_type": "schedule.skipped",
  "name": "nightly-llmwiki-loop",
  "reason": "skip_if_running:in-flight",
  "tick_at": "2026-05-01T04:00:00Z"
}
```

Reason vocabulary:

| Reason | Meaning |
|---|---|
| `skip_if_running:in-flight` | A non-terminal job from this schedule is still running and `skip_if_running: true`. |
| `max_queue_depth:N` | Non-terminal job count for this schedule ≥ N. |

### `schedule.deleted`

Emitted by `Store.DeleteSchedule` when a schedule is removed via
`DELETE /v1/schedules/{name}`. Idempotent — deleting a non-existent schedule
is a no-op (no event written, warning logged).

```json
{
  "event_type": "schedule.deleted",
  "name": "nightly-llmwiki-loop",
  "deleted_at": "2026-05-01T20:15:00Z"
}
```

### Forward compatibility

Schedule events use the daemon-wide `LedgerSchemaVersion` (currently `1`). The
schedule vocabulary is **additive**:

- new event types may be added without bumping the schema version
- older daemons replaying newer ledgers skip unknown event types and continue
- `ListSchedules` derives state from `schedule.created` − `schedule.deleted`,
  so unknown intermediate events are tolerated

If a future schema introduces a non-additive change (renaming a payload field,
removing an event type), the version bumps and the migration becomes a hard
boundary rather than a skip-and-log.

## Executor Idempotency Contract

> ⚠️ **Status: experimental — stubs only.** The `llmwiki.loop` job type's stage handlers (Ingest, Query, Lint, Promote) ship as stubs in v1.0; they produce frontmatter-only output until the real-bodies follow-up lands (see `f-2026-05-01-011` in `.agents/findings/registry.jsonl`). For production schedules in v1.0, use `dream.run` (real-bodied via `overnight.RunLoop`) and `wiki.forge` (real-bodied via `wikiworker.Worker`).

`JobTypeLLMWikiLoop` (`llmwiki.loop`) materializes the Karpathy LLM-Wiki
pattern as a daemon job. The executor at
`cli/internal/llmwiki/executor.go` selects one of four stages per tick and
delegates to the per-stage handler in `cli/internal/llmwiki/stages.go`.

Three contracts are non-negotiable across all stages:

1. **SCOPE GUARD** — every write goes through `SafeAtomicWrite`; out-of-scope
   paths fail fast with `*WriteScopeError`.
2. **ATOMIC WRITE** — tmp file + fsync + rename so a partial write never
   leaves a corrupted artifact visible to readers.
3. **CTX PLUMBING** — handlers check `ctx.Err()` before each new write so
   cancellation aborts mid-batch without leaving torn state.

Per-stage idempotency:

| Stage | Idempotency strategy | Resume behavior |
|---|---|---|
| INGEST | Atomic write (tmp+fsync+rename); deterministic destination derived from source path. | Skip if existing artifact has valid frontmatter + matching `attempt`. |
| QUERY | Same atomic-write; deterministic slug from query hash (`QueryKey` field). | Skip if frontmatter `query_key` matches. |
| LINT | Date-keyed artifact (`YYYY-MM-DD`); overwrite is the contract. | Track `attempt` in frontmatter; re-run safely. |
| PROMOTE | Delegates to `harvest.Promote`. | Already idempotent post-TB-01 — skips destinations that already exist. |

### Crash recovery

If the daemon crashes mid-stage:

1. The job's lease expires; supervisor re-claims on next tick.
2. The handler reads the destination file's frontmatter on entry. Valid
   frontmatter + matching attempt → skip (already done). Missing or torn
   frontmatter → rewrite atomically.
3. Because writes are atomic (tmp+rename), there is no torn-frontmatter
   case at steady state — only "absent" or "complete".

The contract is locked in at the executor; the stage logic itself (NLP
quality, extraction heuristics) is intentionally a stub in the initial
landing and gets replaced by real forge / harvest / knowledge calls in
follow-up issues.

## Migration Recipe

The pre-daemon pattern wired pipeline jobs through a wrapper script
`~/ops/agentopsd-bridges/submit-with-window.sh` (psite-agu.13) driven by
`systemd --user` timers. The new pattern collapses both into a single
`schedule.yaml` consumed by `agentopsd`.

### Before (systemd timer + wrapper)

```ini
# ~/.config/systemd/user/gemma-entity-linker.service
[Service]
ExecStart=/usr/bin/env bash %h/ops/agentopsd-bridges/submit-with-window.sh \
    wiki.entity-extract --window 07-22 \
    --source %h/wiki/reviewed
```

```ini
# ~/.config/systemd/user/gemma-entity-linker.timer
[Timer]
OnCalendar=*:0/10
```

### After (`.agents/schedule.yaml`)

```yaml
schedules:
  - name: gemma-entity-linker-day-window
    cron: "*/10 7-22 * * *"   # every 10 min between 07:00 and 22:59
    job_type: wiki.forge
    timeout: 5m
    payload:
      tier: entity-extract
      source_pattern: wiki/reviewed/*.md
    backpressure:
      skip_if_running: true
      max_queue_depth: 5
```

```bash
# Register against a running daemon:
ao schedule add --file .agents/schedule.yaml --token-file ~/.agents/daemon-token

# OR boot the daemon with auto-detect:
ao daemon run --workers 4 --schedule-file .agents/schedule.yaml
```

### Migration checklist

- [ ] Inventory existing systemd timers wrapping `submit-with-window.sh`.
- [ ] Translate each `OnCalendar=` to a 5-field cron expression.
- [ ] Translate `--window HH-HH` arguments to the cron `hour` field.
- [ ] Move per-job arguments into `payload`.
- [ ] Add `backpressure` for any job that took the wrapper's "skip if a prior
      run is still active" semantics.
- [ ] `ao schedule add --file <path>` and verify `ao schedule list` shows the
      new schedules.
- [ ] Disable the systemd timers (`systemctl --user disable --now <unit>.timer`).
- [ ] Tail the ledger for `schedule.fired` events to confirm the cron cadence
      lands as expected.
- [ ] Remove the systemd unit files only after one full cron cycle has passed
      cleanly.

## Troubleshooting

### Schedule isn't firing

Run through the checklist:

1. **Is the schedule registered?**
   ```bash
   ao schedule list
   ```
   If empty, re-run `ao schedule add --file <path>` — adds are not auto-loaded
   on every daemon restart unless `--schedule-file` was passed at boot.

2. **Is the cron expression valid?**
   `ao schedule add` validates at parse time. The error names the offending
   field and preserves the original cron string. Six-field expressions with
   seconds are rejected (DoS protection).

3. **Is backpressure skipping the tick?**
   Tail the ledger for `schedule.skipped` events. Reasons:
   `skip_if_running:in-flight` (a prior job is still running) or
   `max_queue_depth:N` (queue depth at cap).

4. **Is the daemon running with schedules loaded?**
   ```bash
   ao daemon status
   ```
   Look for the schedule projection. Boot the daemon with `--schedule-file
   <path>` if `.agents/schedule.yaml` lives outside the cwd auto-detect path.

### Backpressure stuck — schedules keep skipping

```bash
# Check current queue depth and in-flight jobs:
ao daemon jobs list --status=running
ao daemon jobs list --status=queued

# Cancel a stuck job (token required):
ao daemon jobs cancel <job-id> --token-file ~/.agents/daemon-token
```

Recovery is **next-tick** — the supervisor does not wake on queue-drain
events. After cancelling stuck jobs, wait one cron period before declaring
the schedule unhealthy.

### Auth failure on `ao schedule add | run | remove`

```
forbidden: daemon mutation denied: mutation token mismatch
```

- Verify the header name. The canonical spelling is **`X-AgentOps-Daemon-Token`**.
  `X-Agentops-Mutation-Token` is a common drift point and is not accepted.
- Resolve the token via `--token <value>` or `--token-file <path>`. The
  token-file mode must be ≤ `0600`; permissive modes fail with
  `ErrUnsafeTokenFileMode`.
- Mutation routes are loopback-only by default. A request from a non-loopback
  remote address fails with `remote address is not local`. Tunnel via
  `ssh -L` or use a token with the appropriate scope.

### Daemon refuses to start with malformed schedule.yaml

```
schedule load failed at .agents/schedule.yaml: schedule: ... (daemon refuses
to start with malformed schedule.yaml; fix the file or remove --schedule-file)
```

This is **by design** (fail-closed). Options:

- Fix the YAML and re-run `ao daemon run --schedule-file <path>`.
- Validate without restarting the daemon: `ao schedule add --file <path>`
  uses the same parser.
- Temporarily skip schedule loading: `mv .agents/schedule.yaml
  .agents/schedule.yaml.disabled` (auto-detect now finds nothing) or omit
  `--schedule-file` on the next start.

### Schedule fired but no job materialized

Check the queue snapshot:

```bash
ao daemon jobs list --json | jq '.jobs[] | select(.payload.schedule_name == "<name>")'
```

If `schedule.fired` is in the ledger but no job appears in the queue,
the supervisor crashed between ledger append and queue submit. The next
tick re-tries with the same `submission_id` and the queue's idempotency
layer collapses the duplicate — wait one cron period and re-check.

### Cron expression rejected at parse time

Common mistakes:

- **Six-field expression** (`* * * * * *`) — rejected (DoS protection).
- **Sub-minute period** (`* * * * *` with operator-tightened
  `AGENTOPS_SCHEDULE_MIN_PERIOD_SECONDS`) — measured between two consecutive
  ticks; gap below floor fails.
- **Out-of-range field** (`60 * * * *`, `0 25 * * *`) — robfig/cron rejects
  with a per-field error.
- **Stray timezone descriptors** (`CRON_TZ=...`) — not supported in the
  shipped surface; all schedules run in the daemon's local time.

## Rollback

The scheduling surface is opt-in and additive. Rollback options:

### Disable scheduling at runtime

```bash
mv .agents/schedule.yaml .agents/schedule.yaml.disabled
# Restart agentopsd; the auto-detect path now finds nothing and the daemon
# starts without the recurrence supervisor active.
```

### Disable via flag

Omit `--schedule-file` on the next daemon start. Already-registered schedules
remain in the ledger but are not ticked because the supervisor only loads them
at boot. Use `ao schedule remove <name>` to drop them entirely.

### Disable a specific job type

Edit `.agents/schedule.yaml` to remove the offending entries (e.g., all
`llmwiki.loop` rows) and restart the daemon. Other job types are unaffected.

### Full rollback

Revert the merge commits in reverse order. Older daemon versions replaying
ledgers that contain `schedule.*` events skip-and-log unknown types — this is
why the event vocabulary is additive (see [Forward Compatibility](#forward-compatibility)).
The ledger remains valid; the schedule events are simply ignored.

## Validation Hooks

This contract is valid when it names:

- `.agents/schedule.yaml` as the operator-edited config file
- `RecurringJobTemplate` as the in-memory shape
- `schemas/schedule.schema.json` as the canonical schema
- `RecurrenceSupervisor` and `shouldFire` as the supervisor surface
- `submission_id` as the deterministic per-tick dedup key
- ledger-append-before-queue-submit as the write-order rule
- `X-AgentOps-Daemon-Token` as the canonical mutation header
- `registerMutationRoute` + `scripts/check-mutation-route-coverage.sh` as the
  mutation auth coverage pair
- `JobTypeLLMWikiLoop` as the new job-type constant
- the four ledger event types: `schedule.created`, `schedule.fired`,
  `schedule.skipped`, `schedule.deleted`

## Operator Runtime — launchd / systemd templates

These are templates; operator chooses platform/time. agentops repo doesn't
ship platform-specific service files — substrate-independence by design.
Copy + adapt to your environment.

> **Note (per amendment B4):** If you also run a separate nightly-evolution
> wrapper (e.g., from a systemd timer or cron — see `soc-b8jo` family of
> tooling), that's fine. Both can coexist: agentopsd handles cron-cadence
> durable jobs via the queue/ledger; your wrapper handles whatever it
> handled before. They share the same `.agents/` corpus and don't conflict.

### Mac — launchd

Save as `~/Library/LaunchAgents/<reverse-dns-prefix>.agentopsd.plist`.

**Replace `<reverse-dns-prefix>` with your own reverse-DNS identifier**
(e.g., `com.example`, `local.<username>`). Do NOT use `com.boshu2` — that's
the maintainer's namespace.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- BundleID — REPLACE <reverse-dns-prefix> with your own (e.g., com.example, local.myname) -->
    <key>Label</key>
    <string><reverse-dns-prefix>.agentopsd</string>

    <!-- Replace /path/to/repo with the absolute path of the repo this daemon serves -->
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/ao</string>
        <string>daemon</string>
        <string>run</string>
        <string>--workers</string>
        <string>2</string>
        <string>--schedule-file</string>
        <string>/path/to/repo/.agents/schedule.yaml</string>
    </array>

    <key>WorkingDirectory</key>
    <string>/path/to/repo</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <!-- Logs land in ~/Library/Logs/agentopsd/ — adjust as desired -->
    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/Library/Logs/agentopsd/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/Library/Logs/agentopsd/stderr.log</string>
</dict>
</plist>
```

Activate:

```bash
mkdir -p ~/Library/Logs/agentopsd
launchctl load ~/Library/LaunchAgents/<reverse-dns-prefix>.agentopsd.plist
launchctl start <reverse-dns-prefix>.agentopsd
```

### Linux — systemd-user

Save as `~/.config/systemd/user/agentopsd.service`.

```ini
[Unit]
Description=AgentOps daemon (continuous knowledge worker)
After=network.target

[Service]
Type=simple

# Replace /home/USER/path/to/repo with the absolute path of the repo
WorkingDirectory=/home/USER/path/to/repo

ExecStart=/usr/local/bin/ao daemon run --workers 2 --schedule-file /home/USER/path/to/repo/.agents/schedule.yaml

Restart=on-failure
RestartSec=10

# Logs go to journalctl --user -u agentopsd by default

[Install]
WantedBy=default.target
```

> **WorkingDirectory is REQUIRED** (per amendment A2). Without it,
> systemd-user units default to operator-undefined cwd, and `ao daemon`
> fails to find the schedule file silently.

Activate:

```bash
systemctl --user daemon-reload
systemctl --user enable --now agentopsd.service
systemctl --user status agentopsd
journalctl --user -u agentopsd -f   # tail logs
```

### Verifying it works

After activation, verify the daemon is running and serving the schedule:

```bash
ao daemon status                    # should show running + schedule loaded
ao schedule list                    # should show schedules from .agents/schedule.yaml
ao daemon events tail | head -20    # should show schedule.fired events on cron tick
```

## See Also

- [AgentOps Daemon Contract](agentops-daemon.md) — daemon ledger, job
  lifecycle, mutation auth, projection precedence.
- [JobSpec OpenAPI v0](jobspec-openapi-v0.yaml) — machine-readable job
  submission contract.
- [LLM-Wiki Skill](https://github.com/boshu2/agentops/blob/main/skills/llm-wiki/SKILL.md)
  — the Karpathy LLM-Wiki pattern that `llmwiki.loop` automates.
- `cli/internal/daemon/recurrence.go` — supervisor implementation + amendment A4
  contract comment.
- `cli/internal/schedule/parser.go` — strict YAML parser + env overrides.
- `cli/internal/llmwiki/executor.go`, `cli/internal/llmwiki/stages.go` — the
  `llmwiki.loop` executor and per-stage handlers.
- `cli/cmd/ao/schedule.go` — CLI subcommand implementations.
