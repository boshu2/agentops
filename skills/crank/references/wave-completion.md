# Wave Completion

Crank completes one implementation wave inside one behavioral leaf. The wave
boundary exists for fast factual feedback and an orchestrator decision; it is
not a semantic-validation, learning, delivery, or reporting boundary.

## 1. Prove the wave cheaply

The orchestrator, not the implementer, runs the smallest deterministic checks
that prove the behavior changed in this wave:

- the leaf's affected acceptance examples;
- evidence-schema validity and changed-scope integrity;
- a conditional surface check only when that surface changed; and
- base attribution for a failure before classifying it as introduced.

Do not rerun a broad suite merely because a wave ended. Preserve exact command,
mode, candidate/tree, registry/toolchain, environment, result, and evidence path
so a later phase can reuse the factual receipt.

If `.github/workflows/*.yml` changed, include
`bash scripts/validate-ci-policy-parity.sh`. Other conditional checks follow the
same changed-surface rule.

## 2. Write one canonical wave checkpoint

Write `.agents/crank/wave-${wave}-checkpoint.json` with:

- `schema_version`, `wave`, `timestamp`, and `git_sha`;
- completed/failed work and files changed;
- acceptance result and per-criterion evidence when criteria exist;
- plan-input changes, if any; and
- exact deterministic receipt references.

Validate it once:

```bash
bash skills/crank/scripts/validate-wave-checkpoint.sh \
  ".agents/crank/wave-${wave}-checkpoint.json"
```

Do not copy the checkpoint to another directory, hand-write a phase summary,
archive duplicate notes, or create a checkpoint commit solely to prove Crank ran.
A legacy consumer may receive a link-only projection of this canonical JSON.

## 3. Return the remaining-plan facts

Return `DONE`, `PARTIAL`, or `BLOCKED` plus:

- checkpoint path and digest;
- targeted-check results;
- whether acceptance, dependencies, write scope, or risk materially changed;
- work remaining in the same leaf; and
- the exact next failing proof, if another wave is possible.

Crank stops. It does not invoke Validate, Learn, Premortem, delivery, or tracker
closeout.

The orchestrator may admit another wave without Validate or Learn when all are
true:

1. the same leaf and Premortem-bound plan remain authoritative;
2. targeted acceptance is green;
3. plan inputs and risk are unchanged; and
4. the tranche has fewer than three waves and remains below 90 minutes.

A material plan-input change returns to Discovery and receives one fresh
Premortem. A failed introduced acceptance check is `REPAIR` or `REPLAN`, never an
automatic ANDON. When the leaf is complete, freeze one candidate for one final
Validate and Learn transaction. When three waves or 90 minutes arrives first
and the leaf is incomplete, persist `PARTIAL` resume evidence and stop; the soft
boundary does not authorize proof, Learn, or delivery.

## 4. Optional cleanup

Cleanup is a normal implementation choice, not a fixed gate. Use the same direct
worker when local refactoring is needed under green tests. Spawn another worker
only for an explicitly admitted disjoint write scope. Full deterministic proof
runs once on the final post-repair candidate, not at every wave boundary.
