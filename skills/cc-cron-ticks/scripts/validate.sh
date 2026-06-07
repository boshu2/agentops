#!/usr/bin/env bash
# validate.sh — self-check for the cc-cron-ticks skill (Form A, AUTHORING-STANDARD §6/§8).
# Checks: frontmatter completeness, line-start Triggers marker, required section spine,
# Form-A line budget (<=250), and that the spec sidecar exists + parses.
# Exit 0 on PASS, 1 on FAIL. No network, no side effects.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$DIR/SKILL.md"
SPEC="$DIR/skill.spec.json"
fail=0

err() { printf 'FAIL: %s\n' "$1" >&2; fail=1; }
ok()  { printf 'ok: %s\n' "$1"; }

# --- 0. files exist ---
[ -f "$SKILL" ] || { err "SKILL.md missing at $SKILL"; exit 1; }

# --- 1. frontmatter present + complete ---
if ! head -1 "$SKILL" | grep -qx -- '---'; then
  err "SKILL.md must start with YAML frontmatter '---'"
fi
fm_end=$(grep -n -- '^---$' "$SKILL" | sed -n '2p' | cut -d: -f1 || true)
[ -n "${fm_end:-}" ] || err "frontmatter has no closing '---'"
FM="$(sed -n "1,${fm_end:-1}p" "$SKILL")"

for key in 'name:' 'description:' 'skill_api_version:'; do
  printf '%s\n' "$FM" | grep -q "^$key" || err "frontmatter missing required key: $key"
done
printf '%s\n' "$FM" | grep -q '^  tier:' || err "frontmatter missing metadata.tier"

# name must equal directory name (lowercase-hyphen contract)
dirname_base="$(basename "$DIR")"
fm_name="$(printf '%s\n' "$FM" | sed -n 's/^name:[[:space:]]*//p' | head -1 | tr -d '[:space:]')"
[ "$fm_name" = "$dirname_base" ] || err "name ('$fm_name') != directory ('$dirname_base')"
ok "frontmatter keys present"

# --- 2. Triggers marker at start of its line in the description (FAIL severity, §0/§8) ---
# Inside a YAML 'description: |' block scalar the line carries the block indent,
# which YAML strips — so the rendered description has 'Triggers:' at line-start.
# Accept either bare line-start or the block-scalar-indented form.
if grep -qE '^[[:space:]]*Triggers:' "$SKILL"; then
  ok "line-start Triggers: marker present"
else
  err "no line-start 'Triggers:' marker in description (FAIL-severity per standard)"
fi

# --- 3. required section spine ---
require_section() {
  grep -qF "$1" "$SKILL" && ok "section: $1" || err "missing section: $1"
}
require_section '## ⚠️ Critical Constraints'
require_section '## Workflow / Methodology'
require_section '## Output Specification'
require_section '## Quality Rubric'
require_section '## Examples'
require_section '## Troubleshooting'
require_section '## See Also / References'

# Critical Constraints must appear within first 80 lines (§5)
cc_line=$(grep -n '## ⚠️ Critical Constraints' "$SKILL" | head -1 | cut -d: -f1 || echo 9999)
[ "${cc_line:-9999}" -le 80 ] || err "Critical Constraints at line $cc_line (must be within 80)"

# --- 4. Form-A line budget (<=250 hard) ---
lines=$(wc -l < "$SKILL" | tr -d '[:space:]')
if [ "$lines" -le 250 ]; then
  ok "line budget: $lines/250 (Form A)"
else
  err "line budget exceeded: $lines > 250 (Form A hard cap)"
fi

# --- 5. spec sidecar exists + parses ---
if [ -f "$SPEC" ]; then
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import json,sys; json.load(open('$SPEC'))" \
      && ok "skill.spec.json parses" || err "skill.spec.json is not valid JSON"
  else
    ok "skill.spec.json present (no python3 to parse-check)"
  fi
else
  err "skill.spec.json missing (required per §7)"
fi

# --- 6. no claude -p in tick guidance (project doctrine) ---
if grep -qE 'claude +-p|claude +--print' "$SKILL"; then
  grep -qE 'Never use .claude -p|never uses .claude -p|never use .claude -p' "$SKILL" \
    || err "mentions 'claude -p' without the prohibition framing"
fi

if [ "$fail" -eq 0 ]; then
  echo "PASS: cc-cron-ticks validates."
  exit 0
else
  echo "FAILED: see errors above." >&2
  exit 1
fi
