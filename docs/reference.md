# AgentOps Reference

Deep documentation for AgentOps. For quick start, see [README](https://github.com/boshu2/agentops/blob/main/README.md).

---

## The Pipeline

| Stage | Skill | What It Does |
|-------|-------|--------------|
| **Shift-left** | `/pre-mortem` | Simulate failures BEFORE you write code |
| **Execute** | `/crank` | Orchestrate epic loop, dispatch `/swarm` for each wave |
| **Execute** | `/swarm` | Spawn fresh-context agents for parallel work |
| **Validate** | `/council` | Multi-model consensus (2-6 judges, cross-vendor, debate mode) |
| **Gate** | `/vibe` | Complexity analysis + council validation — must pass to merge |
| **Learn** | `/post-mortem` | Extract learnings to feed future sessions |
| **Release** | `/release` | Pre-flight, changelog, version bumps, tag — everything up to git tag |

---

## Execution Model

`/swarm`, `/crank`, and `/implement` use runtime-native backends for parallel execution:

| Property | How it works |
|----------|-------------|
| **Backends** | Auto-detected at runtime: `spawn_agent` (Codex) → `TeamCreate` (Claude) → `Task(run_in_background=true)` (fallback). For headless cross-runtime execution, `lib/scripts/team-runner.sh` now lets Codex launch Codex or Claude workers from the same team spec. |
| **Dependencies** | None (runtime-native) |
| **Context** | Fresh per agent (team-per-wave) |
| **Coordination** | `wait`/`SendMessage`/`TaskOutput` + `TaskList` |
| **Commits** | Lead-only (workers blocked by hook) |

The headless team backend preserves a shared worker artifact contract:

- team spec selects `runtime: codex` or `runtime: claude`
- both backends write the same `lib/schemas/worker-output.json` artifact
- Codex streams are watched by `lib/scripts/watch-codex-stream.sh`
- Claude streams are watched by `lib/scripts/watch-claude-stream.sh`

---

## Which Skill Should I Use?

| You Want | Use | Why |
|----------|-----|-----|
| Parallel tasks (fresh context each) | `/swarm` | Spawns agents, mayor owns the loop |
| Execute an entire epic | `/crank` | Orchestrates waves via `/swarm` until done |
| Single issue, full lifecycle | `/implement` | Claim → execute → validate → close |
| Gate progress without executing | `/ratchet` | Records/checks gates only |

---

## The `/vibe` Validator

Not just "does it compile?" — **does it match the spec?**

| Aspect | What It Checks |
|--------|----------------|
| Semantic | Does code do what spec says? |
| Security | SQL injection, auth bypass, hardcoded secrets |
| Quality | Dead code, copy-paste, magic numbers |
| Architecture | Layer violations, circular deps, god classes |
| Complexity | Cyclomatic > 10, deep nesting |
| Performance | N+1 queries, unbounded loops, resource leaks |
| Slop | AI hallucinations, cargo cult, over-engineering |
| Accessibility | Missing ARIA, broken keyboard nav, contrast |

**Gate rule:** 0 critical = pass. 1+ critical = blocked until fixed.

---

## Architecture

```
MAYOR (orchestrator)                AGENTS (executors)
--------------------                ------------------

/crank epic-123
  |
  +-> Get ready issues -----------> /swarm selects runtime backend per wave
  |                                   |
  +-> Create tasks ----------------> +-> Workers spawn as sub-agents/teammates
  |                                   |
  +-> Workers report completion <---- +-> Fresh context, execute atomically
  |     (via wait/message/output)     |
  +-> /vibe (validation gate)         +-> Return result via runtime channel
  |     |
  |     +-> PASS = progress locked (/ratchet)
  |     +-> FAIL = fix first
  |
  +-> Loop until DONE
  |
  +-> /post-mortem ----------------> .agents/learnings/
                                       |
NEXT SESSION                           |
------------                           |
auto-inject (hook) <-------------------+
  |
  +-> Starts with prior knowledge
```

### Full Workflow Stages

```
INPUT: SPEC (from superpowers, SDD, or your workflow)
  └── Plan, issues, acceptance criteria

STAGE 1: PRE-MORTEM [validation gate]
  /pre-mortem → Simulate failures BEFORE implementing

STAGE 2: EXECUTE [orchestrated + fresh context]
  /crank → Autonomous loop
    └── /swarm → Parallel agents (fresh context each)

STAGE 3: VALIDATE [validation gate]
  /vibe → 8-aspect check, must pass to commit

STAGE 4: LEARN [compounding memory]
  /post-mortem → Extract learnings for next session

OUTPUT: LEARNINGS (feed your next spec)
  └── .agents/learnings/, .agents/patterns/
```

### The Knowledge Flywheel

```
                    THE KNOWLEDGE FLYWHEEL

  SESSION START
  +------------------------------------+
  | auto-inject prior knowledge        |  <-- hook (automatic)
  +------------------+-----------------+
                     |
                     v
  +------------------------------------+
  | /pre-mortem   catch risks early    |  <-- you run this
  +------------------+-----------------+
                     |
                     v
  +------------------------------------+
  | /crank        parallel execution   |  <-- you run this
  |   +- /swarm   fresh-context agents |
  +------------------+-----------------+
                     |
                     v
  +------------------------------------+
  | /vibe         validation gate      |  <-- you run this
  +------------------+-----------------+
                     |
                     v
  +------------------------------------+
  | /post-mortem  extract learnings    |  <-- you run this
  +------------------+-----------------+
                     |
                     v
  SESSION END
  +------------------------------------+
  | auto-extract new learnings         |  <-- hook (automatic)
  +------------------+-----------------+
                     |
                     v
               .agents/  (local, git-ignored by default, compounds across sessions)
               |-- learnings/
               |-- patterns/
               |-- plans/
               +-- council/
                     |
                     +--------> next session starts here --------+
                                                                 |
                     +-------------------------------------------+
                     |
                     v
               (back to top)
```

For the science behind the flywheel, see [`knowledge-flywheel.md`](knowledge-flywheel.md) and [`the-science.md`](the-science.md).

---

## Installation Options

### Claude Code (Plugin + Marketplace, preferred)

```bash
# Add/update marketplace source
claude plugin marketplace add boshu2/agentops
claude plugin marketplace update agentops-marketplace

# Install/update plugin
claude plugin install agentops@agentops-marketplace
claude plugin update agentops
```

### Per-Runtime Install (non-Claude plugin path)

```bash
# Codex CLI (installs the native plugin, archives stale raw mirrors when needed, then open a fresh session)
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# OpenCode
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash

# Other agents (example: Cursor)
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)

# Update all
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

### CLI Install (enables hooks)

```bash
# macOS
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops

# Windows PowerShell
irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex

# Any OS with Go
go install github.com/boshu2/agentops/cli/cmd/ao@latest

# From your repo root: create `.agents/` + enable auto-hooks (Claude Code)
cd /path/to/your/repo
ao init --hooks
ao hooks test
```

> **Note:** There's a [known bug](https://github.com/anthropics/claude-code/issues/15178) where plugin skills don't appear when pressing `/`. Skills still work — just type them directly.

---

## Tool Dependencies

The `/vibe` skill runs complexity analysis (radon/gocyclo) then spawns a `/council` validation with the `code-review` preset (error-paths, api-surface, spec-compliance). External linters and scanners are used when available. **All tools are optional** — missing ones are skipped gracefully.

| Tool | Purpose | Install |
|------|---------|---------|
| **gitleaks** | Secret scanning | `brew install gitleaks` |
| **semgrep** | SAST security patterns | `brew install semgrep` |
| **trivy** | Dependency vulnerabilities | `brew install trivy` |
| **gosec** | Go security | `go install github.com/securego/gosec/v2/cmd/gosec@latest` |
| **hadolint** | Dockerfile linting | `brew install hadolint` |
| **ruff** | Python linting | `pip install ruff` |
| **radon** | Python complexity | `pip install radon` |
| **golangci-lint** | Go linting | `brew install golangci-lint` |
| **shellcheck** | Shell linting | `brew install shellcheck` |

**Quick install (recommended):**
```bash
brew install gitleaks semgrep trivy hadolint shellcheck golangci-lint
pip install ruff radon
```

More tools = more coverage. But even with zero tools installed, the workflow still runs.

---

## CLI Reference

The `ao` CLI handles knowledge persistence with MemRL two-phase retrieval, confidence decay (stale knowledge ages out), and citation-tracked provenance so you can trace learnings back to the session that produced them.

```bash
ao quick-start --minimal  # Create .agents/ structure (or use /quickstart skill)
ao init --hooks           # Recommended: create `.agents/` + install minimal hooks
ao hooks install          # Hooks only (does not create `.agents/`)
ao hooks test             # Verify hooks are working
ao inject [topic]         # Load prior knowledge (auto at session start)
ao search "query"         # Search workspace session history plus repo-local knowledge
ao flywheel status        # Knowledge growth rate, escape velocity
ao metrics report         # Flywheel health dashboard
ao forge transcript       # Extract learnings from session transcripts
ao ratchet status         # RPI progress gates (Discovery → Implementation → Validation)
ao pool list              # Show knowledge by quality tier
```

---

## All Skills

### Primary

| Skill | Purpose |
|-------|---------|
| `/pre-mortem` | Simulate failures before coding |
| `/crank` | Autonomous epic execution (orchestrator; runs waves via `/swarm`) |
| `/swarm` | Parallel agents with fresh context (runtime-native backends) |
| `/council` | Multi-model consensus (validate, research, brainstorm) |
| `/vibe` | Complexity + council validation gate |
| `/implement` | Single issue execution |
| `/post-mortem` | Extract learnings |
| `/research` | Deep codebase exploration |
| `/plan` | Break goal into tracked issues |
| `/release` | Pre-flight, changelog, version bumps, tag |
| `/ratchet` | Progress gates that lock (Discovery → Implementation → Validation) |
| `/beads` | Git-native issue tracking |

### Additional

| Skill | Purpose |
|-------|---------|
| `/retro` | Quick retrospective |
| `/inject` | Manually load prior knowledge |
| `/knowledge` | Query knowledge base |
| `/bug-hunt` | Root cause analysis |
| `/complexity` | Code complexity metrics |
| `/doc` | Documentation generation |
| `/standards` | Language-specific rules |

---

## Troubleshooting

- Plugin skills don't show up when you press `/` in Claude Code: type the skill directly (e.g. `/pre-mortem`). (See the [Claude Code issue](https://github.com/anthropics/claude-code/issues/15178).)
- `ao` not found: ensure it's on your `PATH` (`which ao`). For hook setup help, see `cli/docs/HOOKS.md`.
