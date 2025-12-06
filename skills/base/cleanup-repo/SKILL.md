# Skill: cleanup-repo

**Purpose:** Organize repository structure and move orphaned files

**Inputs:**
- `DRY_RUN` - Preview changes without applying (default: false)

**Outputs:**
- Exit 0: Cleanup successful
- Exit 1: Cleanup failed
- STDOUT: Cleanup report (markdown)

**Usage:**
```bash
make cleanup-repo
# OR (preview only)
make cleanup-repo DRY_RUN=true
```

**Dependencies:**
- Bash 4.0+
- grep, find, mv
- Git (for tracking moves)

**Invoked By:**
- operations-organize-repo agent
- Manual repository cleanup
