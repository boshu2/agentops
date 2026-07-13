---
name: plan
spine: true
description: 'Decompose goals into issue plans. Triggers: "plan", "decompose goals into issue plans.", "plan skill".'
practices:
- adr
- agile-manifesto
- pragmatic-programmer
hexagonal_role: domain
consumes:
- standards
produces:
- .agents/plans/*.md
- execution-packet.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
metadata:
  graph_root: true
  tier: execution
  dependencies:
  - research
  - beads-br
  - premortem
  - crank
  - implement
  - scope
  - dueling-idea-genies
context:
  window: fork
  intent:
    mode: task
  intel_scope: topic
output_contract: .agents/plans/YYYY-MM-DD-*.md, beads (via ao beads exec create)
---
# Plan Skill

Decompose intent into behavior-sized, issue-ready slices with dependency waves,
file ownership, and executable acceptance. Execute the workflow; do not merely
describe it. Small local changes under roughly 200 LOC may plan in chat, but a
non-trivial plan must be durable and self-contained.

## Critical Constraints

- **Why: avoid stale scope.** Verify inherited or older bead citations against
  HEAD before decomposition; do not plan from unchecked goal-design packets.
- **Why: preserve behavior.** Each slice delivers one Given/When/Then behavior;
  separate refactors from feature slices.
- **Why: prevent write collisions.** Same-wave writers must have disjoint file
  ownership, including generated companions, manifests, fixtures, and docs.
- **Why: keep acceptance executable.** Every bead carries `## Scenarios` above
  a fenced `acceptance_criteria` YAML block with runnable evidence checks.
- **Why: prevent rediscovery.** Cite real paths, symbols, signatures, tests, and
  reuse points; verify inventory claims at plan time and again when consumed.
- **Why: preserve intent.** Keep WHAT and HOW separated; planning does not
  implement, and `--auto` skips approval only when explicitly selected.
- **Why: avoid false novelty.** Search existing skills and the `ao` surface
  before scoping a new capability; record hits as reuse, not new work.

## Inputs and Boundary

`/plan <goal> [--auto|--fast-path|--deep|--skip-symbol-check|--skip-audit-gate]`
consumes BDD intent, a bead, research, or a goal-design packet through the
`plan_slices` inbound port. It produces a slice plan, issue graph, file matrix,
and validation packet through `persist_issue`, `verify_symbols`,
`retrieve_context`, and `seed_execution_packet`.

When input contains `intent.md` and `driver.md`, run
`scripts/check-goal-design-packet.sh <packet-dir>`. Preserve candidate behavior
and scenario IDs; map `first_failing_proof`, `write_scope`, and `close_signal`
into acceptance; carry non-goals, rollback, and hard rules into boundaries.

## Workflow

1. **Stale-scope pre-flight.** For inherited, full-complexity, reopened, or
   older-than-seven-day beads, run `ao beads verify <id>` first. Stop on stale
   citations until scope is revalidated.
2. **Load context.** Read prior research and run `ao search`/`ao lookup`. Load
   `.agents/planning-rules/*.md` first, then active findings from
   `.agents/findings/registry.jsonl`. Every plan includes `Applied findings:`
   with IDs or `none` and explains how retrieved rules changed the plan.
3. **Choose strategic review.** For multi-session work with a contested
   operator default, recommend `dueling-idea-genies` and an `idea-challenge.v1`
   packet to `ao plan-pawl decide`; keep this advisory.
4. **Explore only as needed.** Inspect the codebase or use a bounded Explore
   agent for file inventory, exact symbols/signatures, reuse points with
   `file:line`, tests, imports, and conventions.
5. **Baseline mechanically.** Record commands and counts for files, LOC,
   sections, tests, fixtures, schemas, and size limits. Search `ms search`
   when available, otherwise `skills/**/SKILL.md`, `docs/SKILLS.md`, and `ao`.
6. **Scale detail.** Minimal for 1-2 simple issues, Standard for 3-6, Deep for
   7+, broad refactors, full complexity, or `--deep`.
7. **Decompose by behavior.** Give each issue a title, scenario, owned files,
   dependencies, test levels proportional to risk, and mechanical conformance
   checks. Custom rubrics name their `agent_judge`.
8. **Compute waves.** Topologically group independent issues. Serialize shared
   writes and read/write conflicts. Include tests, docs, schemas, fixtures,
   runtime copies, Codex companions, parity manifests, and hash markers.
9. **Build matrices.** Produce the file dependency matrix, file-conflict
   matrix, cross-wave shared-file registry, owner and discard path per slice.
   Default to sequential when wave validity is uncertain.
10. **Write the plan.** Use the canonical template and baseline gate in
    [plan-document-template.md](references/plan-document-template.md). Mark the
    result INCOMPLETE when a Planning Rules Compliance justification is empty.
11. **Create tracking.** Prefer br issues with scenarios, validation blocks,
    and `blocks` edges. Run the scenario admission and post-creation validation
    gates from [task-creation.md](references/task-creation.md). If br is absent,
    keep the markdown plan as the durable handoff.
12. **Approve and report.** Unless `--auto`, request approval before declaring
    completion. Report plan path, issue count/IDs, waves, and next route through
    `/premortem` then `/crank`. Record `ao ratchet record plan` when available.

## Required Plan Sections

- context, intent issue, and applied findings
- boundaries, non-goals, rollback, and files to modify
- baseline audit with commands and results
- one slice per scenario with first failing proof and owned write scope
- issue descriptions with scenarios and acceptance criteria
- dependency waves and file dependency/conflict matrices
- planning rules compliance and cross-wave shared-file registry
- verification commands, cleanup, discard paths, and next steps

Detailed decomposition, implementation detail, and matrices live in the
references below; load only the modules required by the selected complexity.

## Output Specification

- **Path:** `.agents/plans/YYYY-MM-DD-<goal-slug>.md`; optional durable issues
  go to the repository's resolved br ledger.
- **Filename:** the filename convention is ISO date plus a stable goal slug;
  never overwrite an unrelated plan.
- **Format:** Markdown using the canonical template, embedded Gherkin, fenced
  YAML `acceptance_criteria`, issue IDs, dependency edges, and file matrices.
- **Validation command:** run `bash skills/plan/scripts/validate.sh`, relevant
  scenario/validation admission checks, and verify every cited symbol/path.
- **Downstream handoff:** consumed by `/premortem`, `/crank`, `/implement`, and
  future agents without relying on chat-only context.

Report:

```text
Plan: <path>
Issues: <count and IDs>
Waves: <ordered groups or sequential>
Validation: <PASS|WARN|FAIL with commands>
Assumptions: <verified facts and unresolved risks>
Next: </premortem, /crank, or revision>
```

## Quality Rubric

- **Self-contained:** a fresh implementer can act without rediscovery or chat.
- **Behavioral:** slices and acceptance map directly to observable scenarios.
- **Grounded:** load-bearing claims, symbols, counts, and reuse points are cited.
- **Conflict-safe:** waves have explicit ownership and no unresolved collisions.
- **Executable:** every acceptance row names a check and evidence surface.
- **Right-sized:** detail and test levels match complexity and blast radius.
- **Honest:** missing research, stale scope, or incomplete justification is
  reported as WARN/FAIL rather than hidden.

## Examples

**User says:** `/plan "add rate limiting"`

Produce a grounded inventory, behavior-sized issues, validation checks, and a
collision-safe wave order; write the durable plan before reporting done.

**User says:** `/plan --auto .agents/research/auth.md`

Use the research, create the plan and tracker graph when available, skip only
the approval prompt, and hand off to the next loop move.

## Troubleshooting

| Problem | Response |
|---|---|
| br is unavailable | Write the markdown plan and report issue creation skipped |
| Research lacks symbols | Explore until paths, signatures, and tests are verified |
| Same file has parallel writers | Serialize or merge the affected slices |
| Baseline evidence is missing | Mark INCOMPLETE unless the documented opt-out applies |

## References

- [pre-decomposition.md](references/pre-decomposition.md) — research, prevention, exploration, baseline
- [decomposition.md](references/decomposition.md) — issue and acceptance contracts
- [implementation-detail.md](references/implementation-detail.md) — symbol-level plan detail
- [detail-templates.md](references/detail-templates.md) — Minimal/Standard/Deep shapes
- [plan-document-template.md](references/plan-document-template.md) — canonical artifact
- [task-creation.md](references/task-creation.md) — br creation and admission gates
- [wave-matrices.md](references/wave-matrices.md) — ownership and conflict matrices
- [planning-rules.md](references/planning-rules.md) — PR-001 through PR-011
- [plan-mutations.md](references/plan-mutations.md)
- [complexity-estimation.md](references/complexity-estimation.md)
- [examples.md](references/examples.md)
- [plan-to-beads-workflow.md](references/plan-to-beads-workflow.md)
- [sdd-patterns.md](references/sdd-patterns.md)
- [templates.md](references/templates.md)
- [plan.feature](references/plan.feature)
