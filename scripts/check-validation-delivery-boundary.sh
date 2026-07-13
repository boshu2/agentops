#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURES="$ROOT/tests/fixtures/validation-delivery-boundary"

fail() {
  echo "validation-delivery boundary: FAIL: $*" >&2
  exit 1
}

lifecycle=(
  "$ROOT/skills/validate/SKILL.md"
  "$ROOT/skills/learn/SKILL.md"
  "$ROOT/skills/crank/SKILL.md"
  "$ROOT/skills/rpi/SKILL.md"
  "$ROOT/skills/evolve/SKILL.md"
)

for file in "${lifecycle[@]}"; do
  if grep -Eqi 'pawl-review|pawl-land|ao[[:space:]]+land|commit-bound[^.]*verdict|CONFIRMED[^.]*((git[[:space:]]+)?push|land)|no verdict means no push' "$file"; then
    fail "lifecycle source retains LLM landing authority: ${file#"$ROOT/"}"
  fi
done

grep -Fq 'another LLM landing verdict' "$ROOT/skills/validate/SKILL.md" ||
  fail 'Validate does not state the no-second-LLM boundary'
grep -Fq 'Do not operate proof, repository,' "$ROOT/skills/learn/SKILL.md" ||
  fail 'Learn does not reject repository/tracker/delivery authority'
grep -Fq 'Crank stops after it writes wave evidence' "$ROOT/skills/crank/SKILL.md" ||
  fail 'Crank does not stop at the implementation evidence boundary'
grep -Fq 'RPI ends at the four receipts and its report.' "$ROOT/skills/rpi/SKILL.md" ||
  fail 'RPI does not stop at its lifecycle report'
grep -Fq 'repository-selected deterministic `/push` adapter' "$ROOT/skills/evolve/SKILL.md" ||
  fail 'Evolve does not route optional delivery through Push'
grep -Fq 'Push cannot change the verdict, close tracker state, or complete the lifecycle' \
  "$ROOT/skills/evolve/SKILL.md" ||
  fail 'Evolve does not preserve caller-owned lifecycle and tracker authority'

# A top-level boundary is insufficient when the skill links reachable reference
# contracts. Scan every reference directly consumed by Crank for imperative
# landing/close machinery. Historical files may remain on disk, but they cannot
# stay connected to the active execution contract.
while IFS= read -r ref; do
  [[ -n "$ref" ]] || continue
  file="$ROOT/skills/crank/$ref"
  [[ -f "$file" ]] || fail "Crank links missing reference: $ref"
  if grep -Eqi 'bd[[:space:]]+close|br[[:space:]]+close|merge[[:space:]]+queue|push[[:space:]]+lands|pawl-review|pawl-land|ao[[:space:]]+land' "$file"; then
    fail "reachable Crank reference retains delivery/tracker authority: skills/crank/$ref"
  fi
done < <(grep -Eo 'references/[A-Za-z0-9._/-]+\.md' "$ROOT/skills/crank/SKILL.md" | sort -u)

grep -Fq 'Crank may report the tracker mutations that appear appropriate, but it does not' \
  "$ROOT/skills/crank/references/team-coordination.md" ||
  fail 'Crank team coordination does not preserve caller-owned tracker closeout'
grep -Fq 'it is not a repository integration loop or a tracker-closing loop' \
  "$ROOT/skills/crank/references/fire.md" ||
  fail 'Crank FIRE reference does not preserve the phase boundary'

if grep -Eqi 'pawl-review|pawl-land|ao[[:space:]]+land|commit-bound[^.]*verdict|no verdict means no push' "$ROOT/skills/push/SKILL.md"; then
  fail 'Push still requires LLM landing authority'
fi
bash "$ROOT/skills/push/scripts/validate.sh" >/dev/null

packet_value() {
  local file="$1" key="$2"
  awk -F= -v key="$key" '$1 == key {sub(/^[^=]*=/, ""); print; exit}' "$file"
}

check_packet() {
  local file="$1" role proof delivery git tracker verdict_mutation git_queue llm checks lifecycle_authority
  role="$(packet_value "$file" role)"
  proof="$(packet_value "$file" proof)"
  delivery="$(packet_value "$file" delivery)"
  git="$(packet_value "$file" git)"
  tracker="$(packet_value "$file" tracker)"
  verdict_mutation="$(packet_value "$file" verdict_mutation)"
  git_queue="$(packet_value "$file" git_queue)"
  llm="$(packet_value "$file" llm_verdict)"
  checks="$(packet_value "$file" checks)"
  lifecycle_authority="$(packet_value "$file" lifecycle_authority)"

  case "$role" in
    validate|learn|crank|rpi)
      [[ "$proof" == immutable ]] || return 1
      [[ "$delivery" == none || "$delivery" == caller-owned ]] || return 1
      [[ -z "$git" || "$git" == none ]] || return 1
      [[ -z "$tracker" || "$tracker" == none ]] || return 1
      [[ -z "$verdict_mutation" || "$verdict_mutation" == none ]] || return 1
      [[ -z "$git_queue" || "$git_queue" == none ]] || return 1
      [[ -z "$llm" || "$llm" == none ]] || return 1
      ;;
    push)
      [[ "$checks" == deterministic ]] || return 1
      [[ "$delivery" == repository-owned ]] || return 1
      [[ "$llm" == none ]] || return 1
      [[ "$lifecycle_authority" == false ]] || return 1
      ;;
    *) return 1 ;;
  esac
}

for fixture in "$FIXTURES"/positive/*.packet; do
  check_packet "$fixture" || fail "positive fixture rejected: ${fixture#"$ROOT/"}"
done

for fixture in "$FIXTURES"/negative/*.packet; do
  if check_packet "$fixture"; then
    fail "negative fixture accepted: ${fixture#"$ROOT/"}"
  fi
done

echo 'validation-delivery boundary: PASS'
