#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

failures=0

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

# ag-2vz5v: resolve skill paths through the dispositions ledger so folds/cuts
# auto-retarget. Identity fallback keeps hermetic test copies self-contained.
if [[ -f "$REPO_ROOT/scripts/lib/resolve-skill-path.sh" ]]; then
  # shellcheck source=lib/resolve-skill-path.sh
  source "$REPO_ROOT/scripts/lib/resolve-skill-path.sh"
else
  resolve_skill_path() { printf '%s\n' "$1"; }
fi

require_contains() {
  local file
  file="$(resolve_skill_path "$1")"
  [[ -n "$file" ]] || return 0 # cut slug: resolver warned; skip visibly
  local needle="$2"
  local message="$3"
  if ! grep -Fq -- "$needle" "$file"; then
    fail "$message
  missing: $needle
  file: $file"
  fi
}

require_not_contains() {
  local file
  file="$(resolve_skill_path "$1")"
  [[ -n "$file" ]] || return 0 # cut slug: resolver warned; skip visibly
  local needle="$2"
  local message="$3"
  if grep -Fq -- "$needle" "$file"; then
    fail "$message
  unexpected: $needle
  file: $file"
  fi
}

require_regex() {
  local file
  file="$(resolve_skill_path "$1")"
  [[ -n "$file" ]] || return 0 # cut slug: resolver warned; skip visibly
  local pattern="$2"
  local message="$3"
  if ! grep -Eq -- "$pattern" "$file"; then
    fail "$message
  missing pattern: $pattern
  file: $file"
  fi
}

require_not_regex() {
  local file
  file="$(resolve_skill_path "$1")"
  [[ -n "$file" ]] || return 0 # cut slug: resolver warned; skip visibly
  local pattern="$2"
  local message="$3"
  if grep -Eq -- "$pattern" "$file"; then
    fail "$message
  unexpected pattern: $pattern
  file: $file"
  fi
}

echo "=== Codex lifecycle guard validation ==="

# age-focus-membrane-bookkeeper-m1wg.18: the entry/closeout/tracker twins split
# into SPINE (source carries `spine: true` — codex-sync actively regenerates the
# parity ones; bespoke ones are hand-tended) and FROZEN AMBIENT (non-spine —
# their twins are no longer restained from source, and a later twin-deletion bead
# may remove some). Spine twins keep the HARD lifecycle-marker requirement;
# frozen twins are validated WHILE PRESENT but EXEMPT from the existence
# requirement (assert-if-present via resolved_exists), so this gate stays green
# both now and after ambient twins are frozen/removed.
#
# resolved_exists <repo-rel-path> — true iff the path resolves (dispositions
# ledger) to a file that exists. A cut slug resolves to nothing; a removed frozen
# twin resolves to a missing file — both skip.
resolved_exists() {
  local resolved
  resolved="$(resolve_skill_path "$1")"
  [[ -n "$resolved" && -f "$resolved" ]]
}

# Spine entry/closeout/tracker twins (discovery/implement/status/handoff): HARD
# requirement — the lifecycle guard must always hold on the actively-maintained
# spine.
spine_entry_files=(
  "skills-codex/discovery/SKILL.md"
  "skills-codex/implement/SKILL.md"
  "skills-codex/status/SKILL.md"
  "skills-codex/discovery/prompt.md"
  "skills-codex/implement/prompt.md"
  "skills-codex/status/prompt.md"
)

# Frozen ambient entry twins (research/recover/crank/rpi): assert-if-present.
frozen_entry_files=(
  "skills-codex/research/SKILL.md"
  "skills-codex/recover/SKILL.md"
  "skills-codex/crank/SKILL.md"
  "skills-codex/rpi/SKILL.md"
  "skills-codex/research/prompt.md"
  "skills-codex/recover/prompt.md"
  "skills-codex/crank/prompt.md"
  "skills-codex/rpi/prompt.md"
)

spine_closeout_files=(
  "skills-codex/handoff/SKILL.md"
  "skills-codex/handoff/prompt.md"
)

spine_tracker_guidance_files=(
  "skills-codex/status/SKILL.md"
  "skills-codex/implement/SKILL.md"
  "skills-codex/handoff/SKILL.md"
  "skills-codex-overrides/catalog.json"
)

# Frozen ambient tracker-guidance twins: assert-if-present. Postmortem is not
# listed: it became causal analysis under the four-umbrella split and must not
# own closeout or tracker execution (age-tpeel).
frozen_tracker_guidance_files=(
  "skills-codex/recover/SKILL.md"
  "skills-codex/crank/SKILL.md"
  "skills-codex/rpi/prompt.md"
)

check_entry() {
  local file="$1"
  require_contains "$file" 'ao codex ensure-start' "entry skill must use ao codex ensure-start"
  require_not_contains "$file" 'ao codex start 2>/dev/null || true' "entry skill must not hand-roll ao codex start guards"
  require_not_contains "$file" '.agents/ao/codex/state.json' "entry skill must not parse Codex lifecycle state directly"
}

check_closeout() {
  local file="$1"
  require_contains "$file" 'ao codex ensure-stop' "closeout skill must use ao codex ensure-stop"
  require_not_contains "$file" 'ao codex stop --auto-extract' "closeout skill must not call ao codex stop directly"
}

check_postmortem_boundary() {
  local file="$1"
  require_contains "$file" 'retrospective causal analysis' "postmortem must remain causal analysis"
  require_contains "$file" 'not a completion gate' "postmortem must not regain completion-gate authority"
  require_not_contains "$file" 'ao session close --auto-extract' "postmortem must not close sessions"
  require_not_contains "$file" 'ao flywheel close-loop --quiet' "postmortem must not own flywheel closeout"
  require_not_contains "$file" 'ao codex ensure-stop' "postmortem must not own Codex lifecycle closeout"
  require_not_regex "$file" '\bbr (close|update)\b' "postmortem must not operate tracker state"
}

for file in "${spine_entry_files[@]}"; do
  check_entry "$file"
done
for file in "${frozen_entry_files[@]}"; do
  resolved_exists "$file" || continue  # frozen ambient twin removed — exempt
  check_entry "$file"
done

for file in "${spine_closeout_files[@]}"; do
  check_closeout "$file"
done
check_postmortem_boundary "skills-codex/postmortem/SKILL.md"

# quickstart folded into status, using-agentops into inject (ag-s43tg, 2026-06-12);
# the ensure-start/stop lifecycle assertions live on status (the Codex entry/closeout doc).
require_contains "skills-codex/status/SKILL.md" 'ao codex ensure-start' "status (absorbs quickstart) should describe ensure-start for Codex entry skills"
require_contains "skills-codex/status/SKILL.md" 'ao codex ensure-stop' "status (absorbs quickstart) should describe ensure-stop for Codex closeout skills"
require_contains "skills-codex-overrides/catalog.json" 'ao codex ensure-start' "Codex override catalog should reference ensure-start"
require_contains "skills-codex-overrides/catalog.json" 'ao codex ensure-stop' "Codex override catalog should reference ensure-stop"

check_tracker_guidance() {
  local file="$1"
  require_regex "$file" '\bbr\b' "Codex tracker guidance should point at br/beads_rust"
  require_not_regex "$file" '(^|[^[:alnum:]_-])`?bd([`[:space:]]|$)|BD_' "Codex tracker guidance must not default to legacy bd/Dolt"
}

for file in "${spine_tracker_guidance_files[@]}"; do
  check_tracker_guidance "$file"
done
for file in "${frozen_tracker_guidance_files[@]}"; do
  resolved_exists "$file" || continue  # frozen ambient twin removed — exempt
  check_tracker_guidance "$file"
done

if [[ $failures -ne 0 ]]; then
  echo "Codex lifecycle guard validation failed with $failures issue(s)." >&2
  exit 1
fi

echo "Codex lifecycle guard validation passed."
