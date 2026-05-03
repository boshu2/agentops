# Local Compute Routing

Status: policy draft
Owner: AgentOps policy, with Mt. Olympus runtime evidence
Last reviewed: 2026-05-03

## Purpose

Local compute is useful when it improves measured software-factory yield, not
because hardware happens to be available. AgentOps uses local models as measured
substrate around frontier implementation: scouting, review, eval, corpus work,
background loops, resilience, privacy, and offline operation.

Frontier models remain the default implementation worker for shipping code.
Local model output is read-only or advisory unless a task-class promotion gate
passes.

## Split Of Responsibility

| Layer | Responsibility |
|-------|----------------|
| AgentOps | Policy, planning rules, durable yield ledger, pilot templates, decision packets, and promotion or demotion rules. |
| Mt. Olympus | Runtime role materialization, orchestrator dispatch, RUN_ID-backed evidence, model-lane labels, and qualification packet caveats. |
| Frontier worker | Default implementation owner and final validation collaborator. |
| Athena, mechanical gates, or human operator | Final verdict authority. Local advisory lanes do not self-grade. |

Mt. Olympus can host local sidecar roles. AgentOps decides what those roles are
allowed to influence and which evidence promotes or demotes them.

## Authority Levels

| Level | Authority | Allowed Use | Promotion Required |
|-------|-----------|-------------|--------------------|
| `OBSERVE` | Local model runs and records output, but the active task does not consume it. | Calibration, shadow runs, dead-token checks, speed and format probes. | No. |
| `ADVISORY` | Frontier or human may read the output, but it cannot block, approve, merge, or write shipping code. | Scout memos, review notes, corpus summaries, eval suggestions. | No for advisory use; yes before gate authority increases. |
| `GATED_PATCH` | Local model may propose a patch sketch or candidate diff inside a controlled pilot. Frontier owns implementation and final edits. | Narrow replay experiments, one-off patch sketches, code-change hypotheses. | Yes, task-class specific. |
| `IMPLEMENTATION` | Local model may own a narrow implementation task class. | Only after repeated promotion evidence and independent final validation. | Yes, with fresh baseline, RUN_ID, independent verdict, and failure accounting. |

`IMPLEMENTATION` is never the default. It is granted by task class, not by model
name or hardware ownership.

## Task Classes

| Task Class | Default Lane | Notes |
|------------|--------------|-------|
| Shipping code implementation | Frontier | Local remains `OBSERVE` or `ADVISORY` unless promoted for the exact task class. |
| Codebase scouting | Local `ADVISORY` allowed | Useful for file maps, prior-art searches, and risk inventories. |
| Review and critique | Local `ADVISORY` allowed | Local findings can inform reviewers but cannot issue final verdicts. |
| Eval and replay assistance | Local `ADVISORY` allowed | Local can propose checks or inspect artifacts; independent gates decide. |
| Corpus work | Local `ADVISORY` allowed | Summaries, retrieval candidates, and extract drafts must keep provenance. |
| Background scheduling | Local `OBSERVE` or `ADVISORY` | Background lanes must avoid mutation endpoints unless auth, origin, bind, and file-permission rules exist. |
| Resilience, privacy, offline work | Local allowed by policy | Record these benefits separately from quality-yield claims. |
| Stable baseline model | Local `OBSERVE` allowed | Useful for regression probes, not final quality claims. |

## Yield Metrics

Every metric must name the routing decision it informs. Metrics that cannot
change a keep, expand, retire, rerun, promote, or demote decision are telemetry
noise.

| Metric | Decision Used For | Direction |
|--------|-------------------|-----------|
| `final_validation_result` | keep or retire a local lane | PASS is better. |
| `frontier_tokens_saved` | expand a proven advisory lane | Higher is better only when quality and recovery do not regress. |
| `local_runtime_seconds` | route or retire latency-sensitive lanes | Lower is better. |
| `elapsed_wall_seconds` | compare baseline versus intervention | Lower is better when quality is equal. |
| `manual_recovery_minutes` | retire, rerun, or demote | Lower is better. |
| `defects_caught` | keep or expand review/scout lanes | Higher is better when false positives stay bounded. |
| `false_positives` | retire or demote review/scout lanes | Lower is better. |
| `downstream_reuse_count` | keep corpus lanes | Higher is better when citations are faithful. |
| `consumed_by_frontier` | attribute intervention effects | Consumed, ignored, corrected, and contradicted states are all meaningful. |
| `failure_count_by_type` | demote or harden | Lower is better. |

Privacy, security, offline capability, and resilience are valid reasons to use
local compute. Record them as separate value claims instead of blending them into
quality yield.

## Local Scout Lane

Authority: `ADVISORY`

`local-scout` reads task context, prior artifacts, code maps, and relevant
source files. It writes a scout memo with likely files, risks, prior decisions,
and suggested validation checks.

`local-scout` cannot block, approve, merge, or write shipping code. The frontier
worker may consume, ignore, correct, or contradict the memo. That consumption
state must be recorded in the yield ledger.

## Local Review Lane

Authority: `ADVISORY`

`local-review` reads a plan, diff, or evidence packet. It writes structured
findings with severity, file references, and suggested checks.

`local-review` cannot block, approve, merge, or write shipping code. Athena,
mechanical gates, frontier validators, or humans own the final verdict.

## Promotion Gates

Promotion is task-class-specific. A model that passes local-review promotion
does not automatically pass implementation promotion.

Minimum promotion packet:

- Fresh baseline for the same task class.
- Baseline and intervention run IDs.
- Concrete provider and model identity.
- Authority level under test.
- Input and output artifact paths.
- Consumed-by metadata showing whether frontier used the local output.
- Failure accounting: timeout, malformed output, empty output, bad advice, false
  positive, contradicted advice, manual recovery, and defect escape.
- Independent final verdict from frontier, Athena, mechanical gates, or human
  operator.
- Decision threshold: keep, expand, retire, rerun, promote, or demote.

Local `ADVISORY` evidence does not count as release-grade independent V&V. It
may appear in qualification packets only as a substrate caveat or decision input
unless release-mode model-lane independence exists.

## Demotion Gates

Demote a local lane to its prior authority level when any of these occurs:

- Fresh baseline expires or cannot be reproduced.
- Final validation regresses versus frontier-only baseline.
- Manual recovery or false positives exceed the task-class threshold.
- Local output causes a defect escape or blocks progress.
- Evidence lacks RUN_ID, model identity, consumed-by metadata, or failure
  accounting.
- Release-mode packet would confuse `ADVISORY` evidence with independent V&V.

## Evidence Shape

Yield evidence should use path pointers and redaction metadata rather than raw
prompt or diff dumps.

Required fields:

- `schema_version`
- `run_id`
- `task_class`
- `role`
- `authority`
- `provider`
- `model`
- `input_artifacts`
- `output_artifacts`
- `consumed_by`
- `baseline_id`
- `intervention_id`
- `metrics`
- `failures`
- `decision`

## Operating Rule

Do not route work locally because the machine is available. Route locally when a
specific role, task class, authority level, and promotion packet show that local
compute improves the factory without hiding quality, recovery, or validation
costs.
