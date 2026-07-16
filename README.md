# AgentOps

Coding agents ship work that looks finished and isn't. AgentOps turns one
behavior into one bounded experiment, hands the exact result to a fresh
validator, and stores a durable verdict you control. Multi-model work is
caller-selected, not automatic: [`council`](skills/council/SKILL.md) for
independent judges on a frozen subject, [`idea-genie`](skills/idea-genie/SKILL.md)
duel mode for sealed perspectives before Plan.

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

## Quickstart

```bash
npx skills@latest add boshu2/agentops --all -g
```

Works for Claude Code, Codex, Cursor, and other skills-compatible agents.
Restart the agent, then run `plan`, `implement`, `validate`, or `rpi`.

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
the repo.

## Optional: `ao` CLI

Deterministic checks, inspection, and skill linking — not required for the
core loop.

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

A single context can share blind spots with the author. For contested ideas,
[`idea-genie`](skills/idea-genie/SKILL.md) duel mode seals independent
perspectives before cross-review and hands dissent to Plan. For high-stakes
judgment of a frozen subject, [`council`](skills/council/SKILL.md) runs
independent judges — across models when you want them — and returns agreement,
disagreement, and minority evidence. Neither writes `verdict.v2`; Validate does.

### 3. Acceptance drifted mid-flight

Without a fixed behavior and write scope, "done" is whatever the agent
improvised. `plan` locks normal and edge acceptance in the existing intent
source before anyone builds.

### 4. Nobody can replay what was judged

Chat scrolls away. `validate` writes a content-addressed `verdict.v2` under
`.agents/ao/verdicts/sha256/` with checked scope, omissions, and evidence
refs. Plain JSON. No hosted service required.

### 5. The loop tried to own delivery

Retries, queues, PRs, and merges are caller policy. AgentOps stops after one
report. `rpi` runs Plan → Implement → Validate once; you decide what happens
next.

## Core skills

| Skill | Job |
|---|---|
| [`rpi`](skills/rpi/SKILL.md) | run Plan, Implement, and fresh Validate at most once |
| [`plan`](skills/plan/SKILL.md) | refine acceptance, evidence, and scope in the intent source |
| [`implement`](skills/implement/SKILL.md) | one RED → GREEN → refactor experiment |
| [`validate`](skills/validate/SKILL.md) | independent judgment; persist `verdict.v2` |

Optional later: [`learn`](skills/learn/SKILL.md) over verdict collections.
Caller-selected strategies: [`council`](skills/council/SKILL.md) (multi-judge
validation), [`idea-genie`](skills/idea-genie/SKILL.md) (portfolio + sealed
duels), [`premortem`](skills/premortem/SKILL.md),
[`postmortem`](skills/postmortem/SKILL.md). Adapters (NTM, Agent Mail, Gas City,
swarms) never change the core sequence or a verdict.

Full inventory: [Skill Router](docs/SKILL-ROUTER.md).

## Evidence contract

A `PASS` binds unchanged acceptance, a deterministic subject manifest, complete
changed-path coverage inside write scope, distinct author and validator context
IDs, a freshness attestation, and criterion-level evidence.

Missing identity, mutation, or incomplete coverage → `NOT_PROVEN`. Proven
out-of-scope change or failed criterion → `FAIL`.

## Boundary

AgentOps owns intent shaping, one experiment, content identity, fresh judgment,
and the verdict. It does not own retries, budgets, queues, claims, Git, CI,
PRs, merges, closure, or release.

Fresh judgment is a practical trust boundary, not cryptographic isolation.
Context IDs and freshness are declared facts.

[Product boundary](PRODUCT.md) · [Operating loop](docs/architecture/operating-loop.md) · [CLI](cli/docs/COMMANDS.md) · [Docs](docs/documentation-index.md)

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
