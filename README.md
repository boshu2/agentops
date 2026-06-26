<div align="center">

# AgentOps

[![GitHub stars](https://img.shields.io/github/stars/boshu2/agentops?style=social)](https://github.com/boshu2/agentops/stargazers)

### Autonomous code validation for coding agents

Coding agents declare "done" on code that is still wrong. AgentOps catches that: every change is independently verified by a fresh-context reviewer — cross-family or deterministic — and reaches *done* only with a proof artifact. **No verdict = not done.** It sits on top of the agent you already use (Claude Code, Codex, Cursor, OpenCode).

</div>

---

## Install

Pick your runtime, then type `/quickstart` in the agent.

```bash
# Claude Code
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex CLI (macOS/Linux/WSL) — OpenCode: install-opencode.sh
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
# Codex CLI (Windows):
irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.ps1 | iex

# Gemini / Antigravity
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash

# Other skills-compatible agents (Cursor, etc.)
npx skills@latest add boshu2/agentops --cursor -g
```

The `ao` CLI is optional but recommended (bookkeeping, retrieval, the release gate):

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops && brew install agentops   # macOS
# Windows: irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex
# Or release binaries / build from source (cli/README.md).
```

Installs hookless — the only hard requirement is an agent runtime and `git`; everything else degrades gracefully. Dependencies: [docs/dependencies.md](docs/dependencies.md) · Day-2 ops (update, backup, recovery): [docs/install-day2-ops.md](docs/install-day2-ops.md).

---

## What you get

<!-- agentops:claim:AOP-CLAIM-README-FACTORY-CONTEXT -->
<!-- agentops:claim:AOP-CLAIM-README-COMPETITIVE-MEMORY -->

- **A validation membrane.** Tests, gates, `/pre-mortem`, `/validate`, `/council`, and cross-family pawl verdicts prove or reject the work before you trust it. No verdict, not done.
- **An evidence trail that's yours.** Every run, decision, and verdict lands in `.agents/` in your repo — grep-able, diff-able, portable to whatever model wins next quarter. AgentOps adds no hosted control plane and no telemetry; the corpus lives in your repo, not on our servers. Apache-2.0.
- **It runs on the agent you already pay for.** Claude Code, Codex, Cursor, OpenCode — same skills, same corpus.

```text
> /validate --mixed   # the agent reported this PR done

[membrane] evidence sealed → fresh-context judges, Claude Code + Codex CLI
[claude/judge-1] REFUTE  /login has no rate limit — claimed "covered", isn't
[codex/judge-1]  REFUTE  token-bucket refill lacks jitter under burst
[claude/judge-2] PASS    redis integration follows the repo pattern
Verdict: HOLD — not done. Fix /login limit + refill jitter, then re-verify.
Recorded → .agents/council/<run-id>/verdict.md
```

<!-- agentops:claim:AOP-CLAIM-README-FIRST-VALIDATED -->
Already installed? Ask your agent `/quickstart`, or run `/rpi "a small goal"` to take one change through discovery, build, and validation end to end — the evidence lands in `.agents/`.

---

The rest is below the fold for anyone who wants the detail.

## Skills

Every skill works alone; flows compose them. Full catalog: [docs/SKILLS.md](docs/SKILLS.md) · [Skill Router](docs/SKILL-ROUTER.md).

| Skill | Use it when |
|---|---|
| `/quickstart` | you want the fastest setup check and next action |
| `/research` | you need codebase context and prior learnings before changing code |
| `/pre-mortem` | you want to pressure-test a plan before building |
| `/rpi` | you want discovery, build, validation, and bookkeeping in one flow |
| `/council` | you want independent judges (optionally Claude and Codex) to return one verdict |
| `/validate` | you want a code-quality and risk review before shipping |
| `/evolve` | a goal-driven improvement loop that compounds knowledge without mutating source |

## The `ao` CLI

Repo-native control plane behind the skills. Full reference: [CLI commands](cli/docs/COMMANDS.md).

<!-- agentops:claim:AOP-CLAIM-README-EVOLVE-AUTONOMOUS -->

```bash
ao quick-start            # set up AgentOps in a repo
ao search "query"         # search history and local knowledge
ao lookup --query "topic" # retrieve curated learnings
ao context assemble       # build a task briefing
ao gate check --fast      # the release gate — verify before you push
ao compile                # rebuild the corpus
ao metrics health         # flywheel health
```

<!-- agentops:claim:AOP-CLAIM-README-AUTONOMOUS-FLYWHEEL -->
The whole loop runs in a plain session — no daemon, no scheduler, no cloud. For always-on work it opts into a swappable substrate (an NTM tmux swarm, MCP via `ao mcp serve`, or managed-agents) that dispatches a whole loop per ready bead. Details: [docs/3.0.md](docs/3.0.md) · [operating loop](docs/architecture/operating-loop.md).

## What's proven, and what isn't

The verification is the proven product: no verdict, not done. Everything that *compounds* on top of it is an honest, measured bet — not a marketing claim:

- The `.agents/` corpus is an LLM wiki of markdown that agents read and write as they work. Whether it measurably beats the same models with no corpus is unproven ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)).
- When a verdict says CONFIRMED but the code later turns out wrong, that's an *escape*; the membrane compiles each escape into a check that catches it next time. That mechanism is proven; whether the **escape-corpus** compounds into a durable edge is not — a good membrane catches most things at review, so real escapes are structurally rare ([ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).

**Other limits, plainly:** it doesn't write the code (the harness still does); multi-model councils cost tokens; the corpus needs hygiene (`ao defrag`, `ao maturity`) or it rots like any markdown vault.

**What if the labs ship this natively?** They will. Two things don't follow the vendor: the verification discipline you run on every change, and the `.agents/` corpus — plain markdown in your repo, forkable, portable, Apache-2.0.

---

[What 3.0 is](docs/3.0.md) · [docs index](docs/documentation-index.md) · [newcomer guide](docs/newcomer-guide.md) · [architecture](docs/ARCHITECTURE.md) · [FAQ](docs/FAQ.md) · built on the [12-factor doctrine](https://12factoragentops.com).

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) (agents: read [AGENTS.md](AGENTS.md), track work with `br`). License: Apache-2.0.
