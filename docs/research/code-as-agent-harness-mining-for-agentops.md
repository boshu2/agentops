# Mining "Code as Agent Harness" (arXiv:2605.18747) for AgentOps

- **Source analysis:** `code-as-agent-harness-paper-analysis.md` (same directory) — read that first for the paper itself.
- **Purpose of this doc:** extract what the survey validates, names, warns about, or suggests for *this repo* — the AgentOps CLI, its operating loop (RPI → Plan → Implement → fresh Validate → durable verdict), its skills corpus, and its verification economics direction.
- **Date:** 2026-07-19

## 1. Direct validation — the paper independently arrives at AgentOps' core design

The survey (52+ authors, literature through early 2026) converges on positions AgentOps already implements. These are citable confirmations, not new work:

| AgentOps mechanism | Paper's independent formulation | Where |
|---|---|---|
| Fresh-context validation; "the context that authors a candidate cannot issue its binding PASS" | AgentCoder's Test Designer generates tests independently of the code to avoid "the mode-collapse problem where an agent's biased tests pass its own buggy code"; CANDOR audits oracles against the spec, never the implementation; single-agent systems lack "an independent verification channel" | §4.1.1 |
| Deterministic gates decide facts; model judges meaning | "Tools as deterministic sensors"; critique "should interpret sensor outputs rather than replace them" | §3.3.3, §3.4.4 |
| Verdict-gated closure; no verdict = not done | Correctness convergence is "the most principled" criterion; **implicit convergence** (fixed iteration budgets, no quality criterion) is "the most prevalent pattern" and the field's biggest gap | §4.3.2 |
| Termination on verification state, not model self-report | "Termination is governed by verification, not model confidence" | §3.4.4 |
| Plan/bead as durable contract (acceptance, scope, non-goals in the source) | "Planning as contract formation": a plan is "an explicit contract over the next state transition… a harness artifact rather than an unobserved reasoning trace" — files in scope, invariants, validation commands, rollback points, risky ops | §3.4.2 |
| Durable verdict artifacts + evidence offload (summaries in context, full logs on disk) | Context compaction with provenance: "compact summaries and resource identifiers rather than raw logs," full-fidelity artifacts preserved "for audit and replay" | §3.2.6 |
| The repo + git as the shared substrate for multi-lane work | The **Central Gap** position: most MAS use implicit file-only state, "the technical root of system brittleness"; the needed thing is a formal, persistent, queryable shared substrate combining repository (structure) and execution (behavior) views | §4.3 |
| Worktree isolation, file reservations, one-writer-per-file | "Parallel branches with merge" raises "unresolved questions of authority and consistency"; MAGIS's one-Developer-per-file is the cleanest surveyed conflict-avoidance pattern | §4.2.2 |
| Stale-view re-sync before acting (fetch → reset → cherry-pick discipline) | SyncMind formalizes belief-vs-ground-truth divergence \|B−S\| — stale-snapshot clobbering is a named, measured failure class, not folklore | §4.3.1 |
| Membrane scope-grind rule ("≥3 same-class refutes → stop and re-scope") | PairCoder's dead-end detection: same buggy code or same feedback recurring in loop history → stop repairing, switch strategy | §4.1.3 |
| Skills as the product; failure-repeats-twice → durable rule | LYRA: "convert transient human corrections into reusable executable skills"; Voyager's skill library; the frontier problem is "governing the library: forgetting, abstraction, grounding alignment" | §2.2.3, §5.1.3 |
| Evidence-gated memory promotion (no raw transcript stuffing) | MemGovern: "quality of stored experience matters more than its scale"; MemCoder persists only human-validated fixes; long-term memory needs write gates | §3.2.3–3.2.4 |
| dcg / permission guards / HITL on destructive ops | Three-tier permission model (read-only → sandbox-edit → full-access) with mandatory human gates at the top tier; "reduce permissions" as a first-class loop response | §3.4.3, §5.2.5 |
| Cross-family/quorum only at one-way doors (cost law) | Cost-ordered sensor ladder — cheap static checks first, expensive verification later; linguistic fast path + execution as binding oracle | §3.4.4, §4.4 |

**Use:** this table is ammunition for docs/positioning. AgentOps' operating contract reads like an implementation of the paper's §5.2 wishlist. When writing public-facing material, the survey supplies neutral academic vocabulary for things we currently describe in house terms.

## 2. Vocabulary to adopt (theirs is better or citable)

- **"Harness engineering"** — the emerging discipline name for what AgentOps does. The paper treats it as a field ("toward a science of harness engineering"). Aligning our docs with this term connects us to the citation graph (OpenAI/LangChain/Anthropic harness-engineering guides are cited alongside).
- **"Deterministic sensors"** — better than "gates" when explaining *why* gates exist: sensors observe; the governor decides.
- **"Evidence bundle"** (§5.2.2) — the paper's name for what verdict.v2 + receipts already are: "the checks run, the assumptions preserved, the untested regions, and the remaining risks." Our `checked`/`not_checked` disclosure is exactly their "explicit scope" requirement.
- **"Change contract"** (§5.2.3) — for harness/skill mutations: component modified, failure mode targeted, predicted improvement, invariants preserved, falsifying evaluation, rollback path. A sharper template than a free-form commit message for skill-corpus changes.
- **"Oracle adequacy"** — the term for the "green test is not the full specification" trap; useful in validate/pawl docs.
- **"Executable accountability"** (§5.2.5) — approvals as auditable state transitions. Good frame for the verdict + provenance chain.
- **"Belief-state divergence"** (SyncMind) — the formal name for the stale-checkout / concurrent-lane clobber class our memories record.
- **"Lifecycle hooks"** (§3.3.4) — the paper's generic name for PreToolUse/PostToolUse-style control points; cite-able framing for cc-hooks.

## 3. Gaps the paper names that AgentOps already fills — positioning opportunities

The paper's §5.2 open problems are, in several cases, things this repo has working code for. That is a story worth telling deliberately:

1. **Implicit convergence is the field's default; verdict-gated closure is rare.** AgentOps' fail-closed verdict door (PASS/FAIL/NOT_PROVEN with NOT_PROVEN on unattested freshness, subject mutation, incomplete coverage) is precisely the "principled criterion for convergence" the survey says most systems lack.
2. **Failure attribution at 14–53% accuracy** (best published step-level attribution). Pawl refute transcripts + findings registry + per-bead verdict binding give attribution *by construction* — the verdict names the subject, scope, and evidence. Harvesting refute evidence before worktree cleanup (an existing operational lesson) is exactly the "structured traces needed for principled debugging" the paper says are missing.
3. **"No system unifies repository and execution views"** (§4.4 pattern 4). AgentOps' verdict binds a commit SHA (structure) to check receipts (behavior) in one artifact. That is the unification the survey calls the deepest version of the shared substrate. Worth making this explicit in architecture docs.
4. **Human feedback as durable harness state** (§5.2.5). The flywheel (failure repeats → planning rule/skill/constraint) and memory write-gating implement "each approval, rejection, or reviewer correction should update the harness's… verification criteria and future memory retrieval." Most of the field treats human input as ephemeral chat.
5. **Governed harness mutation** (§3.5.3). The skills corpus *is* our harness; the existing discipline (pawl review before skill lands, regen gates, holdout grading in bdd-foundry, promotion-by-allowlist) matches the paper's "promote only changes that improve reliability without regressing previously solved cases." The paper gives this a name and a five-stage loop (observe → diagnose → propose → evaluate → promote).

## 4. Concrete techniques worth adopting (candidate backlog)

Ranked by fit-to-effort. These are *ideas from the paper not currently implemented* (verify against live tree before filing beads — recon findings are hypotheses):

1. **Verifier scope declarations.** §5.2.2's core ask: each verifier declares *what it verifies, what it cannot verify, and what confidence it provides*. Our gates emit pass/fail receipts; adding a static `scope`/`cannot_verify` field per gate (and surfacing it in verdict evidence) would make `not_checked` disclosure derivable instead of hand-written. Small, high-leverage, directly on the verification-economics direction.
2. **Feedback-type routing.** §5.2.2: compiler error → syntax repair; test failure → behavioral diagnosis; coverage gap → test generation; reviewer conflict → arbitration. Today our loop treats most red signals uniformly. A routing table in the implement/validate skills (even prose-level) is cheap and matches the "epistemically aware" harness goal.
3. **Change contracts for skill/harness edits.** §5.2.3 template (target failure mode, predicted improvement, invariants, falsifying eval, rollback). Could be a lightweight frontmatter block on skill PRs or a bead template for harness-touching work. The pawl already supplies the falsifying-eval half.
4. **Dead-end detection as an explicit loop rule.** PairCoder's trigger (identical artifact or identical feedback twice in the retry history → switch strategy) is a mechanizable version of our "same-class refute ≥3 → re-scope" — tighter (2 not 3) and checkable by diffing consecutive attempts. Candidate for the membrane/andon logic.
5. **Deep-telemetry fields we don't capture:** *rejected alternatives* and *permission requests* per trajectory (§3.5.1). The paper argues these are what turn debugging into comparative diagnosis. Worth considering for pawl/flywheel record shapes.
6. **Trajectory-replay evaluation for harness changes.** §3.5.2: evaluate a proposed harness change against *replayed traces* of past runs, not just fresh runs — cheap regression evidence. We keep verdicts; we mostly don't keep replayable trajectories. (Cost caveat applies; fixture-fidelity rule applies — replay real traces, never synthetic.)
7. **Simulated-execution fast path.** QualityFlow's Imagined Execution (98%+ precision on MBPP) supports a cheap pre-verdict triage tier — but the paper is explicit that only real execution catches crashes/resource/boundary/perf classes, so it can gate *entry* to expensive verification, never *replace* the binding oracle. Fits the cheap-tier/ruler strategy.
8. **Environment + oracle co-synthesis** (SWE-smith, EnvScaler): when building eval corpora for skills/gates, generate the validator *with* the fixture. Aligns with the existing fixture-fidelity rule (round-trip the production writer/reader).
9. **KnowNo-style calibrated escalation.** Conformal-prediction-triggered clarify-before-act is a principled andon-trip criterion. Research-grade; note for the andon/helper-rung design, not near-term.

## 5. Warnings from the paper that apply to us

- **The green-test overconfidence trap (§5.2.2).** "A harness can become overconfident precisely because it has executable feedback." Our whole product is executable feedback; the mitigation is #1 above (scope-declared verifiers) plus continued adversarial/cross-family review at doors. Never let gate-green become the closure story on its own — this is why the verdict layer exists, and the paper says the quiet part: a weak verifier teaches the agent to optimize against the wrong signal.
- **Converging on self-authored tests (§4.3.2).** FlowGen's caution — a lane that writes both code and its acceptance tests can converge on its own bias. bdd-foundry's holdout grading (judges pull scenarios the implementing lane never sees) is the right shape; keep test-authorship independence a hard rule wherever acceptance tests are machine-authored.
- **Evaluation artifacts (§3 gaps).** Reported gains from planning/memory mechanisms can be artifacts of weak benchmarks. Applies to our own flywheel metrics: a measured improvement in refute rate is only as good as the corpus behind it (memory: "metric at exactly 0.00 → audit the instrument").
- **Silent action failures (§2.2).** Exit-code-clean operations can still fail semantically (the paper's grasp-outside-workspace example). Our analog: ops that "succeed" while doing the wrong thing (stale binary, wrong cwd, autostash drops). Post-hoc state-diff verification — compare intended vs realized state — is the general fix, and matches existing lessons (verify-landed before close; full-surface bounded verify).
- **Cost is unpriced in the paper.** Verification stacks, evidence bundles, transactional state all carry token/latency budgets the survey never quantifies. Our verification-economics direction (cheap tier feeding the corpus, quorum only at doors) is *ahead* of the literature here; don't import their maximalism wholesale.
- **Provisional citations.** Several load-bearing systems (AutoHarness, Meta-Harness, Aethelgard, AHE) are 2026 preprints or anonymous submissions. Treat their capability claims as direction, not proof.

## 6. One-paragraph synthesis

The survey names the discipline AgentOps practices — harness engineering — and independently derives its core commitments: deterministic sensors under a governing loop, plans as contracts, fresh independent verification, verdict-like evidence bundles with explicit scope, permission tiers with human gates, governed self-improvement with no-regression promotion, and a formal shared substrate unifying repo structure with execution behavior. Where the paper describes open problems, AgentOps has working mechanisms for several (verdict-gated convergence, structured failure evidence, repo+execution unification, durable human-feedback state); where it describes techniques we lack, the highest-fit imports are verifier scope declarations, feedback-type routing, change contracts for harness mutations, and two-strike dead-end detection. The main caution it sharpens: executable feedback breeds overconfidence exactly where the oracle is weakest, so verifier scope — not verifier count — is the next frontier for the membrane.
