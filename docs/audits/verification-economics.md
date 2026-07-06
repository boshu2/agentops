# Verification Economics — the same guarantee, fewer tokens

> **Status:** assessment + proposed direction (2026-07-06). Not doctrine yet — promote via ADR
> after the meter (roadmap below) produces two weeks of real data.
> **Tracked as:** epic `age-verification-economics-ebec` (+6 children).
> **Question answered:** *"You could just throw tokens at something and it'll eventually be
> right. How do we get better results with less tokens?"*
> **One-line answer:** tokens buy attempts; only independent discrimination turns attempts into
> correctness. The membrane's next fitness axis is **discrimination per token** — hold an
> explicit escape SLO at minimum cost, and let every escape compile into a free deterministic
> check so the cost curve bends *down* over time.

## 1. Sharpening the question

"Throw tokens until it's right" is best-of-N resampling. It converges only when an independent
selector exists: a model re-checking its own work fails on correlated errors (measured in this
repo — cross-family review caught 7 bugs self-review missed). Brute force buys *attempts*; what
turns attempts into correctness is **bits of discrimination**, and bits have wildly different
prices:

| Source of discrimination | Marginal price per use |
|---|---|
| Compiler / test / linter / schema / drift gate | ~0 tokens (CPU) |
| Compiled escape-check (EM spine) | ~0 tokens after a one-time compile |
| Scoped single-model review (contract + diff only) | ~10³–10⁴ tokens |
| Fresh-context cross-family duel with repo access | ~10⁴–10⁵ tokens per family |
| Multi-family quorum / council | ~10⁵–10⁶ tokens |

The product is the discrimination layer — **no verdict = not done** is unchanged by everything
in this document. The improvement axis is buying the *same confidence* higher up this table.
Two rules fall out:

1. **Buy bits where they are cheap first.** Deterministic ground truth before model judgment —
   already doctrine (the windshield); economics makes it quantitative.
2. **Spend model tokens only where uncertainty remains.** A fixed N-family review on every
   close is a flat tax. Adaptive spend — escalate only on risk or disagreement — buys the same
   escape rate for a fraction of the cost.

## 2. What the data says (measured 2026-07-06)

| Datum | Value | Source |
|---|---|---|
| Provenance ledger records | 344 (321 `wasDerivedFrom`, 23 `wasGeneratedBy`) | `docs/provenance/ledger.jsonl` |
| Pawl verdict binds in git | 155 total; 133 CONFIRMED / 4 REFUTED (all since 2026-06-06) | `git log --grep "bind pawl"` |
| Refute rate, current regime | **≈ 3%** (4/137) | same |
| Refute rate, 2026-06-22 yield window | 23.8% (31/130) | ADR-0011 |
| Production escapes, ever | **0** | ADR-0011 + ledger |
| Escape-rate bound implied | ≤ ~2% (95% CI, rule of three on ~150 verdicts) | derived |
| Cheap-tier lab blindspots | none found — cheaper reviewer tier caught the subtle planted bugs **6/6 unaided** (transfer lift 0) | ADR-0011 §Evidence 4 |
| Landed diff size, last 100 commits | p50 3.2 KB (~800 tokens), p90 30 KB (~7.5k tokens) | measured tonight |
| Cost instrumentation | **none** — 30 gates in GOALS.md, zero economic; no verdict record carries tokens/duration/family-count | GOALS.md + ledger schema |

**Finding 0 — the meter doesn't exist.** The question "what does a verdict cost?" cannot be
answered from any artifact in this repo. That is the first fix, before any optimization: the
fitness function for cost is not failing — it is *absent*.

**Finding 1 — the duel now changes the outcome ~3% of the time.** 97% of cross-family duels
confirm work that was already right. Candidate explanations (unresolved; the report bead will
separate them): stronger producers, stronger upstream deterministic gates (per-commit cockpit),
lane mix shifting toward docs/provenance chores, and deterrence. Note what a low refute rate is
*not*: proof the gate is useless — the 4 REFUTEDs are exactly the catches that matter, and
deterrence is real. It *is* proof the current flat-tax shape is over-provisioned for most lanes.

**Finding 2 — payload is not the cost.** The median reviewed change is ~800 tokens of diff. A
fresh-context reviewer that grinds the repo spends orders of magnitude more than the payload it
judges. Most duel spend is process overhead, not information transfer — which is why the
context-diet lever (§5) exists.

**Finding 3 — the cheap tier had no blindspot where we looked.** In ADR-0011's lab runs the
cheaper reviewer tier caught every subtle compiling-but-wrong bug unaided. On tested classes,
the expensive tier bought *zero additional discrimination*.

**Finding 4 — statistical honesty cuts both ways.** 0 escapes in ~150 verdicts bounds the true
escape rate below ~2%, so there is headroom to cut spend and *measure* what happens. But at this
volume no superiority claim is provable either — which is why volume (adoption, §7) is part of
the economics story, not a separate growth story.

## 3. Diagnosis — over-provisioned, and that starves the flywheel

ADR-0011's core result: *a competent membrane structurally generates ~0 of its own misses* —
self-improvement-from-escapes is anti-correlated with membrane quality. Running maximum rigor
on every lane produces the worst of both worlds: **expensive AND learning nothing.** The token
bill and the demoted compounding hypothesis (ADR-0004, ADR-0011) are the same fact seen from
two sides.

ADR-0011 already names the exit, as a revival condition:

> "A deployed CHEAP tier with genuine blindspots — if production runs a cheaper membrane than
> the strong reviewing tier, a strong-tier catch becomes a real blindspot for the cheap tier,
> and catch-fuel gains demonstrable transfer value."

So cost-reduction and corpus-compounding **rescue each other**:

```
cheap tier runs everywhere
  → occasional real misses get caught by deterministic backstops / sampled strong tier
    → each catch is REAL escape data (the input ADR-0011 says is missing)
      → each escape compiles into a free deterministic check (EM spine — proven e2e)
        → the deterministic floor rises → less model judgment needed → cheaper still
```

Restated: **the corpus was never going to compound in *quality* at the frontier; it can compound
in *cost*.** A compiled check is amortized inference — pay tokens once at compile time, replay
free forever (OpEx → CapEx). Escapes caught by inner backstops are corpus food, not user harm;
the SLO (§4) budgets the difference.

## 4. The SRE frame — buy the nines you need

100%-catch-everywhere is over-nining: each additional nine costs roughly an order of magnitude,
and the current regime pays frontier-duel prices for lanes whose observed defect rate is ~3%.
The membrane's guarantee should be stated the way reliability targets are stated:

- **Escape SLO:** user-visible escapes per 100 verified-dones, with an explicit error budget.
  (Inner-layer catches — a later gate or sampled audit overturning a cheap-tier CONFIRMED —
  burn no SLO; they are the flywheel's food.)
- **Spend to the budget, not to zero.** When the budget is untouched for weeks at current spend,
  the membrane is over-provisioned; move a lane one tier cheaper and watch the meter.
- **The honest product claim** this enables: *"holds an escape SLO of X at ≤Y% verification
  overhead"* — measurable, marketable without leaning on any unproven compounding story, and a
  claim neither brute-force resampling nor flat-tax review products can make.

## 5. Levers — mapped to what already exists

| # | Lever | Mechanism | Surface today | Status |
|---|---|---|---|---|
| L0 | Deterministic gates | compiler/tests/lint/schema/drift; ~0 tokens | `ao gate check`, pre-push suite, cockpit | **exists** — keep first in line |
| L1 | Compiled escape-checks | escape → derived check → future block | EM spine | **exists, starved** — §3 feeds it |
| L2 | Scoped cheap review | one cheap family; packet = contract + diff + evidence | agy flash lane; local lane (eval-proven) | partial — needs context diet + default status |
| L3 | Cross-family duel | fresh-context, ≥2 families, fail-closed | pawl (cc/cod/agy) | **exists — today's flat default; becomes the *risky-lane* tier** |
| L4 | Quorum / council | N independent judges | `/council`, tri-family pawl | exists — one-way doors only (already doctrine) |
| R | Risk router | deterministic rules pick the tier; fail-closed upward | — | **new** (bead .4) |
| D | Context diet | ship ~800-token payloads, not repo grinds | — | **new** (bead .5) |
| M | Verdict memoization | identical tree ⇒ rebind, don't re-review | pawl rebind practice | exists informally — formalize |
| S | Shift left | verify the *plan* before the build; catches are 10× cheaper per stage earlier | plan-pawl | exists — underused |
| A | Adaptive stopping | stop at confidence, escalate on disagreement — not fixed N-of-M | — | new, later phase |
| P | Sampling on trivial lanes | 1-in-N audit instead of binary waiver | `#trivial` waiver precedent | new policy, later phase |

Design notes: **independence beats redundancy** (two verifiers with uncorrelated failure modes
— different family, or model + deterministic — buy more than three same-family passes; choose
the *cheapest uncorrelated pair*). **Focus beats coverage** (a reviewer holding exactly the
acceptance contract and the diff discriminates better per token than one wandering the repo).

## 6. The ruler — fitness functions to add (warn-only until data)

| Metric | Definition | Source |
|---|---|---|
| **TPVD** | tokens per verified done (all verification layers summed per closed bead) | meter fields on verdict binds |
| **VOR** | verification overhead ratio = verify tokens ÷ produce tokens | meter + producer estimate |
| **CPCD** | cost per caught defect = verification spend ÷ REFUTEDs-that-led-to-a-fix | meter + ledger |
| **Escape SLO** | user-visible escapes per 100 verified-dones vs budget | ledger + overturn records |

Honesty rules: thresholds are set from the first two weeks of measured data, never invented;
claims stay inside the confidence interval the sample supports (rule of three until volume
grows); the meter reads the harness, never producer self-report.

## 7. Why this is the direction (product + growth)

- **Continuity, not pivot.** The verdict invariant is untouched; every move here is already
  latent in recorded doctrine (windshield-first, cost law at gates, ADR-0011's own revival
  conditions). What is new is only the *fitness axis* — and a fitness axis is what "direction"
  concretely means.
- **The adoption wedge.** A verifier that doubles the bill does not get adopted; one that
  provably pays for itself does. For subscription-quota operators (most of the target users),
  the scarce resources are weekly quota and wall-clock, so falling API prices do not dissolve
  the value — the *ratio* is the product property, and agent volume grows faster than prices
  fall.
- **Volume is the missing evidence.** ADR-0004/0011 are starved for statistical power. Cheap
  membrane → more users and more lanes run it → more verdicts → the ruler tightens → honest
  claims become possible. The growth path and the evidence path are the same path.
- **Unclaimed ground.** The gate itself is table stakes (GOALS.md already says so — CodeRabbit,
  Qodo, Copilot review). *SLO-priced verification* — an explicit escape budget held at measured
  minimum cost — is a claim none of them make and brute force cannot make.

## 8. Red team — what would prove this wrong, and the tripwires

1. **Deterrence collapse** — producers get sloppier once lanes are known-cheap. *Tripwire:*
   refute-rate trend per lane in the weekly report; sampled strong-tier audits keep the strong
   tier unpredictable.
2. **Cheap-tier blindspot class in production** worse than the lab's 6/6. *Tripwire:* that IS
   the escape data (§3) — bounded by the SLO budget and the deterministic backstops; each one
   compiles into a check.
3. **Router misclassification** ships a risky change down a cheap lane. *Response:* rules are
   deterministic, auditable, fail-closed upward; a misroute is an escape and compiles into a
   router rule. (The `#trivial` bypass was already closed once this way.)
4. **The meter gets gamed or adds drag.** *Response:* measure at the harness boundary,
   append-only beside the existing bind records; no self-report.
5. **"Token prices will fall anyway."** They will; the ratio still decides whether verification
   is dead weight, quota and wall-clock stay scarce, and Jevons applies — volume outruns price.

If after four weeks the meter shows verification overhead is already trivially small, the
correct conclusion is "economics was not the constraint" — the doc self-demotes exactly the way
ADR-0004/0011 demoted their theses. That is the point of building the ruler first.

## 9. Roadmap (beads filed 2026-07-06)

| Bead | Slice | Done when |
|---|---|---|
| `…ebec.1` | **Meter** — cost fields on every verdict bind | 10 consecutive real verdicts carry families/wall/tokens |
| `…ebec.2` | **Report** — `verification-economics-report.sh` + bats | weekly refute rate + cost/verdict (UNMEASURED until .1) green in bats |
| `…ebec.3` | **Ruler** — GOALS.md directive + warn-only gate | `ao goals validate` green; thresholds from data |
| `…ebec.4` | **Router** — risk-tiered close path, fail-closed | 20+ routed closes with per-tier refute yield |
| `…ebec.5` | **Context diet** — packet = contract + diff + evidence | cost/verdict drops at held refute rate |
| `…ebec.6` | **Cheap tier + escape harvest** — ADR-0011 revival condition wired | 2 weeks of sampled audits; escapes compiled to checks |

Phase discipline: `.1`–`.3` are ready now (measure first); `.4`–`.6` are blocked on the ruler —
act on data, not on this document.

## 10. Decision asks

1. Land phase 1 (meter/report/ruler). Nothing about the close path changes until measured.
2. After two weeks of data: promote or demote this assessment via ADR, and only then flip the
   router/context-diet/cheap-tier beads to ready.
