# Fork decision — cross-family duel (2026-07-05)

> **Decision made by cross-family adversarial judgment** (dueling-idea-wizards, decision mode). The
> question: now that factory-chain E1 found a real, load-bearing gascity bug and we've already
> patched it, **how do we handle the fork?** Options A (formalize the fork now) / B (thin interim +
> upstream-first) / C (operational-only, no patch). Brief:
> `scratchpad/fork-duel-brief.md`.

## The verdicts

| Family | Ranking | A | B | C |
|---|---|---|---|---|
| **codex (gpt)** | B > A > C | 710 | **890** | 80 |
| **claude** | B > A > C | 620 | **815** | 340 |
| agy (gemini) | — | — | — | — | *(headless auth unavailable; not run)* |

**Decisive 2-of-2 convergence** — genuinely different families, same ranking, B scored highest by
both, and — the high-signal part — **the same objection and the same mitigation**:

- **Both** chose **B**: fix the reliability failure *today* without prematurely becoming a
  fork-maintainer after a single patch. The bug is severe (so C — running stock — is
  "irresponsible"/near-dead: 80/340), but the patch is small, upstreamable, and on a low-churn file,
  so A (formalize now) is *premature* — one load-bearing patch is the *minimum* evidence for a fork,
  not enough to justify LICENSE clearance + rebasing against ~658 commits/month. "Fork the apps, pin
  the libs" points to deferral until a *second* load-bearing patch proves a pattern.
- **Both** raised the *same* objection to B: a bare local-branch-plus-`.patch` is a **weak
  supply-chain story** — the fix can rot as tribal knowledge or be silently lost by a fresh clone,
  a machine move, or an upstream rebuild. Exactly the unattended-reliability failure we just fixed.
- **Both** offered the *same* mitigation: commit the `.patch` **plus a reproducible rebuild path**
  into the repo's fork-maintenance lane, so the patched build is deterministic and durable — not
  fragile.

## The decision: **B, hardened against its own objection**

1. **Upstream-first.** File the `getAllDescendants` cycle-guard as an upstream PR to
   gastownhall/gascity — the fix's rightful home. If it merges, we drop our patch and carry zero
   maintenance.
2. **Make the local build durable** (kills the SPOF both judges flagged): the `.patch` is committed
   to AgentOps (`docs/audits/gc-mvp-2026-07-05/patches/`); add a **checked-in apply-and-rebuild
   script** so any clone/machine reproduces the patched `gc` binary deterministically, plus a doctor
   check that the running factory binary carries the patch. The fix lives in tracked AgentOps, not
   as tribal knowledge on a local gascity branch.
3. **Do NOT formalize a published/maintained fork yet.** E6's fork-formalization stays **deferred**
   until a *second* load-bearing patch proves the pattern. The "does the fork-patch pull E6 forward"
   pivot from the patch-surface research is answered: **no — thin interim.**

## Chain implication
- **E1** (zero-touch): the spawn fix is captured durably per (1)+(2); remaining gate step is the
  0-nudge quest re-run + supervisor stability.
- **E6** (selective fork patches / fork capability): **stays deferred** — the decision is thin
  interim, not fork-formalization. Revisit E6 when patch #2 lands.
