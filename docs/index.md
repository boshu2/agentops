---
title: AgentOps
description: Autonomous code validation for coding agents. Prove whether agent-written code is right, compile the evidence, and compound the context so each session starts loaded, not cold.
hide:
  - navigation
  - toc
---

# AgentOps { .landing-hero }

### Autonomous code validation for coding agents.

<!-- agentops:claim:AOP-CLAIM-DOCS-INDEX-CORPUS -->
AgentOps keeps the books, compiles context, gates output, and compounds learning so coding agents can prove their work before you grant them more autonomy. Its substrate is `.agents/`: a wiki of markdown files in your repo, version-controlled with your code, that agents read, traverse, and contribute to.

AgentOps uses software-engineering practice people already understand — Agile/XP, BDD/Gherkin, DDD, hexagonal architecture, TDD, CI/CD, SRE, ADRs, provenance, and durable knowledge — then compiles those practices into dense, verifiable context for LLM agents under token scarcity. The internal lifecycle is the CDLC: context gets developed, tested, delivered, observed, and improved like any other software asset.

*The only verifiable moat in this uncertain time is context. Models will get smarter, harnesses will commoditize, agents will get cheaper. Your accumulated context — the lessons learned about your individual problems, the patterns that worked, the decisions that survived review — is the one asset that compounds and doesn't get eaten by the next vendor release. That's what your company actually is.*

*AgentOps is the shovel. Start digging.*

<p class="hero-actions" markdown>
[:octicons-rocket-24: Install](#install){ .md-button .md-button--primary }
[:octicons-play-24: See It Work](#see-it-work){ .md-button }
[:octicons-mark-github-24: GitHub](https://github.com/boshu2/agentops){ .md-button }
</p>

---

## The Problem

Every agent session starts cold. Same mistakes. Same rework. The landmine in `auth.py`, the two-hour timeout debug, the flag the reviewer always catches — none of it carries forward.

**AgentOps solves this** with four product layers:

| Layer | What it does |
|-------|-------------|
| **Bookkeeping** | Records what agents tried, changed, validated, and learned so the work leaves evidence |
| **Context Compiler** | Assembles the right context for the right phase — decay-ranked, token-budgeted, loaded at session start |
| **Validation Membrane** | `/validate` checks the declared acceptance behavior; `/pre-mortem` and `/council` add independent challenge when warranted |
| **Knowledge Flywheel** | Extracts learnings, scores them, and resurfaces them so the next session starts smarter |

Session 1, your agent spends two hours debugging a timeout bug. Session 15, a new agent finds the lesson in seconds because the corpus kept it.

```mermaid
flowchart LR
    B[Bookkeeping] --> C[Context Compiler]
    C[Context Compiler] --> S[Session work]
    S --> G[Validation Gates]
    G --> F[Knowledge Flywheel]
    F --> B
```

All AgentOps state lives in local `.agents/` — auditable, versionable, yours. Plain text you can grep, diff, review, and exclude from source control. No AgentOps-managed telemetry or hosted control plane; model runtimes, Git remotes, installers, and external tools are operator-selected dependencies. For constrained environments, see the [Assurance Profile](assurance-profile.md).

---

## Install

Pick the runtime you use.

=== "Claude Code"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
    ```

=== "Codex CLI (macOS / Linux / WSL)"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
    ```

=== "Codex CLI (Windows PowerShell)"

    ```powershell
    irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.ps1 | iex
    ```

=== "OpenCode"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash
    ```

=== "Gemini / Antigravity"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
    ```

Restart your agent after install, then make one small validated change:
`/plan` → `/implement` → `/validate`. The [first-value path](first-value-path.md)
walks through the evidence each move must produce.

Day-2 install, update, backup, permission, recovery, and escalation paths:
[Install And Day-2 Operations](install-day2-ops.md).

The `ao` CLI is optional but recommended. It unlocks repo-native bookkeeping, retrieval, health checks, and terminal workflows.

=== "macOS"

    ```bash
    brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
    brew install agentops
    ao version
    ```

=== "Windows PowerShell"

    ```powershell
    irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex
    ao version
    ```

---

## See It Work

One behavior, carried from intent to proof.

### Full loop: acceptance through validation

```text
> /plan "add retry backoff to rate limiter"

[plan]       Given a transient failure, when a retry is scheduled,
             then delay grows with bounded jitter
[implement] acceptance test RED → implementation → GREEN
[validate]  scenario mapped, behavior independently reproduced
Verdict: CONFIRMED — evidence recorded
```

Use `/rpi "add retry backoff to rate limiter"` when you want the same skill loop
coordinated as one full tick. Add `/pre-mortem` before implementation or
`/council` after acceptance exists when the stakes justify more independent
judgment.

The point is not a bigger prompt. The point is behavior with an explicit
acceptance surface, independent verification, and durable evidence.

---

## The headline skills

Every skill works alone. Compose flows for end-to-end cycles.

| Skill | Use it when |
|-------|-------------|
| [`/plan`](skills/plan.md) | You need testable acceptance behavior and vertical slices |
| [`/implement`](skills/implement.md) | You want one scoped task built and validated |
| [`/validate`](skills/validate.md) | You need an independent verdict against the declared behavior |
| [`/rpi`](skills/rpi.md) | You want discovery, build, validation, and bookkeeping in one flow |
| [`/pre-mortem`](skills/pre-mortem.md) | You want to pressure-test a plan before implementation |
| [`/council`](skills/council.md) | You want additional independent judges for a high-stakes plan, change, or decision |
| [`/research`](skills/research.md) | You need codebase context and prior learnings before changing code |
| [`/evolve`](skills/evolve.md) | You want a goal-driven improvement loop with regression gates |

!!! info "Full catalog"
    [:octicons-book-24: **Skills reference**](SKILLS.md) — current entry points and links to generated source maps.
    [:octicons-routes-24: **Decision tree**](skills-decision-tree.md) — "which skill do I need next?"

---

## Unsupervised Cycles

<!-- agentops:claim:AOP-CLAIM-DOCS-INDEX-AUTONOMOUS-CYCLES -->
**Day: autonomous improvement.** `/evolve` reads `GOALS.md`, fixes the worst fitness gap, runs regression gates, records each cycle.

```text
> /evolve

[evolve] GOALS.md loaded
[cycle-1] Worst gap selected
[rpi]     Implements the fix
[gate]    Tests and quality checks pass
[learn]   Post-mortem feeds the flywheel
```

**Night: knowledge compounding.** An adopted substrate can run bookkeeping-only compounding over `.agents/`: consolidate learnings, dedupe patterns, defragment the corpus, and report health. Source code stays untouched unless the operator dispatches a foreground `/rpi` loop.

```text
> /curate --mode=forge
> /compile

[compile] INGEST  harvest new artifacts
[compile] REDUCE  dedup, defrag, close loops
[measure] corpus quality recorded

Report: .agents/compile/<run-id>/summary.md
```

Run compounding on the substrate's schedule, then Evolve in the morning against a fresher corpus. Same model, smarter environment.

---

## Next steps

1. **[Install](#install)** — pick your runtime.
2. **Seed** your repo with `ao quick-start` (`ao quickstart` also works).
3. **Make one validated change:** `/plan` → `/implement` → `/validate` (or `/rpi "a small goal"`) — see [Intent → Validated Code](architecture/intent-to-validated-code.md) and the [first-value path](first-value-path.md). Use `/council` for high-stakes review after acceptance exists; use `BEADS_DIR="$(ao beads dir)" br ready` then `/implement` to continue tracked work.

Read the lineage at [12factoragentops.com](https://12factoragentops.com) — DevOps applied to coding agents in twelve factors.

---

## Explore

<div class="grid cards" markdown>

-   :material-book-open: **[Newcomer Guide](newcomer-guide.md)**

    ---

    Repo orientation, mental model, and a fast path to becoming productive.

-   :material-school: **[Levels L1–L5](levels/index.md)**

    ---

    Progressive learning curriculum from single-session work to full autonomous orchestration.

-   :material-console-line: **[CLI Reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md)**

    ---

    Every `ao` command, flag, and exit code. Auto-generated on every build.

-   :material-file-tree: **[Architecture](ARCHITECTURE.md)**

    ---

    System design: context compiler, validation gates, knowledge flywheel, RPI pipeline.

-   :material-compare: **[Comparisons](comparisons/README.md)**

    ---

    AgentOps vs Spec-Driven Development, Claude-Flow, Superpowers, Compound Engineer.

-   :material-file-document-multiple: **[Contracts](contracts/index.md)**

    ---

    RPI run registry, finding registry, Dream runs, and every
    other inter-component contract.

</div>

---

<p class="hero-footer" markdown>
Built by the AgentOps contributors. [Philosophy](philosophy.md) · [The Science](the-science.md) · [Strategic Direction](strategic-direction.md)
</p>
