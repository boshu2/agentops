# Skill Lease Audit

> **Status:** V0 audit.
> **Owner bead:** `soc-m6v5.9.9.2`.
> **Purpose:** classify every shared skill by lease-on-life disposition without
> cutting, merging, or rewriting skills prematurely.

This audit answers which skills have a clear lease, which skills are candidates
for merge/split/retire work, and which skills need evidence before a decision.
It depends on the [Skill Domain Map](skill-domain-map.md) and uses the
vocabulary in [Skill Ports and Adapters](skill-ports-and-adapters.md).

## Rule

This audit does not delete, merge, or rewrite any skill. A non-`keep`
disposition only creates a future proof slice.

```gherkin
Feature: Skill lease audit
  Scenario: A skill appears redundant
    Given a mapped skill with overlapping intent and output contract
    When the lease audit classifies it as merge-candidate
    Then the audit records the evidence
    And no merge happens until a later proof bead validates the replacement
```

## Disposition Summary

| Disposition | Count | Meaning |
|---|---:|---|
| `keep` | 69 | Clear current responsibility and no evidence-backed cut this wave |
| `merge-candidate` | 3 | Overlap exists; prove merge or rename in a future slice |
| `split-candidate` | 2 | Skill has multiple responsibilities or phase/runtime concerns |
| `retire-candidate` | 1 | Replacement path appears likely; prove before removal |
| `unknown` | 2 | Needs usage/evidence before keep or cut decision |
| **Total** | **77** | Full shared `skills/*/SKILL.md` catalog |

## Candidate Decisions

| Skill | Disposition | Evidence | Follow-up bead shape |
|---|---|---|---|
| `codex-team` | `merge-candidate` | Overlaps with `swarm` on multi-agent dispatch while also owning Codex CLI background-process details. | Compare `codex-team` vs `swarm` ports; keep only the Codex runtime adapter behavior outside `swarm` if proven. |
| `discovery` | `split-candidate` | Shapes intent, delegates several child skills, creates packets, persists issues, and has known token-bloat pressure. | Pilot Discovery-to-Plan boundary through `soc-m6v5.9.9.4`; split policy, orchestration, and adapter prose only if the pilot proves value. |
| `grafana-platform-dashboard` | `unknown` | Specialized dashboard adapter; product role is not obvious from the general skill-domain map. | Decide whether it is a retained exemplar vertical adapter or should move outside the core catalog. |
| `inject` | `retire-candidate` | `ao inject` is deprecated and current direction favors `ao context assemble`, `ao lookup`, and knowledge briefs over resident context injection. | Reconcile with CDLC docs and context assembly; prove replacement before retiring the skill. |
| `llm-wiki` | `unknown` | Skill body says the proposal is open and has no implementation yet. | Validate whether wiki behavior belongs in `compile`/`knowledge-activation` or deserves an implemented standalone skill. |
| `rpi` | `split-candidate` | Full lifecycle orchestrator crosses Discovery, Crank, and Validation; prior measurement shows phase context isolation is the main token-reduction prize. | Split phase packets/fresh-context contracts before changing CLI; align with operator-loop token-reduction work. |
| `security-suite` | `merge-candidate` | Overlaps with `security` while adding deterministic suite/red-team composition. | Prove whether `security-suite` is a mode of `security` or a distinct composition adapter. |
| `validate` | `merge-candidate` | Skill says it was introduced additively while older validators remain; name overlaps with `validation`, `vibe`, `pre-mortem`, and `council`. | Clarify canonical validation ports and whether `/validate` becomes the facade while others remain adapters. |

## Full Roster

| Skill domain | Skill | Disposition | Lease reason |
|---|---|---|---|
| Domain Language and Standards | `domain` | `keep` | Owns ubiquitous language; core DDD surface. |
| Domain Language and Standards | `shared` | `keep` | Owns reusable AgentOps skill contracts. |
| Domain Language and Standards | `standards` | `keep` | Provides coding and architecture standards. |
| Domain Language and Standards | `using-agentops` | `keep` | Explains operator workflow; useful as onboarding adapter. |
| Product and Discovery | `brainstorm` | `keep` | Produces BDD-shaped intent before discovery. |
| Product and Discovery | `design` | `keep` | Product-fit gate before expensive discovery. |
| Product and Discovery | `discovery` | `split-candidate` | Dense packet maker plus orchestration; pilot the boundary before splitting. |
| Product and Discovery | `grafana-platform-dashboard` | `unknown` | Specialized adapter needs product-surface evidence. |
| Product and Discovery | `openai-docs` | `keep` | External-doc adapter for OpenAI surfaces. |
| Product and Discovery | `product` | `keep` | Product positioning and goal-shaping source. |
| Product and Discovery | `research` | `keep` | Turns code/facts into findings and evidence. |
| Product and Discovery | `reverse-engineer-rpi` | `keep` | Useful while RPI behavior is being decomposed. |
| Planning and Work Graph | `autodev` | `keep` | Manages bounded autonomous development loops. |
| Planning and Work Graph | `beads` | `keep` | Issue graph adapter; no replacement in this wave. |
| Planning and Work Graph | `goals` | `keep` | Owns measurable product fitness directives. |
| Planning and Work Graph | `plan` | `keep` | Slice and wave planning core. |
| Planning and Work Graph | `scenario` | `keep` | Holdout scenario boundary for behavioral validation. |
| Execution and Orchestration | `codex-team` | `merge-candidate` | Overlaps with `swarm`; Codex-specific adapter may remain. |
| Execution and Orchestration | `crank` | `keep` | Epic/wave execution core. |
| Execution and Orchestration | `dream` | `keep` | Scheduled compounding execution surface. |
| Execution and Orchestration | `evolve` | `keep` | Autonomous improvement loop. |
| Execution and Orchestration | `implement` | `keep` | One-slice TDD implementation surface. |
| Execution and Orchestration | `refactor` | `keep` | Bounded refactor workflow. |
| Execution and Orchestration | `rpi` | `split-candidate` | Lifecycle orchestrator crosses several domains and token contexts. |
| Execution and Orchestration | `scaffold` | `keep` | Creates implementation starting surfaces. |
| Execution and Orchestration | `swarm` | `keep` | Parallel worker dispatch primitive. |
| Validation and Risk | `bug-hunt` | `keep` | Root-cause investigation surface. |
| Validation and Risk | `complexity` | `keep` | Focused complexity/refactor hotspot scanner. |
| Validation and Risk | `council` | `keep` | Multi-judge consensus primitive. |
| Validation and Risk | `deps` | `keep` | Dependency-risk adapter. |
| Validation and Risk | `perf` | `keep` | Performance evidence and optimization skill. |
| Validation and Risk | `pre-mortem` | `keep` | Pre-implementation risk gate. |
| Validation and Risk | `red-team` | `keep` | Adversarial docs/skills/code probe. |
| Validation and Risk | `review` | `keep` | Code and diff review adapter. |
| Validation and Risk | `scope` | `keep` | Scope guard adapter. |
| Validation and Risk | `security` | `keep` | General repository security scan surface. |
| Validation and Risk | `security-suite` | `merge-candidate` | Overlaps with `security` but may own composition/red-team suite. |
| Validation and Risk | `test` | `keep` | Test generation and coverage planning. |
| Validation and Risk | `validate` | `merge-candidate` | Additive validator facade overlaps with older validator skills. |
| Validation and Risk | `validation` | `keep` | Post-implementation closeout orchestrator. |
| Validation and Risk | `vibe` | `keep` | Per-slice code-readiness verdict. |
| Corpus and Learning Flywheel | `compile` | `keep` | Compiles `.agents` knowledge into durable surfaces. |
| Corpus and Learning Flywheel | `curate` | `keep` | Mines corpus and work state for durable deltas. |
| Corpus and Learning Flywheel | `flywheel` | `keep` | Health check for compounding context. |
| Corpus and Learning Flywheel | `forge` | `keep` | Mines transcripts into learnings. |
| Corpus and Learning Flywheel | `harvest` | `keep` | Promotes `.agents` knowledge. |
| Corpus and Learning Flywheel | `inject` | `retire-candidate` | Deprecated injection path should move toward context assembly/lookup. |
| Corpus and Learning Flywheel | `knowledge-activation` | `keep` | Activates mature knowledge into runtime context. |
| Corpus and Learning Flywheel | `llm-wiki` | `unknown` | Proposal/no-implementation status needs validation. |
| Corpus and Learning Flywheel | `post-mortem` | `keep` | Reviews completed work and emits learning input. |
| Corpus and Learning Flywheel | `provenance` | `keep` | Traces artifact origin and citations. |
| Corpus and Learning Flywheel | `ratchet` | `keep` | Records one-way progress gates. |
| Corpus and Learning Flywheel | `retro` | `keep` | Captures session learnings. |
| Corpus and Learning Flywheel | `trace` | `keep` | Traces decisions through artifacts. |
| Documentation and Skill Packaging | `bootstrap` | `keep` | Initializes project/install surfaces. |
| Documentation and Skill Packaging | `converter` | `keep` | Converts skill formats across runtimes. |
| Documentation and Skill Packaging | `doc` | `keep` | Generates and validates repo docs. |
| Documentation and Skill Packaging | `heal-skill` | `keep` | Repairs skill hygiene. |
| Documentation and Skill Packaging | `hooks-authoring` | `keep` | Retain as guard-adapter authoring until hook lease work proves otherwise. |
| Documentation and Skill Packaging | `oss-docs` | `keep` | OSS doc scaffold/audit surface. |
| Documentation and Skill Packaging | `readme` | `keep` | README-focused doc surface. |
| Documentation and Skill Packaging | `skill-auditor` | `keep` | Skill quality and density audit surface. |
| Documentation and Skill Packaging | `skill-builder` | `keep` | Skill scaffold/absorption surface. |
| Documentation and Skill Packaging | `update` | `keep` | Skill/plugin sync surface. |
| PR and Release Delivery | `pr-implement` | `keep` | OSS PR implementation adapter. |
| PR and Release Delivery | `pr-plan` | `keep` | OSS PR planning surface. |
| PR and Release Delivery | `pr-prep` | `keep` | PR commit/body preparation. |
| PR and Release Delivery | `pr-research` | `keep` | Upstream repository research adapter. |
| PR and Release Delivery | `pr-retro` | `keep` | PR outcome learning surface. |
| PR and Release Delivery | `pr-validate` | `keep` | PR-specific validation adapter. |
| PR and Release Delivery | `push` | `keep` | Commit/push gate adapter. |
| PR and Release Delivery | `release` | `keep` | Release-readiness gate. |
| Runtime Continuity and Operator State | `handoff` | `keep` | Compact continuity packet. |
| Runtime Continuity and Operator State | `quickstart` | `keep` | Next-action operator adapter. |
| Runtime Continuity and Operator State | `recover` | `keep` | Session recovery adapter. |
| Runtime Continuity and Operator State | `status` | `keep` | Current work/status report surface. |
| Runtime Continuity and Operator State | `system-tuning` | `keep` | System responsiveness and process hygiene. |

## Next Proof Slices

Do these in small slices, not as a catalog rewrite:

1. Prove the Discovery-to-Plan boundary with `soc-m6v5.9.9.4`.
2. Reconcile `inject` claims through `soc-m6v5.9.4.1` and packet-density work
   through `soc-2c1p.1`.
3. Compare `codex-team` and `swarm` by ports before changing either skill.
4. Decide whether `validate` is the validation facade or only one validator
   among `validation`, `vibe`, `pre-mortem`, and `council`.
5. Classify `grafana-platform-dashboard` and `llm-wiki` with usage evidence
   before moving them out of the core catalog.
