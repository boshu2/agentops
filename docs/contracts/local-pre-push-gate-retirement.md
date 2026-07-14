# Superseded Historical ADR: Local Push-Gate Experiments

> **Status:** Superseded. This file preserves the disposition of two failed
> policies; it is not an active delivery contract.

AgentOps tried both CI-only enforcement and a large local push hook. Each moved
the product into repository delivery and duplicated expensive proof. The local
variant also serialized mutable checks and model judgment at the moment Git was
trying to publish, which made ordinary delivery slow and fragile.

## Current decision

Validation stops after one exact candidate has deterministic evidence, one
immutable verdict from fresh context, and one Learn receipt. The consumer
repository owns delivery for local and cloud agents. It may use direct push, a
PR, hosted CI, or a small deterministic hook.

A repository delivery adapter may verify identity and reuse an exact-input
receipt. It must not:

- invoke a model or perform another semantic review;
- mutate or upgrade the Validate verdict;
- close tracker work as a side effect of Git;
- own a global delivery queue for AgentOps; or
- replay an unchanged full suite merely because delivery started.

## Deletion ownership

K7 deletes the old delivery command, queue, and semantic push-hook machinery
while installing deterministic delivery recording. Dedicated scripts, hooks,
tests, and fixtures disappear in that same candidate. F4 later removes the
build profiles that kept alternate command owners compilable. D2 regenerates
command projections after executable ownership is final.

## Rollback

Revert the complete K7 candidate before downstream consumers depend on its
receipt contract. Do not restore only a hook or only a removed delivery owner.
