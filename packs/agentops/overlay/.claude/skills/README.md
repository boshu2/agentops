# AgentOps skills overlay — DEFERRED (populated post-rip)

> **PLACEHOLDER. TODO(gvkj6-finalize): populate this directory with the lean,
> post-rip AgentOps skill corpus snapshot.**

## What lands here (and why it's deferred)

`overlay_dir` recursively copies this directory's contents into every spawned
agent's working directory at startup (`docs/reference/config.md:106`). This is the
canonical GC mechanism for provisioning skills — GC has **no skill primitive**
(`skills`/`mcp`/`skills_append` are tombstones removed in v0.16,
`config.md:209-212`), so AgentOps brings its own runtime in this overlay.

At finalize, this directory holds a **pinned snapshot** of the AgentOps skill
corpus — a copy of the repo's `skills/**` (source of truth stays `skills/`;
installed copies are derived, per the repo's "Skill File Locations" rule).

## Why it is intentionally EMPTY right now

It is built in PARALLEL with a code-rip that is removing killed features. Shipping
a skills snapshot now would freeze skills for features the rip is deleting. The
snapshot is taken **after** the rip lands the lean `skills/` set, so the City does
not ship dead-feature skills.

Pinned-snapshot vs pull-latest-at-spawn is the GC-idiomatic call (overlays are
static pack content) — see design doc Part 5, gap #1. The `assets/scripts/
install-ao.sh` `pre_start` handles the `ao` *binary*; this overlay handles the
static skill *files*.

## Finalize checklist (gvkj6-finalize)

- [ ] Wait for the code-rip to land the lean `skills/` set.
- [ ] Copy the post-rip `skills/**` here (copy, never symlink — repo CI rejects
      symlinks; a snapshot must be self-contained).
- [ ] Replace this README with the snapshot (or keep it as a provenance note).
- [ ] Validate the City finalizes: `gc config show --validate` + `ao validate --gate`.
