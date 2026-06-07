#!/usr/bin/env bash
# collect-metrics.sh — gather the Phase-3 metrics for a codebase report.
# Execute from the repo being reported on: bash scripts/collect-metrics.sh [path]
# Prints a Markdown-ready metrics block. Any metric whose tooling is absent is
# reported as "not measured" rather than estimated (per the skill's constraints).
set -euo pipefail

ROOT="${1:-.}"
cd "$ROOT"

echo "### Metrics"
echo

# Commit / activity (git only)
if git rev-parse --git-dir >/dev/null 2>&1; then
  echo "- Commit (SHA): $(git rev-parse --short HEAD)"
  echo "- Last change: $(git log -1 --format='%cd' --date=short)"
  echo "- Total commits: $(git rev-list --count HEAD)"
  echo "- Contributors: $(git log --format='%ae' | sort -u | wc -l | tr -d ' ')"
else
  echo "- Git activity: not measured (not a git repo)"
fi

# Size: file + line counts by extension (top 8 langs)
echo "- Tracked files: $(git ls-files 2>/dev/null | wc -l | tr -d ' ' || echo 'not measured')"
echo "- Lines by extension (top 8):"
git ls-files 2>/dev/null \
  | sed -n 's/.*\.\([A-Za-z0-9]\{1,8\}\)$/\1/p' \
  | sort | uniq -c | sort -rn | head -8 \
  | awk '{printf "    - .%s: %s files\n", $2, $1}' \
  || echo "    - not measured"

# Test signal
tests="$(git ls-files 2>/dev/null | grep -ciE '(_test\.|\.test\.|/tests?/|spec\.)' || true)"
echo "- Test-related files: ${tests:-not measured}"

# Tooling signals
for f in Makefile .github/workflows .gitlab-ci.yml Dockerfile; do
  [[ -e "$f" ]] && echo "- Tooling: $f present"
done
