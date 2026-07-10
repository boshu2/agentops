# PerspectivePlan P1 — Merge-Door Redesign, LENS: PRODUCT / USER VALUE

> 2026-07-09. Input: `.agents/research/merge-door-{constraint-model,rebaseline,design-space}.md`.
> Target design: Composite A (two-phase done + WIP cap + compensation). Frame: AgentOps is a shipped
> product (skills + `ao` CLI); the typical END USER runs bd, single-agent, no warm pawl service,
> possibly ONE reviewer CLI. A bead/commit is an EXPERIMENT; the product's job is to observe,
> measure, and monitor the experiment stream — not just gate it. No moat claims (ADR-0004/0011).

## Goals (this lens)

1. **Make the daily loop simpler, not scarier.** Composite A's user story is the one every dev
   already knows: *push, checks run, results arrive.* Push is gated only by the deterministic
   battery (fast, boring, familiar-CI-shaped); the probabilistic verdict trails. That is LESS
   exotic than today's "wait minutes for an LLM before you may push." Scary only if the pending
   state is invisible or if the system reverts things behind the user's back — so ship visibility
   and never default auto-revert.
2. **Move the invariant to where users already believe it lives:** the BEAD, not the push.
   "No verdict = not done" binds *close* (and release), never integration. Re-scoped, not weakened.
3. **Ship the experiment-stream surface as the daily driver**, not as telemetry exhaust.

## The smallest user-facing contract

- **`ao land` is instant** (deterministic gate only; seconds-to-a-minute). No reviewer on the push path.
- **`ao yield report` is the daily driver** (coordinate with in-flight age-mv67 — extend, don't
  duplicate): pending window (landed-awaiting-verdict, with age), verdicts as they arrive, andon
  queue (ESCALATE/HOLD/REFUTED-with-open-bead), verified-frontier sha.
- **`br close` / `ao done` still refuses without a verdict** — unchanged door, unchanged command.
- **REFUTED-after-land auto-files a P0 fix bead** carrying the defect list — it just shows up in
  `br ready`. The user's response to a catch is the response they already know: work the bead.
- **Zero-setup degrade (ADR-0009 honesty):** most users have no cron/NTM/warm service. Default
  shape = *review-at-close*: the verdict is produced foreground when the user closes the bead (or
  runs `ao verify drain`). Async-vs-sync is WHERE the wait sits (close, not push), not new infra.
  Tracker-agnostic: every new check must work on bd AND br.

## Posture matrix (defaults vs opt-in)

| Capability | Default (in-the-loop, single-agent) | Opt-in (on-the-loop, swarm/operator) |
|---|---|---|
| Push path | Deterministic gate only — ON for all | same |
| Verdict timing | At close, foreground (`ao done` triggers/drains review) | Scheduled drain via user's substrate (cron/launchd/NTM tick) |
| WIP cap | Present, generous (N≈3), invisible at WIP=1; cap-hit message *routes* ("drain reviews"), never just refuses | Data-sized from meter; andon on repeated cap-hits |
| REFUTED handling | Auto-file fix bead (additive, safe) | + BC freeze valve; auto-revert ONLY here, escalation classes only, repro-required, mechanical `git revert` bound to the refuting verdict |
| Reviewer tier | fresh-context, single family (matches one-CLI users; pawls.md:90-91, resolve :229 in its favor) | multi-model duel / council |
| Releases | Tag from verified frontier only (tooling refuses beyond it) | same |

## The experiment-stream product surface

Each land renders as an experiment card in the report: **hypothesis** = bead acceptance
(Given/When/Then) · **guardrail** = deterministic gate (passed by construction) · **readout** =
verdict (PENDING / CONFIRMED / REFUTED+defect list). A REFUTED is a *readout, not a failure alarm*
— its fix bead is the published result, and its catch class feeds `known_risks` in the next
discovery packet (surface already landed, 14875496e). This is the honest pitch: AgentOps watches
your agent's output stream and prices the watching (D17 meter) — claim the readout loop, never
corpus compounding.

## What DONE means (keep it honest and legible)

Three states, one word reserved: **LANDED** (on main, deterministically green) → **VERIFIED**
(sealed verdict bound to the landed sha) → **DONE** (bead closed, verdict-stamped). Only the third
is ever called done. Legibility mechanics: the report prints all three columns + frontier; the
close door refuses below VERIFIED; docs/skills language rule — "landed" is never written as
"done"; release tooling is frontier-only. Anti-drift: the psychological failure ("landed feels
done") is countered by surface, not discipline — pending age > threshold turns the report line
into an andon row.

## Top risks (product lens)

1. **Landed-feels-done drift** — user demos/ships from an unverified tip. → frontier pointer +
   frontier-only releases + report as the daily habit.
2. **Single-reviewer users gain little if they wait at close anyway.** Be honest in the pitch:
   the win is unblocked pushing + batching several beads' reviews at one drain; cold reviewer
   calls are 3–6 min each. Don't promise "2× throughput" to a WIP=1 user.
3. **Auto-revert breaks trust.** Never default; escalation classes + runnable repro only; a
   revert-of-revert is an unconditional stop-the-line. (Revert arm doesn't exist yet — ship last.)
4. **Three-state model confuses the one-word mental model.** Ship the status surface and the
   legend in the SAME slice as the flow flip, never after.
5. **br-only drift** — pending-window/close checks built against br internals strand bd users.

## Slice ordering (by user value delivered)

1. **Visibility first:** pending-window + frontier + experiment readouts in `ao yield report`
   (with age-mv67). Value with zero flow change; makes every later slice legible.
2. **Instant land for L0** (docs/provenance) — formalize what the #trivial waiver already does;
   proves trailing-bind bookkeeping, zero semantic risk.
3. **Flip the default: push-then-verify, review-at-close, cap N=3** + the DONE legend. The
   user-contract slice; rollback = flip the order back.
4. **Auto-file-on-REFUTED fix bead** — the readout loop closes into the tracker + known_risks.
5. **Opt-in on-the-loop kit:** scheduled drain recipes (cron/launchd/NTM), cap sizing from meter.
6. **Escalation compensation (revert lane + freeze valve)** — last, opt-in, after the repro rule
   is enforced. Router (Composite B) stays behind D17's data gate; not a product promise yet.
