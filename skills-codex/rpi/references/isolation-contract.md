# Loop Responsibility And Isolation Contract

RPI preserves typed authority without turning every phase into a new model call.
The visible orchestrator keeps the lifecycle objective and records each
next-move disposition against evidence.

## Required boundaries

| Responsibility | Context rule | Authority |
|---|---|---|
| Discovery | visible orchestrator or thin skill runner | shape the leaf plan and acceptance |
| Premortem | fresh context, author != judge | admit the exact plan |
| Crank | visible leaf owner by default | implement 1–3 waves and return targeted facts |
| Validate | fresh context, author != judge | judge one frozen candidate |
| Learn | visible orchestrator | copy immutable observations into one receipt |

Fresh context is load-bearing for independent semantic judgment. It is not
required for deterministic bookkeeping, orchestration, or a direct single-writer
implementation wave.

## Artifact boundary

The execution packet carries one ordered receipt index pointing to:

1. Discovery packet and bound Premortem verdict;
2. canonical Crank wave checkpoints;
3. canonical Validate `result.json`; and
4. canonical `learn-receipt.json`.

Raw reasoning does not cross roles. Neither do four hand-written phase summaries.
A legacy summary is a link-only compatibility projection and never proof that a
fresh context existed.

## Optional isolated transport

A runtime may use a subagent, process, or phase runner when a responsibility
requires fresh context or when the user explicitly requests delegation. The
transport loads the declared skill, receives only the bounded handoff, and returns
artifact identity plus next action. It must not replace the skill's contract.

Do not isolate all four responsibilities by default. That multiplies context and
token cost without improving the two semantic independence boundaries.

## Compression and theater

Invalid compression:

- the plan author self-issues Premortem PASS;
- the implementation author self-issues Validate PASS;
- Crank silently changes the admitted plan; or
- Learn changes the immutable verdict or controls retry/delivery.

Invalid theater:

- fresh models for deterministic Learn/bookkeeping;
- per-wave Validate/Learn on an unchanged plan;
- repeated exact-input deterministic checks;
- fixed counts of phase runners or Markdown summaries; or
- Swarm/NTM startup for one direct write scope.

`scripts/check-skill-isolation.sh` catches authored cross-role compression in
entrypoint `SKILL.md` files. It supplements semantic review; it does not prove
runtime invocation counts.

Cross-references:

- [strict delegation](../../shared/references/strict-delegation-contract.md)
- [phase data](phase-data-contracts.md)
- [operating loop](../../../docs/architecture/operating-loop.md)
