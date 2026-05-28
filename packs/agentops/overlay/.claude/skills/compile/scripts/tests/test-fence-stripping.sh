#!/usr/bin/env bash
# test-fence-stripping.sh — empirically verify that the splitter in
# compile.sh skips markdown code fences (```markdown / ```) emitted by
# some LLM runtimes (notably `claude -p`).
#
# Standalone: does NOT source compile.sh (it tops-out in side-effect code);
# instead it replicates the fence-skipping rule and asserts it matches
# every fence variant we've seen in real smoke runs.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPILE_SH="$SCRIPT_DIR/../compile.sh"

# 1. Static check: the splitter contains the fence-skipping clause.
if ! grep -q 'markdown.*code fences' "$COMPILE_SH"; then
  echo "FAIL: compile.sh splitter is missing fence-stripping clause" >&2
  exit 1
fi

# 2. Static check: system prompt tells the LLM NOT to emit fences.
if ! grep -q 'Do NOT wrap your output in markdown code fences' "$COMPILE_SH"; then
  echo "FAIL: compile.sh system prompt is missing fence-suppression instruction" >&2
  exit 1
fi

# 3. Behavioral check: the regex must match exactly the fence-lines we want
# to strip and nothing else. Mirror the splitter's regex.
fence_regex='^[[:space:]]*```(markdown)?[[:space:]]*$'

expect_match() {
  local label="$1" line="$2"
  if [[ "$line" =~ $fence_regex ]]; then
    return 0
  fi
  echo "FAIL: expected match for $label: [$line]" >&2
  exit 1
}

expect_nomatch() {
  local label="$1" line="$2"
  if [[ "$line" =~ $fence_regex ]]; then
    echo "FAIL: expected NO match for $label: [$line]" >&2
    exit 1
  fi
}

expect_match "opening-markdown-fence" '```markdown'
expect_match "bare-closing-fence"      '```'
expect_match "indented-opening-fence"  '  ```markdown'
expect_match "trailing-space-fence"    '```   '
# Must NOT match legitimate in-article fenced blocks with languages we care about.
expect_nomatch "python-fence"    '```python'
expect_nomatch "bash-fence"      '```bash'
expect_nomatch "inline-backticks"    'the `foo` command'
expect_nomatch "prose-with-backticks" 'run ```bash then continue'

echo "OK: fence-stripping invariants hold."
