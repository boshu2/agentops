# Program

This repository defaults to native execution with optional skills (ADR-0017). PROGRAM is
repository execution guidance, not a product retry or delivery controller.

## Experiment unit

One invocation consumes and produces:

1. one resolved caller-owned intent, planning only when needed and a content
   snapshot only when necessary;
2. bounded implementation and checks, with direct repair of known defects and
   evidence-based approach revision within unchanged acceptance;
3. one runtime-derived subject manifest and factual check receipts with
   complete or honestly incomplete changed-path proof;
4. one fresh author-distinct read over exact content only when the caller asks,
   a mistake cannot be cheaply undone, or no deterministic check covers the
   change; a check confirms each repair without another review;
5. a subject-led report, with a durable verdict only when requested or required
   by a declared consumer.

Finish when acceptance is met on the checks, CI and any owed fresh read. A true
causal stall admits at most one bounded fresh helper within the existing
allowance; cancellation, refusal and spent hard resources stop work. Do not
renew an allowance through replanning, compaction or delegation. Changed
acceptance requires caller authority. Repository Git and release procedures
remain separate.

## Mutable scope

- product and doctrine: `README.md`, `PRODUCT.md`, `GOALS.md`, `PROGRAM.md`, `AGENTS.md`;
- implementation: `cli/**`, `skills/**`, `schemas/**`, `scripts/**`, `tests/**`;
- generated projections: registries, routers, maps, CLI docs;
- repository checks and docs: `.github/workflows/**`, `docs/**`, `evals/**`.

Secrets, credentials, user configuration outside the repository, production
data, release tags, package publication, and unrelated user edits remain outside
ordinary implementation authority.

## Evidence

Use the cheapest targeted checks during implementation. Before reporting the
complete change, run the ordinary deterministic suite appropriate to the final
surface; with CI it is the gate for an ordinary change. Obtain one fresh
author-distinct read only where the
[agent workflow reference](docs/agent-workflow-reference.md) requires one.
Record exactly what was checked and not checked.

The repository may then commit, push, merge, release, or roll back through its
normal Git/CI policy. Those transitions cannot upgrade or replace the semantic
verdict.

## Concurrency

One writer is the default. User-requested parallel work requires disjoint write
scopes and isolation. Factory adapters may dispatch explicit disjoint packets
once; they do not select work, retry, validate, integrate, or deliver.
