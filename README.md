# AgentOps

Coding agents ship work that looks finished and isn't. AgentOps turns one
behavior into one bounded experiment, hands the exact result to a fresh
validator, and stores a durable verdict you control. For contested calls, opt
into [`council`](skills/council/SKILL.md) (independent judges) or
[`idea-genie`](skills/idea-genie/SKILL.md) duel mode (sealed perspectives before
Plan).

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

## Quickstart

```bash
npx skills@latest add boshu2/agentops --all -g
```

Works for Claude Code, Codex, Cursor, and other skills-compatible agents.
Installs skills under your agent skill roots (for example `~/.agents/skills`).
Restart the agent, then:

```text
> use plan for bead agentops-123
acceptance and write scope locked in the bead

> use implement
RED -> GREEN -> refactor; subject manifest derived

> use validate
verdict.v2: FAIL — burst refill violates scenario S2
checked: S1, S2, subject identity, write scope
not_checked: load above declared limit
```

Or run `rpi` once for the full loop.

## Plugins (Claude Code / Codex)

Prefer a managed bundle that updates with the release:

```bash
# Claude Code
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex
codex plugin marketplace add boshu2/agentops
codex plugin add agentops@agentops-marketplace
```

`npx` copies skills you can edit. Plugins keep a read-only bundle current with
the repo. Remove with your runtime's plugin uninstall, or delete the linked
skill directories.

## Intent lives in a bead

```bash
brew install beads
```

[Beads](https://github.com/steveyegge/beads) is an issue tracker built for
agents. AgentOps uses a bead as the intent source for each experiment: Plan
writes acceptance and write scope into it, Implement builds from it, and
Validate judges against a hashed snapshot of those same bytes under
`.agents/ao/intents/sha256/`.

## Optional: `ao` CLI

Deterministic checks, inspection, and skill linking. Skip it if you only need
the skills.

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
```

Without Homebrew: `go install github.com/boshu2/agentops/cli/cmd/ao@latest`

## Why AgentOps exists

### 1. The agent said it was done

Same session that wrote the code also declared victory. AgentOps separates
authorship from judgment: `implement` produces a candidate; a fresh `validate`
issues `PASS`, `FAIL`, or `NOT_PROVEN`.

### 2. One perspective rubber-stamped another

A single context can share blind spots with the author. Opt into
[`idea-genie`](skills/idea-genie/SKILL.md) or [`council`](skills/council/SKILL.md)
for sealed or multi-judge review. They return a report;
[`validate`](skills/validate/SKILL.md) writes `verdict.v2`.

### 3. Acceptance drifted mid-flight

Without a fixed behavior and write scope, "done" is whatever the agent
improvised. `plan` locks acceptance in the bead before anyone builds. Later
phases bind to that digest.

### 4. Nobody can replay what was judged

Chat scrolls away. `validate` writes a content-addressed `verdict.v2` under
`.agents/ao/verdicts/sha256/` with checked scope, omissions, and evidence
refs. Plain JSON. No hosted service required.

## Core skills

| Skill | Job |
|---|---|
| [`rpi`](skills/rpi/SKILL.md) | run Plan, Implement, and fresh Validate at most once |
| [`plan`](skills/plan/SKILL.md) | refine acceptance, evidence, and scope in the bead |
| [`implement`](skills/implement/SKILL.md) | one RED → GREEN → refactor experiment |
| [`validate`](skills/validate/SKILL.md) | independent judgment; persist `verdict.v2` |

Optional later: [`learn`](skills/learn/SKILL.md). Strategies:
[`council`](skills/council/SKILL.md), [`idea-genie`](skills/idea-genie/SKILL.md),
[`premortem`](skills/premortem/SKILL.md), [`postmortem`](skills/postmortem/SKILL.md).

Full inventory: [Skill Router](docs/SKILL-ROUTER.md).

## Evidence contract

A `PASS` binds unchanged acceptance, a deterministic subject manifest, complete
changed-path coverage inside write scope, distinct author and validator context
IDs, a freshness attestation, and criterion-level evidence.

Missing identity, mutation, or incomplete coverage → `NOT_PROVEN`. Proven
out-of-scope change or failed criterion → `FAIL`.

[Operating loop](docs/architecture/operating-loop.md) · [CLI](cli/docs/COMMANDS.md) · [Docs](docs/documentation-index.md)

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
