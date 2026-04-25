# Working with `.agents/` — Operator Guide

> **Audience:** contributors and operators who need to write to `.agents/`, add a new write surface, or understand why the contract gate flagged their PR.
> **Companion docs:** the contract in [docs/contracts/agents-write-surfaces.md](contracts/agents-write-surfaces.md), the layered build pattern in [docs/patterns/agents-hygiene-contract.md](patterns/agents-hygiene-contract.md), and the `ao agents` CLI in [CLI commands reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md).

## What `.agents/` is

`.agents/` is AgentOps' repo-local operating ledger. It holds the durable state, evidence, and generated artifacts that let agent sessions continue across turns, runtimes, and worktrees without depending on a separate service. Every artifact a skill produces — a learning, a finding, a pre-mortem report, a phase summary, a citation, a session transcript, a release receipt — lands here as plain text the operator can grep, diff, and review.

There are two kinds of top-level entries under `.agents/`:

1. **Catalogued write surfaces.** Each one is listed in [docs/contracts/agents-write-surfaces.md](contracts/agents-write-surfaces.md) with `Subdir | Owner | Lifecycle | Purpose`. Production code (Go in `cli/` excluding tests, plus shell in `scripts/`, `hooks/`, `lib/`) writes to these.
2. **Skill-owned subdirs.** Any active skill at `skills/<name>/SKILL.md` is automatically allowed to use `.agents/<name>/` — no contract edit required when adding a skill.

Use `.agents/` for agent-operational state, not for ad hoc scratch files. If an artifact should be preserved, searched, cited, or replayed by AgentOps, give it a documented owner, lifecycle, and validation path.

## Inspect the current surface

Use these commands before changing the surface:

```bash
ao agents inspect          # list catalogued + skill-owned write surfaces (text or --json)
ao agents lint             # diff actual writes against the contract (non-zero on violation)
ao agents doctor           # inspect + lint + orphan/stray-dir report (--strict to fail on orphans)
```

`inspect` shows the documented surface; `lint` (and the equivalent `scripts/check-agents-write-surfaces.sh`) checks production code against the contract; `doctor` rolls both up plus reports skill-owned subdir orphans and undocumented dirs with one-line fix hints.

## How to add a new write surface

> Follow the five-ring pattern in [docs/patterns/agents-hygiene-contract.md](patterns/agents-hygiene-contract.md): contract doc, shell lint, regression tests, CLI surface, and pre-push wire-in. Small stacked changes are easier to validate than one broad surface expansion.

If the new surface is **skill-owned** (the writes happen because a skill named `<X>` runs, and the writes go under `.agents/<X>/`), you do not need the steps below — adding the skill at `skills/<X>/SKILL.md` is enough; the lint auto-allows the path.

If the new surface is **production-owned** (CLI code, shell scripts, or hooks write to `.agents/<X>/` outside the skill-owned convention), follow the four-step contributor flow:

### 1. Add an entry to the contract

Edit [docs/contracts/agents-write-surfaces.md](contracts/agents-write-surfaces.md):

- Add a row to the surfaces table: `Subdir | Owner | Lifecycle | Purpose`. Owner names the producing component (`cli (.../...)`, `scripts (...)`, `hooks (...)`). Lifecycle is one of `persistent`, `rolling`, or `regenerated`.
- Add the literal subdir name, on its own line, between `<!-- BEGIN agents-write-surfaces-allowlist -->` and `<!-- END agents-write-surfaces-allowlist -->`. The lint reads this block directly.

### 2. Run the contract gate locally

```bash
ao agents lint                           # exit 0 = clean, 1 = violation
# or, equivalent:
bash scripts/check-agents-write-surfaces.sh
```

The gate scans `cli/**/*.go` (excluding `*_test.go`), `scripts/**/*.sh`, `hooks/**/*.sh`, and `lib/**/*.sh` for `.agents/<X>` literals and checks each `<X>` against the allowlist plus active skill names. Adding the entry above silences the violation.

### 3. Triage adjacent gaps

```bash
ao agents doctor
```

`doctor` rolls up `inspect` and `lint` and additionally reports skill-owned subdir orphans (skills that exist but never wrote to `.agents/<name>/`) and stray dirs (entries under `.agents/` that are neither catalogued nor skill-owned). Use `--strict` to make orphans block the gate when wiring this into a CI step.

### 4. Update consumer docs and skills

If the new surface is part of a user-visible workflow, mention `ao agents inspect | lint | doctor` from the relevant skill or doc. The shared catalog lives in `skills/using-agentops/SKILL.md` (Claude side) and `skills-codex/using-agentops/SKILL.md` (Codex side); after editing the Codex copy, run `scripts/regen-codex-hashes.sh` and verify with `scripts/check-codex-parity-drift.sh`.

Pure read-side mirrors usually do not need a new contract. Prefer an end-to-end smoke test that locks the read invariant, as described in the hygiene pattern.

## Verifying the surface is wired correctly

Two tests cover the contract↔code link:

- `scripts/check-agents-write-surfaces.sh` (and `tests/scripts/check-agents-write-surfaces.bats`) — production code references must be catalogued.
- `cli/cmd/ao/agents_smoke_test.go` (`TestAgentsWriteSurfaces_EachAllowlistEntryHasProductionReference`) — every catalogued surface must have at least one production reference.

Together they catch both directions: undocumented writes and stale catalog entries.

## Contributor flow at a glance

1. Identify the owner and lifecycle: decide whether the artifact is persistent, rolling, regenerated, or skill-owned.
2. Patch code and contract together: production writes and the documented allowlist must land in the same change.
3. Prove the surface: run `ao agents inspect`, `ao agents lint`, `ao agents doctor`, and focused tests for the changed owner.
4. Close the loop: update linked docs or CLI help when behavior changes, record validation evidence in bd, and keep the branch clean before push.

## When the gate fails on an existing PR

If `security-toolchain-gate` or the `.agents/` write-surfaces lint flags a PR you did not expect, the most common causes are:

| Symptom | Cause | Fix |
|---|---|---|
| `undocumented .agents/ write surface "<X>"` | Production code references `.agents/<X>` and `<X>` is neither in the allowlist nor an active skill | Add `<X>` between the BEGIN/END markers; or rename to use a skill-owned path |
| Doctor reports an orphan skill | A skill exists but never wrote anywhere under `.agents/<skill>/` | Either remove the skill if unused, or document the exemption (e.g. in the skill's SKILL.md) |
| Doctor reports a stray dir | `.agents/<X>/` exists on disk but neither catalogued nor a skill name | Remove the dir, rename it to a skill name, or add it to the allowlist if production code legitimately writes there |
| Codex parity audit fails after editing `skills/using-agentops/SKILL.md` | The Codex side at `skills-codex/using-agentops/SKILL.md` drifted | Mirror the change on the Codex side, then run `scripts/regen-codex-hashes.sh` |

Do not bypass the contract by moving writes to a less precise path. The purpose of the gate is to make ownership and cleanup rules visible to the next operator.

## See also

- [`.agents/` Write Surfaces contract](contracts/agents-write-surfaces.md) — canonical inventory
- [`.agents/` hygiene contract pattern](patterns/agents-hygiene-contract.md) — five-ring build pattern
- [CLI reference for `ao agents`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md) — `inspect`, `lint`, `doctor` subcommands
