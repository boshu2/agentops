# Operating Discipline — Rules for an Agent Software Factory

> The detailed transition model lives in the
> [Operating Loop](../architecture/operating-loop.md). This document keeps the
> substrate-neutral engineering rules. It does not define a second gate,
> retry controller, delivery system, or tracker.

## Doctrine in one line

Shape one behavior, let one writer build it, prove facts for one frozen
candidate, obtain one immutable verdict from fresh context, Learn once, and let
the repository deliver it.

## The kernel rules

### D1 — Admission before mutation

Tracked work starts with one accepted behavior, one owner, one exact write
scope, one rollback, and one proof boundary. A goal or epic is demand, not WIP.

### D2 — Completion is a claim

An agent saying “done” is input to validation. Completion requires acceptance
evidence and an author-distinct verdict bound to the exact candidate.

### D3 — Author is not judge

The context that created a candidate cannot issue its binding verdict. A fresh
same-family context is sufficient by default; a council is an optional risk
tier, not a universal lifecycle.

### D4 — One writer per leaf

One writer owns one active leaf. Parallel writers require disjoint checked
scopes, separate worktrees, and one integration owner.

### D5 — Evidence or it did not happen

Deterministic checks record facts for exact inputs. Validate cites those facts
and the acceptance claims it judged. Chat confidence is not evidence.

### D6 — Fail closed on meaning, not ceremony

Missing or failed required proof prevents completion. Cosmetic, pre-existing,
theoretical, and out-of-scope findings are NOTE and never block.

### D7 — Identity is recorded

Candidate base, commit, tree, owned blobs or deletions, commands, registries,
toolchains, environments, author, and validator identities are explicit.

### D8 — No self-greening

A worker may run deterministic checks but cannot convert its own semantic claim
into PASS. Any candidate edit invalidates the prior verdict.

### D9 — One source of truth per concern

Executable behavior and generated projections outrank narrative docs. Tracker,
candidate, verdict, Learn, and delivery records each have one owner; do not
mirror them into a second mutable state machine.

### D10 — Typed transitions

Use READY, ACTIVE, FROZEN, VALIDATED or REFUTED, LEARNED, DELIVERED,
REMOTE_VERIFIED, and REPORTED. Use NOTE, REPAIR, REPLAN, HOLD, and ANDON for
orchestrator decisions. Narrative labels do not create transitions.

### D11 — Roles are structure

Discovery shapes, Crank builds, Validate judges, Learn records consequence, and
the repository delivers. An adapter may automate mechanics but cannot inherit a
role's semantic authority.

### D12 — Inference proposes; deterministic code repeats

Models discover and judge meaning. Stable repeated defects may become proposed
tests or checks, but enforcement requires runnable positives, negative controls,
an owner, rollback, and expiry.

### D13 — Pull one leaf; make ticks idempotent

Read durable state before acting and perform only the next legal transition. A
retry count is not a budget and an empty ready queue is not convergence.

### D14 — Destructive writes reconcile or refuse

Before mutation, compare the expected identity with current reality. Changed
dependencies or overlaps return to the earliest invalidated move. Never clobber
foreign work.

### D15 — Operator attention is the constraint

Ordinary REFUTED work auto-routes to one consolidated REPAIR or REPLAN. A run
governor may enter HOLD and consult one bounded helper. Only genuine human
authority or a spent hard ceiling earns ANDON.

### D16 — Mechanisms stay general; policy stays bounded

AgentOps owns intent, independent judgment, evidence, and learning. Runtimes,
trackers, local shells, cloud agents, Git providers, CI systems, and release
pipelines connect through ports and remain replaceable.

## Delivery boundary

Validation and Git delivery are separate. Direct push, PR, hosted CI, or a
dedicated merge service may consume the same exact candidate and proof. Delivery
does not invoke another model, upgrade the verdict, or replay an unchanged full
suite. AgentOps records the resulting identity; the repository owns the action.

## What was dropped

The former one-way-door review lifecycle, model-driving Git checks, queue
admission, nested retry budgets, and semantic push-hook enforcement are retired.
They duplicated the four umbrellas and turned validation into a CI product.
Executable owners disappear through the checked K7/K9 direct-cut leaves; this
authority change does not pretend that code has already been removed.

## Provenance

This doctrine distills the earlier D1–D16 corpus while replacing its repository
gate embodiment with the lean operating loop. Git history preserves the prior
mapping for forensic use.
