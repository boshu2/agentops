# Formal verification for validated agent output — what's real, what's adoptable

> **Date:** 2026-08-04 · **Status:** research synthesis (no code changes) · companion to
> [skill-eval-sota-standards-2026-08.md](skill-eval-sota-standards-2026-08.md) and
> [eval-architecture.md](../architecture/eval-architecture.md)
> **Method:** bounded deep-research workflow (3 angles → 17 sources → 85 claims extracted →
> top 8 adversarially verified: 3 confirmed, 5 killed) plus two targeted follow-up passes for
> the areas the sweep didn't reach. Labels: **verified** = survived adversarial refutation
> against the primary source; **follow-up** = cited by the gap pass, single-sourced;
> **refuted** = failed a single-vote adversarial check (weak evidence of falsity — listed so
> the numbers don't get laundered into downstream docs).
> **The question:** Bo wants Antithesis-grade confidence in agent-produced code. Which formal
> and formal-adjacent methods are actually adoptable by a small team whose acceptance criteria
> are mostly tests today?

---

## TL;DR

1. **The two organizations most famous for deterministic simulation testing both rank the
   simulator last.** TigerBeetle's style doctrine orders correctness work: mental model →
   **assertions** → reviewer-justifying code → the VOPR simulator "as the final line of
   defense," because "a fuzzer can prove only the presence of bugs, not their absence."
   Antithesis states the simulator cannot judge without **properties you define**. Verified
   against primary sources, mechanically re-counted.
2. **Therefore the cheapest, most transferable rung is assertion/invariant density** —
   executable oracles inside the code: NASA Power of Ten Rule 5, hardened by TigerBeetle from
   "should" to "must average ≥2 assertions per function, covering arguments, return values,
   pre/postconditions, invariants." No formal-methods tooling required. It is a human-review
   norm with a countable metric — and a known Goodhart hole (NASA pairs the count with a
   checker proving each assertion *can fail*; TigerBeetle dropped that guard; we must not).
3. **The synthesis, flagged as inference:** for a harness whose acceptance criteria are mostly
   tests, the next rung is oracles, not executors. **The oracle is the scarce artifact.** This
   is the membrane thesis said in formal-methods language: agentops' product has always been
   the verification surface, and every rung of the formal ladder is a way of manufacturing
   more oracle per engineer-hour.
4. **Five of seven sub-questions produced zero adversarially-verified claims** in the main
   sweep (model checking, proof assistants, the LLM×FM wave, runtime verification, cost
   ladder) — covered below by the follow-up passes at correspondingly lower confidence.
5. **The load-bearing open question for agentops** (no verified evidence either direction):
   *can an LLM agent author assertions for its own code that catch bugs its own tests miss —
   or does the misunderstanding that produced the bug also produce the assertion?* This is the
   self-grading failure mode the membrane exists to prevent, and it is directly measurable in
   our own harness (§4).

---

## 1. What survived adversarial verification

### 1.1 Assertion density: the verified floor (confidence: high)

From [TIGER_STYLE.md](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md),
re-fetched and mechanically re-counted 2026-08-04, wording identical at tag 0.17.4 and main:

- Verbatim: **"The assertion density of the code must average a minimum of two assertions per
  function."** And: **"Assert all function arguments and return values, pre/postconditions and
  invariants."** "A function must not operate blindly on data it has not checked."
- **Provenance (carry this):** it is [NASA/JPL Power of Ten Rule 5](https://spinroot.com/gerard/pdf/P10.pdf)
  adopted and hardened from "should" to "must" — cite P10, not TigerBeetle, as the origin.
- **Norm, not gate:** TigerBeetle's own mechanical checker (`src/tidy.zig`) enforces ~12 style
  rules and does **not** count assertions. The floor is enforced by human review.
- **Goodhart exposure:** NASA's Rule 5 pairs the count with a static check that every assertion
  can actually fail (no `assert(true)` padding). TIGER_STYLE drops that guard and compensates
  qualitatively ("assert positive AND negative space"; "Assertions are a safety net, not a
  substitute for human understanding"). Any adoption here must restore the killability check.
- **Generalization risk:** n=2 organizations, one a vendor; a Zig financial database with a
  small exhaustively-assertable state machine. No verified source anywhere correlates
  assertion density with defect-escape rate. Adopt as a *hypothesis to measure*, not a law.

### 1.2 The simulator is ranked last by its own inventors (confidence: high)

Mechanically verified against the 511-line primary doc: the prescribed ordering is (1) build a
precise mental model, (2) encode it as assertions, (3) write code/comments that justify it to a
reviewer, (4) only then the VOPR, "as the final line of defense." VOPR appears exactly once in
the document; assertion vocabulary 31 times. Rationale, verbatim: "a fuzzer can prove only the
presence of bugs, not their absence."

### 1.3 Antithesis: determinism is the substrate, properties are the judge (confidence: high, vendor sources)

From [Antithesis' own docs](https://antithesis.com/docs/resources/deterministic_simulation_testing/)
(+ [how-it-works](https://antithesis.com/docs/introduction/how_antithesis_works/)) and an
independent teardown ([databases.systems](https://databases.systems/posts/open-source-antithesis-p1)):

- Determinism is **not** the only hard part: "achieving thorough and efficient exploration of
  the state space … is a complex undertaking as well," which is why DST is paired with
  property-based testing/fuzzing and fault injection.
- **"To tell if a particular state is a bug, Antithesis relies on properties you define."**
  The platform explores; *your* properties judge (crash/hang classes aside). Determinism also
  powers the search itself (snapshot/restore, "a multiverse of branching execution paths").
- The teardown decomposes DST into four separable components — deterministic testbed, guided
  explorer, fault injector, report generator — with FoundationDB's `BUGGIFY` macros and
  TigerBeetle's checksum state-checker as the DIY oracle examples.

### 1.4 The synthesis (explicitly inference, confidence: low)

Both primary sources converge from opposite directions: the practitioner puts assertions ahead
of its simulator; the vendor says the simulator cannot judge without supplied properties. For a
harness whose acceptance criteria are mostly example tests, **the next rung is executable
oracles inside the code — before any simulator, model checker, or prover.** Nothing in the
verified corpus measures the cost or yield of any rung; treat as a cheap local experiment
(§4), not a scaling decision.

---

## 2. The gap areas (follow-up passes; single-sourced, lower confidence)

> The main sweep's fetch agents did reach these areas but none of their claims made the top-8
> adversarial-verification cut; the two targeted follow-up passes below carry them at
> follow-up confidence (single-sourced, memory-flagged items marked).

### 2.1 Model checking in industry (TLA+/TLC, Alloy, P, Kani/CBMC) — follow-up pass

**TLA+ is the proven cheap tier, and it is design-level, never code-level.**
[Lamport's industrial-use page](https://lamport.azurewebsites.net/tla/industrial-use.html)
(updated Oct 2025) documents Intel (cache-coherence, zero coherence bugs on silicon), AWS
(since 2011 — "Formal methods find bugs in system designs that cannot be found through any
other technique we know of"), Microsoft (Xbox 360 memory-coherence deadlock; Azure; Cosmos DB
consistency bug found pre-implementation), Dropbox, Elastic (replication bug found before
production), Confluent/Kafka (four data-loss edge cases). Governance: the
[TLA+ Foundation](https://foundation.tlapl.us/) (Linux Foundation umbrella; AWS/Oracle/NVIDIA)
runs grants and a "GenAI-accelerated TLA+ challenge." AWS's current umbrella statement is
"Systems Correctness Practices at AWS" ([CACM 2025](https://dl.acm.org/doi/10.1145/3729175)).
Alloy 6 remains niche ([alloytools.org](https://alloytools.org/)).

**The P language** graduated from Microsoft's USB stack to AWS flagship services (S3's
strong-consistency migration, EBS, DynamoDB) — communicating state machines, design-level;
**PObserve validates production logs against P specs at runtime**
([case studies](https://p-org.github.io/P/casestudies/)) — the model-checking-to-runtime
bridge.

**Code-level bounded model checking has a real cost datapoint now:**
[verify-rust-std](https://arxiv.org/html/2510.01072v3) (AWS + Rust Foundation, $5–25k bounties,
≥50 contributors, ~1 year): 27 challenges, 9 solved, **~4% of core's unsafe-function surface
contracted with Kani, zero safety bugs found, two spec bugs**. That is what code-proof coverage
costs even with money and a community. [Kani](https://model-checking.github.io/kani/) itself is
solid for targeted properties (UB, panics, user assertions in Rust); concurrency unsupported.

**LLM-assisted model checking is early:** LLMs do not yet write reliable TLA+ (best public
baseline 26.6% parse / **8.6% semantic model-check success**,
[TLA-Prover](https://arxiv.org/html/2606.06133v3);
["Can LLMs Write Correct TLA+?"](https://arxiv.org/abs/2606.05792)), though agentic
push-button systems are appearing ([Specula](https://arxiv.org/html/2607.25333)).

### 2.2 Proof assistants and verification-aware languages — follow-up pass

- **Lean 4:** mathlib >1.5M lines; the industrial beachhead is **AWS Cedar** — formal model +
  proofs in Lean with **differential randomized testing of the production Rust engine against
  the model** ([cedar-spec](https://github.com/cedar-policy/cedar-spec), started in Dafny).
  This "executable model + differential testing" shape, not full proof, is the adoptable
  pattern.
- **Rocq:** the Coq rename is complete (9.0.0, Mar 2025; [rocq-prover.org](https://rocq-prover.org/releases/9.0.0)).
- **Dafny:** stable 4.x cadence; AWS's original Cedar models and crypto tooling
  ([amazon.science](https://www.amazon.science/blog/how-we-built-cedar-with-automated-reasoning-and-differential-testing)).
- **Verus:** the research momentum leader — OSDI'24 Best Papers (Anvil verified K8s-style
  controllers; VeriSMo for AMD SEV-SNP), SOSP'25 systems, USENIX'25 artifacts
  ([projects list](https://verus-lang.github.io/verus/publications-and-projects/)) — but still
  breaking-changes unstable, and every listed project is researcher-built. LLM-assist:
  AutoVerus reports >90% proof generation on a 150-task benchmark
  ([arXiv 2409.13082](https://arxiv.org/pdf/2409.13082)).
- **F*/HACL\*:** the real production story — verified crypto in Firefox NSS, the Linux kernel,
  WireGuard, Python 3.12's hash implementations (last item from memory)
  ([hacl-star.github.io](https://hacl-star.github.io/)).
- **seL4 anchors the top rung's price:** ~10 KLOC verified ↔ proof base now >1M lines of
  Isabelle, **zero functional-correctness defects in the verified code in 15+ years**
  ([whitepaper](https://sel4.systems/About/seL4-whitepaper.pdf),
  [proofs](https://sel4.systems/Verification/proofs.html)); canonical effort ~20 person-years
  (≈2.3 py/KLOC; exact dollar figures from memory — verify before quoting).
- **The tiering that emerges:** design-level model checking = weeks to learn, days-to-weeks
  per spec, repeatedly finds production-grade bugs. Code-level proof = person-years per KLOC
  or bounty-community scale for percent-level coverage. **No public datapoint exists for
  "verified X per engineer-month, non-specialist team"** — the field's own version of our
  missing sample-size prescription.

### 2.3 The 2025–2026 LLM×formal-methods wave (follow-up pass)

**Verified code generation is real and language-lopsided.** The vericoding benchmark (12,504
formal specs, POPL 2026 Dafny workshop) puts spec-to-verified-code success at **~82% Dafny,
~44% Verus/Rust, ~27% Lean** — success tracks training-data availability, not language power
([alphaxiv 2509.22908](https://www.alphaxiv.org/overview/2509.22908v1)). DafnyBench went
**68% → 96% in roughly a year** (saturating); DafnyPro hits 86% proof success on it
([POPL 2026](https://popl26.sigplan.org/details/dafny-2026-papers/12/DafnyPro-LLM-Assisted-Automated-Verification-for-Dafny-Programs)).
But **end-to-end NL → code+spec+proof is still the hard part**: Verina's best general model
scores 72.6% code / 52.3% sound specs / **4.9% proofs**
([arXiv 2505.23135](https://arxiv.org/html/2505.23135v3)); VerifyThisBench sits near the floor
([openreview](https://openreview.net/pdf?id=4MuxGYjAYO)). The 2026 wave is adversarial and
anti-reward-hacking: AlphaVerus adds a critique phase that blocks vacuous/reward-hacked specs
([CMU, ICML 2025](http://www.contrib.andrew.cmu.edu/~bparno/papers/alpha-verus.pdf));
**AxDafny gates agentic codegen behind deterministic checks plus a reviewer LLM screening for
proof-bypass "cheating patterns"** (no `assume`, no bypass constructs)
([arXiv 2606.32007](https://arxiv.org/html/2606.32007)) — the clearest published instance of a
membrane with anti-reward-hacking guards.

**Clover** (Stanford Barrett group) is the cheap-pattern standout: closed-loop
**code/docstring/formal-annotation mutual-consistency checking**, accepting up to 87% of
correct programs with **zero false positives on adversarial incorrect variants**
([arXiv 2310.17807](https://arxiv.org/abs/2310.17807)). Trust from triangulated consistency,
not from proof alone.

**LLM-written properties, quantified** (directly relevant to our false-PASS thesis):
Claude-4-Sonnet-generated property-based tests alone detect 68.75% of planted bugs — same as
example tests alone — but **combined reach 81.25%**
([arXiv 2510.25297](https://arxiv.org/abs/2510.25297)); and an ACM 2025 study finds **30–32%
of LLM-generated solutions only partially satisfy correctness properties and 18–23% fail
outright — unit-test-only evaluation overestimates correctness**
([ACM](https://dl.acm.org/doi/pdf/10.1145/3696630.3728702)). That last sentence is the
external, quantified version of agentops' false-PASS claim.

**Neural theorem proving** crossed the credibility line: AlphaProof in *Nature* (Nov 2025)
([nature.com](https://www.nature.com/articles/s41586-025-09833-y)); Goedel-Prover-V2-32B at
88% miniF2F ([arXiv 2508.03613](https://arxiv.org/pdf/2508.03613)); Seed-Prover saturating
miniF2F and proving 5/6 IMO 2025 problems in Lean
([arXiv 2507.23726](https://arxiv.org/abs/2507.23726)). Math, not yet software — but it moves
the "LLMs will drop the cost of proof itself" argument from speculation to trend.

**AWS is the only shipped productization of formal verification over LLM output:** Bedrock
Guardrails **Automated Reasoning checks**, GA Aug 2025 — upload a rules document, the service
extracts a formal-logic policy (with a fidelity report), a solver checks each LLM response
against it, findings are policy-relative and **detect-only**; the NL→logic translation is
itself an FM step
([AWS docs](https://docs.aws.amazon.com/bedrock/latest/userguide/guardrails-automated-reasoning-checks.html)).
Lineage: Zelkova, Cedar's verification-guided development
([arXiv 2403.04651](https://arxiv.org/abs/2403.04651)), and S3 ShardStore's "lightweight
formal methods" (PBT + stateless model checking in production, SOSP 2021,
[amazon.science](https://www.amazon.science/publications/using-lightweight-formal-methods-to-validate-a-key-value-storage-node-in-amazon-s3)).

### 2.4 Runtime verification / contracts / invariant monitoring (follow-up pass)

The middle ground is being **reinvented in agent-safety papers rather than reported from the
DbC library ecosystem**: Agent Behavioral Contracts (formal pre/post/invariant + recovery
clauses with runtime enforcement, [arXiv 2602.22302](https://arxiv.org/html/2602.22302v1)),
Agent-C (SMT-enforced temporal constraints during generation), VeriGuard (offline policy
verification + online monitoring). One practitioner pipeline worth reading whole: Bhatti's
triple-engine design (LLM generates → Dafny/TLA+/Z3 verify → LLM assists specs) with an
explicit tiered-rigor table (unit tests → runtime contracts → deductive verification), aimed
at the **"critical 20%"** (authz, crypto, financial calc, consensus), claiming ~20% time
overhead shifted from debugging to spec-writing
([plexobject](https://weblog.plexobject.com/archives/7799)). Gap flag: no strong 2025–26
writing found on icontract/deal/Rust-Go contract crates deployed specifically against
AI-generated-code defects (absence claim, unverified).

### 2.5 Practitioner cost-ladder statements (follow-up pass)

- **seL4:** 8,700 LoC of C ↔ ~20 person-years and 200k lines of Isabelle — "23 lines of proof
  and half a person-day per line of implementation"
  ([Congdon, Dec 2025](https://benjamincongdon.me/blog/2025/12/12/The-Coming-Need-for-Formal-Specification/)).
  CompCert ~6 person-years (from memory, unverified). That is the top rung's price tag.
- **Hillel Wayne's economics:** the payoff concentrates in **design-level verification** —
  model the design, verify the model, avoid building the bug; full code proof only at extreme
  stakes ([pragmaticengineer interview](https://newsletter.pragmaticengineer.com/p/formal-methods-with-hillel-wayne)).
- **AWS's learnability claim:** engineers learn TLA+ in ~2–3 weeks (CACM 2015; from memory,
  canonical: [cacm](https://cacm.acm.org/research/how-amazon-web-services-uses-formal-methods/)).
- **NIST's long-standing position:** the cost-effective frontier is the **union of formal
  methods with testing**, not full proof
  ([NIST](https://csrc.nist.rip/staff/Kuhn/kuhn-chandramouli-butler-02.pdf)).
- **Two 2025–26 arguments bend the ladder** (Congdon): LLMs are dropping the marginal cost of
  writing specs and proofs, while AI-generated code volume raises the value of
  machine-checkable acceptance criteria — code-writing cost is falling faster than review
  cost, so the oracle side compounds in value. Bottleneck: "a few hundred people in the world"
  hold the expertise.

---

## 3. Refuted / do-not-cite (single-vote kills — weak evidence of falsity, strong reason to re-verify before use)

| Refuted claim | Note |
|---|---|
| "Assertions are the oracle, fuzzers only explore → DST without assertion density yields little" | plausible, did not survive; genuinely open |
| "DIY FoundationDB-style DST is greenfield-only / impractical to retrofit" | vendor-attributed, did not survive; the minimum-viable-determinism question is open |
| "DST yields value from seed variation alone, without properties" | cuts the *other* way from row 1 — both refuted, the pair brackets an open question |
| arXiv 2506.18315's numbers: +13.4% pass@1 from property-violation feedback; >64% of failed problems fixed | do not cite without re-verification |
| Its mechanism claim (PBT with shrinking in the agent loop; hand the model the simplest counterexample) | the *pattern* is still worth piloting; the evidence for it is unproven here |

## Open questions (ranked by load-bearing-ness for agentops)

1. **Can an agent write assertions for its own code that catch what its own tests miss?** The
   self-grading failure mode, unresolved in the literature, directly measurable in our harness.
2. **Where is the cost/benefit knee?** No practitioner cost-per-rung data survived. The
   follow-up pass gathers what exists; expect it thin.
3. **Minimum viable determinism for a non-greenfield harness** — controlled clock/scheduler/
   seeded replay incrementally, without a hypervisor platform.
4. **Does assertion density correlate with outcomes at all**, or is P10 Rule 5 a two-decade
   convention surviving on authority? Nobody has published the effect size. We can.

---

## 4. What this means for agentops (maps to [eval-architecture.md](../architecture/eval-architecture.md))

1. **The oracle-scarcity frame is the product frame.** Agentops' membrane thesis restated by
   the formal-methods world: executors (models, simulators) are abundant; judges are scarce.
   Every eval-architecture decision that manufactures deterministic oracle — verifiers,
   invariant assertions in fixtures, claim-vs-truth extraction — is the formal-methods ladder
   being climbed from the bottom, which is where its own practitioners say the leverage is.
2. **New wave-2 skill: `tiger-style` (assertion density for agent-generated code).**
   Falsifiable promise: with the skill loaded, generated code averages ≥2 *killable* assertions
   per function (arguments, return values, pre/postconditions, invariants). Discriminator:
   deterministic — count assertions in the diff AND mutation-check that each can fail (restoring
   the NASA guard TigerBeetle dropped; anti-Goodhart pairing per D1). Outcome metric: **escape
   rate** — bugs that survive the agent's own tests but are caught by fault-injection tasks —
   with false-PASS rate as the paired guard. This eval doubles as the answer to open question
   #1: run fault-injection fixtures where the planted defect passes the agent's tests; measure
   whether skill-arm assertions fire where control-arm tests stayed green.
3. **D7 stands, with the rungs reordered by evidence:** hermetic fixtures (have) → **assertion/
   invariant density in both fixtures and generated code (the verified cheapest rung)** →
   property-based tests + fault injection (the explorer, valuable only once oracles exist) →
   metamorphic variants (contamination + robustness) → model checking/proofs only where the
   follow-up evidence says the cost is paid back (§2, pending).
4. **The `ao` CLI keeps its conventional-verification lane** (property tests + fuzzing in CI);
   any TLA+/Kani-grade investment waits on §2's cost evidence and applies to the few
   protocol-shaped components (pawl/locking, verdict state machine), not the estate.
5. **The false-PASS metric now has external quantification:** 18–23% of LLM-generated
   solutions fail correctness properties outright while "unit-test-only evaluation
   overestimates correctness" (ACM 2025, §2.3). That is the published version of our thesis —
   cite it whenever the false-PASS headline metric needs external grounding. And the +12.5pp
   from *combining* LLM-written property tests with example tests justifies adding a
   properties clause to fixture `score.sh` where tasks admit one.
6. **AxDafny's cheat-pattern reviewer is prior art for the membrane.** A reviewer explicitly
   screening for proof-bypass constructs (`assume`, vacuous specs) before acceptance is our
   `validate` + `verdictcheck` (Go gate) posture, published. Absorb its checklist into the validator prompt;
   Clover's triangle (code ↔ doc ↔ spec mutual consistency, zero false positives on
   adversarial variants) is the cheap generalization: our claim-vs-truth instrument extends
   naturally to {diff ↔ stated claim ↔ tests} consistency.
7. **Wave-3 option, explicitly deferred:** with Dafny auto-verification at 82–86% and repair
   loops maturing (ExVerus), a `verified-kernel` skill — route Bhatti's "critical 20%"
   (authz, crypto, money, consensus) through a Dafny/Verus-verified kernel — is plausibly
   within reach. Deferred until the eval program is running and §2's cost evidence is in;
   end-to-end NL→proof at 4.9% (Verina) says the fully-automatic version is not yet real.
8. **The two adoptable industry patterns for `ao` itself** (from §2.1–2.2): the **Cedar
   shape** — a small executable reference model of the verdict/pawl state machine,
   differential-tested against the production Go implementation (model in Go or TLA+-checked;
   full proof never) — and the **PObserve shape** — validate production gate/verdict logs
   against the declared state machine at runtime, turning every real session into a
   conformance test. Both are weeks-not-years investments with precedent at AWS scale.
