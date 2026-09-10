# Compact pilot readout

Consumer: the caller deciding whether to retain, revise or remove skill guidance.
The observed defect is incomplete attempt/cost accounting and comparisons with
unknown or changed configuration. This rebuildable view consumes the existing
development report and external runner receipts; it owns no trial, work, verdict
or release state. Retire it when the selected evaluator directly supplies the
same used decision readout. No model call is needed to collect or render a run.

Use Python 3.12 or 3.13 with `requirements-readout.txt`. NumPy and SciPy are
pinned for the existing `evals/_stats` imports; tests use pytest.

```bash
# In cli/, capture all selected native jobs, including smoke/debug failures.
go run ./cmd/skill-trial-report \
  --job control=/protected/jobs/control-one \
  --job treatment=/protected/jobs/treatment-one > /protected/native-report.json

# From the repository root. This reads explicit files and writes only stdout.
python evals/skills-rpi/readout.py --report /protected/native-report.json \
  --receipts /protected/launch-receipts.json
python evals/skills-rpi/readout.py --report /protected/native-report.json \
  --receipts /protected/launch-receipts.json --format json > /protected/readout.json

python -m pytest -q evals/skills-rpi/test_readout.py evals/_stats
```

Without receipts, all attempts still appear and no pair becomes comparison
evidence. Without the statistics dependencies, accounting still works and uncertainty explicitly says
unavailable. `--control` and `--treatment` select arm labels, including accepted
versus candidate versions. Other arms remain in accounting. `--n-required` is
only for a task-cluster floor justified and fixed before data collection; omit
it for the descriptive pilot. It is not an automatic power calculation.

Receipts come from the external staging/launch consumer, never the worker. The
minimal join is `schema_version: 1`, with `trials` entries containing:

- `job_name`, `task`, positive integer `rep`, and `arm`;
- `configuration_sha256` matching the native job config file bytes and
  `task_checksum` matching Harbor's native task checksum;
- `oracle_sha256` and `skills_sha256` (explicit null for no package);
- `runtime` with `harbor_version`, `codex_version`, `model` (provider/name),
  `reasoning_effort`, `worker_image_id` and `verifier_image_id`;
- `isolation` with `separate_verifier: true`, `private_inputs_excluded: true`,
  and the actual `network_policy`. If contamination is discovered, retain the
  receipt with `contamination_detected: true` at the top level or in isolation.

The runner captures staged byte/image identities before launch and verifies
the native configuration before joining its hash afterward. The readout checks
these links, native executed model/version, effective agent settings and
separate-verifier mode. It compares native job and trial configuration while
ignoring only task/package paths already bound by identity and output paths.
Different runtime, oracle, task, isolation or other configuration disqualifies
the pair. Same package in both arms, session reuse across trials, missing
identity, accounting diagnostics and ambiguous retries also disqualify it.
There is one task/repetition/arm per job; duplicate cells are not resolved by
choosing the favorable attempt. Preserve each start instead of resuming into
an overwritten result directory.

Receipt assertions establish captured availability and configuration. They do
not prove the worker read a package, performed a relevant action, or could not
exploit an unknown isolation gap. False assertions cannot be repaired by a
passing reward: inspect or invalidate the source receipt. Hashes establish
content identity, not truth. This reader is not a sandbox or attestation service.

The primary displayed rate is **endpoint successes / all assigned-or-observed
attempts**, using native expected counts where known and never shrinking below
observed attempts. A missing expected total makes the aggregate rate unknown;
known subtotal and observed outcomes remain visible. Receipt diagnostics, such
as planned repetitions not yet observed, are preserved without inferring trial
starts from them. Receipt jobs absent from
the native report are listed separately and mean incomplete source coverage.
They are not silently reconstructed as successful, failed or started trials.
Infrastructure and unknown outcomes stay in that accounting. Only comparable
endpoint pairs enter descriptive inference; missing/infra pairs are excluded
and listed, while comparable execution errors score zero. This exclusion may
bias the subset, so it cannot replace the overall rate.

`paired_outcomes` pairs by task and repetition. `evals/_stats` resamples task
clusters with all repetitions retained, weighting tasks equally. Repetition
numbers do not guarantee matched provider randomness. A zero-crossing interval,
`no_change`, underpowered or degenerate result never becomes equivalence.
One small development batch is not a powered or held-out skill-benefit claim.
The automated pilot recommendation is `insufficient-evidence`; the specialist
can make a narrower supported, explicitly provisional maintenance decision.

Native verifier reward 1 is an endpoint success. The reader leaves independent
completion, false completion, false acceptance, needless blocking and feasibility
unknown unless separately measured by their source owners. A required fresh
handoff or exact-subject judgment is not waived by a green executable oracle.
Native rewards are retained for inspection; the reader does not invent these
semantic measurements or parse worker claims into acceptance.

Cost and timing retain their measurement windows:

- Harbor `agent_result` counters/cost are per-trial native source values. Partial
  cost sums show the unknown-attempt count. A Harbor cost per endpoint success
  appears only with complete reported cost and assignment coverage; total billed
  cost per independently accepted outcome stays unknown.
- Native cumulative usage remains per session and copy. No copies, parents,
  children or Harbor counters are summed together. Input includes cached input;
  output includes reasoning. Missing counters are null, not zero.
- Trial elapsed distributions cover the recorded start/finish, including setup
  and verifier work. Native phase endpoints remain available in JSON. Unfinished
  duration, outer setup, orchestration and experimental grading/analysis are not
  fabricated. No clock sampling or pricing lookup is performed.

Raw reports, launch receipts and transcripts belong in the caller's protected
external non-Git evidence directory. Tests below use synthetic sanitized native
records and never launch an agent. These replay checks validate the reader's
honesty; they do not demonstrate a live model or workflow succeeding.
