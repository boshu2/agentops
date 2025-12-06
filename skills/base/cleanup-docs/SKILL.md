# Skill: cleanup-docs

**Purpose:** Move misplaced documentation and fix broken links

**Inputs:**
- `DRY_RUN` - Preview changes without applying (default: false)

**Outputs:**
- Exit 0: Cleanup successful
- Exit 1: Cleanup failed
- STDOUT: Cleanup report (markdown)

**Usage:**
```bash
make cleanup-docs
# OR (preview only)
make cleanup-docs DRY_RUN=true
```

**Dependencies:**
- Bash 4.0+
- grep, find, mv
- Diátaxis framework knowledge

**Invoked By:**
- Manual documentation cleanup
- CI/CD post-validation
