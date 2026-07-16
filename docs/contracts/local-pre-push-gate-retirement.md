# Superseded Historical ADR: Local Push-Gate Experiments

> **Status:** Superseded. This file preserves the disposition of two failed
> policies; it is not an active delivery contract.

AgentOps tried both CI-only enforcement and a large local push hook. Each moved
the product into repository delivery and duplicated expensive proof. The local
variant also serialized mutable checks and model judgment at the moment Git was
trying to publish, which made ordinary delivery slow and fragile.

## Current decision

AgentOps stops after one exact candidate has deterministic evidence and one
durable verdict from fresh context. The consumer repository owns delivery for
local and cloud agents. It may use direct push, a PR, hosted CI, or a small
deterministic hook.

A repository hook or CI job may run deterministic repository checks. It must
not treat AgentOps as the delivery controller or:

- invoke a model or perform another semantic review;
- mutate or upgrade the Validate verdict;
- close tracker work as a side effect of Git;
- create an AgentOps delivery queue or receipt; or
- replay an unchanged full suite merely because delivery started.

## Retired surface

The old AgentOps delivery commands, queue, semantic push admission, and
delivery receipts are retired. Existing deterministic repository checks remain
ordinary checks whose exit status means only success or failure.

## Rollback

Revert the complete Cathedral Cut if repository policy needs to restore the old
product boundary. Do not restore only an AgentOps push hook or delivery command.
