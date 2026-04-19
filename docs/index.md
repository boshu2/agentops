---
title: AgentOps
description: The operational layer for coding agents. Memory, validation, and feedback loops that compound between sessions.
hide:
  - navigation
  - toc
---

# AgentOps { .landing-hero }

<p class="hero-tagline">
  The operational layer for coding agents.<br>
  Memory, validation, and feedback loops that compound between sessions.
</p>

<p class="hero-actions" markdown>
[:octicons-rocket-24: Get Started](getting-started/index.md){ .md-button .md-button--primary }
[:octicons-mark-github-24: View on GitHub](https://github.com/boshu2/agentops){ .md-button }
[:octicons-terminal-24: Install](#install){ .md-button }
</p>

---

## What is AgentOps?

AgentOps is a **skills + hooks + CLI system** that gives coding agents the
operational discipline they're missing out of the box. Hooks into **Claude
Code**, **Codex**, **OpenCode**, and any AGENTS.md-aware harness.

<div class="grid cards" markdown>

-   :material-brain: **Memory that compounds**

    ---

    Every session writes to `.agents/`. The next session reads decay-ranked,
    token-budgeted knowledge via `ao inject`. Learnings, findings, and
    retrospectives feed each other.

    [:octicons-arrow-right-24: Knowledge Flywheel](knowledge-flywheel.md)

-   :material-check-all: **Validation, not vibes**

    ---

    A council of judges, the Brownian Ratchet gate, and surface-specific
    contracts turn "looks good" into "shipped and proven".

    [:octicons-arrow-right-24: Brownian Ratchet](brownian-ratchet.md)

-   :material-pipe: **Full RPI lifecycle**

    ---

    `Research → Plan → Implement → Validate`. One command (`ao rpi`) runs the
    entire loop, autonomously, with retry gates and complexity scaling.

    [:octicons-arrow-right-24: How It Works](how-it-works.md)

-   :material-puzzle: **Composable skills**

    ---

    Drop-in skills for discovery, implementation, validation, release, swarms,
    councils, pre-mortems, retros, and more. Every skill is a single
    `SKILL.md` contract.

    [:octicons-arrow-right-24: Skills Catalog](skills/catalog.md)

</div>

---

## Install

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

Installs `ao` CLI, skills, and hooks into your home directory. Works with
Claude Code (`~/.claude/`), Codex, and OpenCode.

Verify:

```bash
ao --version
ao quickstart
```

---

## One-command RPI

```bash
ao rpi "fix the flaky auth test"
```

AgentOps runs:

1. **Discovery** — brainstorm, research, plan, pre-mortem (council-validated)
2. **Implementation** — wave-based execution with automatic retries
3. **Validation** — vibe, post-mortem, retro, forge learnings back into the flywheel

No intermediate prompts. Autonomous by default. Interactive with `--interactive`.

---

## Explore

<div class="grid cards" markdown>

-   :material-book-open: **[Newcomer Guide](newcomer-guide.md)**

    ---

    Repo orientation, mental model, and a fast path to becoming productive.

-   :material-console-line: **[CLI Reference](cli/commands.md)**

    ---

    Every `ao` command, flag, and exit code. Auto-generated.

-   :material-file-tree: **[Architecture](ARCHITECTURE.md)**

    ---

    System design: bookkeeping, validation, primitives, flows, RPI pipeline.

-   :material-school: **[Levels L1–L5](levels/index.md)**

    ---

    Progressive learning path from single-session work to full orchestration.

-   :material-compare: **[Comparisons](comparisons/README.md)**

    ---

    AgentOps vs Spec-Driven Development, Claude-Flow, Superpowers, and more.

-   :material-file-document-multiple: **[Contracts](contracts/index.md)**

    ---

    RPI run registry, finding registry, dream runs, OL-AO bridge, and every
    other inter-component contract.

</div>

---

<p class="hero-footer" markdown>
Built by the AgentOps contributors. [Read the Philosophy](philosophy.md) ·
[The Science](the-science.md) · [Strategic Direction](strategic-direction.md)
</p>
