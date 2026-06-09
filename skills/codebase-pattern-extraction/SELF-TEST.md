# SELF-TEST: codebase-pattern-extraction

Run from the skill directory or any repo you want to analyze.

## 1) References present
```bash
test -f references/PATTERNS.md
test -f references/PACKAGING.md
test -f references/EXAMPLES.md
```

## 2) Pattern discovery smoke test
```bash
# Find repeated struct shapes (Rust example)
rg "struct \w+ \{" /data/projects/jeffreys-skills.md/cli/src -n | head -10

# Find repeated imports (signal shared patterns)
rg "^use " /data/projects/jeffreys-skills.md/cli/src/*.rs | head -10
```

## 3) Optional CASS check (if indexed)
```bash
# cass search "pattern_keyword" --robot --limit 5
```

Expected:
- References exist.
- rg commands return representative results or empty output (both OK).
