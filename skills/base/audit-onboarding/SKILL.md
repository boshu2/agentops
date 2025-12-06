# Skill: audit-onboarding

**Purpose:** Audit Day 1 developer onboarding experience

**Inputs:**
- `DOCS_DIR` - Documentation directory (default: docs)

**Outputs:**
- Exit 0: Onboarding experience complete
- Exit 1: Onboarding gaps detected
- STDOUT: Audit report (markdown)

**Usage:**
```bash
make audit-onboarding
```

**Dependencies:**
- Bash 4.0+
- grep
- Essential repository files

**Checks Performed:**
- P0 Critical: Essential files (README.md, CLAUDE.md, Makefile)
- P1 High: Onboarding documentation exists
- P1 High: Essential Makefile targets (help, quick, validate)
- P1 High: Bootstrap capability
- P2 Medium: README.md quality

**Invoked By:**
- testing-onboarding-audit agent
- CI/CD validation
- Weekly preventive maintenance
