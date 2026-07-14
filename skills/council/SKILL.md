---
name: council
spine: true
description: 'Run multi-judge consensus. Use when: an irreversible or high-stakes decision needs independent judges before committing — architecture forks, one-way doors, scoring options.'
practices:
- llm-eval-harness
- ai-assisted-dev
- design-by-contract
hexagonal_role: domain
consumes:
- standards
produces:
- result.json
- verdict.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
context:
  window: isolated
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  graph_root: true
  tier: judgment
  dependencies:
  - standards
  - agy-native
  - pawl-review
  replaces: judge
output_contract: skills/council/schemas/verdict.json
---

# council — moved to Mount Olympus (2026-06-10)

This skill encodes independent-verdict machinery and now lives with the outer
gate product. Canonical: `~/dev/mt-olympus/.claude/skills/council/SKILL.md` —
read and follow that file. This stub preserves fleet routing until the
using-agentops catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).

## Constraints

- Read the Mount Olympus canonical body before running a panel because this repository copy is a routing stub, not the executable procedure.
- Reserve council for irreversible decisions; use `validate` for per-slice acceptance so one artifact is not double-gated by overlapping authorities.
- Keep author and judges distinct and judge lanes read-only because consensus is evidence only when verdicts are independent of production and mutation.

> **Narrow-waist obligations (must hold at the canonical body):** council is the S5 membrane for irreversible **decisions**, not slice-acceptance closes — `/validate` owns the per-slice acceptance verdict, so do not double-gate. Its verdict binds to the slice's BDD/ATDD acceptance test; author ≠ judge; and every REFUTE feeds a lesson into the next loop's `/premortem` checks (S6). See the [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this).

## Absorbed trigger surfaces (skill-prune phase 2)

Council also fires for the use-cases of two folded-in skills:

- **multi-model-triangulation** — cross-validate decisions using multiple AI
  models (Codex, Gemini, Grok). Use when asked to "get a second opinion",
  when evaluating competing approaches, or for high-stakes decisions: run the
  question through council's independent judges instead of a single model.
- **cross-vendor-trust-gate** — run the skill-factory final trust gate:
  operate `trust-gate.sh`, read `skill.trust.json` (trust_level + trust_score),
  and enforce `--require-cross` so cross-vendor parity gates the verdict.
  Canonical body:
  `~/dev/mt-olympus/.claude/skills/cross-vendor-trust-gate/SKILL.md`.

## Mixed-model (cross-family) panel

When the decision wants a **mixed-model / cross-family** panel rather than
single-model judges, use `agent-native` for durable role-shaped lanes over NTM
or an in-session variant (`codex exec` plus the available native agent surface).
For a landing oracle, each fresh read-only lane is owned by `pawl-review` and
the deterministic `ao pawl` membrane owns the panel decision. `/discovery`
routes one-way-door idea choices through `dueling-idea-genies` before planning.

## Examples

- `/council should we swap the policy engine to Cedar?` — runs at the canonical
  location; this stub forwards. Read
  `~/dev/mt-olympus/.claude/skills/council/SKILL.md`.

## Troubleshooting

- **Skill seems empty / missing scripts:** the body moved to Mount Olympus
  (2026-06-10). Use the canonical path above; this stub exists only to keep
  fleet routing alive until the catalog closer updates the registry.

## Output Specification

- **Path:** the run's declared evidence directory, containing both the panel aggregate and its binding decision handoff.
- **Filename:** `result.json` for judge results and `verdict.json` for the council verdict.
- **Format:** JSON; `verdict.json` must validate against `skills/council/schemas/verdict.json` and retain each concrete finding's location, recommendation, rationale, and reference.
- **Exit code:** validate with `python3 -m jsonschema -i <evidence-dir>/verdict.json skills/council/schemas/verdict.json`; missing judges, author overlap, invalid JSON, or schema failure is nonzero and not consensus.
- **Downstream handoff:** pass the independent results and validated verdict to `pawl-review`/the verification membrane; council does not itself authorize landing.

## Quality Checklist

- Every counted judge is independent of the author context, read-only, and evaluating the same decision packet.
- The verdict preserves dissent and concrete evidence instead of reducing disagreement to an unsupported majority label.
- The chosen option, confidence, findings, and next action validate against the schema and remain traceable to the panel inputs.

## Runtime Contract

Multi-judge runs still bind to the shared Claude runtime surface:
[claude-code-latest-features.md](../shared/references/claude-code-latest-features.md).
