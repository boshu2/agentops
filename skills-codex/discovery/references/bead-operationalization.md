# Operationalizing Ideas Into Self-Documenting Beads

> The fourth and fifth steps of the generate-winnow methodology
> (`ideation-mode.md`): turn a ranked portfolio of ideas into a comprehensive,
> granular, self-documenting set of `br` beads, then run one complete plan-space
> refinement. Repeat once only after a material graph or acceptance change.
>
> This is primarily a `/discovery` responsibility on the open-ended path, but it
> consumes ideation-mode output and uses the same `br`/`bv` discipline.
>
> **Tracking is `br` (beads).** This is AgentOps — use `br`/`bv`.

## Step 4 — Comprehensive bead creation

Take ALL of the ranked ideas (top 5 + next 10 = 15) and elaborate them into a
comprehensive, granular set of beads with tasks, subtasks, and a dependency
structure overlaid. The beads must be **self-documenting**: so detailed that the
original markdown plan never needs to be consulted again.

Each bead's description should answer, for our "future self":

1. **What** — What specifically needs to be done?
2. **Why** — Why it matters; how it serves the project's overarching goals.
3. **How** — Key implementation approach.
4. **Risks** — What could go wrong (carry the red-team findings forward).
5. **Success criteria** — How we know it's done.

Include relevant background, reasoning/justification, and considerations — the
goals, intentions, and thought process that produced the idea.

### Bead structure

```bash
br create "Epic: <Feature Name>" -p 1 -t epic --description "
## Background
[Why this feature matters; which ideation idea(s) it came from]

## Goals
- [Goal 1]
- [Goal 2]

## Non-Goals
- [What we are explicitly NOT doing]

## Considerations
[Technical constraints, user expectations, rubric scores, red-team findings]
"
```

### Subtask + dependency pattern

```bash
# Create the epic, then tasks under it
br create "Design <component> interface" -p 1 -t task --description "..."
br create "Implement <component> logic"  -p 2 -t task --description "..."
br create "Tests for <component>"         -p 2 -t task --description "..."

# Wire dependencies (child depends on parent)
br dep add <impl-id> <epic-id>      # impl depends on epic
br dep add <test-id> <impl-id>      # tests depend on implementation
```

### Test strategy

Each behavioral leaf names the lowest test level that proves its acceptance.
Create a separate test bead only when the test harness is independently
deliverable work. E2e coverage is required only for behavior that crosses a real
system boundary; detailed logging is evidence-driven, not a fixed template:

```bash
br create "Acceptance tests for <component>" -p 2 -t task --description "
## Coverage Requirements
- Core behavior
- Error handling for invalid input
- Edge cases (empty, unicode, concurrent)

## Logging
- Capture only evidence needed to diagnose a failed acceptance claim
"
```

### Overlap check before creating

Compare against existing beads so ideas enhance rather than duplicate:

```bash
br list --json | jq '.issues[]?.title'
```

| Overlap type | Action |
|--------------|--------|
| Direct duplicate | Skip; reference the existing bead |
| Complementary | Merge into the existing bead |
| Conflicts | Note explicitly; flag an architectural decision in the bead body or handoff |

## Step 5 — Refine in plan space

It is far easier and faster to operate in **plan space** before implementing.
Run one complete refinement pass over the bead set. Run one additional pass only
when the first pass materially changes the dependency graph, acceptance, or
bounded-context boundaries:

1. **Re-read AGENTS.md / CLAUDE.md** after context compaction or when scope moved;
   do not reread unchanged instructions as a ritual.
2. Check every bead carefully: Does it make sense? Is it optimal? Could anything
   change to make the system work better for users? Revise in place.
3. **DO NOT OVERSIMPLIFY.** Resist the urge to collapse complexity — complexity
   usually exists for a reason.
4. **DO NOT LOSE FEATURES OR FUNCTIONALITY.** Every capability in the portfolio
   must survive the refinement.
5. Match the test level to the changed surface. Require e2e coverage only when
   the behavior crosses a real system boundary.

Required pass focus:

| Pass | Focus |
|------|-------|
| 1 | Structure, dependency sanity, acceptance, test level, and actionable leaves |
| 2 (conditional) | Re-check only the graph or contract surfaces changed by pass 1 |

### Validation between passes

```bash
br dep cycles --json     # dependency cycles MUST be empty
br ready --json | jq 'length'   # confirm actionable work exists
br lint                  # hygiene: orphans, missing fields
```

> If `br` lacks a direct cycle-detection subcommand in your install, inspect the
> dependency graph with `br dep tree <id>` and `br blocked --json`. The
> invariant is the same: no cycles, every leaf actionable.

## Anti-patterns

| Don't | Do |
|-------|-----|
| Repeated fixed-count passes | One complete pass; a second only after a material graph or contract change |
| Beads that need the markdown plan | Self-documenting beads — what/why/how/risks/success |
| Omit tests | Explicit tests at the lowest level that proves the behavior |
| Oversimplify on refinement | Preserve complexity that exists for a reason |
| Lose features when refining | Every portfolio capability survives |
| `bd`/Dolt | `br`/`bv` — this is AgentOps |
