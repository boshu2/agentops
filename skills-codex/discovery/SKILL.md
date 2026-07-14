---
name: discovery
description: Create dense execution packets from intent
---
# Discovery — Dense Plan-Shaping Adapter

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), run:

```bash
ao codex ensure-start 2>/dev/null || true
```

The CLI records startup once per thread and skips duplicates automatically.

Discovery turns an objective into one self-contained execution packet. It
delegates research, constructs one exact plan through Plan, obtains one binary
Premortem verdict on that plan, and persists behavior-sized tracker leaves.

## Constraints

- Prove a capability is absent before scoping new construction.
- Preserve one active leaf per writer; goals and epics are aggregate demand.
- Shape one bounded tranche with one to three sequential low-risk waves in one
  bounded context. Larger demand remains future tranches.
- Produce one canonical execution packet. Link child artifacts; never copy raw
  transcripts into state.
- Migration plans require Plan's complete checked authority/consumer manifest.
  Only `disjoint` may be proposed as parallel; `shared` serializes and
  `incomplete` blocks admission.
- Goal Design, idea generation, and Dueling Idea Genies are optional shaping
  inputs. They never decide plan readiness.
- Every final exact plan receives one Premortem verdict. Discovery owns no
  approval edge, model-family floor, semantic retry loop, attempt counter,
  helper state, phase budget, implementation proof, or delivery authority.

## Loop position

Discovery owns intent shaping and slice planning before implementation. It is
also the replan route when the orchestrator presents evidence that invalidates
the current plan. Validate and Learn cannot silently invoke it or Premortem.

A Discovery handoff is prospective: Crank is `pending`; Validate and Learn are
`not_checked` until the frozen implementation exists.

## Workflow

1. **Clarify intent.** Separate WHAT from HOW and write Given/When/Then
   acceptance, non-goals, rollback, and evidence required for done.
2. **Retrieve bounded prior art.** Search the repo and directly matched
   learning records. Every “missing capability” claim cites the search that
   distinguished absence from existing machinery.
3. **Research.** Invoke Research as its own contract and retain only the
   artifact path, bounded contexts, relevant files/symbols, test levels, and
   constraints.
4. **Optional idea challenge.** For a contested strategic choice, Idea Genie or
   Dueling Idea Genies may produce advisory evidence. Pass its artifact to Plan;
   do not convert it into an approval or readiness decision.
5. **Plan.** Invoke Plan to create one exact, durable plan plus behavior-sized
   tracker leaves, dependency waves, ownership, executable acceptance, and a
   checked authority/consumer manifest when required.
6. **Premortem.** Dispatch one fresh-context judge against the exact final plan.
   Require `premortem-plan-verdict.v1`, live plan digest, distinct author/judge,
   and `PASS`. A `FAIL` returns its complete blocker set to the orchestrator.
7. **Compile.** Write one prospective execution packet with artifact links,
   six density fields, tracker proof, test levels, risks, and one next action.

The executable steps and state shape live in [dag.md](references/dag.md).

## Open-ended goals

When the goal is “improve the project” or explicitly requests ideation:

1. use Idea Genie to generate and winnow an evidence-grounded portfolio;
2. optionally use Dueling Idea Genies to challenge a consequential choice;
3. let Plan accept, reject, or combine that advisory evidence;
4. operationalize selected behaviors into self-documenting leaves; and
5. run one complete graph/acceptance refinement before Premortem.

No idea artifact is an implementation admission verdict.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--auto` | on | Skip human planning prompts; never skips Premortem |
| `--interactive` | off | Ask for human input during research and Plan |
| `--skip-brainstorm` | auto | Skip clarification when the goal is already specific |
| `--ideate` | auto | Force the open-ended idea portfolio path |
| `--complexity=<level>` | auto | Select `fast`, `standard`, or `full` plan detail |
| `--no-scaffold` | off | Skip optional project/module scaffolding |

## Output Specification

- **Artifacts:** one durable plan, tracker graph, exact-plan Premortem JSON, and
  `.agents/rpi/execution-packet.json`
- **Validation command:** `bash skills/discovery/scripts/validate.sh`
- **Downstream handoff:** checker-clean prospective packet and first ready leaf
  to Crank; the final frozen candidate later goes to Validate

## Quality Checklist

- Capability-gap claims cite the absence search.
- The packet preserves behavior, non-goals, rollback, evidence, constraints,
  and one next action without raw transcripts.
- Every leaf is behavior-sized, dependency-valid, and owned.
- The exact plan has one valid Premortem PASS.
- No local planning controller or alternate readiness authority survives.

`<promise>DONE</promise>` requires persisted slices and the exact-plan PASS.
`<promise>BLOCKED</promise>` reports evidence to the orchestrator and creates no
private retry state.

## References

- [bead operationalization](references/bead-operationalization.md)
- [brainstorm behavior](references/brainstorm.feature)
- [DAG](references/dag.md)
- [Discovery behavior](references/discovery.feature)
- [goal clarification](references/goal-clarification-brainstorm.md)
- [goal-design packet input](references/goal-design-packet-input.md)
- [idea rubric](references/idea-rubric.md)
- [ideation mode](references/ideation-mode.md)
- [complexity routing](references/complexity-auto-detect.md)
- [red-team checklist](references/red-team-checklist.md)
- [resume](references/idempotency-and-resume.md)
- [phase data](references/phase-data-contracts.md)
- [output templates](references/output-templates.md)
- [troubleshooting](references/troubleshooting.md)
