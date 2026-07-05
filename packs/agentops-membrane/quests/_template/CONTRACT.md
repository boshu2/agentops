# Acceptance Contract — {{QUEST}}

> This file is the **ruler** the membrane's close door judges the build against —
> NOT the builder's spec. The close gate reads it from `main` only
> (`git show main:CONTRACT.md`); the builder never reads it. Every clause below
> is **default-FAIL**: it starts unsatisfied (`[ ]`) and remains a finding until
> the diff *demonstrably* makes it pass. "No verdict = not done"; here, "no
> satisfied clause = a finding."

## Ask (one line, verbatim)

{{ASK}}

## Non-goals

- <what the builder must NOT touch — the planner fills this from the ask>

## Acceptance clauses (numbered, default-FAIL)

Each clause is a checkable **Given / When / Then** with an exact verification
command whose exit code proves it. `[ ]` = FAILING (the default state). A clause
is satisfied ONLY when its verification command exits 0. There must be **at least
two** clauses, and every clause MUST be command-checkable — if a clause cannot be
proven by an exit code or a concrete artifact, reshape it until it can (a clause
that can't fail can't be an acceptance clause).

1. [ ] **Given** <initial state> **When** <exact action / command> **Then** <observable result>.
   - Verify: clause 1 assertion in `./test.sh` exits 0.
2. [ ] **Given** <initial state> **When** <exact action / command> **Then** <observable result>.
   - Verify: clause 2 assertion in `./test.sh` exits 0.

<!-- planner: add clauses as the ask requires; NEVER fewer than two; keep each
     line's leading `N. [ ]` shape so the default-FAIL state stays machine-visible;
     flip `[ ]`→`[x]` is the BUILDER's job via a green ./test.sh, never yours. -->

## Done

`./test.sh` exits 0 (every clause satisfied) AND the diff introduces no change
outside the ask / into a non-goal.
