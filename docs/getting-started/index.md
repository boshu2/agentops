# Getting Started

New to AgentOps? You're in the right place. This section answers three questions in order: **what is it**, **how do I install it**, and **what's the first useful thing I can run**.

If you're evaluating AgentOps for a team, start with the [Newcomer Guide](../newcomer-guide.md) — it frames the product in fifteen minutes. If you're ready to ship code with it, skip to [Install](#install) and then [Golden Paths](#golden-paths). If you want a structured curriculum, jump to the [Levels](../levels/index.md) path at the bottom. If you are upgrading from an older release, read [Upgrading](../UPGRADING.md) first.

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

Pick the installer for your runtime.

=== "Claude Code"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
    ```

=== "Codex CLI"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
    ```

    Codex installs hookless by default. Use `install-codex.sh --with-hooks`
    only when you intentionally want native Codex hooks.

=== "OpenCode"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash
    ```

=== "Gemini / Antigravity"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
    ```

Then install the `ao` CLI for repo seeding, health checks, and terminal
workflows.

=== "macOS"

    ```bash
    brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
    brew install agentops
    ```

=== "Windows PowerShell"

    ```powershell
    irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex
    ```

For other skills-compatible agents, install selected skills with:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

Day-2 install, update, backup, permission, recovery, and escalation paths are in
[Install And Day-2 Operations](../install-day2-ops.md).

## Verify the install

```bash
ao doctor              # Checks runtime, skills, hooks, paths
ao --version           # Prints installed ao version
```

`ao doctor` is the canonical health check. It prints a per-component status line
and non-zero exits on a real problem. If anything looks wrong, see
[Troubleshooting](../troubleshooting.md) or run `ao doctor --verbose`.

## Product map (read once)

AgentOps is the **operating loop** from intent to validated code. Skills are
the front door. The membrane only accepts work against a behavior contract
(Gherkin → ATDD).

- [Intent → Validated Code](../architecture/intent-to-validated-code.md) — full flow
- [Skills Matrix](../skills-matrix.md) — every skill on the loop
- [First-value path](../first-value-path.md) — first session via skills

## Golden Paths

Pick one path. Each path ends with proof against a **named acceptance behavior**
(and usually an artifact under `.agents/` or an explicit PASS/WARN/FAIL verdict).

| I want to... | Run | Success signal |
|--------------|-----|----------------|
| Set up a repo for the first time | `ao quick-start`, then `/bootstrap` if needed | Readiness summary; skills resolve in the agent |
| Make the first validated change | `/plan` → `/implement` → `/validate` (or `/rpi "a small goal"`) | Gherkin scenario exists; acceptance went RED then green; `/validate` cites that scenario |
| Stress-test a plan before build | `/pre-mortem` | Plan HOLD/PASS before implement |
| High-stakes review | `/validate` then `/council` | Independent judges + acceptance mapping |
| Continue tracked work | `BEADS_DIR="$(ao beads dir)" br ready`, then `/implement <issue-id>` or `/crank <epic-id>` | Bead acceptance examples still map to evidence |
| Full loop map | Read [Intent → Validated Code](../architecture/intent-to-validated-code.md) | You can name moves 1–7 and their primary skills |

## Command Reference

```bash
ao quick-start     # Canonical repo seed and readiness repair
ao quickstart      # Stable alias for the same golden path
ao doctor          # Health check (skills, reviewers, paths)
ao status          # Where was I?
# Drive the loop in-session via skills: /plan → /implement → /validate (or /rpi)
```

## Learning path

If you want a structured curriculum, walk through the progressive **[Levels
L1–L5](../levels/index.md)**:

1. **L1 — Basics**: single-session work
2. **L2 — Persistence**: cross-session bookkeeping
3. **L3 — State Management**: issue tracking with beads
4. **L4 — Parallelization**: wave-based execution
5. **L5 — Orchestration**: full autonomous operation
