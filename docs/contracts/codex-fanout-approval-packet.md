# Codex Fanout Approval Packet

This contract is the Codex discovery shape for open-ended or high-risk work.
It preserves multiple planning perspectives, narrows them into one candidate
packet, and requires an independent Fable approval edge before implementation
beads are created.

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
  - "<question Fable should judge>"
risk_register:
  - "<risk carried into the plan>"
approval_request:
  reviewer: fable
  required_artifacts:
    - "<all PerspectivePlan paths>"
    - "<this SynthesisPacket path>"
  decision_question: "Approve this packet for bead creation?"
```

`ApprovalEdge` connects the synthesis packet to the independent verdict:

```yaml
kind: ApprovalEdge
id: approval-<run-id>
source_packet: .agents/discovery/<run-id>/synthesis-packet.yaml
validator_lane: "<tmux session:pane>"
judge_source: fable-claude-family
request_artifact: .agents/council/<date>-fable-request-<slug>.md
capture_path: .agents/council/ntm-captures/<target>_<stamp>.txt
verdict_artifact: .agents/council/<date>-fable-approval-<slug>.md
verdict: PASS | WARN | FAIL
required_changes:
  - "<required plan change for WARN/FAIL>"
accepted_risks:
  - "<explicit accepted risk for unresolved WARN only>"
decided_at: "<ISO-8601>"
```

## Gate Semantics

- `PASS` permits `/plan` to create or update beads from the approved
  `SynthesisPacket`.
- `WARN` is not a silent pass. Discovery must either update the
  `SynthesisPacket` and rerun approval, or record an explicit accepted-risk note
  in the `ApprovalEdge` before bead creation.
- `FAIL` blocks bead creation. Discovery returns to fanout/synthesis or stops
  with `<promise>BLOCKED</promise>` after three failed approval attempts.

The Fable pane must review artifact paths directly. A transcript-only memory or
Codex-authored summary is not an `ApprovalEdge`.
