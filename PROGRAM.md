# Program

This repository self-hosts the same single-pass boundary it ships. PROGRAM is
repository execution guidance, not a product retry or delivery controller.

## Experiment unit

One invocation contains:

1. one behavior-first PlanPacket;
2. one bounded RED -> GREEN -> refactor implementation experiment;
3. one CandidatePacket with complete or honestly incomplete changed-path proof;
4. one fresh author-distinct Validate judgment over exact content;
5. one durable verdict and report.

The invocation stops after the report. A later revision is a caller-created new
invocation. Repository Git and release procedures remain separate.

## Mutable scope

- product and doctrine: `README.md`, `PRODUCT.md`, `GOALS.md`, `PROGRAM.md`, `AGENTS.md`;
- implementation: `cli/**`, `skills/**`, `schemas/**`, `scripts/**`, `tests/**`;
- generated projections: `skills-codex/**`, registries, routers, maps, CLI docs;
- repository checks and docs: `.github/workflows/**`, `docs/**`, `evals/**`.

Secrets, credentials, user configuration outside the repository, production
data, release tags, package publication, and unrelated user edits remain outside
ordinary implementation authority.

## Evidence

Use the cheapest targeted checks during implementation. Before reporting the
complete change, run the ordinary deterministic suite appropriate to the final
surface and obtain one fresh Validate verdict. Record exactly what was checked
and not checked.

The repository may then commit, push, merge, release, or roll back through its
normal Git/CI policy. Those transitions cannot upgrade or replace the semantic
verdict.

## Concurrency

One writer is the default. User-requested parallel work requires disjoint write
scopes and isolation. Factory adapters may dispatch explicit disjoint packets
once; they do not select work, retry, validate, integrate, or deliver.
