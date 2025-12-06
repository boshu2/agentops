# Skill: audit-workflow

**Purpose:** Audit multi-agent workflow outputs for constraint violations

**Inputs:**
- `WORKFLOW_DIR` - Path to workflow outputs (default: tmp/agent-phases/)

**Outputs:**
- Exit 0: No violations found
- Exit 1: Violations detected
- STDOUT: Audit report (markdown)

**Usage:**
```bash
make audit-workflow
# OR
make audit-workflow WORKFLOW_DIR=/path/to/outputs
```

**Dependencies:**
- Bash 4.0+
- grep, awk, find
- Git (for checking modifications)
- Repository constraints in CLAUDE.md

**Invoked By:**
- meta-workflow-auditor agent
- CI/CD validation
- Manual operator audits
