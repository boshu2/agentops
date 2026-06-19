# Codex Fanout Approval Packet

This contract is the Codex discovery shape for open-ended or high-risk work.
It preserves multiple planning perspectives, narrows them into one candidate
packet, and requires an independent cross-family approval edge — the **plan-pawl
duel** (two distinct model families) for fanout/irreversible class, or a single
Fable edge under `--no-duel` — before implementation beads are created.

## Artifacts

`PerspectivePlan` is one independent planner view. Discovery writes at least
three for this path:

```yaml
kind: PerspectivePlan
id: perspective-<lens>
lens: product | architecture | operations | custom
author_runtime: codex
source_artifacts:
  - .agents/research/<topic>.md
objective: "<bounded behavior objective>"
plan_outline:
  - "<candidate slice or wave>"
assumptions:
  - "<claim that must survive synthesis>"
risks:
  - "<risk or failure mode>"
validation:
  - "<gate or test expectation>"
score:
  confidence: high | medium | low
  expected_leverage: high | medium | low
reject_if:
  - "<condition that would make this perspective invalid>"
```

`SynthesisPacket` is the selected candidate. It is not a bead set and does not
create tracker rows. It records why one plan, or a merge of plans, wins:

```yaml
kind: SynthesisPacket
id: synthesis-<run-id>
objective: "<bounded behavior objective>"
perspective_plans:
  - .agents/discovery/<run-id>/perspective-product.md
  - .agents/discovery/<run-id>/perspective-architecture.md
  - .agents/discovery/<run-id>/perspective-operations.md
selected_plan: "<id or merged>"
rejected_plan_ids:
  - "<id>"
synthesis_rationale:
  - "<why this shape wins>"
open_questions:
  - "<question the judges should weigh>"
risk_register:
  - "<risk carried into the plan>"
approval_request:
  reviewers:                     # the duel: >=2 distinct families (single: [fable])
    - claude
    - gpt
  required_artifacts:
    - "<all PerspectivePlan paths>"
    - "<this SynthesisPacket path>"
  decision_question: "Approve this packet for bead creation?"
```

`ApprovalEdge` connects the synthesis packet to the independent verdict. It has
two forms, selected by `mode`:

- **`mode: duel`** *(default for fanout/irreversible class — the plan-pawl)* —
  records **two judge panes, one per distinct model family** (the multi-model
  diversity floor of [`docs/contracts/pawls.md`](pawls.md)). The edge `decision` is
  the deterministic `ao plan-pawl decide` outcome over the two panes' verdicts, not
  a human read.
- **`mode: single`** *(valid under `--no-duel`)* — the legacy single-Fable form: one
  judge pane (`judges` has exactly one entry).

```yaml
kind: ApprovalEdge
id: approval-<run-id>
mode: duel                       # duel (>=2 families) | single (--no-duel)
source_packet: .agents/discovery/<run-id>/synthesis-packet.yaml
duel_verdict_dir: .agents/duel/<run-id>/   # one judge-verdict JSON per pane (duel mode)
judges:                          # >=2 entries for duel; exactly 1 for single
  - family: claude               # roster: claude | gpt | gemini (distinct per entry)
    validator_lane: "<tmux session:pane>"
    request_artifact: .agents/council/<date>-claude-request-<slug>.md
    capture_path: .agents/council/ntm-captures/<target>_<stamp>.txt
    verdict_artifact: .agents/council/<date>-claude-approval-<slug>.md
    disposition: PASS | FAIL | WARN    # the decider JSON tag (ao plan-pawl decide)
    warn_class: mechanical | judgment   # required when disposition == WARN
    judgment_flag: false         # true => hard breaker (escalate)
  - family: gpt
    validator_lane: "<tmux session:pane>"
    request_artifact: .agents/council/<date>-gpt-request-<slug>.md
    capture_path: .agents/council/ntm-captures/<target>_<stamp>.txt
    verdict_artifact: .agents/council/<date>-gpt-approval-<slug>.md
    disposition: PASS | FAIL | WARN
    warn_class: mechanical | judgment
    judgment_flag: false
decision: PASS | REDO | BLOCKED  # `ao plan-pawl decide` outcome (duel); PASS/WARN/FAIL (single)
required_changes:
  - "<required plan change for FAIL / REDO>"
accepted_risks:
  - "<explicit accepted risk for an unresolved judgment WARN only>"
decided_at: "<ISO-8601>"
```

The **duel** form is what `ao plan-pawl decide --dir <duel_verdict_dir>` reads: each
`judges[]` entry corresponds to one judge-verdict JSON
(`{family, disposition, warn_class, judgment_flag}`) in `duel_verdict_dir`. Quorum =
no FAIL **and** `>= 2` distinct roster families. A WARN must declare `warn_class`
(mechanical auto-applies + re-judges; judgment is surfaced, accepted-risk, non-blocking).

## Gate Semantics

**Duel mode** (`ao plan-pawl decide` is authoritative — the exit code IS the
decision):

- `PASS` (quorum: no FAIL and `>= 2` distinct families) permits `/plan` to create or
  update beads from the approved `SynthesisPacket`.
- `REDO` is the auto-redo path (no human): a FAIL re-runs fanout/synthesis; a
  mechanical WARN auto-applies + re-judges. A judgment WARN is surfaced, not blocking.
- `BLOCKED` halts: a circuit breaker tripped (round > max, an explicit
  `judgment_flag`, or oscillation) → `<promise>BLOCKED</promise>`.

**Single mode** (`--no-duel`):

- `PASS` permits bead creation.
- `WARN` is not a silent pass. Discovery must either update the `SynthesisPacket`
  and rerun approval, or record an explicit accepted-risk note in the
  `ApprovalEdge` before bead creation.
- `FAIL` blocks bead creation. Discovery returns to fanout/synthesis or stops with
  `<promise>BLOCKED</promise>` after three failed approval attempts.

Each judge pane must review artifact paths directly. A transcript-only memory or a
Codex-authored summary is not an `ApprovalEdge`. In duel mode the two judges MUST be
distinct, roster-validated model families (a self-approval from the author's family
or a same-family pair does not meet the multi-model floor).

> **Scope.** This file defines the contract shape only. The consumer-side wiring that
> emits a duel `ApprovalEdge` — the discovery skill's STEP 3.5 plan-pawl duel and the
> `codex-approval` invocation — is delivered by epic `age-plan-pawl-9yib` (PP.3/PP.4);
> until those land, a consumer may still emit the `mode: single` form under `--no-duel`.
