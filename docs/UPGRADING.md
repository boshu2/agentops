# Upgrading

This page records **breaking changes, deprecations, and migration steps** for AgentOps users — skill authors, plugin maintainers, CLI users, and anyone who has wired AgentOps into a CI pipeline.

For the full release log, see [`CHANGELOG.md`](CHANGELOG.md). This page is deliberately a thinner, forward-looking companion: it only captures changes that require action.

## Before upgrading

```bash
# Repo-root .agents/ is local/private runtime state and should not be committed
git status .agents/

# Record current version so you can compare
ao --version
ao doctor > /tmp/ao-doctor-before.txt
```

## How to read this page

Each section is keyed by the target version you are upgrading **to**. If you are jumping across several versions, read every intermediate section top-down.

The "Action required" callout distinguishes hard breakages (must fix before running) from advisories (works but deprecated).

---

## Cathedral Cut transition

AgentOps now has one semantic operating loop:

```text
RPI → Plan → Implement → fresh Validate → durable verdict → report and stop
```

Plan shapes one behavior. Implement performs one bounded experiment. Validate
computes exact content identity, obtains one independent judgment, and writes a
standalone `verdict.v2`. RPI reports the result and stops. Learn is an optional
later consumer of verdict collections.

**Action required:** remove automation that expects AgentOps to retry, queue,
claim work, mutate Git, merge, release, close work, or drive semantic review from
a hook. Keep repository-owned deterministic CI and delivery. No legacy or
flywheel build profile restores removed commands.

### Removed CLI lifecycle commands

The cut removes Pawl, Plan-Pawl, land, done, close, governor, yield, claim,
next-work, state/worktree lifecycle, semantic `ao validate`, converge,
reconcile, membrane, and Crank behavior. Their one-release tombstones fail with
a migration hint and never load old code.

**Action required:** invoke the RPI skill for the semantic loop, use `ao gate
check` only for deterministic repository checks, and use your repository's Git
or CI system for delivery.

### Removed goal mutation commands

`ao goals` retains measurement and analysis. It no longer creates, migrates,
prunes, re-steers, or auto-applies changes to operator intent.

**Action required:** edit goal specifications with your normal authoring tools;
use `ao goals validate`, `measure`, `drift`, and related read-only views to
inspect them.

### Removed alternate build profiles

The old `flywheel` and `legacy` build tags and their archived command sets were
deleted. `ao flywheel status` and `ao flywheel compare` remain ordinary
read-only observations in the supported binary.

**Action required:** remove `make build-flywheel`, `AGENTOPS_LEGACY`, and tagged
build invocations. External specialist tools remain caller-selected and do not
become AgentOps lifecycle authorities.

---

## Upgrading to 3.0.x

3.0 is a deliberate narrowing: AgentOps becomes **in-session only**. Everything that tried to be an always-on runtime was deleted, and out-of-session orchestration moves to a substrate you choose. Full narrative: [MIGRATION-3.0.md](MIGRATION-3.0.md).

### All runtime hooks deleted (the repo is hookless)

**Affects:** anyone who ran the hook bundle or relied on PreToolUse/PostToolUse side effects.

Every runtime hook was deleted (ADR-0002) after an A/B showed the injected-context delta was zero.

**Action required:** rely on hookless skills + the `ao` CLI. Configure any
deterministic hook, PR check, or external CI in the consumer repository; it is
delivery policy, not AgentOps validation.

### `agentopsd` daemon, `ao schedule`, and `ao overnight` deleted

**Affects:** anyone who ran the always-on daemon, the scheduling lane, or the overnight compounding runner.

AgentOps ships no always-on runtime (ADR-0009 — "delete, not deprecate").

**Action required:** move that lane to an external substrate you choose — **NTM** (a local tmux swarm) + `ao mcp serve` (MCP tool surface) + `ao agent` (managed agents). Out-of-session scheduling belongs to whatever substrate you run.

### bd / Dolt tracker retired

**Affects:** anyone tracking work with the `bd`/Dolt backend.

The old tracker was a single-host server with no offline lane; it is retired.

**Action required:** track with `BEADS_DIR="$(ao beads dir)" br <cmd>` and triage with `bv`. Your old `.beads/` is preserved but non-authoritative.

full map: [MIGRATION.md](MIGRATION.md)

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

### Daemon mode is available as an opt-in product runtime

> **⚠ Superseded in AgentOps 3.0:** the standalone daemon (`agentopsd`) was **removed** in the 3.0 rearchitecture — AgentOps is in-session only, and out-of-session orchestration is delegated to a swappable substrate (reference: NTM + MCP + managed-agents). See [`MIGRATION-3.0.md`](MIGRATION-3.0.md) and [ADR-0009](adr/ADR-0009-daemon-deletion-in-session-only.md). The entry below is retained as historical record of the (now-removed) v2.x daemon.

**Affects:** anyone migrating RPI, Dream, wiki/forge, or OpenClaw integrations
from foreground command ownership to `agentopsd`.

`agentopsd` is the new local always-on AgentOps control plane. It owns the
daemon ledger under `.agents/daemon`, job acceptance, projection rebuilds,
OpenClaw snapshots, and authorized mutation gates. Existing foreground commands
remain valid during this migration; daemon mode is selected explicitly with
flags such as `--daemon-submit`, `--daemon-url`, `--daemon-token`, and
`--daemon-fallback`.

**Action required:** do not assume daemon mode is the default yet. Start with
foreground readiness proof:

```bash
ao daemon run --addr 127.0.0.1:8765 --token "$AGENTOPS_DAEMON_TOKEN"
ao daemon ready
ao doctor --json
```

This entry is historical. Current AgentOps has no daemon migration path; see
[`MIGRATION.md`](MIGRATION.md) for the supported single-pass boundary.

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

The crank premortem gate now denies ambiguous state by default. If your pipeline ran crank jobs with missing premortem context, they will now stop early rather than proceed silently.

**Action required:** either set `AGENTOPS_PREMORTEM_MODE=advisory` for exploratory runs, or ensure premortem artifacts are generated before invoking crank.

---

## Upgrading to 2.37.1 and earlier

See [`CHANGELOG.md`](CHANGELOG.md) directly. No hard breakages were introduced in 2.37.1 or prior 2.37.x releases — all changes were additive.

---

## After upgrading

```bash
# Verify the new install
ao --version
ao doctor

# Run deterministic checks selected by the repository
ao gate check --fast --scope head
```

If `ao doctor` reports drift between installed skills and your repo copy, re-run the install script from [Getting Started](getting-started/index.md#install).

## Reporting upgrade issues

If you hit a breakage not described above, open an issue with:

- Before/after `ao --version` output
- `ao doctor` output from both versions
- The exact command or hook that failed
- Runtime (Claude Code, Codex, OpenCode, other)

See [`SECURITY.md`](SECURITY.md) for issues that involve credentials or isolated data.
