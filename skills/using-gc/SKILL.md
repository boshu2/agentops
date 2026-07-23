---
name: using-gc
description: 'Orchestrate a caller-selected Gas City through its Mayor: drive the standing dispatch shepherd (human attaches, or an agent tells it bead ids), and keep GC runtime state out of AgentOps verdicts. Triggers: "using gc", "gas city", "drive the mayor", "dispatch through gc".'
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
  effects: [operate_gas_city]
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
2. **Dispatch by id.** `invoke.sh --city C mayor tell "dispatch <bead-id>"`. Hand
   the Mayor BEAD IDS ONLY — never prose work. A paraphrased task is a telephone
   game that drifts from the acceptance the bead already carries; the id is the
   one unambiguous reference.
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
- Two known codex wedge classes and their durable fixes:
  - **Update nag:** codex blocks on an "update available" prompt. Fix: update
    codex so no pending-update prompt exists before the run.
  - **Folder trust:** codex blocks asking to trust the working directory. Fix:
    add exact-path `trust_level` entries for the rig and worktree-root in
    `~/.codex/config.toml` (bootstrap does not write them yet).

When the roster says active but the pane is wedged, the pane wins.

## Stall protocol

One nudge or re-tell, maximum, then STOP and classify — do not loop.

- Re-drive once: `invoke.sh --city C mayor tell "dispatch <bead-id>"` (or wake:
  `mayor start`).
- Still stuck: capture the pane (`tmux -L <socket> capture-pane`) and run
  `invoke.sh --city C doctor`, then report the classification. No hidden retries,
  no lifecycle bypass. **Never repair the city from inside the city** — diagnose
  from the invoke surface and hand the finding out.

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
