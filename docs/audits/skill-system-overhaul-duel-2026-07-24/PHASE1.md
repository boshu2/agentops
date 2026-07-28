# Dueling Idea Wizards — Phase 1: sealed architecture proposal

You are one of two independent architecture wizards reviewing the proposed
AgentOps skill-system overhaul. The other wizard receives this exact prompt and
the same frozen evidence packet. This is a research and planning task only.

## Identity and write boundary

- If your exact model is `claude-fable-5`, you are **FABLE** and may write only
  `docs/audits/skill-system-overhaul-duel-2026-07-24/raw/WIZARD_IDEAS_FABLE.md`.
- If your exact model is `gpt-5.6-sol`, you are **SOL** and may write only
  `docs/audits/skill-system-overhaul-duel-2026-07-24/raw/WIZARD_IDEAS_SOL.md`.
- Do not edit product code, skills, plans, contracts, generated projections,
  Git state, beads, configuration, or the other wizard's artifact.
- Do not read any `WIZARD_*` file written by the other wizard during this
  phase. Independence is part of the evidence.
- Do not delegate.

## Frozen evidence packet

Read these completely before judging:

1. `AGENTS.md`
2. `README.md`
3. `docs/architecture/operating-loop.md`
4. `docs/contracts/skill-ports-and-adapters.md`
5. `docs/architecture/component-map.md`
6. `docs/plans/2026-07-24-skill-system-overhaul.md`
7. `docs/audits/2026-07-24-go-cli-deep-audit.md`
8. Every canonical `skills/*/SKILL.md`, using the exact 49-skill inventory in
   the overhaul plan as the coverage ledger
9. Relevant live Go CLI source, schemas, generators, and gates needed to verify
   claims; source behavior outranks narrative

Treat `AGENTS.md` as a runtime and authority constraint, not as a substitute for
architectural reasoning. In particular, independently recover:

- the intent of every skill;
- the reason it exists rather than being a section of another skill;
- its owning system layer and lifecycle seam;
- whether it belongs inside one RPI experiment, around a campaign of RPI
  experiments, after verdict collections, or outside the loop;
- what it consumes, produces, may mutate, and must never own;
- what observable proof would distinguish useful specialization from skill
  sprawl.

## Question

Produce the strongest final architecture and migration plan for the complete
skill system, grounded in the current evolved RPI contract and the companion Go
CLI audit.

Do not merely review wording. Challenge the plan's premises and recover the
architecture from live sources. The result must preserve the core rule that one
RPI invocation owns exactly one bounded:

`Plan -> Implement -> fresh Validate -> durable verdict -> report and stop`

while explaining how product intent, campaign control, optional judgment
strategies, evidence, runtime adapters, capability evolution, and CLI policy
fit around that experiment.

## Required method

Generate at least 30 candidate improvements or alternative decisions. Winnow
them to your five highest-leverage proposals. A proposal may retain, revise,
split, reorder, or reject part of the current plan.

For each finalist include:

1. precise claim;
2. defect or ambiguity it resolves;
3. evidence from specific skill, plan, contract, schema, gate, or Go source;
4. target architecture and ownership boundary;
5. migration steps and dependencies;
6. proof/acceptance criteria;
7. failure modes and rollback or containment;
8. which of the 49 skills and which CLI findings it affects;
9. confidence from 0–100% and what would falsify it.

Also include:

- a one-row-per-skill disposition appendix covering all 49 canonical skills;
- a critique of tranche ordering and generated-write serialization;
- a decision on whether the CLI audit belongs inside migration tranches, as a
  prerequisite substrate tranche, or as a separate program;
- the three most dangerous ways the existing plan could appear complete while
  leaving the system semantically incoherent;
- explicit `keep`, `change`, and `reject` rulings on the current plan.

Prefer architectural clarity and falsifiable proof over ceremony. Use the full
context available to you. When the artifact is complete and verified, reply
only `IDEAS_WRITTEN`.
