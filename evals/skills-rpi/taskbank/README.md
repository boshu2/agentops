# Development task fixtures

These five sanitized standalone Go tasks complement `learning-read-error`.
They are exposed development cases, not hidden holdouts or cross-repository
evidence. Their consumer is the development-only Harbor comparison suite and
its decision readout. They test the missing input, recovery, handoff, review
and stopping distinctions in the accepted evaluation plan. Retire or version
a case when its oracle is invalid, its contract changes, or its scenario stops
representing that distinction; preserve original trial dispositions.

| Directory | Endpoint and discriminating controls | Workflow coverage |
|---|---|---|
| `input-scope` | Merge input union; inventory-scan wrong control fails unchanged-input exclusion | Native review is separately observed |
| `persistence-recovery` | Real file replacement; cancellation, reopen, exact payload and cleanup; metadata-only wrong control | Deterministic cancellation hook, not process death or power loss; no access-preservation variant |
| `fresh-handoff` | Two source files; parser-only and summary-only candidates both fail integration | Requires native producer termination and fresh successor; file presence is not handoff evidence |
| `validator-controls` | Seeded deadline defect, correct counterpart and missing original check record; blanket PASS and blanket FAIL both lose | Review of recorded submissions; no finding-count rewards |
| `bounded-stop` | Real overflow repair plus independent recorded repair/stop/helper decisions | Recorded decision replay; actual cancellation, hard-bound enforcement and helper usefulness need native observations |

The validation controls deliberately judge a frozen submission. The acceptance
requires correct behavior and an original successful check record bound to its
source. A and B records were produced by actual local fixture checks and state
the observed toolchain. C has correct source but no original record; rerunning
tests can check today's behavior but cannot recover that original event. It is
therefore NOT_PROVEN under this acceptance. The records are development data,
not production attestations. The grader compares exact case verdicts, never
finding counts, and source/receipt bytes are immutable during the review.
`grade.json` retains each case's expected and actual verdict plus its disposition,
including false acceptance, false blocking and acceptance with missing evidence.

## Staging boundary

Each task's worker build context contains `environment/Dockerfile`,
`environment/source` as `source`, and the same public `helpers` in both arms.
No `tests`, `solution`, prior Git history, host home or sibling output belongs
in that context. The Dockerfile creates a fresh one-commit baseline from the
sanitized snapshot and pins Codex 0.154.0 and Go 1.27.1. Actual image availability
and runtime compatibility are the runner's checks; local calibration reports
its actual host Go version rather than asserting the container pin was tested.

For the separate verifier, stage `tests/*`, `environment/source` as `source`,
and this directory's `verify.py`. The task Dockerfile copies the pristine
source to `/baseline` and the fixed oracle to `/tests`. Export the worker's
actual `/app/work` to that isolated verifier. Its network is disabled. Only
permitted production bytes enter a temporary trusted baseline copy; the
worker's added tests are preserved as output but cannot replace the oracle.
The grader rejects changed public tests, module files, extra source and
symlinks. It does not provide a security sandbox for malicious Go code; runtime
container isolation and restricted mounts remain required.

`reward.txt` is **endpoint correctness only**. The readout must also consume
`grade.json` and native evidence before claiming completed workflow acceptance.
Every unobserved review, handoff, live stop or enforcement criterion remains in
`not_checked`; it cannot be erased because the endpoint score is 1. The parent
runner owns joining those native facts, not the task's code or worker output.
For persistence, source review must also establish precommit flush/close order;
the file-system oracle establishes cancellation and reopened payload behavior.
Explicit exclusions such as power-loss durability are recorded as limitations,
not silently treated as tested acceptance.

For `fresh-handoff`, use `instruction.md` as the producer prompt, end that
session after `HANDOFF.md`, then run `successor.md` in a separate native context
over the same exported workspace with trajectory replay disabled. Record the
two real context identities and final subject externally. A single session
that repairs both files can pass the endpoint but cannot establish the required
fresh successor. The task source does not invent Harbor multistep configuration.

## Local calibration

Run from the repository root, placing raw output outside Git:

```sh
python3 evals/skills-rpi/taskbank/calibrate.py --output /absolute/external/evidence
python3 -m unittest discover -s evals/skills-rpi/taskbank -p 'test_*.py'
```

`--tasks input-scope` restricts calibration while developing. The calibration
runner makes a fresh candidate per control, overlays the intended solution or
an intentionally wrong/incomplete candidate, and calls the same verifier used
by the separate container. Public baseline tests remain green; hidden tests
make no-op repairs fail. Reset and integrity tests ensure failed candidates
cannot contaminate the pristine baseline or substitute their own tests.

No model calls, live handoffs, real stop delivery, container builds or claimed
skill benefit occur in these local checks. The public task and grader versions
must be frozen with the package, model and runner before any live batch.
