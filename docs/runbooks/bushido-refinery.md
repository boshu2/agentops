# Bushido Refinery — continuous main-validation backstop

The refinery (`ao refinery`) is the **backstop** half of the push-to-main model
(ag-qidx). Push-to-main makes the local pre-push gate the pre-merge wall; the
refinery is the always-on net behind `main` on bushido.

## What it does

Every tick it checks `origin/main`. On a **new** commit it runs the full gate
(`ao gate check --full`). On a **blocking** failure it:

1. re-runs each failing check N times (default 3) to tell **deterministic** from
   **flaky** (the repo's 18–30% flake rate means naive escalation would be noise);
2. for deterministic failures only: writes a **poison beacon** (`.refinery-poison`
   + a `refinery` git note on the bad SHA) and files a **blocking fix-bead**
   (`BEADS_DIR="$(ao beads dir)" br create --labels refinery,blocking`);
3. on green: clears the beacon.

It **never reverts.** A poisoned commit stays on `main`; the team fixes forward.
Revert remains a human/quorum decision (ag-qidx P2.5, deferred to the quorum
infra `ag-k99u`).

## Backstop, not gatekeeper

If bushido (or its Wi-Fi) is down, the refinery is simply blind — **merges still
succeed**, nothing blocks. On restart it resumes from `.refinery-state`
(`last_checked_sha`) and catches up. This is the deliberate posture after the
2026-06-05 control-plane crash: no single host is in the merge path.

## Run it

```bash
ao refinery once               # one tick (manual / cron)
ao refinery run --interval 5m  # loop (the systemd service)
```

## Install on bushido

```bash
cp deploy/agentops-refinery.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now agentops-refinery
systemctl --user status agentops-refinery
journalctl --user -u agentops-refinery -f   # logs
```

State + beacon (repo-root, gitignored runtime):
+ `.refinery-state` — `{last_checked_sha, poison[]}`
+ `.refinery-poison` — present iff `main` is currently poisoned

## Tuning

+ `--interval` — poll cadence (default 5m).
+ Re-run count is 3 (deterministic = fails all 3). Raise for noisier suites.
