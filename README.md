# AgentOps

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Claude Code](https://img.shields.io/badge/Claude_Code-Plugin-blueviolet)](https://github.com/anthropics/claude-code)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://makeapullrequest.com)

<div align="center">

## Your AI agent has amnesia. Let's fix that.

**Other tools make Claude faster. AgentOps makes Claude *smarter*.**

</div>

---

<!-- Accessibility: Comparison showing 4 sessions without AgentOps (repeating same questions) vs with AgentOps (progressive learning, knowledge compounds) -->
```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│    WITHOUT AGENTOPS                    WITH AGENTOPS                    │
│    ─────────────────                   ─────────────                    │
│                                                                         │
│    Session 1:  "How does auth work?"   Session 1:  "How does auth..."  │
│    Session 2:  "How does auth work?"   Session 2:  "I remember this"   │
│    Session 3:  "How does auth work?"   Session 3:  "Auth? Easy."       │
│    Session 4:  "How does auth work?"   Session 4:  *instant recall*    │
│                                                                         │
│    ════════════════════════════════    ════════════════════════════    │
│    Repeating                           Compounding                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Why AgentOps?

| Other Tools | AgentOps |
|------------|----------|
| Make Claude faster *this session* | Make Claude smarter *every session* |
| Knowledge dies when chat ends | Knowledge persists in git |
| Same bugs rediscovered | Bugs remembered & prevented |
| Linear workflow | **Knowledge compounds exponentially** |

**The science:** Organizational knowledge decays ~17% per week ([Darr et al., 1995](https://pubsonline.informs.org/doi/abs/10.1287/mnsc.41.11.1750)). If retrieval rate > decay rate, knowledge compounds instead. AgentOps achieves escape velocity.

---

## How It Works

<!-- Accessibility: Flowchart showing Research → Plan → Pre-mortem → Crank → Post-mortem → Extract Learnings, which feeds back to Research. The cycle compounds knowledge over ~100 sessions. -->
```
                    THE KNOWLEDGE FLYWHEEL
                    ──────────────────────

        ┌──────────┐                      ┌──────────┐
        │ RESEARCH │                      │  LEARN   │
        │  (Day 1) │                      │(Day 100) │
        └────┬─────┘                      └────▲─────┘
             │                                 │
             │    ┌─────────────────────┐      │
             │    │                     │      │
             ▼    ▼                     │      │
        ┌─────────────┐           ┌─────┴─────┐
        │    PLAN     │           │  EXTRACT  │
        │             │           │ LEARNINGS │
        └──────┬──────┘           └─────▲─────┘
               │                        │
               ▼                        │
        ┌─────────────┐           ┌─────┴─────┐
        │ PRE-MORTEM  │           │   POST-   │
        │(catch fails)│           │  MORTEM   │
        └──────┬──────┘           └─────▲─────┘
               │                        │
               ▼                        │
        ┌─────────────┐                 │
        │    CRANK    │─────────────────┘
        │ (implement) │
        └─────────────┘

        By Session 100: Domain Expert
```

---

## The Complete System

<!-- Accessibility: Comprehensive diagram showing full RPI workflow with all 20 agents, 5 gates, Brownian Ratchet pattern at each stage, and Knowledge Flywheel feedback loop -->
```
═══════════════════════════════════════════════════════════════════════════════════
                           AGENTOPS: THE COMPLETE SYSTEM
═══════════════════════════════════════════════════════════════════════════════════

  ╔═══════════════════════════════════════════════════════════════════════════════╗
  ║                           THE KNOWLEDGE FLYWHEEL                              ║
  ║  SessionStart hook ──► injects prior learnings ──► you start smarter          ║
  ╚═══════════════════════════════════════════════════════════════════════════════╝
                                        │
                                        ▼
  ┌───────────────────────────────────────────────────────────────────────────────┐
  │ STAGE 1: RESEARCH                                                    GATE 1   │
  │                                                                               │
  │  /research ───► CHAOS: Explore agent deep-dives codebase                      │
  │       │                                                                       │
  │       ├───────► FILTER: 4 validators check research quality                   │
  │       │         • coverage-expert    • depth-expert                           │
  │       │         • gap-identifier     • assumption-challenger                  │
  │       │                                                                       │
  │       └───────► RATCHET: .agents/research/*.md (locked)     [USER APPROVAL]   │
  └───────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌───────────────────────────────────────────────────────────────────────────────┐
  │ STAGE 2: PLAN                                                        GATE 2   │
  │                                                                               │
  │  /plan ───────► CHAOS: Decompose goal into issues with dependencies           │
  │       │                                                                       │
  │       ├───────► FILTER: Dependency graph validates execution order            │
  │       │                                                                       │
  │       └───────► RATCHET: .agents/plans/*.md + bd issues     [USER APPROVAL]   │
  └───────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌───────────────────────────────────────────────────────────────────────────────┐
  │ STAGE 3: PRE-MORTEM (catch failures before building)                 GATE 3   │
  │                                                                               │
  │  /pre-mortem ─► CHAOS: 4 failure experts simulate disasters                   │
  │       │         • integration-failure-expert  • ops-failure-expert            │
  │       │         • data-failure-expert         • edge-case-hunter              │
  │       │                                                                       │
  │       ├───────► FILTER: Rank risks by severity, identify mitigations          │
  │       │                                                                       │
  │       └───────► RATCHET: .agents/pre-mortems/*.md           [USER APPROVAL]   │
  └───────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌───────────────────────────────────────────────────────────────────────────────┐
  │ STAGE 4: IMPLEMENT                                                            │
  │                                                                               │
  │  /crank ──────► CHAOS: Loop through issues, Explore agent per issue           │
  │       │                                                                       │
  │       │         ┌─────────────────────────────────────────────────────┐       │
  │       │         │  FIRE LOOP (per issue):                             │       │
  │       │         │  FIND ──► bd ready (get unblocked issues)           │       │
  │       │         │  IGNITE ► /implement (Explore + code changes)       │       │
  │       │         │  REAP ──► commit with issue reference               │       │
  │       │         │  ESCALATE ► handle failures, retry or mail human    │       │
  │       │         └─────────────────────────────────────────────────────┘       │
  │       │                                                                       │
  │       └───────► RATCHET: git commits (locked, can't regress)                  │
  └───────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌───────────────────────────────────────────────────────────────────────────────┐
  │ STAGE 5: VALIDATE                                                    GATE 4   │
  │                                                                               │
  │  /vibe ───────► CHAOS: 6 validation agents in parallel                        │
  │       │         • security-reviewer     • code-reviewer                       │
  │       │         • architecture-expert   • code-quality-expert                 │
  │       │         • security-expert       • ux-expert                           │
  │       │                                                                       │
  │       ├───────► FILTER: 8-aspect validation (semantic, security, quality,     │
  │       │                  architecture, complexity, performance, slop, a11y)   │
  │       │                                                                       │
  │       │         CRITICAL = 0 ──► PASS                                         │
  │       │         CRITICAL > 0 ──► BLOCK (must fix before proceeding)           │
  │       │                                                                       │
  │       └───────► RATCHET: .agents/vibe/*.md                  [AUTO-GATE]       │
  └───────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌───────────────────────────────────────────────────────────────────────────────┐
  │ STAGE 6: POST-MORTEM (validate + extract learnings)                  GATE 5   │
  │                                                                               │
  │  /post-mortem ► CHAOS: 6 agents validate completion + extract knowledge       │
  │       │         • plan-compliance-expert   • goal-achievement-expert          │
  │       │         • ratchet-validator        • flywheel-feeder                  │
  │       │         • security-expert          • code-quality-expert              │
  │       │                                                                       │
  │       ├───────► FILTER: Synthesize findings, resolve conflicts                │
  │       │                                                                       │
  │       └───────► RATCHET: .agents/retros/*.md + .agents/learnings/*.md         │
  │                                                         [USER: TEMPER/ITERATE]│
  └───────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ╔═══════════════════════════════════════════════════════════════════════════════╗
  ║                           THE KNOWLEDGE FLYWHEEL                              ║
  ║  SessionEnd hook ──► extracts learnings ──► indexes for next session          ║
  ║                                                                               ║
  ║     .agents/                                                                  ║
  ║     ├── learnings/   "Auth bugs stem from token refresh timing"               ║
  ║     ├── patterns/    "How we handle retries in this codebase"                 ║
  ║     ├── research/    Deep exploration outputs                                 ║
  ║     └── retros/      What worked, what didn't                                 ║
  ║                                                                               ║
  ║  NEXT SESSION: Hooks inject relevant knowledge ──► You start smarter          ║
  ╚═══════════════════════════════════════════════════════════════════════════════╝

  ┌───────────────────────────────────────────────────────────────────────────────┐
  │                              THE BROWNIAN RATCHET                             │
  │                                                                               │
  │   At every stage:  CHAOS (explore options) ──► FILTER (validate) ──► RATCHET  │
  │                                                                     (lock)    │
  │                                                                               │
  │   Progress only moves forward. Knowledge compounds. You never go backward.    │
  └───────────────────────────────────────────────────────────────────────────────┘

═══════════════════════════════════════════════════════════════════════════════════
                    20 AGENTS │ 5 GATES │ 1 FLYWHEEL │ ZERO AMNESIA
═══════════════════════════════════════════════════════════════════════════════════
```

---

## The Workflow: 5 Commands

These are **Claude plugin commands** (run in Claude Code chat):

```bash
/research → /plan → /pre-mortem → /crank → /post-mortem
```

| Command | What It Does |
|---------|--------------|
| `/research` | Explores codebase + injects prior knowledge |
| `/plan` | Breaks goal into tracked issues with dependencies |
| `/pre-mortem` | Simulates failure modes *before* you build |
| `/crank` | Implements each issue → validates → commits |
| `/post-mortem` | Validates code + extracts learnings for next time |

**Everything else is automatic.** Skills call each other. Hooks capture knowledge. The flywheel turns itself.

---

## Quick Start

```bash
# 1. Install Plugin (in Claude Code)
claude plugin add boshu2/agentops

# 2. Install CLI (Optional: installs CLI)
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops

# 3. Initialize in your project (Optional: enables automatic knowledge capture)
cd your-project
ao init && ao hooks install    

# 4. Start working (in Claude Code chat)
/research "understand the auth system"

# Expected: Claude explores your codebase, creates .agents/research/ output
# Next: /plan to break work into issues
```

**Terminal CLI** (`ao`) is optional but enables the full knowledge flywheel.

---

## How AgentOps Fits In

**Use it alongside your other plugins.** AgentOps focuses on the memory layer — it plays well with others.

| Plugin | What It Does Best | + AgentOps |
|--------|-------------------|------------|
| [Superpowers](https://github.com/obra/superpowers) | TDD, planning, autonomous work | Superpowers executes, AgentOps remembers |
| [Claude-Flow](https://github.com/ruvnet/claude-flow) | Multi-agent swarms, performance | Claude-Flow orchestrates, AgentOps learns |
| [cc-sdd](https://github.com/gotalab/cc-sdd) | Spec-driven development | SDD specs, AgentOps captures learnings |
| [GSD](https://github.com/glittercowboy/get-shit-done) | Lightweight shipping | GSD for prototypes, AgentOps for production |

*Feature comparisons as of January 2026. See [detailed comparisons](docs/comparisons/) for specifics.*

**What AgentOps uniquely adds:**

| Feature | Others | AgentOps |
|---------|:------:|:--------:|
| Cross-session memory | ❌ | ✅ |
| Knowledge compounding | ❌ | ✅ |
| Pre-mortem failure simulation | ❌ | ✅ |
| 8-aspect semantic validation | ❌ | ✅ |

> [Detailed comparisons →](docs/comparisons/)

---

## The `/vibe` Validator

Not just "does it compile?" — **does it match the spec?**

<!-- Accessibility: Table showing 8 validation aspects: Semantic, Security, Quality, Architecture, Complexity, Performance, Slop, Accessibility. Gate rule: 0 critical = pass, 1+ critical = blocked. -->
```
┌─────────────────────────────────────────────────────────────────┐
│                    8-ASPECT SEMANTIC VALIDATION                 │
├─────────────────────────────────────────────────────────────────┤
│  ✓ Semantic      Does code do what spec says?                   │
│  ✓ Security      SQL injection, auth bypass, hardcoded secrets  │
│  ✓ Quality       Dead code, copy-paste, magic numbers           │
│  ✓ Architecture  Layer violations, circular deps, god classes   │
│  ✓ Complexity    CC > 10, deep nesting, parameter overload      │
│  ✓ Performance   N+1 queries, unbounded loops, resource leaks   │
│  ✓ Slop          AI hallucinations, cargo cult, over-engineering│
│  ✓ Accessibility Missing ARIA, broken keyboard nav, contrast    │
├─────────────────────────────────────────────────────────────────┤
│  GATE: 0 CRITICAL = pass │ 1+ CRITICAL = blocked until fixed   │
└─────────────────────────────────────────────────────────────────┘
```

---

## The Brownian Ratchet

<!-- Accessibility: Diagram showing Brownian Ratchet pattern: Chaos (spawn agents) → Filter (validate, retry on fail) → Ratchet (commit, store in .agents/). Knowledge injects into next session. -->
```
     ┌─────────────────────────────────────────────────────────────┐
     │                                                             │
     │   CHAOS              FILTER              RATCHET            │
     │  (explore)          (validate)         (lock)              │
     │                                                             │
     │   ░░░░░░░░░          ┌─────┐           ══════════           │
     │   ░ spawn ░    ───▶  │pass?│   ───▶    ║ COMMIT ║           │
     │   ░ agents░          └──┬──┘           ══════════           │
     │   ░░░░░░░░░             │                  │                │
     │       ▲                 │ fail             │                │
     │       └─────────────────┘                  │                │
     │                                            ▼                │
     │                                     ┌─────────────┐         │
     │                                     │  .agents/   │◀─────┐  │
     │                                     │  (memory)   │      │  │
     │                                     └──────┬──────┘      │  │
     │                                            │         inject │
     │                                            └────────────────│
     │                                            next session     │
     │                                                             │
     └─────────────────────────────────────────────────────────────┘

     Progress compounds. You never go backward.
```

---

## What Gets Captured

Everything lives in `.agents/` — **git-tracked, portable, yours**.

```
.agents/
├── research/      # Deep exploration outputs
├── plans/         # Implementation plans
├── pre-mortems/   # Failure simulations
├── specs/         # Validated specifications
├── learnings/     # Extracted insights ("Auth bugs stem from token refresh")
├── patterns/      # Reusable patterns ("How we handle retries")
├── retros/        # Session retrospectives
├── vibe/          # Validation reports
├── complexity/    # Complexity analysis
└── ...            # + other skill outputs (doc, assessments, etc.)
```

**Automatic hooks:**
- **SessionStart** → Injects relevant prior knowledge (with decay applied)
- **SessionEnd** → Extracts learnings, indexes for retrieval

You don't run `ao` commands manually. The flywheel turns itself.

---

## The Compound Effect

<!-- Accessibility: Comparison showing progression over 4 sessions. Without AgentOps: repeating. With AgentOps: compounding knowledge. -->
```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│  WITHOUT AGENTOPS                                                    │
│  ════════════════                                                    │
│                                                                      │
│  Session 1   Session 2   Session 3   Session 4                       │
│  ┌──────┐    ┌──────┐    ┌──────┐    ┌──────┐                        │
│  │repeat│ ─▶ │repeat│ ─▶ │repeat│ ─▶ │repeat│  Repeating             │
│  └──────┘    └──────┘    └──────┘    └──────┘    (0 learning)        │
│                                                                      │
│  WITH AGENTOPS                                                       │
│  ═════════════                                                       │
│                                                                      │
│  Session 1   Session 2   Session 3   Session 4                       │
│  ┌──────┐    ┌──────┐    ┌──────┐    ┌──────┐                        │
│  │learn │ ─▶ │recall│ ─▶ │refine│ ─▶ │expert│  Compounding           │
│  └──────┘    └──────┘    └──────┘    └──────┘    (mastered)          │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**By session 100, your agent knows:**
- Every bug you've ever fixed
- Your architecture decisions and *why*
- Your team's coding patterns
- What approaches failed and why

---

## Agent Farm

Spawn multiple agents to work on issues in parallel:

```bash
ao farm validate          # Pre-flight checks (cycles, bd availability)
ao farm start --agents 3  # Spawn 3 agents + witness in tmux
ao farm status            # Check progress
ao inbox                  # View messages from agents
ao farm stop              # Graceful shutdown
```

**How it works:**
1. Validates issues have no cycles and bd CLI is available
2. Spawns N Claude agents in tmux sessions (30s stagger for rate limits)
3. Witness monitors progress, sends summaries to mayor
4. Circuit breaker stops farm if >50% agents fail
5. Use `ao farm resume` to continue after interruption

---

## All 22 Skills

**You run 6 commands. The rest fire automatically.**

| Category | Skills | How They Run |
|----------|--------|--------------|
| **Core workflow** | `/research`, `/plan`, `/pre-mortem`, `/crank`, `/post-mortem` | You invoke |
| **Multi-agent** | `/farm` | You invoke |
| **Called by /crank** | `/implement`, `/vibe` | Auto |
| **Called by /post-mortem** | `/vibe`, `/retro` | Auto |
| **Issue tracking** | `/beads` | Library |
| **Language rules** | `/standards` | Library |
| **Deep dives** | `/bug-hunt`, `/complexity`, `/doc` | On demand |
| **Background** | `/forge`, `/extract`, `/inject`, `/knowledge`, `/provenance`, `/flywheel`, `/ratchet` | Hooks |

---

## CLI Reference

```bash
ao init                # Initialize AgentOps in repo
ao hooks install       # Install session hooks
ao inject [topic]      # Manually inject knowledge
ao forge search        # Search past sessions
ao forge index         # Index artifacts
ao feedback            # Mark learnings as helpful/harmful
ao metrics             # View flywheel health

# Agent Farm
ao farm validate       # Pre-flight checks
ao farm start --agents 3  # Spawn agents
ao farm status         # Check progress
ao farm stop           # Graceful shutdown
ao inbox               # View agent messages
ao mail send --to mayor --body "message"  # Send message
```

---

## Known Limitations (v1.1.0)

Agent Farm is production-ready for supervised use (1-3 agents). Large/unattended farms have known edge cases:

| Issue | Impact | Workaround |
|-------|--------|------------|
| **Race condition on issue claims** | Multiple agents may claim same issue | Use `--agents 3` or fewer for reliability |
| **Witness crash not auto-detected** | Farm hangs if witness dies | Run `ao farm status` periodically to check |
| **Graceful shutdown required** | Ctrl+C may leave orphaned sessions | Always use `ao farm stop` |
| **Project name spaces** | tmux session names break with spaces | Avoid spaces in directory names |

These will be addressed in the Mt-Olympus orchestrator project.

---

## The Science

Built on peer-reviewed research, not vibes.

| Concept | Source | Finding |
|---------|--------|---------|
| Knowledge Decay | [Darr, Argote & Epple (1995)](https://pubsonline.informs.org/doi/abs/10.1287/mnsc.41.11.1750) | Org knowledge depreciates ~17%/week |
| Memory Reinforcement | [Ebbinghaus (1885)](https://en.wikipedia.org/wiki/Forgetting_curve) | Retrieval strengthens memory, slows decay |
| MemRL | [Zhang et al. (2025)](https://arxiv.org/abs/2502.06173) | Two-phase retrieval enables self-evolving agents |

**The math:**
```
dK/dt = I(t) - δ·K + σ·ρ·K

Where:
  δ = 0.17/week (decay rate)
  σ = retrieval effectiveness
  ρ = usage rate

Goal: σ·ρ > δ  →  Knowledge compounds instead of decays
```

---

## Built On

| Tool | Author | What We Use |
|------|--------|-------------|
| [beads](https://github.com/steveyegge/beads) | Steve Yegge | Git-native issue tracking |
| [CASS](https://github.com/Dicklesworthstone/coding_agent_session_search) | Dicklesworthstone | Session indexing |
| [multiclaude](https://github.com/dlorenc/multiclaude) | dlorenc | Brownian Ratchet pattern |

---

## License

Apache-2.0

---

<div align="center">

### Stop starting from zero.

**Your agent's knowledge should compound, not reset.**

[Get Started](#quick-start) · [Documentation](docs/) · [Comparisons](docs/comparisons/) · [Changelog](CHANGELOG.md)

</div>
