# Daemon Factory Admission Runbook

This runbook exercises the daemon-native factory admission path without
automatic merge. It is the rehearsal lane for always-on factory work:
admission first, RPI handoff only after an allowed decision, manual PR landing
only.

## Inputs

Prepare a work order that conforms to
[`factory-admission.md`](../contracts/factory-admission.md):

```json
{
  "schema_version": 1,
  "work_order_id": "factory-work-example",
  "generated_at": "2026-05-04T23:30:00Z",
  "expires_at": "2026-05-05T00:30:00Z",
  "base_sha": "<current git sha>",
  "target": {
    "type": "bead",
    "id": "soc-example",
    "summary": "Bounded factory admission rehearsal"
  },
  "allowed_files": ["cli/internal/daemon/factory_admission_executor.go"],
  "validation_commands": ["cd cli && go test ./internal/daemon -run FactoryAdmission"],
  "landing_policy": "manual_pr",
  "digest_policy": "required",
  "open_pr_blockers": [],
  "main_ci_baseline": {
    "status": "green",
    "checked_at": "2026-05-04T23:29:00Z",
    "failed_jobs": []
  },
  "unknown_evidence_policy": "block"
}
```

Use `git rev-parse HEAD` for `base_sha`. If the repo is dirty, admission must
block.

## Submit Admission

For an admission-only decision:

```bash
ao factory admit --work-order @work-order.json --run-id factory-run-example
```

For a local pilot that may enqueue an RPI child job:

```bash
ao factory admit \
  --work-order @work-order.json \
  --run-id factory-run-example \
  --local-pilot \
  --rpi-handoff \
  --execution-packet .agents/rpi/execution-packet.json \
  --epic-id soc-example
```

The daemon executor policy controls the handoff:

- `fake` and `gascity` may enqueue the admitted `rpi.run` child job;
- `cli-fallback` may enqueue the admitted `rpi.run` child job and executes it
  through `scripts/ao-rpi-autonomous-cycle.sh` with `landing-policy=off`.

`cli-fallback` is still a manual rehearsal surface: it proves daemon-owned
admission and local execution wiring without enabling recurring host
scheduling, automatic merge, or default-branch pushes.

## Readback

Wait for the parent job, then inspect the factory projection:

```bash
ao daemon jobs wait <factory-job-id>
ao daemon status --json
```

Expected readback:

- parent job is terminal `completed`;
- blocked admission has `allowed=false` and at least one reason;
- allowed handoff has `child_job_id`;
- `projections.factory.admissions[]` contains the decision;
- artifact paths point under `.agents/daemon/factory/runs/<run_id>/`.

## Schedule Shape

Schedules can run the same path by materializing a `factory.local-pilot`
payload:

```yaml
schedules:
  - name: factory-local-pilot
    cron: "0 3 * * *"
    job_type: factory.local-pilot
    payload:
      work_order:
        schema_version: 1
        work_order_id: factory-work-example
        generated_at: "2026-05-04T23:30:00Z"
        expires_at: "2026-05-05T00:30:00Z"
        base_sha: "<current git sha>"
        target:
          type: bead
          id: soc-example
          summary: Bounded factory admission rehearsal
        allowed_files:
          - cli/internal/daemon/factory_admission_executor.go
        validation_commands:
          - cd cli && go test ./internal/daemon -run FactoryAdmission
        landing_policy: manual_pr
        digest_policy: required
        open_pr_blockers: []
        main_ci_baseline:
          status: green
          checked_at: "2026-05-04T23:29:00Z"
          failed_jobs: []
        unknown_evidence_policy: block
```

The recurrence layer fills `schema_version`, `job_type`, `run_id`, and `mode`
for the daemon job wrapper. It does not infer the work order.

## Stop Conditions

Stop and inspect before dispatching child work when:

- the admission reason includes `dirty_worktree`, `tracked_agents`, or
  `base_sha_mismatch`;
- open PR blocker count is non-zero;
- main CI status is `red` or unknown in mutating mode;
- artifact files are missing from `.agents/daemon/factory/runs/<run_id>/`;
- the child `rpi.run` appears without an allowed admission decision.
