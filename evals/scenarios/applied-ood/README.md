# Applied-OOD scenario set (tracked, reproducible)

The moat-eligible holdout scenarios for the `ao eval scenario-ab` gold/corpus
A/B. Tracked here (not in gitignored `.agents/holdout/`) so the moat evidence is
reproducible — the gap age-wp1 flagged.

Each scenario is **applied-OOD**, per the pre-registered rule in
[`docs/evals/applied-ood-claim-rule.md`](../../../docs/evals/applied-ood-claim-rule.md):

- graded by `acceptance_vectors` (no `answer_key`) → `verdict_class: applied-ood`,
  `moat_eligible: true`;
- the corpus holds **doctrine the agent must APPLY**, not a string to recall;
- the discriminator is **applied judgment, not a leakable secret** (nothing a doc
  or bead could leak lets the control pass);
- the shape is **contrarian-default**: the model's standard answer is *wrong* for
  this repo, so the control fails by confidently applying the wrong default — not
  from ignorance of a string.

| Scenario | Repo doctrine applied | The wrong default it beats | Gradable |
|---|---|---|---|
| `s-2026-06-17-001` reinforcement-gate | reward deposits gate on a verified outcome | reinforce on popularity / co-presence / citation | one-shot |
| `s-2026-06-17-002` merge-sha-after-push | read the landed SHA from origin/main after push | `git rev-parse HEAD` (local) right after push | one-shot |
| `s-2026-06-17-003` single-writer-per-file | re-cut to one writer per file; compose via data contract | lock/lease/flock so writers take turns on the shared file | one-shot |

All three are **one-shot-gradable**: the single-turn runner produces a
design/answer the acceptance-vector judge can grade. **Execution-required**
applied-OOD scenarios (ship code that trips a gate; dispatch a process that must
engage) set `runner_mode: "agentic"` on the scenario file — the agentic runner
(age-5tv) performs multi-turn work in an isolated workspace.

## Running one

```sh
ao eval scenario-ab --scenario evals/scenarios/applied-ood/s-2026-06-17-001-reinforcement-gate.json \
  --output /tmp/scorecard.json
```

The control arm runs first under filesystem isolation (age-9a9). If it already
clears the threshold the run aborts as a `ceiling_violation` (no headroom — a
guessable scenario is caught here). A passing card carries
`verdict_class: applied-ood` + `moat_eligible: true`; read the publication rule
before claiming a moat result from it. **n=1 over a stochastic judge is not a
proof** — the moat stays UNPROVEN until a multi-scenario, multi-seed run.
