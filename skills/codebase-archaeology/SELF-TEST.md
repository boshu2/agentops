# SELF-TEST: codebase-archaeology

Run from the skill directory or any repo you want to explore.

## 1) Reference files exist
```bash
test -f references/LANGUAGES.md
test -f references/PATTERNS.md
test -f references/EXAMPLES.md
```

## 2) Documentation-first check
```bash
cat AGENTS.md README.md | head -40
```

## 3) Orientation + entry points
```bash
ls -la src/ lib/ cmd/ pkg/ || true
rg "fn main|def main|function main|export default" .
```

## 4) Configuration + integrations
```bash
rg "process\.env|std::env|os\.environ" .
rg "reqwest|fetch\(|requests\.|sqlx|rusqlite|prisma" .
```

Expected:
- Commands run without errors (some repos may have no matches).
- You can produce a short architecture summary based on findings.
