# Canonical Validation Protocol

This reference carries the detailed mode, target, and evidence rules for the
`validate` kernel. Load only the sections selected by the current mode.

## Mode details

- Default: one independent fresh-context judge; PASS requires that judge to
  return PASS. Additional judges are an explicit depth or council choice, not
  part of the default validation contract.
- `--quick`: inline structured review. It may guide work but is stamped
  `waived`, never independently validated.
- `--deep`: four perspectives—missing requirements, feasibility, scope, and
  specification completeness. PASS uses the declared majority rule, with any
  unresolved blocker still failing closed.
- `--mixed`: the same perspectives across explicitly selected model families.
- `--debate`: two adversarial rounds with critique and rebuttal.
- `--mode=post-impl`: complexity, bug sweep, acceptance roll-up, then isolated
  judges. A refactor that changes acceptance/unit test text is a new behavior
  slice, not a refactor.
- `--mode=pre-impl`: plan/spec checks, optionally specialized by `--target`.
- `--mode=pr`: upstream alignment first, contribution rules, atomic isolation,
  scope containment, then tests/lint.

The eight-mode budget is fixed. A ninth mode must replace or merge an existing
mode rather than growing the public surface.

## Pre-implementation targets

| Target | Required checks |
|---|---|
| default plan | temporal interrogation, error/rescue map, FAIL patterns, test pyramid, enum/input validation |
| scenario | holdout scenario and falsifying edge |
| fitness | each GOALS.md gate against current measured state |
| ratchet | current checkpoint, evidence, and legal next transition |
| scope | frozen paths, authority boundary, and escape prevention |
| skill | strict hygiene plus profile-aware deep audit |
| health | executable repository health probes and disclosed gaps |

A goal-design packet containing `intent.md` and `driver.md` first runs
`scripts/check-goal-design-packet.sh <packet-dir>`; nonzero is FAIL evidence.

### Documentation behavior scenarios

When the artifact is prose whose correctness depends on meaning, use the
`scenario` target with an explicit behavior contract. Deterministic prechecks
may pin the artifact, resolve links, confirm commands, and validate evidence
schemas. They must not infer whether prose is misleading, primary, historical,
contradictory, coherent, or semantically correct.

Judge only the actual pinned artifact. A blocking documentation finding must:

1. cite an exact artifact passage;
2. name the acceptance scenario it violates;
3. show a material user decision affected by that passage; and
4. survive independent reproduction or reconciliation.

Counterfactual wording that is absent from the artifact is holdout input, not a
current blocker. Do not expand acceptance during review by converting every
imagined phrase into a new deterministic recognizer. After two failures from
the same semantic family, re-plan the mechanism at the abstraction boundary
instead of growing regexes or keyword windows.

## Post-implementation acceptance

Every Given/When/Then maps to a passing acceptance test for its own vertical
slice. Activity logs and parent-issue summaries do not close a bead. Apply the
Completion-Claim Kernel in `skills/shared/validation-contract.md` to every
DONE/closed/green claim.

The acceptance judge is blind and context-isolated. `judge_id == author_id`
cannot independently PASS. Inline fallback is marked waived and cannot satisfy
an assurance close.

## PR checks

Run in order:

1. upstream alignment (`git rev-list --count HEAD..origin/main`) and conflict risk;
2. repository contribution rules;
3. one thematic/atomic change shape;
4. scope-creep containment;
5. relevant tests and lint.

FAIL includes executable remediation such as rebase or split-by-type; it never
mutates the branch from the judge lane.

## Evidence discipline

The validator reruns cited commands on the actual artifact. It does not accept
the author's evidence file as proof. Every count, timing, commit, and pass rate
is pasted from captured output. Uncited figures fail until measured.

Corrections are appended as dated errata crediting the source measurement;
silently rewriting an evidence claim is forbidden.

Verdict text uses anchored lines:

```text
VERDICT: PASS

COMMANDS RUN:
judge=<validator_session> command=<command rerun by this judge>
<bounded output excerpts produced by this judge>
REASONS:
- each reason cites a command or artifact
```

Exactly one verdict is allowed. `COMMANDS RUN:` must include at least one
`judge=<validator_session> command=<command>` line, and the pinned author identity
must differ from that validator session for PASS. A judge that ran nothing is a
reader, not a verifier.

## Judge isolation

Every judge brief includes this exact clamp:

> READ-ONLY except writing your single verdict file at `<path>`. Do NOT commit,
> push, or run tracker/infra ops (git push, br/bd, dolt).

Register dispatch intent before spawning. Two validators accidentally assigned
to the same lane/bead are a dedup incident, not an independent quorum.

## Verdict boundary

Validation may use runtime-native fresh-context judges. The result is an
immutable proof artifact, not implementation, learning, retry, tracker, or
delivery authority. After the verdict is written, Validate returns it to the
caller and stops. Repository-specific publication policy remains outside this
skill.

## Failure handoff

WARN or FAIL identifies the owning producer, evidence, and one executable next
action. The caller decides whether to repair, re-plan, stop, or escalate;
Validate does not control that loop.
