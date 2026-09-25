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

`fresh-handoff` declares Harbor's native `producer` and `successor` steps, with
complete prompts in `steps/<name>/instruction.md`. Set the job's
`agent.resume_trajectory=false` so the successor starts a fresh conversation
over the same workspace; root `instruction.md` is an overview, not an additional
step prompt. Record both actual native context identities and the final subject
externally. A single conversation that repairs both files cannot establish the
required fresh successor. Parsing this configuration does not prove execution;
the grader retains handoff and independent review in `not_checked` until the
external native observations establish them.

Both steps inherit the separate root verifier and run the whole-task endpoint
oracle. The producer-only repair is intentionally incomplete and should receive
0. No `min_reward` threshold is configured, so that ordinary zero reward does
not prevent the successor from running. Infrastructure failures without a
verifier result can still abort the task. `multi_step_reward_strategy="final"`
selects the successor's verifier result instead of averaging away that expected
producer incompleteness. Retain both step results in the attempted-run record.

Each agent step has a 450-second timeout; the task-level default remains 900.
These are agent-phase bounds, not a cap on setup, two verifier passes, or the
whole experiment. Harbor's job-level `agent.override_timeout_sec`, when set,
overrides each step's value; leave it unset or preserve 450 for this task. The
native runner and caller still own the aggregate allowance. No configuration
alone proves real stop delivery, cleanup, or enforcement.

## Local calibration

Run from the repository root, placing raw output outside Git:

```sh
python3 evals/skills-rpi/taskbank/calibrate.py --output /absolute/external/evidence
python3 -m unittest discover -s evals/skills-rpi/taskbank -p 'test_*.py'
```

`--tasks input-scope` restricts calibration while developing. The calibration
runner makes a fresh candidate per control, overlays the intended solution or
an intentionally wrong/incomplete candidate, and imports the verifier on the host.
This checks grader logic, not packaging. Before live use, `prepare.py` additionally
runs those controls through the frozen verifier image's actual entry point,
copying candidates and results through Docker without requiring host bind mounts.
Public baseline tests remain green; hidden tests
make no-op repairs fail. Reset and integrity tests ensure failed candidates
cannot contaminate the pristine baseline or substitute their own tests.

No model calls, live handoffs, real stop delivery, container builds or claimed
skill benefit occur in these local checks. The public task and grader versions
must be frozen with both packages, model and runner before any live batch; use
`prepare.py --check-staged` on the complete comparison immediately before either
arm launches. Packaging errors cannot satisfy a negative control, and all declared
controls, including valid semantic alternatives, must appear in calibration.

## Three-shape skill development cases

`cass-source-boundary`, `validator-controls` and `builder-recovery` are exposed
cases for the CASS adapter, Validate judgment and Skill Builder executable
workflow. They are not heldouts or evidence of causal benefit from shorter
prose. The task instructions and skill jointly guide execution. Forced skill
invocation does not measure normal catalog selection.

The two runtime cases reuse `verify.py`, `calibrate.py`, task-local Go oracles
and ordinary `workflow.sh` production overlays. Their fixed setup runs the
actual AO source-reader or builder against disposable synthetic state; no
replacement implementation of those interfaces is supplied. Correct, no-op,
false-completion and plausible wrong controls are in each task's `controls.json`.
CASS's wrong control confuses denied disclosure with no match. Builder's wrong
control overwrites the original failed report after otherwise valid recovery.
Valid formatting controls include detailed CASS evidence and equivalent Builder
failure wording. The oracle must not reject correct behavior for an undisclosed
output shape or English phrase. Builder's authored adapter semantics require
independent judgment; mechanical source/projection checks cannot establish them.
Native loading, resource use and action traces remain separate `not_checked`
items. A supplied synthetic observation is not host-delivery attestation.
The CASS verifier supplies shared baseline installed-resource paths for replayed
code that rechecks them. Worker loading is measured in the native worker trace;
verifier resource availability is not proof of that loading.

Before calling the existing `prepare.py`, copy each task to protected external
staging and populate its `environment/runtime/` with the selected current public
snapshot. Copy only `cli/`, `scripts/`, `skills/`, `skills-codex/`,
`skills-codex-overrides/`, `docs/`, `images/`, `.claude-plugin/` and `registry.json`;
reject symlinks and record relative paths and SHA-256 of every copied file.
Never copy repository state, tracker routing/data, operator home, native sessions,
prior output or evaluator tests/solutions into that runtime. Use current bytes,
not `git archive HEAD`, when judging an uncommitted candidate. The task image
builds AO from that frozen `cli/` and sets `AO_RUNTIME_ROOT=/opt/agentops` and
`AO_SKILL_BUILDER_BIN=/usr/local/bin/ao`. The Dockerfile never supplies evaluator
controls to the worker. In an old/new installed-package comparison, keep executable
owners identical across arms and use the same baseline instruction prose in any
runtime source or dormant projections accessible to both. Otherwise a control can
read the revised skill through the runtime tree. Record this deliberate runtime
composition separately from each installed package identity. Freeze the staged
task/runtime/package before admission.

For an isolated canonical development host, append image setup that symlinks
each selected complete canonical package from `/opt/agentops/skills/<slug>` into
`/root/.agents/skills/<slug>`; include the required sibling-resource closure.
Supply no Harbor skill bundle for this arm. For the generated host, use the
existing Harbor Codex adapter's skill-bundle upload/copy into
`$HOME/.agents/skills`, and do not also install canonical symlinks. These are
separate task/config identities, not two aliases for the same installation.
Use isolated containers, never change the operator's installed roots.

Capture the host's actual resolved skill path and bytes, active invocation
policy, native reads of required resources and endpoint output. Package staging
hashes and catalog visibility alone are insufficient. Count descriptions,
bodies, references, repeated reads and tool output when reporting context cost.
For CASS the missing-resource case is a deliberately incomplete operation-local
fixture; the grader checks no third AO call, but only native trace can establish
that the model stopped the dependent action rather than fabricating a marker.
The Builder endpoint checks retained contents and reports; native trace must
establish that recovery did not delete and recreate the source.

Local calibration uses the same actual runtime with no model calls:

```sh
AO_RUNTIME_ROOT=/absolute/frozen/public/runtime \
AO_SKILL_BUILDER_BIN=/absolute/current/ao \
python3 evals/skills-rpi/taskbank/calibrate.py \
  --tasks cass-source-boundary builder-recovery validator-controls \
  --output /absolute/external/new-calibration
```

Honor the caller's shared window even for calibration. In the S3/S4 pilot the
window begins at the first evaluation preflight or calibration, and failed starts
and retries consume the same allowance. The existing executable `prepare.py`
pins Harbor 0.22.0; do not substitute the newer requirements-file pin while
claiming the same runtime. Runtime availability remains a separate check.
