---
name: using-gc
description: 'Drive a caller-selected Gas City through its Mayor dispatch shepherd; keep GC runtime state out of AgentOps verdicts. Triggers: "using gc", "gas city", "drive the mayor", "dispatch through gc".'
practices: [team-topologies, design-by-contract]
hexagonal_role: driving-adapter
consumes: [explicit-packets]
produces: [gas-city-runtime-evidence]
context_rel:
- kind: partnership
  with: agent-native
skill_api_version: 1
user-invocable: true
metadata:
  tier: execution
  dependencies: []
  capabilities: [dispatch_explicit_packet, observe_gc_runtime, drive_mayor_shepherd]
  effects: [operate_gas_city, configure_codex_trust]
  canonical_status: canonical
  disposition: keep_optional_adapter
output_contract: runtime evidence per supplied packet
---

# Using GC

Use Gas City only when the caller explicitly selects it. Treat it as a
replaceable execution adapter, not a completion or correctness boundary. This
skill teaches an agent to ORCHESTRATE the Mayor, which in turn propels the city —
the same standing session a human drives, steered through native primitives.

## The model: one shepherd, two doors

The city has a standing city-scoped **Mayor** session. It is a DISPATCH
SHEPHERD: it watches ready rig step beads and slings each to its run-target with
a nudge, which spawns that worker to claim the rig-scoped bead. Workers claim;
**the Mayor never claims and never authors work.** You reach the one Mayor
session through two doors:

- **Human door:** `invoke.sh --city C mayor status` prints a `tmux -L <socket>
  attach -t <session>` line; the human attaches and drives interactively.
- **Agent door:** `invoke.sh --city C mayor tell "dispatch <bead-id>"` delivers a
  notified mail message. No keystroke injection — GC ships mail/sling as
  first-class control, so an agent steers the resident session the way a human
  would, NTM-style.

One-line why: on GC v1.3.5 demand-spawn is broken for rig work (#4586), so a
shepherd that sling-nudges ready steps is the propulsion path — and it is also
the stock GC mayor pattern, so this flow survives the upstream fix.

## The drive loop (for an orchestrating agent)

1. **Author intent — caller-owned.** `invoke.sh --city C create "<title>" -d
   "<why/how>"` writes one source bead with EXACT acceptance; `invoke.sh --city C
   feed <bead-id>` homes it and attaches the native formula. Intent lives in the
   bead, never in a chat paraphrase.
2. **Dispatch by id (on-demand).** `invoke.sh --city C mayor tell "dispatch
   <bead-id>"`. Hand the Mayor BEAD IDS ONLY — never prose work. A paraphrased
   task is a telephone game that drifts from the acceptance the bead already
   carries; the id is the one unambiguous reference. Steady-state propulsion is
   the scheduled heartbeat (the Mayor runs a dispatch pass every few minutes);
   `mayor tell` is for on-demand nudges. Dispatch each bead once — for a bead
   already routed, see Stall protocol (re-dispatch is a no-op).
3. **Read state from GC, not from prose.** The exact surfaces:
   - `invoke.sh --city C mayor status` — Mayor session state + attach line.
   - `invoke.sh --city C status` — city/session health.
   - `gc bd --rig <N> ready --json` / `gc bd --rig <N> show <id> --json` — what is
     ready and each step's `gc.run_target`.
4. **Completion is bead/verdict state, never pane prose.** A step is done when the
   bead graph and the fresh validate verdict say so — not because a pane printed
   "done".

## Liveness truth stack (GC edition)

Robot/session state can report a session **active** while the provider pane is
wedged on an interactive prompt doing nothing. Trust ground truth:

- `gc session list --json` / `mayor status` is the roster claim.
- `tmux -L <socket> capture-pane -pt <session>` is ground truth — read the pane.
  The `<socket>` and `<session>` here (and at every `tmux -L` site below) are the
  ones `mayor status` prints in its `tmux -L <socket> attach -t <session>` line.
- Two known codex wedge classes and their durable fixes:
  - **Update nag:** codex blocks on an "update available" prompt. Fix: update
    codex so no pending-update prompt exists before the run.
  - **Folder trust:** codex blocks asking to trust the working directory. Fix:
    add exact-path `trust_level` entries for the rig and worktree-root in
    `~/.codex/config.toml` (bootstrap does not write them yet). This edits the
    user's global codex config — a host-configuration mutation; make it only
    with explicit caller authorization, scoped to those exact paths.

When the roster says active but the pane is wedged, the pane wins.

## Stall protocol

First classify what is stalled — the fix differs, and the obvious retry is a
dead path.

- **A bead that is still `ready` (never routed).** `mayor tell "dispatch <id>"`
  once to route it, then STOP and classify.
- **A bead already routed to its run-target (`in_progress`).** Re-telling
  `dispatch <id>` or re-slinging it is a **NO-OP** — do not cargo-cult it. `gc
  sling` takes an idempotent early-return for an already-routed bead and sends NO
  nudge, and an `in_progress` bead is not in `gc bd ready`, so nothing re-fires.
  The recovery is to wake the WORKER that holds it, not to re-dispatch the bead:

  ```sh
  gc session wake <run_target>    # the rig-qualified worker alias, e.g.
                                  # <rig>/agentops.implementer, from gc.run_target
  ```

  Then inspect pane-truth (below). One wake, maximum, then STOP and classify.

Ground-truth every stall before you report it: capture the pane (`tmux -L
<socket> capture-pane -pt <session>`) and run `invoke.sh --city C doctor`. No
hidden retries, no lifecycle bypass. **Never repair the city from inside the
city** — diagnose from the invoke surface and hand the finding out.

## Visibility: the four layers

Every canary stall was invisible to at least one layer and visible in another.
Cycle all four; each lies in its own way.

- **Layer 1 — Robot state.** `gc session list` / `invoke.sh --city C status` —
  reports "active" even when the provider pane is wedged on a prompt or the
  network is dead. Lifecycle shape only, never proof of thinking.
- **Layer 2 — Bead graph.** `gc bd --rig <rig> ready` / `gc bd --rig <rig> show
  <id>` (workflow/step statuses) — the only completion truth; but a claimed step
  with a wedged worker looks identical to a working one from here.
- **Layer 3 — Pane truth.** `tmux -L <socket> capture-pane -t <session> -p` —
  ground truth for wedges (update nags, trust prompts, API/DNS failures print
  here first). It is a snapshot, not history.
- **Layer 4 — Health machinery.** `gc doctor`, `gc order history <order>` (e.g.
  shepherd-heartbeat) — proves the city's metabolism (orders firing, stores
  resolving), not whether any specific work is progressing.

Native `gc dashboard` aggregates layers 1-2; the metrics/logs plane (OTel to
local collectors, Grafana) is the 3.3.1 roadmap layer and not required for
operating a city today. When layers disagree, trust the LOWER layer (pane over
robot state) and run the Stall protocol.

## Boundaries (kept)

- GC state stays in GC. GC quests, attempts, stalls, and internal `close` do not
  become Plan, Candidate, RPI, or verdict state. Named failure mode —
  **quest-state leakage**: a GC stall, retry, or internal close surfacing in a
  report as if it were an RPI phase or verdict.
- A GC `close` is **not** an AgentOps completion. A fresh GC judge may supply
  evidence to Validate; only Validate writes `verdict.v2`.
- Explicit selection only. Falling back to Gas City because it happens to be
  running is the anti-pattern; an available substrate is not a selected one.

This skill performs no automatic selection, retry, semantic validation, Git,
integration, closure, release, or delivery.
