# Canonical Validation Protocol

This reference carries the detailed mode, target, and evidence rules for the
`validate` kernel. Load only the sections selected by the current mode.

## Mode details

- Default: two independent judges; PASS requires both PASS.
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

## Assurance and landing

Non-merge validation may use runtime-native judges. Assurance close requires the
policy-selected independent-family floor. Benched families are not presented as
live strict lanes.

Landing is a different door: `ao pawl` produces the commit-bound verdict and the
pre-push gate authorizes the remote-main write. A validation PASS never replaces
or pre-authorizes that landing proof.

## Failure routing

WARN/FAIL/REFUTED findings return to the operating loop as re-plan evidence.
Repair and remeasure automatically while the bounded budget has progress.
HOLD/ESCALATE occurs only on a breaker: exhausted attempts/time/cost, oscillation,
refusal, unavailable authority, or explicit risk/waiver judgment.
