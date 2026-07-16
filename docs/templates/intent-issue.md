# Intent worksheet

Use this worksheet to shape one behavior in the caller-owned issue, bead, or
conversation. It is optional source material for Plan, not a second planning
artifact, work-readiness record, ownership claim, or scheduling state.

## Intent

<What should change, and for whom?>

## One active behavior

<One observable capability in domain language.>

## Acceptance examples

```gherkin
Scenario: <normal behavior>
  Given <precondition>
  When <action>
  Then <observable result>

Scenario: <critical edge>
  Given <edge precondition>
  When <action>
  Then <observable safe result>
```

## Non-goals

- <Expected adjacent behavior that is deliberately excluded.>

## Required evidence

- <Concrete command result, artifact, observation, or criterion evidence.>

## Write scope

```yaml
include:
  - <path or glob>
  - <generated companion when applicable>
exclude:
  - <explicitly protected path or glob>
```

## First acceptance check

Choose one:

```yaml
command: <command expected to fail before the behavior exists>
```

```yaml
artifact_path: <artifact whose absence or content proves RED>
```

## Rollback or containment

<Caller-owned way to undo or contain the experiment, or an explicit statement
that no rollback exists.>

The runtime snapshots and hashes these resolved intent bytes when their source
is not already durable. Plan emits no owner, priority, attempt, wave, queue,
lease, admission, next action, closure, release, or delivery field.
