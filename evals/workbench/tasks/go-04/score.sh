#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=3

# Check 1: go test ./... passes
if go test ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: ProcessBatch tests pass (the function still works)
if go test ./internal/calc/ -run 'TestProcessBatch' >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 3: No function has cyclomatic complexity > 10
# Heuristic: count if/for/switch/case/&& /|| tokens inside each function
# and flag any that exceed 10
# Use a Python script for reliable complexity counting
cc_ok=true
if ! python3 - "$WORKDIR/internal/calc/calc.go" << 'PYEOF'
import re, sys

src = open(sys.argv[1]).read()

# Extract function bodies
func_pattern = re.compile(r'^func\s+(\w+(?:\([^)]*\)\s+\w+)?)\s*\(', re.MULTILINE)
funcs = []
for m in func_pattern.finditer(src):
    name = m.group(0).strip()
    start = m.start()
    # Find matching closing brace
    brace_count = 0
    body_start = src.index('{', start)
    i = body_start
    while i < len(src):
        if src[i] == '{':
            brace_count += 1
        elif src[i] == '}':
            brace_count -= 1
            if brace_count == 0:
                funcs.append((name, src[body_start:i+1]))
                break
        i += 1

high = False
for name, body in funcs:
    # CC = 1 + count of decision points
    cc = 1
    cc += len(re.findall(r'\bif\b', body))
    cc += len(re.findall(r'\bfor\b', body))
    cc += len(re.findall(r'\bcase\b', body))
    cc += len(re.findall(r'&&', body))
    cc += len(re.findall(r'\|\|', body))
    cc += len(re.findall(r'\belse if\b', body))
    if cc > 10:
        high = True
        print(f"HIGH_CC: {name} cc={cc}", file=sys.stderr)

sys.exit(1 if high else 0)
PYEOF
then
  cc_ok=false
fi

if [ "$cc_ok" = "true" ]; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
