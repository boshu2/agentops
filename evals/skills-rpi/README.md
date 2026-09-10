# Coding evaluations for AgentOps skills

This development suite measures a frozen installed skill package on actual Go
work. Harbor owns task execution and isolated verification. Native sessions own
usage and conversation identity. The report is a read-only view; `ao eval`
remains retired. Nothing here becomes a required AgentOps user runtime.

Use this suite when a skill change or repeated engineering failure needs a
retain/revise/remove decision. Use `scripts/probe-skill.sh` for a narrow behavior
probe. A saved lesson needs a separate transfer experiment before claiming
memory helped later work.

## Prepare a comparison

Prerequisites: Python 3.12 or 3.13, Docker with Compose and Buildx, an available native
Codex account, and the explicit pinned model. Install `requirements.txt` in an
external virtual environment. The runtime pins Harbor 0.22.0 and Codex 0.154.0;
older Codex versions may be rejected by the selected model. Check Docker's
health and native authentication before spending a live trial.

Choose a protected external, non-Git output directory and a frozen complete
`skills-codex/` projection. Do not pass operator home, production work trackers,
private repositories, transcripts, or a solution archive as task input.
Authentication uses an explicit native runtime file locator; its content is not
copied into a public fixture or staging manifest.

```sh
python evals/skills-rpi/prepare.py \
  --task evals/skills-rpi/tasks/learning-read-error \
  --output /absolute/protected/cohort/learning-read-error \
  --skills /absolute/frozen/skills-codex \
  --auth-file /absolute/native/codex/auth.json --reps 2
```

Preparation builds the worker and separate verifier, freezes their image IDs,
stages public source and the full package, and writes native Harbor job configs.
Digest-named local retention tags keep earlier images available when another
variant replaces a build tag; explicit Docker image removal can still remove them.
It starts no agents and refuses to overwrite an existing output. The initial
Go incident is a historical development case; this suite does not present it
as an unseen holdout. Other cases are sanitized standalone Go modules.

Before launching, declare the decision, assigned cases/repetitions, cumulative
start limit, concurrency and actual time ceiling. Include infrastructure
failures, interrupted attempts, setup, reviewers and analysis. Changing a
configuration never refunds a start. The September pilot permits 24 cumulative
coding starts, at most two concurrent, and eight separately budgeted downstream
memory starts. Those are experiment bounds, not universal RPI rules.

Run selected config files through Harbor directly. For example, with a host
`timeout` utility and a predeclared 1,320-second outer limit:

```sh
timeout -k 30s 1320s harbor run -c /absolute/protected/cohort/learning-read-error/learning-read-error-control-1.json
```

Each config disables automatic retries and executes one trial. Task timeouts
bound the worker and separate verifier. Multi-step tasks have per-step limits;
the host timeout includes all steps and cleanup. Do not start another run while
a previous handle is merely waiting. Read its native terminal result. Harbor's
process exit status alone is not a task outcome. Check that its owned containers
are gone; do not remove unrelated containers.

These utilities do **not** enforce a cumulative caller/desktop goal budget.
The selected consumer must admit starts and enforce its own aggregate limits.
Report parent-only observed limits separately from runtime-enforced limits.

## Rebuild the readout

No model call or hand-written evaluation report is required for a new Harbor
trial to appear. Select its native output directories explicitly:

```sh
cd cli
go run ./cmd/skill-trial-report \
  --job control=/absolute/cohort/task/jobs/task-control-1 \
  --job treatment=/absolute/cohort/task/jobs/task-treatment-1 > /absolute/protected/native.json
```

From the repository root, join frozen launch identities and render the decision
view (readout dependencies are in `requirements-readout.txt`):

```sh
python evals/skills-rpi/receipts.py \
  --staged /absolute/cohort/task/staged.json > /absolute/protected/receipts.json
python evals/skills-rpi/readout.py \
  --report /absolute/protected/native.json \
  --receipts /absolute/protected/receipts.json --format markdown
```

Repeat `--staged` for each prepared task and `--job` for every admitted native
job, including invalid attempts. Receipts check frozen source/package/config
bytes. They establish declared comparison compatibility, not proof that a
worker read or benefited from a skill. Do not rewrite a receipt after observing
an unfavorable result. A broken oracle requires a new version; keep affected
runs and label their comparisons invalid.

Endpoint rewards are not independent semantic acceptance. Inspect verifier
`grade.json`, exact-subject review and native identities for criteria outside
the executable oracle. Keep false completion, false acceptance and needless
blocking separate; do not infer any of them from scalar reward alone. Missing
review, context transition, usage, billing or phase coverage stays unknown.
Do not add native and Harbor usage together or sum repeated cumulative copies.

For an ordinary coding observation, the same reader accepts explicit
`--session /absolute/native/session.jsonl` inputs. This collects execution facts;
an optional one-subject join accepts the existing provenance flags (`--root`,
`--manifest`, `--intent`, `--author-context-id`, `--required-profiles`,
`--allowed-provider`, `--evidence-root`, and repeated `--verdict`). It reuses
`evidence.VerifyJudgments` over caller-owned exact subject, acceptance and native
reviewer receipts, then requires a unique observed author association and
completed execution before reporting accepted work. No skill invocation is
required. Endpoint rewards, verified FAIL, missing proof and unfinished work
remain separate. See [the readout contract](readout.md) for the complete example,
denominators and limitations, including unresolved arbitrary criterion evidence
references. It does not automatically discover or join arbitrary sessions to
Git acceptance or reviews. Historical imports remain observational, with missing
baseline fields visible. They do not establish skill efficiency.

A small baseline pilot normally ends with **insufficient evidence** about
uplift. Keep case-level outcomes, uncertainty, all unsuccessful costs and missing
cells. Do not add runs to obtain separation. General compounding, cross-model
or cross-repository claims require their own later evidence.
