# Goals

Negative fixture for tests/goals/validate-goals.sh: a goals file whose Gates
section is gone entirely. This is the exact regression that emptied the real
table between 2026-07-14 and 2026-08-24 — the parser returned zero goals with
no error and the whole fitness surface reported green on 0/0.

## Fitness properties

1. **Behavior before activity.** Prose survives; the executable table does not.

## Measured learning hypothesis

No gates here. The validator must reject this file.
