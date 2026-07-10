# AgentOps-native skill adoption — frozen behaviors

Status: frozen before implementation on 2026-07-09.

This packet replaces selected external-skill habits with AgentOps-owned behavior.
It does not copy protected prose, scripts, examples, fixed idea counts, or provider
mechanics. The official external corpus and local companions are evidence inputs;
the contracts below are written from AgentOps product doctrine and live repo seams.

## B1 — Evidence-grounded idea generation

```gherkin
Scenario: Produce a BDD-ready opportunity portfolio
  Given an operator has an open-ended product or engineering question
  And the repository rules, product goals, live skills, CLI surface, and beads are readable
  When idea-genie explores the opportunity space
  Then it separates observations, assumptions, and candidate mechanisms
  And it reconciles every survivor against existing capabilities and work
  And it stops when a new pass produces no materially new candidate
  And it writes an `idea-portfolio.v1` artifact without creating tracker rows
  And discovery remains the sole owner of BDD intent packets and persistence

Scenario: Return no new work honestly
  Given every supported candidate duplicates existing behavior or lacks evidence
  When idea-genie evaluates the candidates
  Then it returns a no-new-work outcome with overlap evidence
  And it does not manufacture a backlog to satisfy a fixed count
```

## B2 — Independent idea challenge

```gherkin
Scenario: Challenge a contested one-way-door idea
  Given a decision materially changes a port, public contract, or irreversible posture
  When dueling-idea-genies runs
  Then independent contexts generate proposals before seeing one another
  And each proposal is cross-reviewed by rubric dimension
  And disagreements and attempted refutations survive synthesis
  And the result is an `idea-challenge.v1` packet for the existing plan-pawl decider
  And `ao plan-pawl decide` remains the sole owner of PASS/REDO/BLOCKED

Scenario: Avoid ceremony for a reversible choice
  Given the decision is cheap to reverse and one context can produce adequate evidence
  When dueling-idea-genies classifies the door
  Then it routes to idea-genie or a single fresh-context challenge
  And it does not require NTM, Agent Mail, or a multi-model panel
```

## B3 — Improve the existing codebase-recon capability

```gherkin
Scenario: Reconstruct a repository with reusable evidence
  Given a repository and its local source-of-truth precedence
  When codebase-recon runs
  Then it traces representative entry-to-domain-to-integration-to-test paths
  And it labels fact, inference, unknown, confidence, and uninspected scope
  And it separates mental model, bounded audit, pattern evidence, and synthesis
  And every material claim cites a repository path or executable observation

Scenario: Prefer a verified delta over another replacement report
  Given a prior codebase-recon pack exists
  When codebase-recon runs again
  Then it verifies the prior baseline against the current commit
  And it writes a delta plus any changed synthesis
  And it preserves still-valid prior evidence instead of regenerating it blindly
```

## B4 — Mine patterns only when the abstraction is earned

```gherkin
Scenario: Promote a recurring implementation pattern
  Given at least three independent repository exemplars
  When pattern-mining evaluates them
  Then it separates invariants, variation points, and incidental similarity
  And it tests the candidate abstraction against every exemplar and a holdout
  And it routes the result through operationalize to a skill, gate, library, template, or no-action outcome

Scenario: Keep a weak pattern as a hypothesis
  Given fewer than three independent exemplars or a failed holdout
  When pattern-mining evaluates the candidate
  Then it records a bounded hypothesis
  And it does not package the hypothesis as a reusable skill or rule
```

## B5 — Drive factory agents through a substrate-neutral lifecycle

```gherkin
Scenario: Run a role-shaped software factory through agent-native
  Given a bead and acceptance contract name a worker role, workspace ownership, and bounded restart policy
  And persistent attachable panes materially improve the run
  When the NTM AgentWorker adapter starts the worker through the AgentWorker lifecycle
  Then orientation and readiness complete before work delivery
  And engagement is proven rather than inferred from send success
  And the worker dispatches whole AgentOps loop skills rather than duplicating their phases
  And NTM robot state, transcripts, artifacts, and terminal state remain observable
  And Agent Mail provides identity, reservations, acknowledgements, and handoffs only when multiple live lanes require them
  And success requires usable persisted evidence plus clean handoff or retirement

Scenario: Refuse an unjustified factory
  Given the work is one-shot, sequential, or has colliding write scopes
  When automation-shape-routing and agent-native evaluate the work shape
  Then it routes to one native Codex agent, an in-session bounded fanout, or a sequential bead chain
  And it does not start NTM or Agent Mail merely because they are installed
```

## B6 — Drive pawl reviewer lanes without conflating transport and judgment

```gherkin
Scenario: Route a pawl review through NTM reviewer panes
  Given ao pawl requires fresh independent oracle evidence
  When pawl-review dispatches an immutable ReviewRequestV1 through ReviewLanePort
  Then NTM may own warm-pane lifecycle and liveness
  And the NTM review-lane adapter returns ReviewLaneResultV1 with the echoed nonce
  And pawl-review owns the immutable request, fresh-context lane result, and evidence handoff
  And ao pawl owns deterministic diversity decisions, verdict binding, and fail-closed admission
  And the author is never counted as an oracle
  And Agent Mail is optional unless separate live actors need acknowledgements or reservations

Scenario: Hold when reviewer evidence is unavailable
  Given the requested pawl tier cannot produce its required independent evidence
  When the route deadline or breaker trips
  Then transport loss is classified separately from a semantic refutation
  And no CONFIRMED verdict is written
  And it never fabricates a refutation or silently lowers the required tier
```

## B7 — Keep Gas City additive and composable

```gherkin
Scenario: Let a Gas City factory delegate through portable worker and review lanes
  Given an operator explicitly chose Gas City for city-shaped work
  And a quest contains a bounded role topology better served by attachable NTM panes
  When the GC driver delegates through AgentWorker or ReviewLanePort
  Then agent-native or pawl-review receives the same portable work contract
  And Agent Mail may coordinate the delegated panes
  And the GC membrane remains the quest close door
  And neither substrate becomes an automatic fallback for the other

Scenario: Emit a canonical GC pawl verdict
  Given a GC membrane reviewer panel has completed
  When the pack finalizes its result
  Then the artifact validates against schemas/pawl-verdict.v1.schema.json
  And every evidence path names a file that exists
  And transport degradation is not encoded as a semantic disposition
  And native GC may still consume the failed check attempt according to its dispatcher contract

Scenario: Run either substrate independently
  Given only GC or only NTM is available
  When the chosen factory executes
  Then it remains operable without the other substrate
  And the AgentOps loop and membrane contracts stay invariant
```

## B8 — Retire duplicate ATM-era skill surfaces without losing behavior

```gherkin
Scenario: Migrate callers to AgentOps-owned NTM language
  Given using-atm and retired vibing terminology overlap agent-native, pawl-review, ntm, and agent-mail
  When the migration lands
  Then using-atm is recorded as merged into agent-native with split destinations documented
  And pre-land-refuters is recorded as merged into pawl-review with an additive alias route
  And active routers, profiles, skill docs, and Codex twins point to agent-native, pawl-review, ntm, or agent-mail
  And ATM-specific alias mechanics are absent from canonical AgentOps skill contracts
  And tests prove the tending, pane lifecycle, and coordination behaviors still have owners

Scenario: Preserve external tool boundaries
  Given NTM and Agent Mail are external executables behind AgentOps ports
  When AgentOps skills drive them
  Then the skills discover live capabilities before state changes
  And executable behavior outranks remembered command syntax
  And no AgentOps skill claims ownership of the external binaries
```

## B9 — Mesh skills through real entry points

```gherkin
Scenario: Route every new capability from an entry-point owner
  Given a new AgentOps skill has been admitted
  When the generated and curated skill maps are rebuilt
  Then at least one existing entry-point skill names when to invoke it
  And the new skill names its outbound dependencies and artifact handoff
  And context relationships describe the direction without creating a cycle
  And a user can discover the route without already knowing the leaf skill name

Scenario: Keep the mesh behavior-oriented
  Given several entry points can reach the same capability
  When their references are wired
  Then each entry point names the behavior or decision boundary it delegates
  And it does not duplicate the leaf skill workflow
  And generated maps agree with source frontmatter and the disposition ledger
```

## B10 — Generate a non-stale skill graph

```gherkin
Scenario: Regenerate the complete skill graph after a skill edit
  Given every live skill declares metadata in its SKILL.md frontmatter
  When make regen-all runs
  Then the generated skill catalog contains every live skill exactly once
  And a generated skill-graph document contains every live skill exactly once
  And typed dependency and context-relationship edges come only from source frontmatter
  And the graph reports entry points, zero-inbound skills, dangling targets, and dependency cycles
  And `user-invocable` alone cannot disguise an unmeshed leaf as a graph root

Scenario: Block graph drift and broken topology
  Given a skill is added, retired, renamed, or rewired
  When make regen-check runs without regenerating the graph
  Then the check fails with the exact regeneration command
  And dangling skill targets or dependency cycles fail even after regeneration
  And only an explicitly declared graph root may remain zero-inbound without failing validation
```

The existing generated `docs/contracts/context-map.md` is the maintained human
projection. `skills/catalog.json` is the machine projection and `ao skills
graph` is the query/render surface. Graphify may import either projection for
exploration, but it is not a source of truth and no Graphify database is
committed.

## Non-goals

- Do not add `runtime=gc`, `ao gc`, a daemon, or automatic GC routing.
- Do not recreate the external package names or copy their prose/scripts/examples.
- Do not make NTM or Agent Mail a single-writer startup tax.
- Do not create a second loop beside discovery/RPI/crank/validate.
- Do not make a same-author or same-context vote count as independent pawl evidence.
- Do not replace the existing `codebase-recon` name with `repo-recon`.
- Do not ship a new skill with no inbound route from an existing entry point.
- Do not hand-maintain node or edge lists in the generated graph document.

## Rollback

Rollback is per slice: revert the slice commit, restore any retired source skill
from its parent commit, regenerate catalogs/twins/context-map, and rerun that
slice's acceptance test. Go ports/adapters and GC pack changes roll back with
their own commits before dependent routing slices; runtime binaries, bead
history, GC cities, and Agent Mail state are never mutated by installation.
