# Operating Discipline — the rules every agent factory runs on

> **What this is.** The general, substrate-neutral operating rules for running a fleet of
> AI coding agents: admission control, independent validation, close discipline, and
> multi-lane coordination. Distilled from the mt-olympus *triangulated kernel* (D1–D16,
> three independent model distillations reconciled to consensus) and **folded into AgentOps
> as the surviving factory** — because AgentOps is now the one self-contained factory and the
> operating rules should live where the work runs.
>
> **Adapted, not blind-copied.** The load-bearing invariants (fail-closed, author≠reviewer,
> evidence-bound, single-writer) are **already embodied** in the shipped ratchet pawl-gate —
> each is cited below to the exact mechanism. The rules that were *about the cut cathedral*
> (an OS-account privilege floor, peercred kernel identity, a daemon, a Rust gate spec) are
> intentionally **dropped** — the ratchet pawl-gate ([`pawls.md`](../contracts/pawls.md))
> embodies them in AgentOps terms, and re-importing the floor as live doctrine would resurrect
> the cathedral. This is a reference doc, not a cathedral.

## The doctrine in one line

**Nothing ships on a lie while you're not looking**: nothing is real until it is admitted to
the tracker; "done" is a claim until an independent, fresh-context reviewer verifies it against
the live commit with real evidence; one writer closes; everything ambiguous fails toward a HOLD;
destructive writes reconcile or refuse — and the whole machine exists to give the operator's
attention back.

## How the gate embodies the load-bearing rules

The shipped ratchet is the enforcement surface. Read it as: **chaos between pawls, a fail-closed
gate at each one-way door.**

- **The pawls** ([`docs/contracts/pawls.md`](../contracts/pawls.md)) — the short static list of
  irreversible actions (mutate-shared-trunk · delete · external-send · schema/contract change ·
  credential change · spend). Everything *not* on the list is ungated chaos.
- **The verdict** ([`scripts/pawl-verdict.sh`](../../scripts/pawl-verdict.sh),
  schema [`schemas/pawl-verdict.v1.schema.json`](../../schemas/pawl-verdict.v1.schema.json)) — an
  evidence-bound, commit-bound verdict that requires a real reviewer run: `disposition` ∈
  {CONFIRMED, REFUTED, ESCALATE, HOLD}, an `author_context_id`, ≥1 refuter with a distinct
  `context_id`, non-empty reviewer evidence, roster-validated families.
- **The merge path** ([`scripts/reconcile-pr.sh`](../../scripts/reconcile-pr.sh)) — green CI is
  necessary but **never sufficient**: a CONFIRMED verdict tied to this bead+PR with `head_sha` ==
  the PR's live head must exist, or the merge is refused (`exit 5` HOLD). A new commit after review
  → STALE → HOLD. Lookup failure → HOLD. Fail-closed throughout.

## The kernel rules (D1–D16) and their disposition

Each rule: one-line statement, then **disposition** — *embodied-in-gate* (with citation),
*doctrine* (advisory; no single mechanism yet), or *dropped-as-cathedral* (floor-specific, not
imported as live doctrine).

### D1 — Admission first
No work exists until it is admitted and persisted in the tracker. Chat can propose; it cannot
admit. "Admit first, then work" — never "do work, then file."
**Doctrine** (operating rule). Embodied operationally by the beads tracker (`br`/`bd`): a bead is
the unit of admitted work; the pawl-verdict and merge path are keyed by `bead_id`, so an
un-admitted change has nothing to close against.

### D2 — Completion is a claim, not a state
A worker's "done" is an event, not a transition; only an independent typed verdict produces a
verified close. The gate is what makes "don't watch" safe.
**Embodied-in-gate.** `reconcile-pr.sh` refuses to close on a worker's say-so — it requires a
CONFIRMED verdict from `pawl-verdict.sh check`; absent it, exit 5 (HOLD).

### D3 — Author ≠ judge, by context identity
The context that produced the work never validates it; a close needs ≥1 distinct non-author
reviewer context, blind to authorship. Self-grade exclusion is by **context identity**, not model
family (cross-family is a strengthening mode, not the floor).
**Embodied-in-gate.** `pawl-verdict.sh` default `fresh-context` mode requires ≥1 refuter whose
`context_id` != `author_context_id` (model-agnostic). `multi-model` mode (≥2 roster-validated
families) is the opt-in strengthener for the highest-irreversibility doors — exactly the
context-vs-family resolution the kernel's DISPUTED-1 settled.

### D4 — Single writer
One decision, one binding artifact, one writer. Workers propose and never close; reviewers verify
and never close; only the orchestrator writes the close, and only on verified PASS.
**Embodied-in-gate.** The merge/close is performed solely by `reconcile-pr.sh` (the
orchestrator-only reconcile step); the implementer subagents never merge. Competing writers are an
integrity failure, not a race.

### D5 — Evidence or it didn't happen
Every close cites a proof surface; every verdict carries the commands it ran. A verdict with no
cited evidence is rejected as unverified.
**Embodied-in-gate.** `pawl-verdict.sh check` requires real, **non-empty** per-refuter evidence;
an empty or missing evidence file fails the verdict (HOLD). Green CI is recorded but is not the
proof — the reviewer evidence is.

### D6 — Fail closed
Genuine uncertainty defaults to FAIL. Ties, thin councils, missing minimums → HOLD/escalate; the
resolver never synthesizes a winner; a quorum of one is a self-grade.
**Embodied-in-gate.** Every non-CONFIRMED disposition (REFUTED/ESCALATE/HOLD), a STALE head, an
unresolvable head, a missing verdict, or an unmet diversity floor all exit 5 (HOLD: no merge, no
close) in `reconcile-pr.sh`. Non-convergence never auto-lands.

### D7 — Identity is attested, not asserted
**Dropped-as-cathedral** (mostly). The kernel's D7 was anchored to the OS privilege floor —
kernel peercred, a separate OS account, `provision-accounts.sh`. That floor is the cut cathedral;
do **not** import it as live doctrine. The *surviving* grain: a reviewer's `context_id`/`family`
in the pawl verdict is validated against a known roster (off-roster families are rejected so
"≥2 distinct families" can't be gamed) — but the threat model is explicitly a *sloppy* agent that
self-stamps, **not** a cryptographic forger (single-operator trusted loop; see the pawls.md scope
note). No peercred, no signatures, no OS writer-separation.

### D8 — No self-greening
A gate and the thing it checks are adversaries; a pass derives from observed behavior, never a
self-declared status; tightening/loosening a gate is its own reviewed change, never bundled with
the work.
**Embodied-in-gate.** The pawl gate cannot be satisfied by the author — it requires an external
fresh-context refuter and live `head_sha` binding. **Doctrine** rider: gate changes (editing the
pawl list, the verdict schema, or `reconcile-pr.sh`) ship as their own reviewed change — never in
the same PR as work they would have caught.

### D9 — One source of truth
Every declared capability has exactly one mechanical source of truth; "done" means the mechanical
check is green, not that a doc says so.
**Embodied-in-gate** + **doctrine.** AgentOps' `make regen-check` is this rule for derived
artifacts: the registry/codex/context-map are generated, never hand-asserted, and drift fails the
gate. This doc is itself an instance: it is "done" because regen-check is green, not because this
sentence claims it.

### D10 — Typed state transitions, not narrative persuasion
Components publish and consume contract surfaces (tracker, reservations, evidence/verdict files)
and act; a decision is real only when it appears as the required state transition.
**Embodied-in-gate.** The pawl verdict is a typed JSON artifact against
`pawl-verdict.v1.schema.json`; the merge path reads *that file*, not a chat assertion. Agent
coordination rides Agent Mail locks/reservations, not narrative.

### D11 — Policy is executable; roles are structure
Policy is versioned, testable, default-deny, hard-reject code that travels with the repo; roles
carry duty contracts (reviewer read-only; worker write-scoped; orchestrator coordinates and never
labors).
**Embodied-in-gate** + **doctrine.** The pawl list + verdict schema + `reconcile-pr.sh` are
executable, repo-resident admission policy. The role split (worker proposes / reviewer verifies /
orchestrator closes) is the ship-beads/reconcile discipline; the orchestrator-doesn't-labor rule
is advisory doctrine for swarm runs.

### D12 — Inference mines and codifies; deterministic code reconciles forever
The steady-state control loop is deterministic and cheap; inference lives in the learning loop
(mine → codify into controllers), never inside the control loop it is meant to make reliable.
**Doctrine** — and the core AgentOps shape: the knowledge flywheel (`/forge`, `/curate`, `ao
lookup`) is the learning loop; the deterministic gates/scripts are the control loop. Keep models
out of the reconcile path; let them propose new deterministic checks, not run inside the gate.

### D13 — Atomic lease-bounded claims; idempotent ticks; precise state words
Claiming work is an atomic lock with a visible TTL lease that expired-reaps a dead worker; every
tick reads state first and does only the next undone increment; NO_READY ≠ CONVERGED.
**Doctrine.** Operating rule for the loop/swarm substrate (beads claim + Agent Mail reservations
approximate the lease; evolve/crank ticks are read-state-first). No daemon-level lease reaper is
imported — that was floor machinery.

### D14 — Destructive writes reconcile or refuse
Any write that would replace existing state merges against current reality or fails closed; a
close is not durable until merged to trunk.
**Embodied-in-gate.** `reconcile-pr.sh` is the named reconcile step; the STALE-head check (a new
commit after review invalidates the verdict) is exactly "reconcile against current reality or
refuse." "Mutate shared trunk" is the first pawl. **Doctrine** rider: same root applies to
tracker/notes/snapshot writes — merge, don't clobber.

### D15 — The operator's attention is the constraint
The scarcest resource is the human's attention; babysitting burns it because an unverified "done"
must be personally checked. The system subordinates everything to that constraint and spends
attention only where the human is irreplaceable (intent, acceptance, one-way doors).
**Embodied-in-gate** (as escalation economy) + **doctrine.** The pawls.md circuit-breaker model is
this rule operationalized: the gate runs model-to-model and auto-redoes on REFUTED *with no human*;
a human is pulled in **only** when a tunable breaker trips (max-attempts, time, cost, oscillation,
explicit-judgment-flag). The andon ("Hey! Listen!") is rare and earned, never the default.

### D16 — Mechanisms stay general; policy lives in its bounded context; layers meet through ports
A shared mechanism never has a context's law welded into it — the context configures it via a mode
plus a pointer to its enforcing gate; the factory layer calls the worker layer only through
external ports and never bypasses its local gates or rewrites its evidence.
**Doctrine** — and the architectural reason this fold is legitimate: the operating rules are the
*general mechanism*; the ratchet pawl-gate is AgentOps' *enforcing gate*. The per-pawl `mode`
(fresh-context vs multi-model) is exactly "the context configures the mechanism via a mode." See
[`architecture/ports-and-adapters.md`](../architecture/ports-and-adapters.md).

## What was dropped (and why)

These were real in mt-olympus but are **floor/cathedral-specific** — not imported as live AgentOps
doctrine:

- **The OS privilege floor** — separate OS account, kernel peercred identity,
  `provision-accounts.sh --check`, the `olympusd` daemon as sole writer. The ratchet pawl-gate is
  the AgentOps embodiment of "single writer / attested reviewer"; the OS floor is the cut cathedral.
- **The 55-case Rust gate spec / typestate `Accepted`** — a specific implementation of "the gate
  is the only door," not a substrate-neutral rule. AgentOps' door is `reconcile-pr.sh` + the pawl
  verdict.
- **Host-policy instances** (e.g. the `claude -p` LAW 0 ban) — real, mechanically enforced, but a
  *host-policy instance* of D11 (forbidden-pattern admission), not a method invariant. (LAW 0 still
  holds operationally; it just isn't kernel doctrine.)
- **3-family quorum / Gemini-benched specifics** — operational policy-mode detail of a particular
  fleet, superseded here by the per-pawl `mode` and the context-identity floor.

## Provenance

- Source kernel: `mt-olympus/docs/doctrine/fleet-operating-discipline/corpus/specs/triangulated_kernel.md`
  (D1–D16, three-model triangulation, evidence anchors `Q-<SRC>-<n>`).
- Enforcement surface in this repo: [`docs/contracts/pawls.md`](../contracts/pawls.md),
  [`scripts/pawl-verdict.sh`](../../scripts/pawl-verdict.sh),
  [`scripts/reconcile-pr.sh`](../../scripts/reconcile-pr.sh),
  [`schemas/pawl-verdict.v1.schema.json`](../../schemas/pawl-verdict.v1.schema.json).
- The structural-corpus validators (`validate-corpus.py`, `check-doctrine-drift.py`) were
  **not ported** — see the note at the bottom of this fold's task record; they validate the
  mt-olympus marker-bounded corpus (anchors, distillations, vendor-drift), which this single
  adapted reference doc does not carry.
