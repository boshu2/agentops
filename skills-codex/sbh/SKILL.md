---
name: sbh
description: Disk-pressure defense for AI coding
---
# SBH — Storage Ballast Helper

Prevents disk-full disasters via ballast files, artifact scanning, and predictive pressure monitoring. Three-pronged: ballast (instant space), scanner (stale artifacts), special locations (/tmp, /dev/shm, swap).

## Critical Constraints

- Start with `sbh --json status`, `sbh check`, or a dry-run; make read-only triage the default. **Why:** pressure evidence must identify the constrained mount before any mutation.
- Require explicit authorization before `clean --yes`, `emergency --yes`, ballast release, `tune --apply`, `unprotect`, or service/config changes. **Why:** these commands delete data or alter durable machine state.
- Preserve `.git/`, open-file, young-file, non-writable-parent, and `.sbh-protect` vetoes; keep ballast on the same mount as the pressure source. **Why:** freeing the wrong volume or deleting live work cannot recover the objective.
- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, apply the smallest safe recovery, then rerun status/check on the same mount. **Why:** pressure and failed checks are recovery evidence, not an andon by themselves.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. Hold write-heavy work and use one bounded local-shell helper for status, scan, or dry-run evidence—never another runtime by default. **Why:** one bounded pass can restore progress without masking a real capacity stop.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** only irreversible deletion judgment, unavailable authority, or exhausted recovery earns the human andon.

## Quick Check

```bash
sbh status                     # Pressure level + free space
sbh status --json | jq '.pressure'  # Machine-parseable
sbh check --need 5G            # "Do I have 5 GB free?"
sbh check --predict 30         # "Will I run out in 30 min?"
```

Exit codes: 0 = healthy, 1 = pressure, 2 = error.

---

## Daemon

```bash
sbh daemon                          # Foreground (debugging)
systemctl --user start sbh          # Systemd user scope
sbh install --systemd --user --auto # Install + start (Linux)
sbh install --launchd --auto        # Install + start (macOS)
sbh install --wizard                # Guided interactive setup
```

**Signals:** `SIGHUP` = reload config, `SIGUSR1` = force scan now, `SIGTERM` = graceful stop.

---

## Ballast

Pre-allocated sacrificial files — released in milliseconds, no scanning needed.

```bash
sbh ballast status             # Per-volume inventory
sbh ballast provision          # Create/rebuild pool
sbh ballast release 3          # Free 3 files NOW
sbh ballast replenish          # Rebuild after pressure passes
```

Defaults: 10 x 1 GiB = 10 GiB. Ensure ballast dir is on **same mount** as pressure source.

---

## Scanning & Cleanup

```bash
sbh scan /data/projects --top 20       # Rank artifacts by score
sbh clean /data/projects --dry-run     # Preview what would go
sbh clean --target-free 50G --yes      # Delete until 50 GB free
```

Scoring: Location (.25) + Name (.25) + Age (.20) + Size (.15) + Structure (.15) = 1.0.

---

## Protection

```bash
sbh protect /path              # .sbh-protect marker (subtree)
sbh unprotect /path            # Remove marker
```

Config globs: `scanner.protected_paths`. Hard vetoes (always enforced): `.git/` dirs, open files, age < 10 min, non-writable parents.

---

## Emergency Recovery

Zero-write mode for near-100% full disks. No config file needed.

```bash
sbh emergency /data --yes              # Aggressive cleanup NOW
sbh emergency --target-free 10G        # Stop at 10 GB recovered
```

---

## Observability

```bash
sbh dashboard                  # TUI: 7 screens (1-7 to jump)
sbh stats --window 24h         # Activity over last 24 hours
sbh blame --top 10             # Top 10 pressure sources
sbh explain --id <ID>          # Why was this decision made?
```

---

## Configuration

Config: `~/.config/sbh/config.toml` | Env: `SBH_` prefix | Fallback: `/etc/sbh/config.toml`

```bash
sbh config show                # Current values
sbh config validate            # Check constraints
sbh config set KEY VALUE       # Change a value
sbh tune --apply --yes         # Auto-tune for this system
```

---

## Anti-Patterns

| Don't | Do Instead |
|-------|------------|
| Ballast on `/tmp` | `paths.ballast_dir` on same mount as pressure source |
| Daemon as root, CLI as user | `--user` scope — avoids state file permission mismatch |
| Skip pre-build check | `sbh check --need 10G` in CI/hook |
| Delete `.sbh-protect` by hand | `sbh unprotect /path` |
| Wait for Red to act | Act at Yellow — agent swarms escalate fast |
| `min_file_age_minutes = 0` | Keep >= 5 to protect in-flight writes |

---

## Output Specification

**Artifact directory:** `.agents/evidence/sbh/<run-id>/` when a durable handoff is requested.
**Filename convention:** required `status-before.json`; optional `scan.json`, `clean-dry-run.json`, and `status-after.json` after an authorized action.
**Serialization/schema format:** raw SBH JSON; status artifacts require `command == "status"`, a nonempty `pressure.overall`, and a nonempty `pressure.mounts` array whose entries contain `path`, `level`, and numeric `free`.
**Validator command:** with `OUT=.agents/evidence/sbh/<run-id>`, run `jq -e '.command=="status" and (.pressure.overall|type=="string" and length>0) and (.pressure.mounts|type=="array" and length>0) and all(.pressure.mounts[]; (.path|type=="string" and length>0) and (.level|type=="string" and length>0) and (.free|type=="number"))' "$OUT/status-before.json"`.
**Downstream handoff:** pass the artifact directory, constrained mount, pressure level, selected action, authorization state, command exit codes, and next safe action to the consuming skill; compare `status-after.json` before declaring recovery.

## Quality Checklist

- [ ] Capture machine-readable status for the exact constrained mount before mutation.
- [ ] Verify dry-run candidates against protection vetoes and explicit authorization.
- [ ] Confirm ballast or reclaimed bytes affect the same mount that triggered pressure.
- [ ] Rerun status/check after recovery and preserve both before/after evidence.
- [ ] Consult the pawl on negative results; raise the andon only from a terminal state.

---

## Docs

Full documentation: https://github.com/Dicklesworthstone/storage_ballast_helper
