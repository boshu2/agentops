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

Harbor 0.22.0 multi-step results omit the verifier mode at both trial and step
levels. For that shape only, the reader can use the receipt's explicitly cited
staging manifest and frozen task configuration. It verifies the manifest hash,
rehashes the task with Harbor's `dirhash`, matches native task path/checksum,
runtime image identities and step names, and rejects contradictory modes or
step verifier overrides. This requires the pinned Harbor environment (which
supplies `dirhash`). Missing dependencies or evidence retain the exclusion.
The readout labels this **frozen task configuration (runtime isolation not
measured)**; a green reward alone cannot enable the fallback.

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
With fewer than two task clusters, interval status is `insufficient_clusters`;
with degenerate bootstrap deltas it is `degenerate_no_interval`. Both retain the
existing statistics implementation's point delta and sample counts, and display
null interval bounds/confidence instead of a misleading zero-width interval.
One small development batch is not a powered or held-out skill-benefit claim.
The automated pilot recommendation is `insufficient-evidence`; the specialist
can make a narrower supported, explicitly provisional maintenance decision.

Native verifier reward 1 is an endpoint success. Worker false completion and
feasibility remain unknown. A required fresh
handoff or exact-subject judgment is not waived by a green executable oracle.
Native rewards are retained for inspection; the reader never parses worker
prose into acceptance.

For an ordinary native coding observation, optionally join one explicitly
selected subject using the existing provenance judgment inputs. No skill
invocation or new trial is required. The acceptance, subject, author and required
reviewer profiles must come from the caller, independently of candidate verdicts:

```bash
# From cli/. Add --job for a captured Harbor trial when applicable.
go run ./cmd/skill-trial-report \
  --session /protected/native/author.jsonl \
  --root /absolute/candidate \
  --manifest /protected/proof/manifest.json \
  --intent /protected/proof/acceptance.intent \
  --author-context-id native-author-id \
  --required-profiles /protected/proof/profiles.json \
  --allowed-provider openai \
  --evidence-root /protected/proof \
  --verdict /protected/proof/CONTENT_DIGEST.json > /protected/native-report.json
```

Render from the repository root:

```bash
python evals/skills-rpi/readout.py --report /protected/native-report.json
```

The profile file uses the existing `ao provenance verify-judgments` format: an object with a `profiles`
array of `id`, `runtime`, `model`, `family`, and `effort` strings. Use an empty
effort only when no actual-effort requirement was selected. Repeat `--verdict`
for required legs and `--allowed-provider` for independently authorized
providers; these flags launch nothing. Supply `--base-manifest` for deletions.
Missing required verdicts produce an unproven report. Partial judgment options
are usage errors; invalid or missing subject/receipt/transcript evidence remains
visible in the report's `work.problems` or original `work.judgments` result.

The native Go reader calls `evidence.VerifyJudgments`, the existing provenance
owner. It verifies the expected subject and acceptance, original verdict
integrity, distinct author/reviewer identities, freshness, required profiles,
hash-bound judgment receipts and native reviewer completion. Verdicts, receipts
and reviewer transcripts retain that owner's private non-Git root and size
restrictions. It **does not mechanically resolve arbitrary criterion evidence
references**. The fresh reviewer owns their semantic assessment. This reader
reports supplied independent judgments and never issues a semantic verdict.

`work.status` is `accepted` only with completed native author execution and
satisfied independent PASS coverage. A correctly bound independent FAIL remains
`failed` even though PASS coverage is unsatisfied. Other incomplete or invalid
proof is `not_proven`; original legs and problems remain visible in every case.
`work.execution` separately reports `completed`, `failed`, `unfinished` or
`unknown`. A timed-out attempt cannot become accepted because its code or later
review passes. Missing native author completion is unknown; terminal markers
whose ordering the accounting reader cannot establish also remain unknown.

Association requires the explicitly supplied native author identity. One
session found in one trial joins that trial. Different copies of a session or
an identity shared by several trials are ambiguous; no favorable retry is
selected. A standalone session stays visible under `native_work` without an
invented trial, arm, elapsed interval or cost. Unmatched trials retain unproven
acceptance regardless of their endpoint reward. This association is a declared
caller-selected relationship, not automatic proof of which session wrote Git
content. Reported source hashes and exact subject/acceptance identify its bounds.

Per-arm `independent_work` shows `known_accepted`, `known_failed`, observed
`not_proven`, `unobserved_assignments`, the all-assigned denominator and the
known accepted fraction of it. A zero known count is not proof of zero actual
accepted work when coverage is missing. `independently_completed_outcomes`
remains null until every assignment has a known accepted/failed disposition.
The Python view consumes the native Go report; it does not reverify verdicts or
promote comparison receipts or scalar rewards into independent acceptance.

For validation fixtures, E2 captures the separate verifier's `verifier/grade.json`
in `trial.documents` with `path`, `sha256` and parsed `data`. The readout consumes
its `case_results` only when the external receipt, native separate-verifier
mode, terminal evidence and captured source identity are valid. The oracle emits
cases only after its source, scope and ground-truth checks succeed. Each case
contains `case_id`, `expected`, `actual` and `classification`. All cases remain
visible even when the trial's endpoint reward is zero.

The per-arm `validation_cases` readout reports the raw expected/actual judgments
and these descriptive metrics:

- `false_acceptance`: cases classified false acceptance / cases expected FAIL
  or NOT_PROVEN;
- `false_blocker`: cases classified false blocker / clean cases expected PASS;
- `justified_not_proven`: justified abstentions / cases expected NOT_PROVEN.

Missing actual judgments (`classification: missing`) keep the known expected
denominator but make the affected numerator and rate unknown; known counts and
unknown-case counts remain explicit. An absent, malformed, integrity-invalid or
untrusted grade makes the total denominator unknown, not zero. The table also
states how many attempts lack a usable case grade. Worker-authored result fields
cannot substitute for this separate verifier document. These case observations
do not rehabilitate an invalid paired comparison or establish general uplift.

The same source's `not_checked` entries join `remaining_workflow_gaps` with their
job and source reference. Missing gap coverage remains unknown. Case-level truth
does not prove a fresh native handoff or independent completion of the workflow;
only the explicit native work join above supplies that separate evidence.

Cost and timing retain their measurement windows:

- Harbor `agent_result` counters/cost are incomplete per-trial estimates: the
  adapter may omit nested sessions even when every attempt has a scalar. The
  table labels them **Harbor estimate (incomplete)** and retains both the scalar
  sum and the count of attempts without an estimate. A Harbor estimate per
  endpoint success appears only with a scalar for every reported assignment;
  it is still incomplete, and total billed cost per independently accepted
  outcome stays unknown.
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
