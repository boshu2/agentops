#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Backticks and command substitutions are exact Markdown markers, never shell execution.
# shellcheck disable=SC2016
MARKERS=(
  '## Constraints'
  '- **No verdict means no push.** Require a CONFIRMED commit-bound pawl verdict because green producer tests cannot independently authorize release.'
  '- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED repairs and reruns automatically because ordinary rejection is useful evidence, not a breaker; only a breaker may enter HOLD or consume one helper.'
  '## Breaker State Machine'
  '- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair, recommit, and rerun gate plus pawl; plain rejection never enters HOLD and never consumes the helper lane.'
  '- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** pause landing in HOLD and route exactly one bounded helper consultation.'
  '- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, resume repair, and re-earn gate plus independent verdict before landing.'
  '- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and surface the helper'\''s escalation to the human operator.'
  '- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** stop automation and route directly to the human operator with the helper skipped.'
  '**Checkpoint:** confirm HEAD cites the bead, the fast gate passed on that exact HEAD, and the CONFIRMED verdict is commit-bound before invoking the land command. After landing, run the landed verifier so a canonical provenance-bind tip is separated from its reviewed feature parent without trusting either a marker or path alone.'
  '## Output Specification'
  '- **Path:** remote `origin` ref `refs/heads/main`, with proof at `.agents/pawl-verdicts/<bead>.json`.'
  '- **Filename convention:** `<bead>.json` for the pawl verdict bound to the reviewed commit contained by the pushed ref.'
  '- **Serialization/schema format:** landed-tip and reviewed-commit Git SHAs plus the JSON pawl-verdict schema bound to the reviewed SHA.'
  '- **Validator command:** set `BEAD="<bead-id>"`, then run `bash skills-codex/push/scripts/verify-landed.sh "$BEAD"`; do not replace this with inline `HEAD`/`HEAD^` guessing.'
  '- **Downstream handoff:** send both verifier-reported `tip=<sha>` and `reviewed=<sha>`, verdict path, suites run, and remote-ref proof to closeout; close the tracker only after this helper succeeds.'
  '## Quality Checklist'
  '- The commit subject cites the bead, the diff contains only owned files, and sensitive/private paths are absent from the index.'
  '- Deterministic tests and `ao gate check --fast --scope head` pass on the exact commit reviewed by the pawl.'
  '- The verdict is CONFIRMED, independent, cross-family where required, and bound to the verifier-resolved reviewed SHA contained by the landed tip.'
  '- The landed verifier proves `HEAD == origin/main` and reports both tip and reviewed SHAs; ordinary rejection stays in AUTO-REDO and only the breaker state machine reaches HOLD or HUMAN.'
)

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "push contract validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
}

# Verify SKILL.md exists and has frontmatter
[[ -f "$SKILL_DIR/SKILL.md" ]] || { echo "FAIL: missing SKILL.md"; exit 1; }
[[ -x "$SKILL_DIR/scripts/verify-landed.sh" ]] || { echo "FAIL: missing executable landed verifier"; exit 1; }
[[ -x "$SKILL_DIR/scripts/test-verify-landed.sh" ]] || { echo "FAIL: missing executable landed verifier fixtures"; exit 1; }
head -1 "$SKILL_DIR/SKILL.md" | grep -q "^---$" || { echo "FAIL: missing frontmatter"; exit 1; }
validate_contract "$SKILL"
delete_one_negative_fixture
bash "$SKILL_DIR/scripts/test-verify-landed.sh" >/dev/null
echo "OK: $(basename "$SKILL_DIR")"
