# Gas City Role-Topology Audit — 2026-07-17

This directory is the historical evidence packet behind
[ADR-0015](../../adr/ADR-0015-gas-city-fenced-steward.md) and the canonical
[Gas City factory architecture](../../architecture/gas-city-factory.md). It
records what was inspected, which alternatives were independently proposed and
cross-scored, where the two judges converged, and what remains unproven.

This audit is evidence, not live product authority. Current executable behavior
comes from source, schemas, generated projections, and the current adapter
contract. Dated research cannot make a factory role real; only implemented and
qualified source can do that.

## Question

Which Gas City roles and mechanisms should AgentOps use to let an operator give
one Mayor a large product, fan independent work through Codex and Claude,
validate it with fresh LLM-as-judge contexts, prevent concurrent writers from
colliding, and deliver admitted work through protected PRs without rebuilding
the Mt Olympus cathedral?

The operator additionally fixed two requirements during the study:

1. Candidate work must use branch/worktree isolation and reach `main` only
   through protected PR delivery.
2. A binding `FAIL` or `NOT_PROVEN` returns to the Mayor for a newly scoped
   experiment with fresh workers; Validator or Refinery may not repair the
   rejected subject.

## Method

1. Inspect the current AgentOps operating loop, optional GC adapter, pack,
   provider roles, deployment, tests, and live qualification state.
2. Inspect Gas City's role-agnostic primitives and its official Gastown mapping.
3. Inspect the pinned Gastown pack and the archived Mt Olympus role experiments.
4. Give the same evidence packet independently to Fable and exact
   `gpt-5.6-sol` at high reasoning.
5. Require at least 25 candidate architectures, five finalists, explicit
   authority matrices, cathedral arguments, self-attacks, and kill criteria.
6. Blindly cross-score the opponent's revised finalists on a 1000-point rubric.
7. Reveal authorship, require reaction and rebuttal, and synthesize only after
   both final positions were visible.
8. Separately research Bun's eleven-day Zig-to-Rust port from primary sources
   and test whether its mechanisms support or contradict the topology.

The duel was run in a temporary NTM session only because the operator explicitly
requested the two named model families. The session was stopped after evidence
capture. It was not part of the proposed AgentOps/GC runtime.

## Coverage ledger

The documentation was written only from chunks marked **read**. Skimmed and
skipped surfaces are disclosed and support no normative claim.

| Chunk | Status | Evidence used |
|---|---|---|
| AgentOps repository authority and product boundary | read | `AGENTS.md`, `docs/architecture/operating-loop.md` |
| Current GC execution contract and deployment | read | `docs/contracts/gas-city-execution-adapter.md`, `deploy/gc/city.toml`, `deploy/gc/README.md`, bootstrap and packet help |
| Current provider role configuration | read | four `packs/agentops-executor/agents/*/agent.toml` definitions |
| Current packet schema and static gate | read | `gc-execution-envelope.v1`, packet tests, bootstrap tests, projection doctors |
| Live isolated city | read | status, import status, managed marker, rig inventory, qualification evidence |
| Gas City platform model | read | `how-gas-city-works.md`, `coming-from-gastown.md`, formulas/controller references |
| Pinned Gastown pack roles | read | Mayor, Deacon, Boot, Dog, Witness, Refinery, Polecat, and Crew definitions at the Gas City module pin |
| Mt Olympus outcome and role doctrine | read | archive, final postmortem, no-orchestrator doctrine, relevant role material |
| Independent Fable proposal | read | 26-idea sweep, five finalists, Refinery addendum, reaction, rebuttal |
| Independent GPT-5.6-sol proposal | read | 26-idea sweep, five finalists, Refinery addendum, reaction, rebuttal |
| Blind cross-scores | read | both complete 1000-point scorecards |
| Bun port primary sources | read | official write-up, PR, porting-guide commit, merge-commit workflows, soundness issue |
| Private Bun model/session logs and billing | skipped | unavailable; no claim about exact paid cost or every reviewer invocation |
| Full live factory behavior at study time | skipped | the factory did not yet exist; architecture and proof criteria were proposals |
| Every historical Olympus conversation | skimmed | archive and final outcome were load-bearing; individual brainstorming transcripts were not treated as authority |

## Study-time current-state evidence

At the time of the topology study, the thin executor was intentionally smaller
than the selected factory:

- `agentops-executor` declares only fresh Codex/Claude Implementer and Validator
  pools.
- Every role is rig-scoped, zero-minimum, and max-one per target.
- Portable city policy caps the entire workspace at one active session.
- Generic provider pools and scaffold maintenance roles are suspended.
- `run-packet` dispatches once with `--no-formula --no-convoy` and returns
  transport evidence without semantic completion.
- No `agentops-factory`, Mayor, plan reviewer, graph reducer, Refinery, formula,
  order, or PR state machine existed in the inspected pack.

On 2026-07-17 the live isolated city at `/Users/bo/dev/gc-agentops` had a
supervisor-managed controller, healthy store, one active qualification rig,
seven suspended qualification rigs, and zero running sessions. Its local
AgentOps import resolved directly to the uncommitted working-tree pack.

The study-time static executor gate passed 17 packet tests, four projection tests,
seven bootstrap tests, and all three packet/role/projection doctors. Durable
qualification evidence in rigs 6, 7b, and 8 contained PASS verdicts with
distinct author and Validator context identities. This proves meaningful local
qualification; it did not make the uncommitted pack a released artifact or
prove the then-absent factory.

## Post-study implementation addendum

Later on 2026-07-17, the selected bead-native Fenced Steward topology was
implemented in `packs/agentops-factory/` and exercised in a separate isolated
city. The live path used a Codex Mayor, Claude plan review, one Claude Worker, a
fresh Codex candidate Validator, a Codex Refiner, and a fresh Claude integration
Validator. Program bead `al-5n6`, experiment bead `al-139`, and Refinery bead
`al-h8n` reached their contract-defined terminal states through protected PR
[#916](https://github.com/boshu2/agentops/pull/916), which landed as
`b80a752aad3843af66160b08a823aaed57e07169`.

That canary changes the implementation status, not the historical method or
the proof bar. It proves one real single-experiment happy path. The complete
multi-wave and injected-fault campaign remains unproven. Exact identifiers and
the qualification boundary are recorded in the
[live bead canary](../gas-city-factory-live-bead-canary.md).

## Architecture field

Both independent studies distinguished four meanings that role names often
collapse:

- semantic proposal authority;
- binding judgment authority;
- deterministic lifecycle/state authority; and
- Git/delivery authority.

The main finalists were:

| Family | Core shape | Fable cross-score | GPT-5.6-sol cross-score | Disposition |
|---|---|---:|---:|---|
| Lean/Fenced Steward | Mayor + fresh Judges + deterministic spine + bounded Refinery | 874/1000 | 845/1000 | Build |
| Headless Graph | Same mechanism plane without resident semantic planner | 827/1000 | 727/1000 | Build as substrate and Mayor kill destination |
| Persistent Witness/Governor | Resident read-only assurance authority plus fresh court | 627/1000 | 653/1000 | Kill as v1 topology |
| Mayor + per-quest Apollo | Product Mayor plus quest-scoped conductors | 697/1000 | 583/1000 | Defer behind measured overload |
| Universal Two-Key Court | Codex and Claude on every admission | 680/1000 in the scored opponent field | Not a matching Fable finalist | Keep as high-risk policy, reject universal default |

Fable's and GPT's finalist names differed, so only semantically matched families
appear in both score columns. The shared winner was not selected by averaging
names; it emerged because both independent proposals placed the same authority
seams in nearly the same locations before reveal.

## Convergence after reveal

Both models converged on these conclusions:

- The Mayor earns the only persistent semantic identity because product
  interpretation and replanning recur at the human boundary.
- The Validator earns first-class authority because author and judge cannot
  collapse, but freshness is an asset, so it does not earn a resident session.
- Refinery earns durable delivery authority, but continuity belongs in a
  delivery record, integration branch, PR, and fencing epoch rather than an
  always-running LLM transcript.
- A deterministic graph/reducer plane must exist independently so removing the
  Mayor is a configuration change rather than a rewrite.
- A resident Witness/Governor is the consensus kill. Deterministic observation
  plus event-triggered fresh Judges preserve its useful inputs without context
  gravity or veto creep.
- Gastown's recovery-Witness responsibility is different from an LLM Judge. It
  maps to health reconciliation, events, waits, leases, reaping, branch freeze,
  and alerts.
- Apollo is a dormant Mayor mode until measured scale shows a real semantic
  bottleneck.
- Universal two-provider review belongs behind a risk policy rather than on
  every routine node.

## Final authority model

The synthesis was named **Fenced Steward**:

```text
operator
  -> Mayor proposal
  -> fresh plan review
  -> deterministic graph admission
  -> isolated Codex/Claude experiments
  -> fresh binding candidate validation
      non-PASS -> Mayor -> new experiment identity
      PASS     -> admission certificate
  -> fenced Refinery integration train
  -> fresh integrated-subject validation
  -> protected PR/CI/review/merge
  -> landed-SHA delivery receipt
```

The reducer is the singular graph-state writer. Mayor proposes, Validators
judge, Refinery delivers, and protected repository policy writes `main`. No
model role may combine those authorities.

## Cathedral test

The audit rejected roles that lacked a unique trust seam:

- **Zeus** duplicates Mayor strategy.
- **Apollo** duplicates Mayor conduction until concurrent-quest evidence says
  otherwise.
- **Persistent Witness/Governor** duplicates durable projections and fresh
  Judges.
- **Deacon/Boot/Dog** duplicate controller health and exec-order mechanisms for
  routine lifecycle work.
- **Athena, Themis, Argus, and Hades** retain useful responsibilities as
  deterministic context compilation, fresh validation/admission, audit orders,
  and receipt-gated cleanup.
- **Hermes** is unnecessary because graph state, sidecars, events, waits, PRs,
  and receipts already form the bus.
- **Refinery** remains, but as durable fenced delivery authority plus fresh
  bounded triage—not a resident semantic editor.

The selection rule is reusable: recurring human-facing semantic judgment may
earn a named role; stable mechanics become code; occasional judgment becomes a
fresh context.

## Proof and kill criteria

The proposed architecture remains unproven until one real multi-wave product
uses both providers, at least four writer worktrees, at least two Validator
lanes, two bounded integration trains, one dependent draft wave, and one
independent wave.

The proof must deliberately attempt unauthorized branch writes, race two
Refinery epochs, move a candidate after PASS, kill a Worker before handoff,
collide through a shared generated artifact, seed semantic coupling, exercise
both `FAIL` and `NOT_PROVEN`, expose a semantic CI defect, move `main`, refresh a
dependent wave, remove one provider temporarily, and present stale or
author-collapsed validation.

The rollout passes only when all unauthorized/stale writes fail, every writer is
isolated, only exact PASSed subjects enter Refinery, every published mutation is
freshly validated, non-PASS creates a Mayor-requested new identity, protected
policy alone writes `main`, and the final receipt connects intent through landed
SHA.

Kill or absorb roles whose measured decisions are at least 80 percent mechanical
or add no unique semantic correction. A persistent Witness or Apollo tier
requires a separate controlled promotion trial.

## Preserved duel artifacts

The high-value final scorecards and rebuttals are copied verbatim under `raw/`.
They are model outputs and therefore evidence, not authority:

| Artifact | SHA-256 of original scratch file |
|---|---|
| `raw/WIZARD_SCORES_FABLE.txt` | `dff2cda82612b13ee4078be0c44858983037c51fe5666f2ce2540e82d6e51da9` |
| `raw/WIZARD_SCORES_GPT_SOL_5_6.txt` | `11cddc5ac894561d9899015fa184fddb1a5c59f06431e82455410bff47d52f95` |
| `raw/WIZARD_REBUTTAL_FABLE.txt` | `ebbc8c343ab49fe7e415daef8d2caadaf12d65e76d875c8a6123f1aba46a4b6d` |
| `raw/WIZARD_REBUTTAL_GPT_SOL_5_6.txt` | `e9c56d46a403a0bff233fad91ec6588200c80513f9ab97d21281b211607046ea` |

The larger scratch packet contained independent idea sweeps, prompts,
addenda, reveal reactions, and the working synthesis. Their essential claims,
cross-scores, disagreements, and final rulings are preserved here and in the
canonical ADR/architecture. They were not copied wholesale because thousands
of lines of duplicated model transcript would obscure rather than strengthen
the durable decision.

## Remaining uncertainty

- The `agentops-factory` v1 happy path is implemented and live-qualified for one
  experiment bead, but it has no complete multi-wave proof-week evidence.
- Exact safe concurrency depends on measured host, provider, repository, and
  Validator capacity; proposed counts are initial envelopes, not permanent
  defaults.
- The value of a second provider on routine clean rebases must be measured.
- Mayor semantic yield and context-reload reduction are hypotheses with explicit
  kill criteria.
- Refinery triage may prove unnecessary once routing tables mature.
- Raw private model/session logs and exact Bun billing are unavailable.

## Contents

| Path | Purpose |
|---|---|
| `README.md` | Method, evidence ledger, cross-scores, convergence, final decision, uncertainty |
| `bun-rust-port-research.md` | Primary-source Bun study, transfer rules, contradictions, and unknowns |
| `raw/WIZARD_SCORES_FABLE.txt` | Fable's blind cross-score |
| `raw/WIZARD_SCORES_GPT_SOL_5_6.txt` | GPT-5.6-sol's blind cross-score |
| `raw/WIZARD_REBUTTAL_FABLE.txt` | Fable's final rebuttal and v1 ruling |
| `raw/WIZARD_REBUTTAL_GPT_SOL_5_6.txt` | GPT-5.6-sol's final rebuttal and v1 ruling |
