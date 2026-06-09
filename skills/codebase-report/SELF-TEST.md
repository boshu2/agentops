# SELF-TEST: codebase-report

Run from the skill directory or a repo you want to analyze.

## 1) References present
```bash
test -f references/TEMPLATE.md
test -f references/EXAMPLES.md
```

## 2) Scaffold script smoke test
```bash
python3 scripts/scaffold-report.py /path/to/project | head -40
```

## 3) Manual path (quick scan)
```bash
cat README.md AGENTS.md 2>/dev/null | head -40
ls src/ lib/ cmd/ pkg/ 2>/dev/null
rg "fn main|func main|if __name__" --type-add 'all:*.*' -l | head -5
```

Expected:
- Script outputs a report skeleton with headers and tables.
- Manual scan surfaces entry points and key files to populate the report.
