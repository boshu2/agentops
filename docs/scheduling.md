# Scheduling

> Operator guide for `ao schedule` and the recipes under `examples/schedules/`.

## What is `ao schedule`?

`ao schedule` is the scheduled-jobs interface for AgentOps. It manages
**recurring job templates** that the AgentOps daemon (`ao daemon run`) fires on
a cron tick. Each template names a job type (for example `dream.run`,
`wiki.forge`, `rpi.run`, `skill.invoke`), a cron expression, and a payload that
the daemon hands to the matching executor.

The daemon is what actually runs the work. `ao schedule` only registers,
lists, fires, and removes templates — it talks to a running daemon over HTTP.

```
your shell  ──ao schedule add/list/run/remove──▶  ao daemon run  ──fires on cron──▶  job executor
                                                       │
                                                       └──── artifacts to .agents/, events to projection
```

Start the daemon with a schedule file in one shot:

```sh
ao daemon run --workers 2 --schedule-file .agents/schedule.yaml
```

…or start it bare and add schedules from recipe files at any time:

```sh
ao daemon run --workers 2 &
ao schedule add --file examples/schedules/dream-nightly.yaml
```

The four subcommands:

| Command | Purpose |
|---|---|
| `ao schedule add --file <path>` | Register every schedule in a YAML file. POSTs `/v1/schedules` per template. Requires a mutation token. |
| `ao schedule list [--json]` | List active schedules. Read-only — no token needed. |
| `ao schedule run <name>` | Fire a registered schedule once, immediately. Useful for the `evolve-ondemand` manual recipe. |
| `ao schedule remove <name>` | Deregister a schedule. |

## Recipe catalog

Each recipe under `examples/schedules/` is a single-template YAML file ready
for `ao schedule add`. The starter `.agents/schedule.yaml` is a multi-template
file you can use as a base; the recipes below are smaller, single-purpose
files designed to be picked up à la carte.

### `examples/schedules/dream-nightly.yaml`

- **What it does:** Fires `dream.run` once per night. The dream executor walks
  the daemon-managed dream lifecycle (ingest → reduce → measure → commit →
  report) and writes artifacts under `.agents/overnight/`.
- **Why schedule it:** This is the "leave it running overnight" recipe. It
  compounds session evidence into durable knowledge while you sleep — the
  same workload you'd otherwise launch manually.
- **Cadence:** Nightly at 23:00 local. `skip_if_running: true` protects
  against overlap if a long iteration spills into the next tick.

### `examples/schedules/forge-daily.yaml`

- **What it does:** Fires `wiki.forge` against `.agents/sessions/`. The forge
  worker mines transcripts into structured learnings under
  `.agents/wiki/forge/`.
- **Why schedule it:** Daily forge captures the previous day's session
  evidence while it's still fresh, feeding compile/defrag downstream.
- **Cadence:** Daily at 22:00 local — shortly before `dream-nightly` so
  forged learnings are present when the dream pipeline starts.

### `examples/schedules/compile-weekly.yaml`

- **What it does:** Fires `skill.invoke` against the `compile` skill with
  `--full`. Runs the Mine → Grow → Compile → Lint pipeline against
  `.agents/`.
- **Why schedule it:** Weekly compile keeps the derived wiki coherent without
  burning daily compute. Pair with the nightly dream and daily forge so
  fresh learnings get woven into the wiki on a steady cadence.
- **Cadence:** Weekly, Sunday at 02:00 local. Runs immediately before
  `defrag-weekly`.

### `examples/schedules/defrag-weekly.yaml`

- **What it does:** Fires `skill.invoke` against the `compile` skill with
  `--defrag-only`. Runs `ao defrag --prune --dedup` over `.agents/` to remove
  stale entries and dedupe high-similarity clusters.
- **Why schedule it:** Weekly defrag keeps `.agents/` from drifting into
  bloat without re-mining.
- **Cadence:** Weekly, Sunday at 03:00 local — directly after
  `compile-weekly` so newly compiled entries get a chance to settle before
  pruning.

### `examples/schedules/feedback-drain-hourly.yaml`

- **What it does:** Drains pending operator feedback into the projection
  store on a tight cadence. (Companion recipe; scheduled hourly.)
- **Why schedule it:** Keeps feedback turnaround tight and lets the projection
  cache serve fresh evidence without operator nudging.
- **Cadence:** Hourly.

### `examples/schedules/evolve-ondemand.yaml`

- **What it does:** Registers an `rpi.run` template that you fire by hand on
  demand — typically when `ao goals measure` reports a regression or you
  want to kick off an autonomous fitness-scored improvement cycle.
- **Why schedule it:** Registering the template once lets you fire it through
  the same daemon surface (backpressure, idempotency, projection targets)
  used by the cron-driven recipes.
- **Cadence:** Manual fire only. The recipe carries an `@yearly` placeholder
  cron because the parser requires a valid expression; the expected
  invocation is:

  ```sh
  ao schedule run evolve-ondemand
  ```

### `examples/schedules/nightly-evolve.yaml`

- **What it does:** Fires a daemon-owned `rpi.run` every night with
  `gate_policy: required`, `landing_policy: off`, and `max_cycles: 1`.
- **Why schedule it:** This is the direct daemon schedule equivalent of the
  legacy host timer path that launched
  `scripts/nightly-evolution.sh --execute --run-evolve`. The wrapper already
  submits `rpi.run`; this recipe lets the daemon own the cadence directly.
- **Cadence:** Nightly at 04:30 local. `skip_if_running: true` prevents a long
  cycle from overlapping the next night.

### `examples/schedules/vault-four-tier.yaml`

- **What it does:** Registers the daemon-side vault maintenance pack:
  Tier 1 `wiki.forge` over session evidence, Tier 2 `skill.invoke` for
  `ao forge review --reviewer-model gemma2:9b`, and Tier 3 experimental
  `llmwiki.loop`. Placeholder-producing `llmwiki.loop` stages are skipped by
  default unless a test fixture opts in with `allow_placeholder_outputs=true`.
- **Why schedule it:** It replaces scattered operator-local timers with
  daemon-owned recurrence, backpressure, and ledger evidence.
- **Cadence:** Tier 1 at 22:00, Tier 2 at 01:30, Tier 3 at 02:00. Tier 4 stays
  manual and is intentionally not represented in the schedule pack.
- **Morai note:** The portable Tier 2 entry uses the existing `ao forge review`
  reviewer path. Operators with a validated Morai bridge can fork the recipe
  and replace that `skill.invoke` payload with their host-specific bridge
  command.

## Adding a custom schedule

A schedule is a YAML file with a `schedules:` list. Each entry needs a name, a
cron expression, a job type, and (for typed jobs) a payload that satisfies
that job type's spec.

Minimal example — fires the `flywheel` skill every Monday morning:

```yaml
# examples/schedules/flywheel-weekly.yaml
schedules:
  - name: flywheel-weekly
    cron: "0 6 * * 1"
    job_type: skill.invoke
    payload:
      skill_name: "flywheel"
      args: "--check"
    timeout: "15m"
    backpressure:
      skip_if_running: true
      max_queue_depth: 1
```

Register it:

```sh
ao schedule add --file examples/schedules/flywheel-weekly.yaml
```

Loader rules to be aware of (enforced by `cli/internal/schedule/parser.go`):

- `name` must be unique within the file.
- `cron` is the standard 5-field expression (with descriptors like `@daily`
  and `@yearly`); 6-field expressions with seconds are rejected to prevent
  sub-minute schedules.
- `job_type` must match `^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$` and resolve to a
  registered daemon job type (`dream.run`, `dream.stage`, `wiki.forge`,
  `wiki.build`, `rpi.run`, `rpi.phase`, `skill.invoke`, `eval.suite`,
  `eval.skill-delta`, `factory.admission`, `factory.local-pilot`,
  `openclaw.snapshot`, `plans.projection`, `llmwiki.loop`).
- `skill.invoke` executes the current `ao` binary with `skill_name` as the
  subcommand and `args` split into argv tokens. Use it for existing CLI-backed
  skill surfaces such as `compile`, `forge`, and `feedback-loop`; do not put
  shell pipelines or host-specific wrapper paths in `args`.
- `timeout` accepts Go duration strings (`30m`, `2h`, `4h30m`).
- `backpressure.max_queue_depth` is bounded (default ceiling 1000) by
  `AGENTOPS_SCHEDULE_MAX_QUEUE_DEPTH_CEILING`.
- The minimum effective cron period is 60s (override with
  `AGENTOPS_SCHEDULE_MIN_PERIOD_SECONDS`).
- Unknown YAML fields are rejected — the parser uses strict decoding.

## Inspecting runs

Once schedules are registered and the daemon is running, four surfaces show
what happened:

- **`ao schedule list`** — current registered templates, their crons, job
  types, and backpressure config.
- **`ao watch`** — live job event stream (accepted, claimed, completed,
  failed, lease expired). Best surface for "is the schedule actually
  firing?"
- **`.agents/overnight/`** — dream artifacts. Each `dream.run` writes a
  per-run subdirectory with summaries and stage manifests.
- **`.agents/wiki/forge/`** — wiki forge output. `wiki.forge` writes per-run
  output here.

For a one-shot debug fire that bypasses the cron, use:

```sh
ao schedule run <name>
```

It POSTs a single job with the schedule's payload and prints the assigned
`job_id` so you can correlate against `ao watch` output.
