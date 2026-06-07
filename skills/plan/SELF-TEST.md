# Plan Skill Self-Test

## Trigger Cases

- User says: `/plan "add user authentication"` (or any `/plan <goal>`).
  - Expected: load `plan`, read prior research if present, decompose into issues/waves, and write `.agents/plans/YYYY-MM-DD-<slug>.md`.

- User passes a bead ID: `/plan soc-1234`.
  - Expected: run the Step 0 stale-scope pre-flight (`ao beads verify`) before decomposition when the bead is full-complexity, older than 7 days, or filed by a prior session.

- User says: `/plan --auto "refactor payment module"`.
  - Expected: load `plan` and run decomposition without the Gate-2 human approval step.

## Non-Trigger Cases

- User asks an open investigation question with no goal to decompose ("how does X work?").
  - Expected: route to `/research`, not `plan`.

- User asks to validate already-implemented work or run gates.
  - Expected: route to `/validate`, not `plan`.

## Behavior Checks

These map to the four scenarios in [references/plan.feature](references/plan.feature):

- Plan consumes Discovery output: each slice carries acceptance criteria, write scope, test levels, and ownership, and no slice depends on raw Discovery chat context.
- One slice per Given/When/Then row: a BDD intent issue with N rows yields N vertical slices, each with a first-failing-test target.
- Wave-validity gate before parallelization: a wave passes only when every row holds — distinct write scopes, no shared migration/contract/CLI surface, declared integration order, an owner per slice, and a discard path per slice; otherwise slices default to sequential.
- Durable slice-validation artifact: Plan writes a slice plan to `.agents/plans/*.md` plus an `execution-packet.json`, and a fresh agent can execute the slices from those artifacts alone.

Worker latitude: Plan may create small mechanical files (templates, fixtures, generated companions) when they are required to satisfy a slice's acceptance criteria — these belong in the file dependency matrix as `write` ownership claims.

## Validation Commands

Run from the repo root:

```bash
bash skills/heal-skill/scripts/heal.sh --strict skills/plan
bash scripts/validate-skill-frontmatter.sh --strict
```

## Failure Cases

- Plan written without a baseline audit (file/section/LOC counts): fail the Baseline Audit Gate and quantify ground truth before decomposing (`--skip-audit-gate` for documentation-only plans).
- Acceptance criteria missing the fenced YAML `acceptance_criteria` block: contract violation — add the block to every issue body.
- Two same-wave slices claiming `write` on the same file: serialize with `blockedBy` or merge the slices; do not ship the wave.
