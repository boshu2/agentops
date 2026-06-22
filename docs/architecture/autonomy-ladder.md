# Autonomy Ladder

> The explicit L0–L4 progression for AgentOps dispatch. Makes *progressive
> authorization* a doctrine surface with named promotion gates, instead of the
> implicit floor → mayor → autodev progression. Companion to the
> [Operating Loop](operating-loop.md) (what one tick does), the
> [Canonical Loop Model](canonical-loop-model.md) (where dispatch sits), the
> [Fungibility Charter](fungibility-charter.md) (who runs the tick), and the
> [Google SRE convergence](../convergence/google-sre.md) point #8 that filed
> this (`ag-wrom`).

## Why a ladder

AgentOps already runs work at several autonomy levels — a human approving every
move in-session, a substrate mayor dispatching bounded units, an `autodev`
contract driving multi-tick loops. But the progression was **implicit**: nothing
named the rungs, stated what each one is allowed to mutate, or defined the
evidence that earns a promotion. Implicit autonomy is how a probabilistic agent
quietly acquires more authority than its track record justifies.

This ladder makes the progression explicit, **per-lane, and evidence-gated**. A
lane (a bounded surface — a skill, a package, a doc tree) is promoted only after
it clears the gate for the next rung, and is demoted automatically when risk
spikes. Authority is earned against Golden data, not granted by default.

## The invariant every rung obeys

Both AgentOps and Google's SRE-for-AI model land on the same control-plane
shape: a **non-deterministic reasoning core wrapped by a deterministic,
human-governed enforcement boundary the agent cannot bypass** — no matter how
the model evolves.

| | Reasoning core | Deterministic boundary |
|---|---|---|
| **Google** | `AI Operator` | `Actus` — mandatory dry-run, pre-flight validation, auto-downgrade on risk, Red Button |
| **AgentOps** | the agent loop | **local cockpit gate + CI backstop + ports-and-adapters + the ratchet** — the local gate is the routine release wall, CI is the post-push backstop, the domain core never imports the shell, and knowledge is durable only as a constraint |

The ladder moves the *reasoning core's* leash. It never relaxes the boundary.
Higher autonomy means the agent acts more before a human looks — it never means
the deterministic gates loosen. L4 code passes exactly the same CI as L0 code.

**AgentOps autonomy is build-time, not run-time.** It acts on code and the
`.agents/` context corpus, producing validated changes — not on live production
state. That is the honest divergence from Google's run-time model, and it is why
AgentOps [ships no daemon](../adr/ADR-0009-daemon-deletion-in-session-only.md):
sustained always-on operation is delegated to a substrate, never baked into the
product.

## The rungs

| Level | Name | Human posture | Acts before review? | May merge to `main`? | Maps to |
|---|---|---|---|---|---|
| **L0** | In-session floor | In the loop, every move | No — agent proposes, human actuates | No | the [operating loop](operating-loop.md) run interactively |
| **L1** | Supervised dispatch | On the loop, per mutating step | Reads/edits yes; each commit/PR surfaces for ACK | No | `/rpi`, `/implement` under a watching operator |
| **L2** | Bounded autonomous tick | On the loop, per PR | One bounded unit, then stop | No — opens a PR, human reviews | mayor-driven dispatch (gc / NTM); the autonomous factory tick |
| **L3** | Bounded autodev loop | On the loop, per batch | Multiple ticks under a contract | Green PRs only, under coherent-arc; auto-downgrades to L2 on risk | `/evolve` + Factory reading [`AUTODEV.md`](https://github.com/boshu2/agentops/blob/main/skills/autodev/SKILL.md) |
| **L4** | Continuous autonomous operation | Off the loop, audits after | Sustained, cross-session | Yes, within the contract's immutable scope | a substrate running L3 lanes unattended — **gated, not yet reached** |

Each rung is a superset of the leash below it, bounded by two hard rules that
hold at every level:

1. **No bead, no PR; the PR is the atomic-revert unit.** Autonomy widens *how
   many* ticks run unattended, never the coherent-arc shape of one.
2. **Merge-to-main is its own authority.** Only L3+ may merge, and only green,
   only coherent-arc. An L2 tick that finds itself wanting to merge has hit a
   rung boundary — it stops and queues, it does not promote itself.

## Promotion gates

A lane climbs one rung at a time, and only on evidence. The gate for each
promotion is **statistically-significant success against Golden data** at the
appropriate eval-maturity tier ([Bronze / Silver / Gold](../convergence/google-sre.md),
`ag-fjbu`), plus a clean enforcement-boundary record.

| Promotion | Gate |
|---|---|
| L0 → L1 | Lane has a green eval suite (Bronze+); the agent's proposals on this lane have been accepted without correction across a baseline window |
| L1 → L2 | Per-step ACKs on this lane are near-always "approve"; the coherent-arc shape holds; dry-run preview exists for every mutator the lane touches (`ag-r6l3`) |
| L2 → L3 | A run of L2 PRs merged green with no reactive-fix spiral; an `AUTODEV.md` contract exists declaring mutable scope, immutable scope, validation commands, escalation rules, and stop conditions |
| L3 → L4 | Sustained L3 success vs **Gold** Golden data at statistical significance; escalation and stop conditions proven to fire; a substrate exists to run it unattended |

Promotion is **per-lane and revocable**, never a global mode flip. A lane proven
at L3 for Go refactors says nothing about that lane's authority over schema
migrations or anything external.

## Demotion: the Red Button and auto-downgrade

Autonomy is asymmetric — slow to grant, instant to revoke.

- **Auto-downgrade on risk.** When a lane's signal degrades — a CI gate fails, a
  ratchet regression lands, an eval score drops below the rung's threshold, a
  reactive-fix spiral appears (PR-fixes-fallout-from-prior-PR) — the lane drops
  one rung automatically and the next tick runs supervised. This is the L3→L2
  auto-downgrade, AgentOps' analog of Google's risk-triggered de-escalation.
- **The Red Button.** A human override that forces **every lane to L0**
  immediately, regardless of standing. It is unconditional, it cannot be vetoed
  by an agent, and recovery requires re-earning each promotion. The existing
  primitives that compose into it: the local cockpit gate, CI backstop, `br`
  work tracking, Agent Mail coordination, rebase-on-reject serialization, and a
  human stop marker.

## Hard ceilings the ladder never crosses

No rung — not even a hypothetical L4 — confers authority over the
[hard-gate](https://github.com/boshu2/agentops/blob/main/CLAUDE.md) classes. Anything **external** (send / spend / post
/ publish, email, contacting people), another person's machine, secrets or
legal, **schema migration**, **deletion**, a **strategic fork**, or
**merge-to-main beyond the contract's immutable scope** stays human-actuated.
The ladder governs how autonomously an agent produces *validated build-time
change*; it never converts a draft into an external action. Those remain
draft-and-queue at every level.

Symmetrically, **no autonomous-drive directive crosses a ceiling either**: a
`/goal` Stop-hook ("do not pause to ask"), a `/loop`, or an unattended run moves
the reasoning core's leash, never the boundary — a human STOP marker and a pawl
HOLD outrank any drive directive. See [`pawls.md` § Directive precedence](../contracts/pawls.md#directive-precedence--autonomy-never-overrides-a-human-gate).

## See also

- [Operating Loop](operating-loop.md) — the seven moves of one tick
- [Canonical Loop Model](canonical-loop-model.md) — where dispatch sits in the waist
- [Fungibility Charter](fungibility-charter.md) — fungible by default, specialized on opt-in
- [`/autodev`](https://github.com/boshu2/agentops/blob/main/skills/autodev/SKILL.md) — the L3 contract layer
- [Google SRE convergence](../convergence/google-sre.md) — the L0–L4 source (point #8) and the Bronze/Silver/Gold eval tiers
- [ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md) — why L4 is delegated to a substrate, not shipped
