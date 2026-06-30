# ADR-0012: Focus the Surface on Membrane + Bookkeeper; Archive the Unproven Satellites Behind Build Tags

- **Status:** Accepted (2026-06-30)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven), [ADR-0009](ADR-0009-daemon-deletion-in-session-only.md) (daemon deleted, in-session only), [ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) (escape-corpus compounding unproven, structural starvation)
- **Tracking:** epic `age-focus-membrane-bookkeeper-m1wg` (this epic); source: a 4-agent `/idea-wizard` spike, notes in `.agents/research/2026-06-30-agentops-repo-overview.md`

## Context

The repo's own evals demote the headline story. [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md)
demoted the *knowledge corpus / flywheel* moat to **unproven** (a frontier model already
applies the doctrine unaided, so the corpus shows no marginal uplift at that altitude).
[ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) demoted
the *escape-corpus compounds* claim to an **unproven hypothesis facing a structural
data-starvation headwind** (a competent membrane catches at review, so it self-starves
its own escape supply — self-improvement is anti-correlated with membrane quality).

Two things, by contrast, are **proven**:

1. **The validation membrane** — independent, fresh-context, cross-family (or
   deterministic) verification: **no verdict = not done**. Measured: 0 escapes across 130
   real production verdicts; a stronger weak-producer's subtle compiling bugs still caught
   3/3. This is the product.
2. **The bookkeeper** — durable beads + a hash-chained, append-only, tamper-evident
   provenance ledger that survives sessions and models. Grep-able, verifiable, and the
   substrate every verdict is bound into.

Yet today the **unproven** story is headlined (GitHub description leads with "memory …
feedback loops that compound"; `hash-chained` / `provenance ledger` appear zero times in
the README) and the **proven** bookkeeper is invisible. The maintained surface has also
sprawled — ~91 `ao` subcommands and ~75 skills — much of it serving the unproven corpus/
flywheel/RPI/factory machinery rather than the two-core spine (~22 commands, ~15 skills).

## Decision

1. **Re-headline on the two proven cores.** Public surfaces (GitHub description/topics,
   README, `docs/3.0.md` lede, PRODUCT.md positioning) lead with *independent verification
   that records a verdict* and *the durable, hash-chained bookkeeper*. The corpus /
   flywheel / escape-corpus claims are stated as named-unproven hypotheses, never as the
   differentiator. A canonical "Proven / Still-measuring" honesty block is the single
   source other surfaces point to.

2. **ARCHIVE the satellites behind build tags — never delete.** The corpus/flywheel
   commands + packages move behind a `//go:build flywheel` tag; the RPI/factory commands
   behind a `legacy` tag. The default `go build ./...` (and the shipped `ao`) omit them;
   `make build-flywheel` and the legacy flag restore them. The code stays **buildable and
   revivable** because the revival conditions below require it.

3. **Shrink the maintained spine, don't destroy the optionality.** Skills collapse toward
   a ~15-skill spine; non-spine skills are demoted to an experimental tier (and dropped
   from Codex-twin regeneration — the ~70% regen cut), not deleted. The skill source files
   remain in the tree.

4. **The only sanctioned delete is genuinely dead code.** `ao cron` is a dead compat shim
   (the daemon was already deleted in [ADR-0009](ADR-0009-daemon-deletion-in-session-only.md));
   it may be deleted outright. Everything tied to a revival condition is archived, not
   removed.

## Why archive instead of delete (the revival conditions need the code)

Each demoting ADR left an explicit door open. Deleting the satellites would weld those
doors shut, so the code must stay buildable:

- **[ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) — corpus moat.**
  Re-opens if the corpus shows measurable marginal uplift at a *weaker base-model altitude*
  or on harder multi-task work than the frontier-single-task ceiling tested. Proving that
  needs the corpus/forge machinery runnable.
- **[ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) —
  escape-corpus compounding.** Re-opens if **real production escapes accrue** over time, or
  if a **deployed cheap tier with genuine blindspots** makes strong-tier catch-fuel show
  demonstrable transfer value. The capture/derive path must remain wired to receive that
  input.
- **[ADR-0009](ADR-0009-daemon-deletion-in-session-only.md) — out-of-session orchestration.**
  The substrate is swappable and opt-in; the RPI/factory lane is load-bearing legacy that a
  future substrate experiment may want to drive. Keep it behind the `legacy` tag, buildable.

Archiving preserves every revival path at near-zero maintenance cost (the tagged code is
not compiled, regenerated, or gated by default) while removing it from the headline and the
day-to-day maintained surface.

## Spine target

- **Commands:** ~91 `ao` subcommands → **~22-command spine** (membrane/pawl, beads/bookkeeper,
  provenance ledger, gate, session, eval, the core authoring loop). Corpus/flywheel behind
  `flywheel`; RPI/factory behind `legacy`.
- **Skills:** ~75 → **~15-skill spine**; the rest demoted to experimental and dropped from
  twin regeneration.

## Consequences

- The **proven** claim is unchanged and now load-bearing on the front door: independent
  cross-family / deterministic verification, **no verdict = not done**, plus a durable
  hash-chained record. Positioning rests on the verification/control system that is proven,
  not on a self-improvement flywheel the data has not shown accruing.
- No proven capability is removed. The membrane and the bookkeeper keep their full surface.
- The archived satellites are revivable in one build flag; nothing is foreclosed.
- Maintenance load drops (fewer default-compiled commands, fewer regenerated twins, a
  smaller validated skill set) without deleting optionality.
