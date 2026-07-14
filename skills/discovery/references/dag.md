# Discovery Artifact-First DAG

Discovery runs child skill contracts and compiles their durable outputs into one
prospective execution packet. It does not keep child prose resident in the
caller and does not implement a private controller.

## Child result rule

Each child step returns only:

- artifact path;
- status or immutable verdict;
- one-line extraction for affected density fields; and
- next action or concrete block reason.

## State

```text
discovery_state = {
  goal: "<goal string>",
  objective: "<bounded behavior objective>",
  complexity: "<fast|standard|full>",
  tracking_mode: "<beads|tasklist>",
  artifacts: {
    goal_design_path: null,
    research_path: null,
    idea_challenge_path: null,
    plan_path: null,
    premortem_verdict_path: null
  },
  density: {
    intent: null,
    boundary: null,
    evidence: [],
    decision: null,
    constraint: [],
    next_action: null
  },
  known_risks: [],
  disposition: null
}
```

## STEP 0 — Initialize

Resolve `br`, then `bd`, then a file-backed task list. Record the selected mode
once. Classify detail as `fast`, `standard`, or `full` from explicit input and
objective shape. Do not allocate an attempt map or phase-local budget.

## STEP 1 — Clarify intent

If a checker-clean Goal Design packet exists, consume its intent and driver
paths and preserve scenario IDs. Otherwise clarify the objective directly:

- observable Given/When/Then behavior;
- bounded context and non-goals;
- rollback/containment;
- first failing proof; and
- evidence required for done.

## STEP 2 — Bounded prior art

Search the live repository first. When available, retrieve at most three
directly relevant knowledge pointers and directly matched recurring-catch
classes. Missing knowledge inputs skip silently. Record whether each result is
applicable and which density field it changes.

Every capability-gap claim records the command or query that proved the gap.

## STEP 3 — Research

Invoke Research as its own contract. Extract only:

- `research_path`;
- impacted bounded contexts;
- relevant paths and symbols;
- applicable test levels; and
- constraints Plan must preserve.

## STEP 3.5 — Optional advisory idea challenge

When the objective contains a genuinely contested strategic choice, invoke Idea
Genie or Dueling Idea Genies. Store one `idea_challenge_path` and pass it to
Plan. The packet may contain sealed perspectives, cross-reviews, dissent, and
refutation attempts. It contains no readiness decision and creates no Discovery
state transition.

Skip this step for routine, reversible work.

## STEP 4 — Plan

Invoke Plan with the objective, research path, and optional idea challenge.
Plan owns the exact durable plan and tracker decomposition. Require:

- one Given/When/Then behavior per leaf;
- executable acceptance and first failing proof;
- dependency waves and write ownership;
- non-goals, rollback, and discard path;
- generated companions in write scopes; and
- a checked authority/consumer manifest for migration-shaped work.

Every persisted tracker leaf must pass the scenario admission checker. If the
tracker is unavailable, write the equivalent self-contained task list.

## STEP 4.5 — Optional scaffold

If the exact plan creates a new project, package, module, or service and
`--no-scaffold` is not set, invoke Scaffold and return any changed constraints
to Plan before freezing the plan. Scaffolding never bypasses plan judgment.

## STEP 5 — Exact-plan Premortem

Freeze `plan_path`, compute its SHA-256, and send that exact artifact to one
fresh-context judge. Require the canonical JSON fields:

- `schema_version: premortem-plan-verdict.v1`;
- repository-relative plan path and SHA-256;
- distinct `author_id` and `judge_id`;
- `blockers_complete: true`; and
- binary `PASS` or `FAIL`.

Validate the result with:

```bash
skills/premortem/scripts/validate-output.sh \
  "$PREMORTEM_VERDICT_PATH" "$(git rev-parse --show-toplevel)"
```

Only `PASS` admits compilation. `FAIL` returns the complete blocker set to the
orchestrator. Discovery does not repair, dispatch another judge, count attempts,
or choose escalation.

## STEP 6 — Compile execution packet

Write `.agents/rpi/execution-packet.json` and, when a run ID exists, its run
copy. The packet is prospective and contains:

- linked child artifacts and their digests;
- intent, boundary, evidence, decision, constraints, and next action;
- tracker mode, epic ID, ready leaf, dependencies, and owners;
- acceptance criteria, test levels, and known risks;
- Premortem verdict path and digest; and
- `Crank: pending`, `Validate: not_checked`, `Learn: not_checked`.

Do not embed raw research, plan prose, judge deliberation, implementation proof,
or delivery state.

## DONE gate

Before reporting Discovery complete:

1. the plan path exists and its digest matches the Premortem verdict;
2. the verdict validator passes and the verdict is `PASS`;
3. the epic and at least one behavior leaf resolve in the selected tracker, or
   the file-backed task list contains the same graph;
4. every ready leaf has executable acceptance and one owner; and
5. the packet names exactly one next action.

Discovery completion means planning evidence is ready. It does not claim that
implementation, validation, learning, delivery, or remote verification ran.
