# Goals

## Product outcome

AgentOps helps coding agents implement the behavior the caller requested,
validate it independently, and leave reusable engineering improvements behind.
As the operations layer for agentic engineering, it connects that work to the
consumer's existing tools and delivery policy.

The canonical outcome is:

```text
accepted behavior in the caller's domain language
  -> bounded implementation experiment
  -> exact content identity
  -> fresh independent judgment
  -> PASS | FAIL | NOT_PROVEN, persisted when requested
```

Accepted changes should leave code, tests, tools, or useful decisions that
subsequent work can build on. Records preserve continuity and explain what was
checked. Actual reuse and its effects establish benefit; accumulating records
alone does not.

## Fitness properties

1. **Behavior before activity.** The caller-owned intent states observable
   behavior, acceptance examples where useful, non-goals, evidence, and bounded
   write scope before implementation begins. BDD examples identify the caller,
   event, and observable result; implementation and validation use those same
   examples. Later tests cannot redefine acceptance.
2. **Bounded implementation.** The native agent makes and checks the change, repairing
   known defects directly within the accepted scope and allowance. Aggregate
   budgets and delivery authority remain with the caller or selected runtime.
3. **Fresh judgment.** PASS requires explicit, distinct author and validator
   context IDs plus a freshness attestation.
4. **Exact subject.** Content identity covers files, symlinks, deletions,
   executable bits, roots, exclusions, and canonical digests without requiring
   Git or `ao`.
5. **Honest uncertainty.** Mutation, incomplete changed-path coverage, missing
   identity, or missing proof returns NOT_PROVEN. Proven scope or acceptance
   failure returns FAIL.
6. **Sovereign proof.** When requested, the fresh reviewer uses AO to atomically write
   content-addressed JSON to caller-selected protected external non-Git storage.
   Preserve legacy evidence under owner policy. Provenance is optional audit.
7. **Stop boundary.** The native agent owns the authorized outcome, planning only when
   needed and revising approach within unchanged acceptance. Known defects get
   direct repair; a causal stall gets at most one bounded fresh helper inside
   the existing allowance. Finish when accepted and freshly validated, or stop
   at the actual cancellation, refusal, causal or resource boundary. The native
   caller keeps budgets, stops, queue and work authority.
8. **Open ecosystem.** Callers keep their trackers, Git, PRs, CI, cloud agents,
   merge queues, rollback, and release systems.
9. **Shared domain language.** Use the caller's established terms and rule owners
   consistently across intent, examples, code, tests, and review. Resolve only
   ambiguity that changes behavior or ownership; distinguish meanings across
   bounded contexts rather than imposing one global vocabulary.
10. **Reusable engineering improvements.** Where useful to the accepted task,
    leave regression checks, reusable tools, or supported decisions with the
    existing owner. Preserve useful work and evidence across sessions without
    requiring a lesson or new artifact per task. Judge benefit through later
    use, including maintenance burden and harmful or unnecessary reuse.

## Structural constraints

- Native execution requires zero AgentOps skills. When selected, RPI's
  core hard dependencies are only `rpi -> {plan, implement, validate}`; these
  capability references do not require invoking every skill on every change.
- Memory, Learn and all strategy/factory/specialist skills are optional.
- Core schemas contain no retry, budget, queue, claim, lease, admission,
  next-action, closure, release, or delivery state.
- The pure manifest and verdict helpers make no Git, tracker, queue, network,
  release, or delivery call.
- Deterministic `ao gate check` reports repository check success or failure only.
- Semantic validation belongs to a fresh reviewer; Validate is optional guidance.

## Gates

Each row is one executable check that runs in this repository today, measured
by `ao goals measure`. The `Check` cell runs under
`bash --noprofile --norc -c` from the repository root; exit 0 is a pass, exit
77 is a skip, anything else is a fail. `Description` names the fitness
property above that the check actually evidences — where a check is a floor
under several properties rather than a direct assertion of one, it says so.

This table must never be empty. A goals file with zero rows measures 0/0 and
reports green over an empty set; `ao goals validate` now rejects that state
(`goals-denominator` below is its executable guard).

Disclosed scope: properties 1 (behavior before activity), 7 (stop boundary),
9 (shared domain language), and 10 (reusable engineering improvements) carry no
executable gate here. Behavior, language, and stopping are judged against the
task; reusable improvements also require evidence from subsequent work to show
benefit. A green score below is silent about these properties.

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| go-cli-tests | `cd cli && go test ./...` | 8 | Floor under properties 2-6. The Go runtime that records check receipts, reads verdict.v2 evidence (`cli/internal/verdictcheck`), resolves state paths, and runs the deterministic check registry (`cli/internal/gates`) is asserted only by this suite. |
| go-vet-clean | `cd cli && go vet ./...` | 4 | Floor under the same runtime: vet-clean is the precondition for treating any measurement it emits as a fact rather than an artifact of a latent bug. |
| verdict-contract-corpus | `bash scripts/check-verdict-contract-corpus.sh` | 7 | Properties 3 and 5. Runs the shared golden corpus through all three implementations of verdict.v2 (JSON schema, the Python Validate writer, the Go evidence reader) so PASS, FAIL, and NOT_PROVEN mean one thing across runtimes. Fail-closed: a pass proves all three legs ran, never that a leg was skipped. |
| contract-compatibility | `bash scripts/check-contract-compatibility.sh` | 5 | Property 6. Every schema a durable verdict is written against parses as JSON, every contract reference in the documentation index resolves on disk, and every example conforms to its schema. |
| contracts-structural-floor | `bash scripts/check-contracts-structural-floor.sh` | 4 | Property 8. A consumer keeping their own tracker, CI, and release system integrates through `docs/contracts/**`; each contract must be titled, cataloged in the documentation index, non-trivial, and paired with valid JSON. |
| goals-denominator | `d=$(mktemp -d "${TMPDIR:-/tmp}/ao-goals-den.XXXXXX"); (cd cli && go build -o "$d/ao" ./cmd/ao) && "$d/ao" goals validate --json > "$d/report.json" && jq -e '.goal_count >= 1' "$d/report.json" > /dev/null; rc=$?; rm -rf "$d"; exit $rc` | 6 | Property 5 turned on this file. Builds `ao` from source, requires `goals validate --json` to exit 0 (valid), and requires a nonzero denominator. Guards the exact regression that emptied this table: between 2026-07-14 and 2026-08-24 the Gates section was absent, the parser returned zero goals with no error, and the whole fitness surface reported green on 0/0. |

## Engineering reuse and the learning hypothesis

The product goal is cumulative engineering value across disposable agents.
Track concrete cases: a behavioral example becomes a regression check, a tool
is reused, or a recorded decision changes the next action. For an effectiveness
claim, report later outcomes and total observed cost, including review,
rework, and maintenance; disclose missing comparisons. Guidance draws from the
[Practice Registry](PRACTICE-REGISTRY.md), but a practice's lineage does not prove
that its AgentOps implementation improves an agent's results.

The [September pilot](https://github.com/boshu2/agentops/pull/1125) found no
observed paired endpoint difference in eight comparable coding pairs and no
demonstrated incremental benefit in its separate memory experiment. Keep those
limits alongside any claim about compounding. Use existing task evidence;
measurement does not require a new artifact for every task.

The selected CDLC (Context Delivery Lifecycle) contract maintains external
context/environment around disposable agents; it neither trains weights nor
promises deterministic inference. Discovery, implementation and independent
validation operate within caller-owned intent and allowance. Their corresponding
skills are optional.
Optional Memory recall and mining are skill guidance, not a new runtime or
automatic private-source authorization; evolve restoration belongs only to
T25. No unavailable entrypoint is enabled by this document.

Caller-selected external Markdown/OKF memory requires exact factual support
and destination disclosure before Git ingestion, then later observed utility.
These are distinct claims. A reviewed page alone shows no memory benefit;
that requires evidence from later work. Keep null, failed, missing and harmful outcomes;
there is no numeric utility score, universal context percentage, page quota or
mandatory lesson per session. Learning cannot change a completed verdict or
silently promote policy. [ADR-0016](docs/adr/ADR-0016-state-tiers.md) and
[RPI traversal](docs/architecture/rpi-traversal.md) own the selected contracts;
the executable gates above measure their stated existing floor only.
