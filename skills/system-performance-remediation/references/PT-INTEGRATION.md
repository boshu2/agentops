# PT (Process Triage) Integration

PT provides Bayesian-assisted process triage for intelligent cleanup decisions.

## Overview

PT analyzes running processes and computes probability of abandonment using:
- Process age vs expected lifetime
- CPU/IO activity patterns
- TTY attachment state
- Parent process health
- Memory consumption
- Historical decisions (learns your preferences)

---

## Quick Start

```bash
# Interactive triage
pt

# Information only (no kill prompts)
pt scan

# Deep scan with IO monitoring
pt deep

# Agent-friendly JSON output
pt agent plan --format json

# Token-optimized output
pt agent plan --format toon --deep
```

---

## Four-State Classification

Every process is classified into one of four states:

| State | Description | Typical Action |
|-------|-------------|----------------|
| **Useful** | Actively doing productive work | Leave alone |
| **Useful-but-bad** | Running but misbehaving (stuck, leaking) | Throttle, review |
| **Abandoned** | Was useful, now forgotten | Kill (usually recoverable) |
| **Zombie** | Terminated but not reaped by parent | Clean up parent |

---

## Agent Integration

### THE EXACT PROMPT — Full Workflow

```bash
# 1. Generate plan
SESSION_ID=$(pt agent plan --format json | jq -r '.session_id')

# 2. Review candidates
pt agent plan --session $SESSION_ID --format json | jq '.candidates'

# 3. Execute (after review)
pt agent apply --session $SESSION_ID

# 4. Verify outcomes
pt agent verify --session $SESSION_ID
```

### Output Formats

| Format | Use Case |
|--------|----------|
| `--format json` | Full structured data |
| `--format toon` | Token-optimized, smaller context |
| (default) | Human-readable tables |

---

## Safety Gates

PT enforces safety limits for automated use:

| Gate | Default | Purpose |
|------|---------|---------|
| `min_posterior` | 0.95 | Minimum confidence for automation |
| `max_kills` | 10 | Per-session kill limit |
| `max_blast_radius` | 4GB | Maximum memory impact per session |
| `fdr_budget` | 0.05 | False Discovery Rate control |
| `protected_patterns` | [system services] | Never touched |

### Protected Processes

These are **never** flagged regardless of score:
- System services: `systemd`, `dbus`, `pulseaudio`, `pipewire`
- Infrastructure: `sshd`, `cron`, `docker`, `containerd`
- Databases: `postgres`, `mysql`, `redis`, `elasticsearch`
- Web servers: `nginx`, `apache`, `caddy`
- Root-owned processes (unless explicitly targeted)

---

## Evidence Sources

PT uses multiple signals:

| Evidence | What It Measures | Impact |
|----------|------------------|--------|
| Process type | Test runner? Dev server? Build tool? | Prior expectations |
| Age vs lifetime | Running longer than expected? | Overdue = suspicious |
| Parent PID | Orphaned (PPID=1)? | Orphaned = suspicious |
| CPU activity | Actively computing or idle? | Idle + old = abandoned |
| I/O activity | Recent file/network I/O? | No I/O for hours = suspicious |
| TTY state | Interactive or detached? | Detached + old = suspicious |
| Memory usage | Consuming significant RAM? | High memory + old = priority |
| Past decisions | User spared similar before? | Learns patterns |

---

## Confidence Levels

| Confidence | Posterior | Meaning |
|------------|-----------|---------|
| `very_high` | > 0.99 | Near certain, safe for automation |
| `high` | > 0.95 | High confidence, recommend action |
| `medium` | > 0.80 | Moderate confidence, review first |
| `low` | < 0.80 | Uncertain, more evidence needed |

---

## Blast Radius Assessment

Every candidate includes impact analysis:

```json
{
  "blast_radius": {
    "memory_mb": 1200,
    "cpu_pct": 98,
    "child_count": 3,
    "risk_level": "low",
    "summary": "Killing frees 1.2GB RAM, terminates 3 children; no external impact"
  }
}
```

High blast-radius actions require explicit confirmation.

---

## Identity Validation

PT prevents PID reuse attacks by validating identity before kill:

```
<boot_id>:<start_time_ticks>:<pid>
```

If the process died and PID was reused, the kill is blocked.

---

## Manual Quick Triage

When PT isn't available, use these heuristics:

### Find Obvious Candidates

```bash
# Old + idle = likely abandoned
ps -eo pid,etimes,pcpu,args --sort=-etimes | awk '$2 > 86400 && $3 < 1' | head -20

# Zombies
ps aux | awk '$8=="Z"'

# Orphans (PPID=1, non-system)
ps -eo pid,ppid,etimes,args | awk '$2==1 && $3 > 3600 && $4 !~ /systemd|init|docker/'
```

### Quick Score Calculation

```
abandoned_score = 0
+ 30 if age > 24 hours
+ 30 if CPU < 1%
+ 20 if orphaned (PPID=1)
+ 10 if no TTY
+ 10 if memory > 1GB

if abandoned_score > 60: likely safe to kill
```

---

## Configuration

### Directory Layout

```
~/.config/process_triage/
├── decisions.json     # Learned kill/spare decisions
├── priors.json        # Bayesian hyperparameters
├── policy.json        # Safety policy
└── triage.log         # Operation audit log

~/.local/share/process_triage/
└── sessions/          # Session artifacts
    └── pt-YYYYMMDD-HHMMSS-xxxx/
        ├── manifest.json
        ├── snapshot.json
        ├── plan.json
        └── audit.jsonl
```

### Policy Configuration

```json
{
  "protected_patterns": [
    "systemd", "sshd", "docker", "postgres"
  ],
  "min_process_age_seconds": 3600,
  "robot_mode": {
    "enabled": false,
    "min_posterior": 0.99,
    "max_kills_per_session": 5,
    "max_blast_radius_mb": 2048
  }
}
```

---

## Troubleshooting

### "pt-core not found"

```bash
# Reinstall
curl -fsSL https://raw.githubusercontent.com/Dicklesworthstone/process_triage/master/install.sh | bash
```

### "No candidates found"

This is often correct! Possible reasons:
- Minimum age threshold (default 1 hour)
- No matching suspicious patterns
- System is actually clean

Lower threshold for testing:
```bash
pt robot plan --min-age 60  # 1 minute
```

### Permission Errors

```bash
# If pt-core can't read /proc
sudo setcap cap_sys_ptrace=ep $(which pt-core)

# Or run with elevated privileges
sudo pt deep
```

---

## When to Use PT vs Manual Triage

| Situation | Use |
|-----------|-----|
| Regular maintenance | PT (automated, learned) |
| Emergency (load > 3x) | Manual (faster) |
| Learning system patterns | PT scan (no kill) |
| Need audit trail | PT (full logging) |
| One-off cleanup | Manual |

**Recommendation:** Use PT for regular maintenance, manual commands for emergencies.
