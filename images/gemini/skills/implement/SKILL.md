---
name: implement
description: 'Execute one bounded RED to GREEN experiment from bead or caller intent; return derived subject identity and check facts. Triggers: "implement", "implement this bead", "run the experiment". Full plan-to-validation requests route to rpi.'
practices:
- tdd
- refactoring
- small-batch-flow
hexagonal_role: driving-adapter
consumes: []
produces:
- subject-manifest.v1
output_contract: 'subject-manifest.v1 digest, author context ID, and exact acceptance-check receipts returned through the response or runtime channel'
context_rel:
- kind: customer-of
  with: plan
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: execution
  dependencies: []
  capabilities: [execute_one_experiment, collect_factual_evidence]
  effects: [modify_declared_subject, derive_subject_manifest]
  canonical_status: canonical
  disposition: keep
---

# Implement

Execute exactly one bounded experiment described by the resolved bead or caller
intent. Implement owns subject edits and factual evidence; the runtime derives
identity and receipts.

## Prompt

```text
Implement bead ag-1234 from its text: acceptance "ao gate check lists
skill.probe-coverage", scope cli/internal/gates/** plus regen outputs, first
check `cd cli && go test ./internal/gates/...`. RED first, smallest change,
return the manifest digest and check receipts, stop.
```

## It's working if

- The first transcript command is the acceptance check, and its output shows
  the expected failure (or a green baseline for a relocation or refactor).
- Every path in `git diff --stat` falls inside the declared scope; an outside
  consumer is reported as `file:line`, not absorbed.
- The response carries the `subject-manifest.v1` digest, author context ID,
  and verbatim check output; no `git commit` or `git push` appears.

## Workflow

1. Read the intent, acceptance, and scope from their existing source; before
   the first write, read `boundaries.md` in the rpi skill's `references`
   directory for what Implement does not own.
2. Run the declared first acceptance check before changing behavior. RED-first
   applies when acceptance is behavioral: preserve evidence that the check
   fails for the expected missing behavior. Relocations, doc merges, and pure
   refactors record an honest green pre-change baseline instead.
3. Make the smallest in-scope change that satisfies the active behavior.
4. Run the targeted acceptance checks and capture factual results.
5. Refactor only while those checks stay green. Refactoring does not change the
   acceptance test.
6. Have the runtime derive actual changed paths and `subject-manifest.v1` from
   the before/after subject.
7. When `scripts/evidence-orphans.sh` exists, have the runtime run it over the
   runtime-derived union of changed paths and append its output to the check
   receipts the validator reads, so orphaned evidence is a receipt like any
   other rather than a surprise at verify time. The receipt runs again after
   every repair round, over the union as it stands, because a repair can
   orphan evidence the first pass did not. Each entry carries a `cause`:
   `changed_path`, `digest_drift`, `both`, or `skill_changed`, which separates
   the evidence this change orphaned from drift that was already there. The
   runtime derives the orphan set; the model never lists it by hand.
8. Return the manifest digest, author context ID, and exact check receipts in the
   response or runtime channel. Stop.

Specialists (standards, domain, test, refactor, security) advise only. During
edits, run the smallest deterministic checks that can falsify the change,
reuse exact-input receipts whose subject and tool identity still match, and
run the full suite at the integration boundary unless the intent makes it the
first check.

## Scope conflict rule

On discovering a live consumer of the change outside the declared write scope
(a test asserting the old path, a generated twin, a gate reading the moved
file), stop and report the exact file and line to the caller, who may revise
the intent and start a separate invocation; a different acceptance contract
is a new intent.

Before declaring GREEN, self-audit the diff for mocks, placeholders, TODO
stubs, hardcoded fixture values, weakened assertions, regenerated goldens,
widened tolerances, suppression directives, or specification edits standing
in for real behavior. A changed test, gate, fixture, golden, or acceptance
source must be required by the original intent, with green coming from the
implemented behavior; a check that passes against a substitute or weakened
oracle is not evidence: finish the behavior or report it as not built.

## Boundary

Do not commit, push, claim, close, release, land, reserve, retry, or invoke a
semantic validator. A failed check is evidence for the caller, not permission
to create a packet or validation loop.
