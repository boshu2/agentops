# Getting Started

New to AgentOps? You're in the right place. This section answers three questions in order: **what is it**, **how do I install it**, and **what's the first useful thing I can run**.

If you're evaluating AgentOps for a team, start with the [Newcomer Guide](../newcomer-guide.md) — it frames the product in fifteen minutes. If you're ready to ship code with it, skip to [Install](#install) and then [First command](#first-command). If you want a structured curriculum, jump to the [Levels](../levels/index.md) path at the bottom.

<div class="grid cards" markdown>

-   :material-rocket-launch: **[Newcomer Guide](../newcomer-guide.md)**

    ---

    Practical repo orientation, mental model, and a fast path to becoming
    productive in the AgentOps codebase.

-   :material-puzzle-plus: **[Create Your First Skill](../create-your-first-skill.md)**

    ---

    Fast path for authoring a first skill without tripping CI.

-   :material-school: **[Behavioral Discipline](../behavioral-discipline.md)**

    ---

    Before/after examples of good coding-agent behavior.

-   :material-frequently-asked-questions: **[FAQ](../FAQ.md)**

    ---

    Comparisons, limitations, subagent nesting, and uninstall.

-   :material-hand-coin: **[Contributing](../CONTRIBUTING.md)**

    ---

    How to contribute code, skills, and documentation.

-   :material-shield-check: **[Security](../SECURITY.md)**

    ---

    Vulnerability reporting and security policy.

</div>

## Install

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

## First command

```bash
ao quickstart      # Guided first-run experience
ao status          # Where was I?
ao rpi "goal"      # Full Research-Plan-Implement-Validate loop
```

## Learning path

If you want a structured curriculum, walk through the progressive **[Levels
L1–L5](../levels/index.md)**:

1. **L1 — Basics**: single-session work
2. **L2 — Persistence**: cross-session bookkeeping
3. **L3 — State Management**: issue tracking with beads
4. **L4 — Parallelization**: wave-based execution
5. **L5 — Orchestration**: full autonomous operation
