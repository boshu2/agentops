---
name: council
description: 'Run multi-judge consensus. Triggers: "council", "independent judges", "high-stakes decision review".'
---

# council — moved to Mount Olympus (2026-06-10)

Canonical home: the mt-olympus repository, project skill `council`
(`~/dev/mt-olympus/` repo, project skills directory). Read and follow the
canonical SKILL.md there. This stub preserves routing and twin parity until
the catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).

## Constraints

- Read the Mount Olympus canonical body before running a panel because this repository copy is a routing stub, not the executable procedure.
- Reserve council for irreversible decisions; use `$validate` for per-slice acceptance so one artifact is not double-gated by overlapping authorities.
- Keep author and judges distinct and judge lanes read-only because consensus is evidence only when verdicts are independent of production and mutation.

Narrow-waist obligations (must hold at the canonical body): council is the S5
membrane for irreversible DECISIONS, not slice-acceptance closes — `$validate` owns
the per-slice acceptance verdict, so do not double-gate. Its verdict binds to the
slice's BDD/ATDD acceptance test; author != judge; every REFUTE feeds a lesson into
the next loop's `$pre-mortem` checks (S6).

For mixed-family work, `$agent-native` may drive durable roles over NTM while
bounded in-session lanes use the native Codex agent surface. Landing oracles are
owned by `$pawl-review`; `ao pawl` owns the deterministic panel verdict. Route
contested one-way-door ideas through `$dueling-idea-genies` before planning.

## Examples

- Run council from the canonical location in the mt-olympus repository.

## Troubleshooting

- Body moved to Mount Olympus 2026-06-10; this stub preserves routing/parity.

## Output Specification

- **Path:** the run's declared evidence directory, containing both the panel aggregate and its binding decision handoff.
- **Filename:** `result.json` for judge results and `verdict.json` for the council verdict.
- **Format:** JSON; `verdict.json` must validate against `skills/council/schemas/verdict.json` and retain each concrete finding's location, recommendation, rationale, and reference.
- **Exit code:** validate with `python3 -m jsonschema -i <evidence-dir>/verdict.json skills/council/schemas/verdict.json`; missing judges, author overlap, invalid JSON, or schema failure is nonzero and not consensus.
- **Downstream handoff:** pass the independent results and validated verdict to `$pawl-review`/the verification membrane; council does not itself authorize landing.

## Quality Checklist

- Every counted judge is independent of the author context, read-only, and evaluating the same decision packet.
- The verdict preserves dissent and concrete evidence instead of reducing disagreement to an unsupported majority label.
- The chosen option, confidence, findings, and next action validate against the schema and remain traceable to the panel inputs.
