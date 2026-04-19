# Role-Based Profiles

**Purpose**: Organize 12 domain skills into 3 discoverable profiles for different work contexts.

**Version**: 4.0.0 (Updated for skills-based approach)

---

## How to Use Profiles

Profiles are **documentation groupings**, not executable configs. They organize AgentOps skills by domain so you can quickly find which skills are relevant to your work.

To use a profile:

1. **Find your domain** in the table below
2. **Open the profile YAML** (e.g., `docs/profiles/roles/software-dev.yaml`) to see which skills it groups together
3. **Use those skills directly** via slash commands (`/research`, `/plan`, `/implement`) or by reading the SKILL.md files listed in the profile

You do not "load" or "activate" a profile — you read it to discover which skills apply to your workflow, then invoke those skills individually.

---

## Quick Start

| You're doing... | Profile | Key Skills |
|-----------------|---------|------------|
| Building apps (APIs, frontends, features) | **software-dev** | languages, development, code-quality |
| Operations (incidents, monitoring, deploys) | **platform-ops** | operations, monitoring, security |
| Writing (docs, research, patterns) | **content-creation** | documentation, research, meta |

---

## The 3 Profiles

### 1. Software Development (`software-dev`)

**What you do**:
- Build applications (backend APIs, frontends, full-stack)
- Write code in Python, Go, Rust, TypeScript, Java
- Review code and generate tests
- Deploy with CI/CD pipelines

**Skills included**:

| Skill | Knowledge Areas |
|-------|-----------------|
| **languages** | Python, Go, Rust, Java, TypeScript, Shell (6) |
| **development** | Backend, frontend, fullstack, mobile, iOS, deployment, AI, prompts (8) |
| **code-quality** | Review, improve, test generation (3) |
| **validation** | Assumptions, continuous, planning, tracer bullets (4) |
| **data** | Engineering, science, ML, MLOps (4) |

**Example workflow**:
```
1. /research "API design for user auth"
2. Load languages skill for Python patterns
3. Load development skill for backend architecture
4. Load code-quality skill before commit
```

---

### 2. Platform Operations (`platform-ops`)

**What you do**:
- Respond to production incidents
- Monitor system health and performance
- Debug errors and analyze logs
- Manage deployments and infrastructure

**Skills included**:

| Skill | Knowledge Areas |
|-------|-----------------|
| **operations** | Incident response, triage, postmortems, error detection (4) |
| **monitoring** | Alerts/runbooks, performance engineering (2) |
| **security** | Penetration testing, network engineering (2) |
| **validation** | Assumptions, tracer bullets (4) |
| **meta** | Context, execution, autonomous work (6) |

**Example workflow**:
```
1. Alert fires → Load operations skill
2. Check metrics → Load monitoring skill
3. After resolution → Load meta skill for postmortem
```

---

### 3. Content Creation (`content-creation`)

**What you do**:
- Write documentation (technical and non-technical)
- Extract patterns from completed work
- Conduct research and analysis
- Create tutorials and examples

**Skills included**:

| Skill | Knowledge Areas |
|-------|-----------------|
| **documentation** | Create, optimize, Diátaxis audit, API docs (4) |
| **research** | Code, docs, history, archive, structure, specs (6) |
| **meta** | Context, observer, memory, retro analysis (6) |
| **specialized** | Accessibility, support, UX (6) |

**Example workflow**:
```
1. /research "authentication patterns in our codebase"
2. Load research skill for exploration
3. Load documentation skill for writing
4. Load meta skill for retrospective
```

---

## Skills by Profile

| Skill | software-dev | platform-ops | content-creation |
|-------|:------------:|:------------:|:----------------:|
| languages | ✅ | | |
| development | ✅ | | |
| documentation | | | ✅ |
| code-quality | ✅ | | |
| research | | | ✅ |
| validation | ✅ | ✅ | |
| operations | | ✅ | |
| monitoring | | ✅ | |
| security | | ✅ | |
| data | ✅ | | |
| meta | | ✅ | ✅ |
| specialized | | | ✅ |

---

## Loading Skills

Skills load into main context with full tool access:

```bash
# Read a skill directly
Read skills/languages/SKILL.md

# Or let triggers auto-load
"I need help with Python async patterns"
# → languages skill auto-activates
```

---

## Profile Files

```
profiles/
├── README.md                    # This file
├── roles/
│   ├── software-dev.yaml        # Software development profile
│   ├── platform-ops.yaml        # Platform operations profile
│   └── content-creation.yaml    # Content creation profile
└── examples/
    ├── software-dev-session.md  # Example dev session
    ├── platform-ops-session.md  # Example ops session
    └── content-creation-session.md  # Example writing session
```
