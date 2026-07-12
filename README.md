<div align="center">

# AgentOps

[![GitHub stars](https://img.shields.io/github/stars/boshu2/agentops?style=social)](https://github.com/boshu2/agentops/stargazers)

### Operating loop for coding agents — intent → validated code

Coding agents declare "done" on code that is still wrong. AgentOps is the **operating loop** that turns declared intent into validated code with proof: shape behavior (Gherkin), implement against a failing acceptance test, then bind an independent membrane verdict to **that** contract. **No verdict = not done.** Skills are the front door; it sits on the agent you already use (Claude Code, Codex, Cursor, OpenCode).

</div>

---

## Install

Pick your runtime and install:

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

**Live skills from a clone (optional).** Already have the repo checked out? `ao skills link` *symlinks* its skills into the live tier of **every agent runtime you have installed** — `~/.claude/skills`, `~/.codex/skills`, `~/.gemini/skills` (AGY), `~/.cursor/skills`, `~/.pi/skills` — so, unlike the copy-based installers above (which snapshot the skills at install time), your local edits and every `git pull` take effect next session with **no re-copy**:

```bash
git clone https://github.com/boshu2/agentops && cd agentops
ao skills link              # symlink repo skills into every installed runtime (idempotent, non-destructive)
git pull && ao skills link  # after a pull: mint links for any newly-added skills
```

Opt-in — the live/edit-in-place tier for people working from a clone; the installers above stay the copy-based path for everyone else. Never copies or clobbers: existing non-AgentOps skills (e.g. other marketplaces) are reported as conflicts and left untouched. `--dest <dir>` targets one specific dir instead.

Installs hookless. The only hard requirement is an agent runtime and `git`; everything else degrades gracefully. Dependencies: [docs/dependencies.md](docs/dependencies.md) · Day-2 ops (update, backup, recovery): [docs/install-day2-ops.md](docs/install-day2-ops.md).

---

## What you get

<!-- agentops:claim:AOP-CLAIM-README-FACTORY-CONTEXT -->
<!-- agentops:claim:AOP-CLAIM-README-COMPETITIVE-MEMORY -->

- **An operating loop.** Skills run intent → Gherkin → ATDD → implement → membrane → ratchet. Full map: [Intent → Validated Code](docs/architecture/intent-to-validated-code.md) · [Skills Matrix](docs/skills-matrix.md).
- **A validation membrane.** `/validate`, `/council`, `/pre-mortem`, and `/pawl-review` prove or reject work against the slice's acceptance behavior. No verdict = not done. Without a behavior contract, there is nothing honest to accept.
- **A bookkeeper that outlives the session.** Beads track work; verdicts bind into a hash-chained provenance ledger — tamper-evident, portable across sessions and models.
- **An evidence trail that's yours.** Runs, decisions, and verdicts land in `.agents/` in your repo. No hosted control plane; Apache-2.0.
- **It runs on the agent you already use.** Claude Code, Codex, Cursor, OpenCode. Same skills, same corpus.

```text
> /plan "rate-limit /login"     # freeze Given/When/Then + acceptance
> /implement <bead>             # RED acceptance → green → refactor
> /validate                     # membrane vs those scenarios

[membrane] acceptance mapped → scenarios S1, S2
[judge] REFUTE  S2 burst refill lacks jitter — claimed covered, isn't
Verdict: HOLD — not done. Fix S2, then re-validate.
```

<!-- agentops:claim:AOP-CLAIM-README-FIRST-VALIDATED -->
Already installed? First value is one loop tick via **skills**: `/plan` a small behavior (Gherkin), `/implement` it against a failing acceptance test, `/validate` so the verdict cites that scenario. Or run `/rpi "a small goal"` for the same tick in one flow. Step-by-step: [first-value path](docs/first-value-path.md).

---

The rest is below the fold for anyone who wants the detail.

## Skills

Skills are the front door. Every skill is one move (or a wrapper) in the operating loop; flows compose them.

**Maps:** [Intent → Validated Code](docs/architecture/intent-to-validated-code.md) · [Skills Matrix](docs/skills-matrix.md) · [Router](docs/SKILLS.md) · [SKILL-ROUTER](docs/SKILL-ROUTER.md)

| Skill | Loop role |
|---|---|
| `/plan` | Shape intent as BDD; slice + acceptance-gated beads |
| `/implement` | One bead: RED acceptance → green → refactor |
| `/validate` | Membrane — prove acceptance; no verdict = not done |
| `/rpi` | One full tick (research → plan → implement → validate) |
| `/pre-mortem` | Stress-test the plan before build |
| `/council` | Multi-judge consensus when stakes are high |
| `/post-mortem` | Capture evidence; ratchet learnings into the next loop |

## The `ao` CLI

Supporting control plane behind the skills (bookkeeping, retrieval, release gate) — not the front door. Full reference: [CLI commands](cli/docs/COMMANDS.md).

<!-- agentops:claim:AOP-CLAIM-README-EVOLVE-AUTONOMOUS -->

```bash
ao quick-start            # set up AgentOps in a repo
ao doctor                 # check skills, reviewers, ledger health
ao gate check --fast      # the release gate before you push
ao verify                 # commit/pre-push ratchet (supporting; skills own the loop)
ao provenance show <sha>  # recorded verdict trail
ao done <bead-id>         # close tracked work with its verdict attached

# Experimental (still measuring whether these pay off; see the honest version below):
ao search "query"         # search history and local knowledge
ao lookup --query "topic" # retrieve curated learnings
ao compile                # rebuild the corpus
```

<!-- agentops:claim:AOP-CLAIM-README-AUTONOMOUS-FLYWHEEL -->
The whole loop runs in a plain session via skills. No daemon, no scheduler, no cloud. For always-on work, a substrate can dispatch whole `/rpi` ticks. Details: [docs/3.0.md](docs/3.0.md) · [operating loop](docs/architecture/operating-loop.md) · [Intent → Validated Code](docs/architecture/intent-to-validated-code.md).

## The honest version

**Proven:** independent verification that records a verdict, and a durable, tamper-evident record of it. A change isn't done until something that didn't write it checks it against the declared acceptance behavior, and that verdict is bound into the provenance ledger. No verdict, not done.

The receipts are public: [membrane receipts](docs/evidence/membrane-receipts.md) — every number derived straight from the verdict ledger, none hand-written.

**Still measuring:** whether the accumulated corpus makes the next session measurably better. We won't claim it until the numbers say so ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).

AgentOps proves the work. It doesn't write the code; your agent still does that, and the cross-checks cost tokens. The `.agents/` folder is plain markdown your agents keep up as they go.

When the labs ship their own version of this, your `.agents/` folder comes with you. It's in your repo, in plain markdown, Apache-2.0.

---

[What 3.0 is](docs/3.0.md) · [Intent → Validated Code](docs/architecture/intent-to-validated-code.md) · [Skills Matrix](docs/skills-matrix.md) · [vs hosted code review](docs/comparisons/vs-hosted-code-review.md) · [docs index](docs/documentation-index.md) · [newcomer guide](docs/newcomer-guide.md) · [architecture](docs/ARCHITECTURE.md) · [FAQ](docs/FAQ.md) · [upgrading / removed commands](docs/MIGRATION.md) · built on the [12-factor doctrine](https://12factoragentops.com).

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) (agents: read [AGENTS.md](AGENTS.md), track work with `br`). License: Apache-2.0.
