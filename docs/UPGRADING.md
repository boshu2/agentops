# Upgrading

This page records **breaking changes, deprecations, and migration steps** for AgentOps users — skill authors, plugin maintainers, CLI users, and anyone who has wired AgentOps into a CI pipeline.

For the full release log, see [`CHANGELOG.md`](CHANGELOG.md). This page is deliberately a thinner, forward-looking companion: it only captures changes that require action.

## Before upgrading

```bash
# Back up local state if you commit .agents/
git status .agents/

# Record current version so you can compare
ao --version
ao doctor > /tmp/ao-doctor-before.txt
```

## How to read this page

Each section is keyed by the target version you are upgrading **to**. If you are jumping across several versions, read every intermediate section top-down.

The "Action required" callout distinguishes hard breakages (must fix before running) from advisories (works but deprecated).

---

## Upgrading to 2.38.x (Unreleased)

**Status:** in development — see `[Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md) for latest.

### Strict delegation is now the default for orchestrator skills

**Affects:** anyone invoking `/rpi`, `/discovery`, or `/validation` from wrappers, scripts, or custom skills.

Top-level orchestrator skills now declare strict sub-skill delegation as the default. There is no opt-out flag — strict delegation is always on. Compression is available only through explicit flags:

| Escape | Effect |
|--------|--------|
| `--quick`, `--fast-path` | Short-circuit non-essential phases |
| `--no-retro`, `--no-forge` | Skip post-execution bookkeeping |
| `--skip-brainstorm`, `--no-scaffold` | Skip planning sub-phases |
| `--no-behavioral` | Skip behavioral-discipline gate |
| `--allow-critical-deps` | Permit dependency-risky work |

**Action required:** if you have custom wrappers that inlined orchestrator phases, switch them to invoke the sub-skills directly. See [`skills/shared/references/strict-delegation-contract.md`](https://github.com/boshu2/agentops/blob/main/skills/shared/references/strict-delegation-contract.md).

### `--no-lifecycle` renamed to `--no-scaffold` in `/discovery`

**Affects:** any caller passing `--no-lifecycle` to `/discovery`.

The flag controls STEP 4.5 scaffold auto-invocation only, not broader lifecycle checks. `--no-lifecycle` is honored as a deprecated alias through **v2.40.0**. Other skills (`/crank`, `/validation`, `/implement`, `/evolve`) retain `--no-lifecycle` with its existing semantics.

**Action required:** update scripts and wrappers to use `--no-scaffold` for `/discovery`. `--no-lifecycle` will be removed in v2.41.0.

### Olympus bridge removed

**Affects:** callers referencing `docs/ol-bridge-contracts.md`, `docs/architecture/ao-olympus-ownership-matrix.md`, `.ol/` directories, or `ol-*.sh` scripts.

The AO↔Olympus bridge has been archived. Removed surfaces:

- `docs/ol-bridge-contracts.md`
- `docs/architecture/ao-olympus-ownership-matrix.md`
- MemRL policy contracts
- `skills/*/scripts/ol-*.sh`
- CLI types: `OLConstraint`, `gatherOLConstraints`
- `.ol/` directory collector

**Action required:** remove any automation that read from `.ol/` or invoked `ol-*.sh`. Useful patterns from Olympus now live directly inside `ao`.

---

## Upgrading to 2.37.x

### Swarm evidence schema is now validated

**Affects:** any workflow that writes to `.agents/swarm/results/<task>.json`.

A canonical swarm-evidence schema ([`schemas/swarm-evidence.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/swarm-evidence.schema.json)) is now enforced in release and pre-push gates. Historical artifacts are accepted via a permissive shape; new writers should match the strict shape in [`contracts/swarm-worker-result.schema.json`](contracts/swarm-worker-result.schema.json).

**Action required:** run `scripts/validate-swarm-evidence.sh` before shipping new swarm worker code.

### Lead-only worker git guard

**Affects:** custom multi-agent runners that assumed any worker could commit.

Worker sessions now carry an explicit `lead-only-worker-git-guard.sh` hook in the `PreToolUse` chain. Workers that attempt `git commit` will be blocked.

**Action required:** route commits through the lead agent. If you intentionally run a single-agent flow, no action is needed — the guard is a no-op when no worker metadata is present.

### Pre-mortem gate denies on ambiguity

**Affects:** any workflow that relied on the previous fail-open behavior.

The crank pre-mortem gate now denies ambiguous state by default. If your pipeline ran crank jobs with missing pre-mortem context, they will now stop early rather than proceed silently.

**Action required:** either set `AGENTOPS_PREMORTEM_MODE=advisory` for exploratory runs, or ensure pre-mortem artifacts are generated before invoking crank.

---

## Upgrading to 2.37.1 and earlier

See [`CHANGELOG.md`](CHANGELOG.md) directly. No hard breakages were introduced in 2.37.1 or prior 2.37.x releases — all changes were additive.

---

## After upgrading

```bash
# Verify the new install
ao --version
ao doctor
ao hooks test

# Re-run any local gates touched by the upgrade
scripts/pre-push-gate.sh --fast
```

If `ao doctor` reports drift between installed skills and your repo copy, re-run the install script from [Getting Started](getting-started/index.md#install).

## Reporting upgrade issues

If you hit a breakage not described above, open an issue with:

- Before/after `ao --version` output
- `ao doctor` output from both versions
- The exact command or hook that failed
- Runtime (Claude Code, Codex, OpenCode, other)

See [`SECURITY.md`](SECURITY.md) for issues that involve credentials or isolated data.
