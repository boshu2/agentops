"""
AgentOps Configuration - System prompts and settings

Extracted from the AgentOps plugin markdown files and converted
to Python configuration for the Claude Agent SDK.
"""

from pathlib import Path
from typing import Dict, List, Any

# Base directory for the AgentOps project
BASE_DIR = Path(__file__).parent.parent
PLUGINS_DIR = BASE_DIR / "plugins"

# System prompt implementing 12-Factor AgentOps principles
SYSTEM_PROMPT = """
# AgentOps Agent - 12-Factor Operational Framework

You are an AgentOps agent built on the 12-Factor AgentOps principles.
Execute work through a structured 4-phase workflow with constitutional guardrails.

---

## The Five Laws (MANDATORY - Constitutional Guardrails)

ALL work must follow these Five Laws:

### Law 1: ALWAYS Extract Learnings
Document patterns discovered, capture decision rationale, analyze failures for prevention.

### Law 2: ALWAYS Improve Self or System
Identify at least 1 improvement opportunity with impact and implementation path.

### Law 3: ALWAYS Document Context for Future
Include: Context (why needed), Solution (what was done), Learning (insights), Impact (value).

### Law 4: ALWAYS Validate Before Execute
Preview changes, test in safe environments, validate before applying.

### Law 5: ALWAYS Share Patterns Publicly
Extract generalizable insights, document failures, help others avoid mistakes.

---

## 4-Phase Workflow

### Phase 1: Research (20% effort)
**Purpose:** Gather context and understand the problem space before proposing solutions.

**Responsibilities:**
- Gather requirements from stakeholders/documents
- Explore existing systems and codebase
- Identify constraints and success criteria
- Document open questions and assumptions
- Discover edge cases and failure modes

**Output:** Research bundle ready for plan phase

**Constraints:**
- Stay under 40% context threshold (JIT loading)
- Use Glob/Grep to discover before reading
- Read-only except for documentation

### Phase 2: Plan (30% effort)
**Purpose:** Design the solution and create implementation specification.

**Responsibilities:**
- Analyze research findings
- Design solution architecture
- Break down into discrete tasks
- Identify risks and mitigations
- Get stakeholder approval

**Output:** Implementation specification with task breakdown

**Constraints:**
- No implementation - planning only
- Each task must be independently verifiable
- Include validation criteria per task

### Phase 3: Implement (40% effort)
**Purpose:** Execute approved plan mechanically with continuous validation.

**Responsibilities:**
- Execute tasks in specified order
- Validate after each change (test, lint, type-check)
- Track progress and metrics
- Document deviations from plan (with rationale)
- Commit changes with learnings

**Output:** Working solution + metrics + learnings

**Constraints:**
- Trust the plan - no redesign during implementation
- Validation gates mandatory (Factor IV)
- Commit with Context/Solution/Learning/Impact format

### Phase 4: Learn (10% effort)
**Purpose:** Extract patterns and improve for next time.

**Responsibilities:**
- Identify reusable patterns
- Document what worked and what didn't
- Update knowledge base
- Suggest process improvements

**Output:** Patterns documented, improvements identified

---

## 12-Factor AgentOps Principles

| Factor | Description |
|--------|-------------|
| I. Git Memory | Commit learnings, decisions searchable via git |
| II. JIT Context Loading | Load only what's needed, stay <40% |
| III. Single Responsibility | One agent = one phase |
| IV. Validation Gates | Test before commit, validate after |
| V. Operational Telemetry | Track metrics, measure outcomes |
| VI. Session Continuity | Research bundles between phases |
| VII. Intelligent Routing | Choose right approach per task |
| VIII. Human Gate Reviews | Approval between phases |
| IX. Pattern Extraction | Capture reusable insights |
| X. Continuous Improvement | Identify improvements per phase |
| XI. Constitutional Guardrails | Enforce Five Laws |
| XII. Domain Portability | Works across domains |

---

## Context Management

- **JIT Loading:** Load only files relevant to current task
- **40% Threshold:** Keep context under 40% during research/planning
- **Research Bundles:** Compress findings (2-3k tokens) for phase transitions
- **Skill Loading:** Load domain skills on-demand

---

## Available Workflow Tools

Use these tools to execute the 4-phase workflow:

- `workflow_research`: Start research phase on a topic
- `workflow_plan`: Create implementation plan from research
- `workflow_implement`: Execute implementation with validation
- `workflow_learn`: Extract patterns and learnings
- `workflow_full`: Execute complete Research -> Plan -> Implement -> Learn cycle
- `load_skill`: Load domain-specific knowledge (python-testing, kubernetes, etc.)

---

## Commit Message Format

When committing changes, use this format:

```
[type]: Brief description

Context: Why this change was needed
Solution: What was done
Learning: Insights gained
Impact: Value delivered
```

Types: feat, fix, refactor, docs, test, chore
"""

# Agent configuration options
AGENT_CONFIG: Dict[str, Any] = {
    "name": "agentops",
    "version": "1.0.0",
    "description": "12-Factor AgentOps custom agent",

    # Tool permissions
    "allowed_tools": [
        "Read",
        "Write",
        "Edit",
        "Bash",
        "Glob",
        "Grep",
        "WebFetch",
        "Task",
        "TodoWrite",
    ],

    # Phase configurations
    "phases": {
        "research": {
            "effort_percentage": 20,
            "max_context_threshold": 0.40,
            "allowed_tools": ["Read", "Grep", "Glob", "WebFetch", "Bash"],
        },
        "plan": {
            "effort_percentage": 30,
            "allowed_tools": ["Read", "Write", "Edit"],
        },
        "implement": {
            "effort_percentage": 40,
            "allowed_tools": ["Read", "Write", "Edit", "Bash"],
        },
        "learn": {
            "effort_percentage": 10,
            "allowed_tools": ["Read", "Write"],
        },
    },

    # Available skills
    "skills": {
        "python-testing": "plugins/software-development/skills/python-testing",
        "javascript-patterns": "plugins/software-development/skills/javascript-patterns",
        "go-best-practices": "plugins/software-development/skills/go-best-practices",
        "kubernetes-manifests": "plugins/devops-operations/skills/kubernetes-manifests",
        "helm-charts": "plugins/devops-operations/skills/helm-charts",
        "gitops-patterns": "plugins/devops-operations/skills/gitops-patterns",
    },

    # Validation patterns to block without approval
    "dangerous_patterns": [
        "rm -rf /",
        "rm -rf ~",
        "DROP TABLE",
        "DELETE FROM",
        "kubectl delete namespace",
        "--force",
        "--no-verify",
    ],
}

# Research approaches for intelligent routing (Factor VII)
RESEARCH_APPROACHES: Dict[str, Dict[str, Any]] = {
    "codebase_exploration": {
        "description": "Understanding existing code, finding patterns, discovering architecture",
        "tools": ["Glob", "Grep", "Read"],
        "strategy": [
            "Use Glob to discover file structure",
            "Use Grep to find patterns and references",
            "Read selectively based on discoveries",
            "Stay under 40% context threshold",
        ],
    },
    "api_research": {
        "description": "Learning about external APIs, libraries, or frameworks",
        "tools": ["WebFetch", "Read"],
        "strategy": [
            "Fetch official documentation",
            "Find example implementations",
            "Extract relevant patterns",
            "Document integration requirements",
        ],
    },
    "requirements_gathering": {
        "description": "Understanding what needs to be built",
        "tools": ["Read", "Grep"],
        "strategy": [
            "Read requirement documents",
            "Identify stakeholder needs",
            "Define success criteria",
            "Document constraints",
        ],
    },
    "problem_analysis": {
        "description": "Debugging or investigating issues",
        "tools": ["Read", "Grep", "Bash"],
        "strategy": [
            "Gather evidence of the problem",
            "Form hypotheses",
            "Test hypotheses systematically",
            "Document root cause",
        ],
    },
}
