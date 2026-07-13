# Four-Umbrella Examples

These executable examples are the current lifecycle inventory. The checker
validates the linked files directly so this page does not become a second,
stale copy of the packet contracts.

- [Explicit skill requests](../../tests/fixtures/four-umbrella-examples/explicit-skill-requests.txt)
- [Schema-v3 execution packet](../../tests/fixtures/four-umbrella-examples/execution-packet.json)
- [Learn receipt](../../tests/fixtures/four-umbrella-examples/learning-packet.json)
- [Plan-impact handoff](../../tests/fixtures/four-umbrella-examples/plan-impact.json)

The loop is Discovery → Crank → Validate → Learn. A material plan change
returns to the orchestrator, which may re-plan and invoke Premortem again.
Validation completion is independent from Git delivery; repositories keep
their own push, PR, and CI policy.

Run `bash scripts/check-four-umbrella-examples.sh` to verify this inventory.
