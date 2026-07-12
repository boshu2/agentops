---
name: rch
description: Use when offloading slow builds to remote
---
# RCH — Remote Compilation Helper

`rch` transparently offloads compilation commands to remote workers via a Claude Code PreToolUse hook. The daemon picks the fastest healthy worker, rsync's the workspace, runs the build, syncs artifacts back, and exits with the worker's exit code.

This skill is the operational layer agents use when something about that pipeline isn't working — and the much more common case where it *thinks* it's working but is silently falling back to local execution. The skill is built around a single principle: **self-resolve before asking the human.** Every recovery path here is one the agent can run on its own.

## Constraints

- **Observe before mutation.** Capture the `[RCH]` summary, `rch doctor --json`, and the first failing triage stage before changing daemon, hook, worker, or disk state because an unobserved repair destroys the evidence needed to prove causality.
- **Fail-open is not success.** Treat every `[RCH] local (<reason>)` result as unresolved until the named reason is repaired or deliberately accepted because a green local build does not prove remote offload worked.
- **Bound autonomous repair.** Apply only the documented idempotent or precondition-guarded fixes in this skill, and preview destructive cleanup because remote workers may contain active builds or another agent's state.
- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED contract results repair and rerun automatically because ordinary rejection is diagnostic evidence; only a breaker may enter HOLD or consume the single helper lane.

## Breaker State Machine

- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the named defect, recapture evidence, and rerun the failed check; plain rejection never enters HOLD and never consumes the helper lane.
- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** when recovery cannot proceed safely with available permissions or observability, freeze mutations and route exactly one bounded helper consultation with the evidence packet.
- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, apply the bounded recovery, and re-earn the deterministic check plus pawl verdict before declaring the pipeline healthy.
- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and send the helper-provided concrete escalation packet to the operator.
- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** skip the helper and route directly to the operator; these are the only direct-human states.

**Don't re-learn the command surface here.** `rch --help`, `rch doctor`, and the machine surfaces (`--json`, `--schema`, `--help-json`, `--capabilities` — see [MACHINE_INTROSPECTION.md](references/MACHINE_INTROSPECTION.md)) self-describe every subcommand, flag, and env var. Env-var knobs and config precedence: [CONFIGURATION.md](references/CONFIGURATION.md). This skill carries the triage doctrine and recovery playbook routing.

Tested against rch v1.0.18; concepts apply to v1.0.16+.

---

## Read This First

When a build feels slow, run **one** thing:

```bash
RCH_VISIBILITY=verbose <your-command> 2>&1 | grep -E '^\[RCH\]'
```

The summary line is a contract:

| Pattern | What to do |
|---|---|
| `[RCH] remote <worker> (...)` | Healthy. Done. |
| `[RCH] remote <worker> failed [RCH-Exxx] ...` | Real build/env failure. See [ERROR_CODES.md](references/ERROR_CODES.md). |
| `[RCH] local (<reason>)` | **Fail-open.** See [FAIL_OPEN.md](references/FAIL_OPEN.md) and look up the reason verbatim. |
| *no `[RCH]` line at all* | Hook didn't fire. Run `scripts/protocol_test.sh "<your-command>"`. |

If you can't see why offload isn't happening, **prove the path works in isolation** before doing anything else:

```bash
rch exec -- env CARGO_TARGET_DIR="${TMPDIR:-/tmp}/rch_target_$(basename "$PWD")" cargo check --workspace --all-targets
```

If that prints `[RCH] remote <worker> (...)`, the offload pipeline is healthy. The problem is upstream of `rch exec` — usually the hook classifier or the agent's invocation form. If it also fails, follow [RECOVERY_PLAYBOOKS.md](references/RECOVERY_PLAYBOOKS.md).

---

## Fast Triage Order

Run in this order and stop at the first failing stage:

1. **Availability** — `rch check`, `rch status --workers --jobs`, `rch workers probe --all`, `rch queue`
2. **Config + socket consistency** — `rch config show --sources`, `rch --json config get general.socket_path`, `rch --json daemon status`
3. **Hook integration** — `rch hook status`, `rch agents status`, `rch hook install` (idempotent)
4. **Command classification + path closure** — `rch diagnose --dry-run "<your-command>"`
5. **Remote compile proof** — the `rch exec` probe above
6. **If sync fails or storage looks bad, inspect the worker directly:**

```bash
ssh ubuntu@<host> 'df -h / /tmp && free -h && cat /proc/pressure/memory && cat /proc/pressure/io'
ssh ubuntu@<host> 'du -sh /tmp/rch-* /tmp/rch_target_* 2>/dev/null | sort -h'
```

Always check both `/` and `/tmp` on the worker before deciding what to fix. End-to-end verify: `rch self-test --all`; comprehensive checks: `rch doctor` (`--fix --dry-run` previews auto-fixes).

**Checkpoint:** before the first mutation, record the failing stage, exact command, exit code, and relevant `[RCH]` summary in the run evidence; after repair, rerun that same probe so the before/after claim is falsifiable.

---

## Quick Fixes

| Symptom | Command |
|---------|---------|
| Hook not installed | `rch hook install && rch hook status` |
| Daemon not running | `rch daemon start` |
| Daemon version drift / stale socket state | `rch daemon restart -y` (drains gracefully — safe by default) |
| No workers configured | `rch workers discover --add --yes && rch workers setup --all` |
| Workers unreachable | `rch workers probe --all`, fix SSH key/host — or [SSH_KEY_RECOVERY.md](references/SSH_KEY_RECOVERY.md) |
| All workers busy + fail-open | Queueing is default-on; bump `RCH_DAEMON_WAIT_RESPONSE_TIMEOUT_SECS=120` or raise `total_slots` |
| Transfer churn under target dirs | Add excludes in `~/.config/rch/config.toml`, then `rch daemon reload` |
| Path dependency missing remotely | [PATH_DEPENDENCIES.md](references/PATH_DEPENDENCIES.md) (configurable via `[path_topology]`) |
| Sync fails `Permission denied` in `/data/projects/<repo>` | `ssh ubuntu@<host> 'sudo chown -R ubuntu:ubuntu /data/projects/<repo> && sudo chmod 775 /data/projects/<repo>'` |
| Worker disk pressure (RCH-E210/211/...) | [DISK_AND_PRESSURE.md](references/DISK_AND_PRESSURE.md) — hand off to the `sbh` skill |
| Telemetry / SpeedScore broken | [TELEMETRY_RECOVERY.md](references/TELEMETRY_RECOVERY.md) — move db aside, restart |
| Hook says installed but isn't intercepting | `scripts/protocol_test.sh "<your-command>"` |
| Multiple agents racing on fleet ops | Wrap with `scripts/multi_agent_safety.sh <cmd>` and use Agent Mail file reservations |
| Need full environment diagnosis | `rch doctor --json` and `rch config doctor` |

Debugging fail-opens: `RCH_VISIBILITY=verbose` shows the summary line; `RCH_LOG_LEVEL=debug` surfaces which fail-open path was taken. All other env knobs (priority, env allowlist, SSH keepalives, compression, profiles): `rch --help` + [CONFIGURATION.md](references/CONFIGURATION.md) + [SSH_TUNING.md](references/SSH_TUNING.md).

---

## Anti-Asking Rules

These are the questions agents historically ask the human that they should *just answer themselves*. The answer is in this skill or trivially derivable. **Do not ask. Do.**

- "Can I restart the daemon?" — Yes. `rch daemon restart -y` drains in-flight builds gracefully. It's the documented upgrade path.
- "Can I clean up `/tmp/rch_target_*`?" — If `sudo lsof +D <dir>` is empty, yes. See [DISK_AND_PRESSURE.md](references/DISK_AND_PRESSURE.md). If non-empty, never.
- "Should I fix the chown on the worker?" — If the symptom matches the Permission denied recipe, yes. It's documented.
- "Should I disable an unreachable worker and continue?" — Yes. `rch workers disable <id> --reason "..." --drain -y`, then proceed with what's healthy.
- "Should I reinstall the hook?" — If `rch hook status` says missing, yes. `rch hook install` is idempotent.
- "Should I sync the toolchain to the workers?" — If `RCH-E205` or "toolchain missing on X" appears, yes. `rch workers sync-toolchain --all`.
- "The cooldown is blocking my retry — should I delete it?" — No. Wait `auto_start_cooldown_secs`. If you really need to bypass, use `rch daemon start` directly (it's not gated by the hook autostart cooldown).
- "Can I drop the corrupt telemetry db?" — Yes. [TELEMETRY_RECOVERY.md](references/TELEMETRY_RECOVERY.md). Telemetry is derived data.
- "Should I recover SSH keys from a sibling host?" — If the keys are missing on this host but reachable on another, yes. [SSH_KEY_RECOVERY.md](references/SSH_KEY_RECOVERY.md) Step 3.

When in genuine doubt, capture the escalation packet (Playbook end of [RECOVERY_PLAYBOOKS.md](references/RECOVERY_PLAYBOOKS.md)) and surface that — not a wall of text — to the human.

---

## Output Specification

- **Path:** `.agents/evidence/remote-compilation/<run-id>/evidence.json` in the active repository, with raw command output stored beside it when needed.
- **Filename convention:** the machine handoff is always `evidence.json`; `<run-id>` is a filesystem-safe timestamp or task identifier unique to the recovery attempt.
- **Serialization/schema format:** JSON object `rch-evidence.v1` with nonempty `run_id`, `summary_line`, and `next_action`; enum `status` (`healthy|recovered|breaker`); enum `stage` (`availability|config|hook|classification|remote-compile|worker-pressure|complete`); nullable string `worker`; and a nonempty `commands` array of `{command:string,exit_code:number}` objects.
- **Validator command:** set `OUT=".agents/evidence/remote-compilation/<run-id>/evidence.json"`, then run `jq -e '. as $in | .schema_version=="rch-evidence.v1" and (.run_id|type=="string" and length>0) and (["healthy","recovered","breaker"]|index($in.status))!=null and (["availability","config","hook","classification","remote-compile","worker-pressure","complete"]|index($in.stage))!=null and ($in.summary_line|type=="string" and length>0) and (($in.worker==null) or ($in.worker|type=="string")) and ($in.next_action|type=="string" and length>0) and ($in.commands|type=="array" and length>0) and all($in.commands[]; (.command|type=="string" and length>0) and (.exit_code|type=="number"))' "$OUT"`.
- **Downstream handoff:** a `healthy|recovered` packet returns the verified worker and probe to the build lane; a `breaker` packet enters HOLD and accompanies the single helper, and only the explicit human states above reach the operator.

## Quality Checklist

- The evidence names the first failing stage and preserves the exact pre-repair command, exit code, and `[RCH]` summary instead of inferring success from build completion.
- Every mutation is documented, scoped to the diagnosed failure, and either idempotent or guarded by the playbook's safety precondition.
- The same probe is rerun after repair, and `rch self-test --all` or an isolated `rch exec` demonstrates remote execution rather than local fallback.
- Ordinary rejection remains in AUTO-REDO; HOLD has exactly one helper, and operator escalation is limited to the declared human states.

---

## Reference Index

Everything below ships in the skill. Read whichever is relevant.

**Recognising what's wrong:**
- [FAIL_OPEN.md](references/FAIL_OPEN.md) — every `[RCH] local (...)` reason mapped to a self-fix
- [ERROR_CODES.md](references/ERROR_CODES.md) — full RCH-Exxx catalog with skill-doc cross-refs
- [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) — diagnostic flow + common errors

**Solving specific failure classes:**
- [RECOVERY_PLAYBOOKS.md](references/RECOVERY_PLAYBOOKS.md) — symptom → fix in ≤90s, organized as 12 lettered playbooks
- [SSH_KEY_RECOVERY.md](references/SSH_KEY_RECOVERY.md) — when workers.toml references keys this host doesn't have
- [PATH_DEPENDENCIES.md](references/PATH_DEPENDENCIES.md) — multi-repo workspaces, closure planner, `[path_topology]`
- [DISK_AND_PRESSURE.md](references/DISK_AND_PRESSURE.md) — RCH-E210..217 + the `sbh` handoff
- [TELEMETRY_RECOVERY.md](references/TELEMETRY_RECOVERY.md) — corrupt `~/.local/share/rch/telemetry/telemetry.db`
- [SELF_HEALING.md](references/SELF_HEALING.md) — autostart cooldown, daemon supervision, `[self_healing]`
- [SSH_TUNING.md](references/SSH_TUNING.md) — ControlMaster, keepalives, retry classification

**Operating in fleets and swarms:**
- [MULTI_AGENT_CONTENTION.md](references/MULTI_AGENT_CONTENTION.md) — TOCTOU, fleet deploy races, autostart cooldown sharing
- [OPERATIONS.md](references/OPERATIONS.md) — full runbook + worker fleet lifecycle
- [WORKERS.md](references/WORKERS.md) — worker config, drain/disable/enable, deploy
- [CONFIGURATION.md](references/CONFIGURATION.md) — config precedence, env vars, runtime paths
- [HOOKS.md](references/HOOKS.md) — hook protocol, install, test
- [MACHINE_INTROSPECTION.md](references/MACHINE_INTROSPECTION.md) — `--json`, `--schema`, `--help-json`, `--capabilities`

**Automation scripts (in `scripts/`):**
- `auto_recover.sh` — heuristic, dry-run-by-default fleet recovery
- `worker_disk_triage.sh` — read-only mount-aware disk report per worker
- `protocol_test.sh` — directly probe the hook protocol with synthetic input
- `multi_agent_safety.sh` — flock wrapper for fleet/setup operations
- `mine_rch_history.sh` — find prior agent sessions that hit a given failure
- `diagnose-rch.sh` — comprehensive end-to-end diagnostic (the original)

**Templates and project docs:**
- `assets/workers-template.toml`
- Source: <https://github.com/Dicklesworthstone/remote_compilation_helper>

---

## Adjacent Skills

- **`sbh`** — disk-pressure defense for AI coding workloads. Use when `RCH-E210/211/215/216` fires.
- **`agent-mail`** — file reservations and messaging between agents. Use before `rch fleet deploy` or any worker config edit in a swarm.
- **`agent-native`** / **`ntm`** — portable worker lifecycle and NTM mechanics for agents that hit rch failures.
- **`cass`** — search prior agent sessions; the skill ships `scripts/mine_rch_history.sh` as a fallback when cass index has dead pointers.

---

## Reading Output: TUI vs Hook

`rch` itself, when invoked with **no subcommand**, runs in PreToolUse hook mode (reads JSON from stdin, writes JSON to stdout). Don't run bare `rch` from a terminal expecting help — use `rch --help`. Bare TUIs are at `rch dashboard` (terminal) and `rch web` (browser); both block your session.
