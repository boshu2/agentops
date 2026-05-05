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
| `ao overnight` | Private Dream/wiki knowledge compounding |
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

- Dream: run both `--runner claude --runner codex` when budget allows.
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

## Outputs

Each run writes:

- `digest.json`
- `digest.md`
- `ai-sane.json`
- `dream-setup.json`
- `runtime-inventory.tsv`
- `open-prs.json`
- optional `nightly-brief/`
- optional `systemd/`
- optional `dream.log` and `evolve.log`
