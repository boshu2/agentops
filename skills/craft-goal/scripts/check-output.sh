#!/usr/bin/env bash
set -euo pipefail

die() { printf 'craft-goal output: %s\n' "$*" >&2; exit 2; }
[[ $# -eq 1 ]] || die 'usage: check-output.sh OUTPUT_FILE'
output=$1
[[ -f "$output" && ! -L "$output" ]] || die 'output must be a regular non-symlink file'
bytes=$(wc -c <"$output" | tr -d ' ')
(( bytes > 0 && bytes <= 262144 )) || die 'output must be 1-262144 bytes'
first=$(head -n 1 "$output")
case "$first" in SAFE_TO_CREATE|USE_RPI|UNSAFE_GOAL) ;; *) die 'first line must be exactly SAFE_TO_CREATE, USE_RPI, or UNSAFE_GOAL' ;; esac

require_once() {
  local literal=$1 count
  count=$(grep -Fxc -- "$literal" "$output" || true)
  [[ "$count" -eq 1 ]] || die "expected exactly one line: $literal"
}
require_match() {
  local pattern=$1 label=$2
  grep -Eq -- "$pattern" "$output" || die "missing or invalid $label"
}

require_once '## Assumptions'
require_once '## Lint'
require_match '^- .+' 'assumption bullet'

lint_keys=(outcome evidence admission bead_graph rpi_boundary ratchet discovery wave_budget hard_budget breaker operator_andon scope self_hosting terminal_reports)
for key in "${lint_keys[@]}"; do
  count=$(grep -Ec -- "^- ${key}: (PASS|FAIL|N/A) - .+" "$output" || true)
  [[ "$count" -eq 1 ]] || die "lint dimension must appear exactly once with status and reason: $key"
done
lint_count=$(grep -Ec -- '^- [a-z_]+: (PASS|FAIL|N/A) - .+' "$output" || true)
[[ "$lint_count" -eq 14 ]] || die 'lint section must contain exactly the fourteen declared dimensions'

case "$first" in
  SAFE_TO_CREATE)
    require_once '## Goal prompt'
    require_once '## Goal-tool token budget'
    for heading in 'Goal outcome:' 'Terminal acceptance and evidence:' 'Non-goals and authority:' 'Bead graph:' 'Experiment policy:' 'Wave envelope:' 'Hard goal envelope:' 'Breaker and andon:' 'Wave checkpoint:' 'Terminal reports:'; do
      require_once "$heading"
    done
    require_match '^- RPIs: [1-9][0-9]*$' 'wave RPI budget'
    require_match '^- concurrency: [1-9][0-9]*$' 'wave concurrency budget'
    require_match '^- wall minutes: [1-9][0-9]*$' 'wave wall-time budget'
    require_match '^- tokens: [1-9][0-9]*$' 'token budget'
    require_match '^- live attempts: [1-9][0-9]*$' 'wave live-attempt budget'
    require_match '^- total RPIs: [1-9][0-9]*$' 'hard RPI budget'
    require_match '^- total wall minutes: [1-9][0-9]*$' 'hard wall-time budget'
    require_match '^- total tokens: [1-9][0-9]*$' 'hard token budget'
    require_match '^- total live attempts: [1-9][0-9]*$' 'hard live-attempt budget'
    require_match '^- compactions: [1-9][0-9]*$' 'hard compaction budget'
    require_match '^- changed paths: [1-9][0-9]*$' 'hard surface budget'
    require_match '^- no-ratchet threshold RPIs: [1-9][0-9]*$' 'breaker threshold'
    require_match 'exactly 1 bounded fresh helper' 'fresh-helper bound'
    require_match 'No artifact, repair, helper, subject, or wave resets a total\.' 'nonrenewing hard envelope'
    for terminal in ACHIEVED NOT_ACHIEVED NEEDS_OPERATOR; do require_match "^- ${terminal}: .+" "$terminal terminal report"; done
    if grep -Eq '<[^>]+>' "$output"; then die 'safe goal still contains an unfilled angle-bracket field'; fi
    if grep -Eq '^- [a-z_]+: (FAIL|N/A) - ' "$output"; then die 'SAFE_TO_CREATE requires all fourteen lint dimensions to pass'; fi
    ;;
  USE_RPI)
    require_once '## Rationale'
    [[ "$(grep -Fxc -- '## Goal prompt' "$output" || true)" -eq 0 ]] || die 'USE_RPI must not emit an outer-goal prompt'
    ;;
  UNSAFE_GOAL)
    require_once '## Missing decisions'
    [[ "$(grep -Fxc -- '## Goal prompt' "$output" || true)" -eq 0 ]] || die 'UNSAFE_GOAL must not emit an outer-goal prompt'
    missing=$(awk '/^## Missing decisions$/{inside=1;next} /^## /{inside=0} inside && /^- .+/{count++} END{print count+0}' "$output")
    (( missing >= 1 )) || die 'UNSAFE_GOAL must name at least one missing decision'
    ;;
esac

printf 'craft-goal output: PASS (%s)\n' "$first"
