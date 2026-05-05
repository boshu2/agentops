# Nightly Evolution Runbook

This runbook describes the private local nightly automation lane. It is separate
from GitHub Actions.

## Architecture

| Surface | Role |
|---------|------|
| GitHub Nightly | Public proof harness over repo-visible state |
| Nightly RPI Brief | Evidence packet and prompt issue |
| Bushido scheduler | Local execution host for private runs |
| `scripts/nightly-evolution.sh` | Repo-owned run contract and digest writer |
| `ao daemon jobs submit --type dream.run` | Private Dream/wiki knowledge compounding handoff |
| `ao rpi` / `ao evolve` | Code-mutating implementation cycles |
| Claude Code | Headless worker/reviewer via local CLI or GitHub companion action |
| Codex | Headless worker/reviewer via `codex exec` or local AgentOps runtime |
| Mt. Olympus / Gas City | Future durable orchestration backend |

Bushido is a private dogfood target, not a public AgentOps namespace. The first
production scheduler is a host user timer or cron entry that calls the repo
script. Mt. Olympus can later run the same contract through Gas City once
provider readiness and replay are proven.

## First Safe Run

Preview the plan and write digest artifacts:

```bash
scripts/nightly-evolution.sh --emit-systemd
```

Run only the private Dream/wiki lane:

```bash
scripts/nightly-evolution.sh --execute --run-dream
```

This submits a typed `dream.run` daemon job. If daemon submission fails, the
wrapper falls back to the legacy `ao overnight start` subprocess for operator
compatibility unless `--skip-dream-subprocess` is supplied.

Run one bounded evolve cycle with Codex as the runtime command:

```bash
scripts/nightly-evolution.sh \
  --execute \
  --run-evolve \
  --runtime-cmd codex \
  --runtime-mode direct \
  --max-cycles 1
```

Run both lanes after the dry-run and Dream-only pilot have passed:

```bash
scripts/nightly-evolution.sh --execute --run-dream --run-evolve --max-cycles 1
```

## Scheduling

### Automated Install

Use the install helper to generate, install, and enable the systemd user timer:

```bash
# Preview what will be installed
scripts/install-nightly-scheduler.sh --dry-run

# Install in dry-run mode (safe default — no source mutation)
scripts/install-nightly-scheduler.sh --enable

# Install in execute mode (runs Dream + Evolve nightly)
scripts/install-nightly-scheduler.sh --execute-mode --enable

# Check status
scripts/install-nightly-scheduler.sh --status

# Remove
scripts/install-nightly-scheduler.sh --uninstall
```

Options: `--schedule`, `--runners`, `--runtime-cmd`, `--max-cycles`. See
`scripts/install-nightly-scheduler.sh --help` for full reference.

### Manual Install (alternative)

Generate systemd user timer templates:

```bash
scripts/nightly-evolution.sh --emit-systemd
```

The generated files are written under the run output directory:

- `systemd/agentops-nightly-evolution.service`
- `systemd/agentops-nightly-evolution.timer`

Install manually after reviewing them:

```bash
mkdir -p ~/.config/systemd/user
cp .agents/nightly/<date>/<run-id>/systemd/agentops-nightly-evolution.* ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now agentops-nightly-evolution.timer
```

### Schedule Design

- Default: `*-*-* 12:15:00 UTC` (daily, after GitHub Nightly settles)
- `RandomizedDelaySec=10m` prevents thundering herd if multiple repos are scheduled
- `Persistent=true` catches up on missed runs after sleep/reboot
- Timer is under `timers.target`, NOT `pipeline.target` (agentops-specific, not pipeline infra)
- Kill switches (`STOP`, `KILL` files) are checked both via systemd `ConditionPathExists`
  and in the `ExecStartPre` for defense in depth
- `TimeoutStartSec=3600` (1h) prevents runaway evolve cycles from blocking the timer

## Vendor Policy

Use Claude and Codex differently until eval evidence says mixed mode is stable:

- Dream daemon handoff: submit `dream.run` through `agentopsd`. The legacy
  subprocess fallback still honors the configured Claude/Codex runner list.
- Planning/review: prefer Claude or mixed mode for synthesis-heavy work.
- Implementation: use Codex when local shell/code execution is the primary
  burden.
- CI/GitHub companion: use Claude Code GitHub Actions for repo-visible reviews
  or reports, not private `.agents` mutation.
- Gas City: use `runtime=gc` only after provider readiness and replay are
  proven.

## Safety Controls

- Dry-run is the default.
- `--execute` is required before any phase runs.
- `--run-dream` and `--run-evolve` are separate opt-ins.
- A lock directory prevents overlapping runs.
- These kill switches stop the wrapper:
  - `.agents/evolve/STOP`
  - `.agents/rpi/KILL`
  - `~/.config/evolve/KILL`
- `bushido-box ai-sane` must pass in execute mode unless
  `--no-require-ai-sane` is supplied.

## L3 Rehearsal — First No-Merge Local Pilot

Before enabling recurring scheduled runs, prove the full execute+evolve
path with a mocked work order that cannot merge.

### Step 1: Dry-run

```bash
scripts/nightly-evolution.sh
```

Verify `digest.json` is written and `admission_context` is populated.

### Step 2: Dream-only execute

```bash
scripts/nightly-evolution.sh --execute --run-dream
```

Verify Dream submits (or falls back) without requiring a work order.

### Step 3: Execute+evolve blocked by admission

Create an expired work order to confirm the preflight refuses:

```bash
jq -n '{
  schema_version: 1, work_order_id: "rehearsal-blocked",
  generated_at: "2020-01-01T00:00:00Z", expires_at: "2020-01-01T01:00:00Z",
  base_sha: "abcdef1", target: {type:"goal",id:"t",summary:"T"},
  allowed_files: ["scripts/nightly-evolution.sh"],
  validation_commands: ["echo ok"], landing_policy: "off",
  digest_policy: "required", open_pr_blockers: [],
  main_ci_baseline: {status:"green",checked_at:"2020-01-01T00:00:00Z",failed_jobs:[]}
}' > /tmp/rehearsal-blocked.json

scripts/nightly-evolution.sh \
  --execute --run-evolve \
  --work-order /tmp/rehearsal-blocked.json
# Expected: non-zero exit, "work order expired"
```

### Step 4: Execute+evolve admitted (no-merge)

Create a valid work order and run with `--landing-policy off`:

```bash
jq -n --arg sha "$(git rev-parse HEAD)" \
  --arg ea "$(date -u -d '+1 hour' +%Y-%m-%dT%H:%M:%SZ)" \
  --arg ga "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{
  schema_version: 1, work_order_id: "rehearsal-admitted",
  generated_at: $ga, expires_at: $ea, base_sha: $sha,
  target: {type:"goal",id:"rehearsal",summary:"L3 rehearsal"},
  allowed_files: ["scripts/nightly-evolution.sh"],
  validation_commands: ["echo ok"], landing_policy: "off",
  digest_policy: "required", open_pr_blockers: [],
  main_ci_baseline: {status:"green",checked_at:$ga,failed_jobs:[]}
}' > /tmp/rehearsal-admitted.json

scripts/nightly-evolution.sh \
  --execute --run-evolve \
  --work-order /tmp/rehearsal-admitted.json \
  --landing-policy off \
  --runtime-cmd claude \
  --max-cycles 1
# Expected: exit 0, digest records admitted run
```

### Step 5: Review digest

```bash
cat .agents/nightly/$(date -u +%F)/*/digest.md
cat .agents/nightly/$(date -u +%F)/*/pr-body.md
```

Verify the digest includes the admission context table, stop reasons (step
3), and admitted verdict (step 4).

The pilot is not recurring until this rehearsal passes all five steps.

### BATS validation

Run the full scenario suite:

```bash
bats tests/scripts/nightly-evolution.bats
```

Scenario fixtures for the blocked and admitted rehearsals are at
`tests/scenarios/nightly-evolution/auto-nightly-evolution-l3-rehearsal-*.json`.

## Outputs

Each run writes:

- `digest.json`
- `digest.md`
- `ai-sane.json`
- `dream-setup.json`
- `runtime-inventory.tsv`
- `open-prs.json`
- `blocker-matrix.json`
- `main-ci-baseline.json`
- optional `nightly-brief/`
- optional `systemd/`
- optional `dream-run-payload.json`, `dream-submit.json`, and
  `dream-submit.stderr`
- optional `dream.log` when legacy Dream subprocess fallback is used
- optional `evolve.log`
