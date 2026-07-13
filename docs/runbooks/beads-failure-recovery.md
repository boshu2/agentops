# Runbook: beads (bd) failure recovery

> **RETIRED / HISTORICAL (as of 2026-06-11).** bd/Dolt is retired; the live tracker is `br` at the private ledger resolved by `ao beads dir` (invoke `BEADS_DIR="$(ao beads dir)" br <cmd>`), and the ledger syncs via `git -C "$(ao beads dir)" push` — there is no remote Dolt server to recover. This runbook documents bd/Dolt-server failure recovery and is kept for historical reference only — see AGENTS.md and `docs/runbooks/bd-server-mode-closeout.md`.

> **Bead:** cp-4jac (control-plane). **Scope:** what an agent or operator does when `bd`/beads
> operations fail mid-work in this repo — including the one audited way past a blocked push.
> Failure taxonomy borrowed from control-plane's 4-day post-mortem family (cp-7cko) and the
> divergent-board recovery work (cp-cwiy, `bin/br-reconcile`).

## Why this exists (the hard pre-push dependency)

agentops tracks issues in **bd** (Dolt server mode — `.beads/metadata.json` points at the shared
`bushido` Dolt DB on the tailnet). The cockpit pre-push gate is bd-coupled in two places:

- **check 19b — "bd closeout contract parity"** (`scripts/pre-push-gate.sh`): blocking when it runs.
- **loop-shape check** (Directive 12 posture): warn-only, skips cleanly when bd is missing.

So a corrupted or unreachable bd can block a push that has nothing wrong with the code. A blocked
push must have a documented exit — that exit is below, and it is **audited, not silent**.

## The audited bypass (the only sanctioned exit)

```bash
AGENTOPS_GATE_DISABLED=1 git push
```

This is implemented in `scripts/hooks/pre-push.local`: the bypass is **logged** (timestamp, user,
branch, sha) to `$(git rev-parse --git-common-dir)/agentops-gate-bypass.log` and prints a warning.
Rules:

1. Use it only when the gate failure is caused by infrastructure (bd down, Dolt unreachable), not
   by your change. If `go build` or a real check is red, fix the change.
2. After bypassing, **file or update a bead** describing why, and re-run the gate
   (`scripts/pre-push-gate.sh --fast` or `ao gate check --fast`) once bd is healthy.
3. Never `git push --no-verify` and never uninstall the hook — both are unaudited and defeat the
   mechanism (LAW-3: don't let the gate be self-greened).

## Triage: is bd actually broken?

```bash
bd ping            # connectivity to the Dolt server (fast)
bd context         # which backend/server/database actually resolved
bd doctor          # installation + config health, with repair hints
nc -zv 100.105.194.61 3306         # is the bushido Dolt server reachable at all?
ssh bushido systemctl --user status dolt-bd-server   # is the server up?
```

If `bd ping` is green and `bd context` resolves to the expected server/database, bd is fine — your
problem is elsewhere (look at the actual gate output).

## Failure modes → detection → recovery → prevention

### FM1 — Dolt server unreachable (tailnet down, bushido offline)

- **Symptom:** `bd` commands hang or fail with connection errors; pre-push 19b fails.
- **Detect:** `bd ping` fails; `nc -zv 100.105.194.61 3306` times out.
- **Recover:**
  1. Check the tailnet/host: `ssh bushido systemctl --user status dolt-bd-server`, restart if down.
  2. If the host is genuinely unreachable and the push cannot wait: use the audited bypass above,
     then reconcile when the server returns.
- **Prevent:** don't start a push-heavy session without a green `bd ping`; the server is a single
  point of failure by design (single authoritative writer — see control-plane cp-7e3j).

### FM2 — Divergent board (replicas disagree)

- **Symptom:** bead counts/states differ between checkouts or between the server and a local
  export; beads you closed show open elsewhere; duplicate or missing IDs.
- **Detect:** `bd export` from two vantage points and diff; or compare `bd show <id>` against what
  your session believes it did.
- **Recover:** **union + lifecycle-aware rebuild** — the procedure operationalized in control-plane
  as `bin/br-reconcile` (cp-cwiy): union all bead IDs across replicas, resolve conflicts
  lifecycle-aware (**a closed-with-evidence bead is NEVER lost to a later bare "open" touch** — this
  defeats the reopen-eater), keep replica-only beads, then rebuild from the authoritative union.
  In this repo's Dolt mode the equivalent is: `bd export` each divergent replica, build the union
  with the same lifecycle rule, then `bd import` the union into the authoritative server. Do it on
  a quiet board (no concurrent writers).
- **Prevent:** one authoritative writer; never hand-edit ledger files; don't run two sessions
  writing the same beads without locks (Agent Mail reservations).

### FM3 — Orphaned beads created from the wrong branch/worktree

- **Symptom:** a bead created or closed during branch work is missing after merge — the record
  lives in state that never reached the shared board (control-plane lesson: "beads live on the
  branch, not the stale checkout").
- **Detect:** after landing a branch, `bd show <id>` for every bead the session claims to have
  created/closed. Anything missing is orphaned.
- **Recover:** re-create or re-close the bead against the live server from the canonical checkout,
  citing the original work (commit shas) in the bead notes. If a JSONL export from the branch
  exists, `bd import` the missing lines.
- **Prevent:** in Dolt server mode writes go to the server, not the branch — but **verify** at
  closeout time (`docs/agent-workflow-reference.md`): landing/push AND bead state
  confirmed on the server are both required before a session ends.

### FM4 — Merge-eaten closes (the reopen-eater)

- **Symptom:** a bead that was closed with evidence is open again after a sync/merge; the close
  note is gone or the status regressed.
- **Detect:** audit recently-closed beads after any sync/reconcile: closed beads whose `updated`
  timestamp moved but whose status regressed to open without a human reopen note.
- **Recover:** re-close with a pointer to the original evidence (commit, evidence file). If many
  beads are affected, treat it as FM2 and run the lifecycle-aware union — the union rule exists
  precisely so closed-with-evidence beats a later bare open touch (cp-cwiy).
- **Prevent:** never resolve a bead-state conflict by "latest timestamp wins"; lifecycle-aware
  resolution only. Don't hand-edit `.beads/` ledger files (control-plane is adding dcg deny rules
  for exactly this — cp-06xi).

### FM5 — Local lock/cache contention or corruption

- **Symptom:** `bd` reports busy/locked; stale `.beads/.write.lock`; weird local-cache state while
  the server is healthy.
- **Detect:** `bd doctor`; `bd ping` green but local commands fail.
- **Recover:** retry first — "busy" is usually daemon lock contention, not corruption (do not
  blindly delete locks). If genuinely stale (no live bd process), remove the stale lock and re-run
  `bd doctor`. Local caches can be rebuilt from the server; the server is the source of truth.
- **Prevent:** one writer per workspace; let commands finish; don't `kill -9` mid-write.

## Escalation

If recovery would require destructive action on the shared Dolt DB (dropping tables, rewriting
history), stop and escalate to the operator — that is a one-way door, and the board serves every
repo wired to the `bushido` DB, not just this one.

## See also

- `scripts/hooks/pre-push.local` — the gate hook, the audited bypass, the bash-gate escape hatch.
- [`bash-gate-sunset.md`](bash-gate-sunset.md) — when the legacy bash gate (and its
  `AGENTOPS_GATE_BASH=1` hatch) gets deleted.
- control-plane `cp-cwiy` (br-reconcile), `cp-7cko` (SSOT self-correction family) — the failure
  taxonomy this runbook instantiates.
- `docs/runbooks/bd-server-mode-closeout.md` — server-mode setup detail.
