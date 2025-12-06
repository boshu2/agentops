# Skill: audit-diataxis

**Purpose:** Audit documentation for Diátaxis framework compliance

**Inputs:**
- `DOCS_DIR` - Path to documentation directory (default: docs/)

**Outputs:**
- Exit 0: All docs properly placed
- Exit 1: Diátaxis violations found
- STDOUT: Audit report (markdown)

**Usage:**
```bash
make audit-diataxis
# OR
make audit-diataxis DOCS_DIR=/path/to/docs
```

**Dependencies:**
- Bash 4.0+
- grep, find
- Diátaxis framework knowledge

**Invoked By:**
- documentation-diataxis-auditor agent
- Documentation CI validation
- Manual doc reviews
