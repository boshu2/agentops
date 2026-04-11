#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
WAVE_FILTER=""

usage() {
  cat <<'EOF'
Usage: bash scripts/validate-codex-override-coverage.sh [--repo-root <path>] [--wave <wave-id>]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root)
      ROOT="${2:-}"
      shift 2
      ;;
    --wave)
      WAVE_FILTER="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$ROOT" != /* ]]; then
  ROOT="$(cd "$ROOT" && pwd)"
fi

SKILLS_DIR="$ROOT/skills"
OVERRIDES_DIR="$ROOT/skills-codex-overrides"
GENERATED_DIR="$ROOT/skills-codex"
CATALOG_PATH="$OVERRIDES_DIR/catalog.json"

failures=0

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

[[ -d "$SKILLS_DIR" ]] || {
  echo "Missing source skills root: $SKILLS_DIR" >&2
  exit 1
}
[[ -d "$OVERRIDES_DIR" ]] || {
  echo "Missing Codex overrides root: $OVERRIDES_DIR" >&2
  exit 1
}
[[ -d "$GENERATED_DIR" ]] || {
  echo "Missing generated Codex skills root: $GENERATED_DIR" >&2
  exit 1
}
[[ -f "$CATALOG_PATH" ]] || {
  echo "Missing Codex override catalog: $CATALOG_PATH" >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || {
  echo "jq is required for Codex override coverage validation." >&2
  exit 1
}

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

source_skills_file="$tmpdir/source-skills.txt"
manifest_skills_file="$tmpdir/manifest-skills.txt"
wave_ids_file="$tmpdir/wave-ids.txt"
selected_entries_file="$tmpdir/selected-entries.jsonl"
selected_skills_file="$tmpdir/selected-skills.txt"
selected_bespoke_file="$tmpdir/selected-bespoke.txt"
actual_override_dirs_file="$tmpdir/actual-override-dirs.txt"
expected_override_dirs_file="$tmpdir/expected-override-dirs.txt"

contains_fixed() {
  local needle="$1"
  local path="$2"
  grep -Fq -- "$needle" "$path"
}

contains_regex() {
  local pattern="$1"
  local path="$2"
  grep -Eq -- "$pattern" "$path"
}

strip_generated_operator_contract_block() {
  local prompt_path="$1"
  local first_section="$2"
  local stripped_tmp trimmed_tmp

  stripped_tmp="$(mktemp "$tmpdir/operator-contract-strip.XXXXXX")"
  trimmed_tmp="$(mktemp "$tmpdir/operator-contract-trim.XXXXXX")"

  awk '
    $0 == "<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->" { skip = 1; next }
    $0 == "<!-- END AGENTOPS OPERATOR CONTRACT -->" { skip = 0; next }
    !skip { print }
  ' "$prompt_path" > "$stripped_tmp"

  if grep -Fxq "$first_section" "$stripped_tmp"; then
    awk -v first_section="$first_section" '
      $0 == first_section { exit }
      { print }
    ' "$stripped_tmp" > "$trimmed_tmp"
    mv "$trimmed_tmp" "$stripped_tmp"
  fi

  awk '
    { lines[NR] = $0 }
    END {
      last = NR
      while (last > 0 && lines[last] == "") {
        last--
      }
      for (i = 1; i <= last; i++) {
        print lines[i]
      }
    }
  ' "$stripped_tmp" > "$prompt_path"

  rm -f "$stripped_tmp" "$trimmed_tmp"
}

render_operator_contract_block() {
  local entry="$1"
  local skill
  local marker_index=0
  local remaining_markers=0
  local section_count=0
  local section_index
  local sections=()
  local markers=()

  skill="$(jq -r '.name' <<<"$entry")"
  mapfile -t sections < <(jq -r '.operator_contract.required_sections[]' <<<"$entry")
  mapfile -t markers < <(jq -r '.operator_contract.required_markers[]' <<<"$entry")
  section_count="${#sections[@]}"
  remaining_markers="${#markers[@]}"

  printf '<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->\n'
  printf '<!-- Generated from skills-codex-overrides/catalog.json for %s. -->\n\n' "$skill"

  for ((section_index = 0; section_index < section_count; section_index++)); do
    local sections_left count bullet_index

    printf '%s\n\n' "${sections[$section_index]}"
    sections_left=$((section_count - section_index))

    if (( remaining_markers == 0 )); then
      count=0
    elif (( sections_left == 1 )); then
      count=$remaining_markers
    else
      count=$((remaining_markers - (sections_left - 1)))
      if (( count < 1 )); then
        count=1
      fi
    fi

    for ((bullet_index = 0; bullet_index < count; bullet_index++)); do
      printf '%d. %s\n' "$((bullet_index + 1))" "${markers[$marker_index]}"
      marker_index=$((marker_index + 1))
      remaining_markers=$((remaining_markers - 1))
    done

    if (( section_index < section_count - 1 )); then
      printf '\n'
    fi
  done

  printf '\n<!-- END AGENTOPS OPERATOR CONTRACT -->\n'
}

synthesize_expected_prompt() {
  local entry="$1"
  local override_prompt="$2"
  local expected_prompt="$3"
  local first_section rendered_tmp

  cp "$override_prompt" "$expected_prompt"
  first_section="$(jq -r '.operator_contract.required_sections[0]' <<<"$entry")"
  strip_generated_operator_contract_block "$expected_prompt" "$first_section"

  rendered_tmp="$(mktemp "$tmpdir/operator-contract-render.XXXXXX")"
  render_operator_contract_block "$entry" > "$rendered_tmp"

  if [[ -s "$expected_prompt" ]]; then
    printf '\n\n' >> "$expected_prompt"
  fi
  cat "$rendered_tmp" >> "$expected_prompt"
  rm -f "$rendered_tmp"
}

find "$SKILLS_DIR" -mindepth 1 -maxdepth 1 -type d \
  | while IFS= read -r d; do
      [[ -f "$d/SKILL.md" ]] || continue
      basename "$d"
    done \
  | LC_ALL=C sort -u > "$source_skills_file"

if ! jq -e '
  (.version | type) == "number" and
  (.waves | type) == "array" and
  (.skills | type) == "array" and
  all(.waves[]; (.id | type) == "string" and (.id | length) > 0 and (.description | type) == "string") and
  all(.skills[];
    (.name | type) == "string" and (.name | length) > 0 and
    (.treatment == "bespoke" or .treatment == "parity_only") and
    (.wave | type) == "string" and (.wave | length) > 0 and
    (.reason | type) == "string" and (.reason | length) > 0 and
    (
      (.operator_contract_required? | not) or
      ((.operator_contract_required | type) == "boolean")
    ) and
    (
      (.operator_contract? | not) or
      (
        (.operator_contract | type) == "object" and
        (.operator_contract.required_sections | type) == "array" and
        (.operator_contract.required_markers | type) == "array" and
        all(.operator_contract.required_sections[]; (type == "string") and (length > 0)) and
        all(.operator_contract.required_markers[]; (type == "string") and (length > 0))
      )
    )
  )
' "$CATALOG_PATH" >/dev/null; then
  echo "Invalid Codex override catalog schema: $CATALOG_PATH" >&2
  exit 1
fi

jq -r '.waves[].id' "$CATALOG_PATH" | LC_ALL=C sort > "$wave_ids_file"
jq -r '.skills[].name' "$CATALOG_PATH" | LC_ALL=C sort > "$manifest_skills_file"

duplicate_wave_ids="$(jq -r '.waves | group_by(.id)[] | select(length > 1) | .[0].id' "$CATALOG_PATH")"
duplicate_skill_ids="$(jq -r '.skills | group_by(.name)[] | select(length > 1) | .[0].name' "$CATALOG_PATH")"
unknown_wave_refs="$(jq -r '
  . as $root
  | ($root.waves | map(.id)) as $wave_ids
  | $root.skills[]
  | select((.wave as $wave | $wave_ids | index($wave)) == null)
  | .name
' "$CATALOG_PATH")"

if [[ -n "$duplicate_wave_ids" ]]; then
  while IFS= read -r wave_id; do
    [[ -n "$wave_id" ]] || continue
    fail "duplicate wave id in catalog: $wave_id"
  done <<<"$duplicate_wave_ids"
fi

if [[ -n "$duplicate_skill_ids" ]]; then
  while IFS= read -r skill_name; do
    [[ -n "$skill_name" ]] || continue
    fail "duplicate skill entry in catalog: $skill_name"
  done <<<"$duplicate_skill_ids"
fi

if [[ -n "$unknown_wave_refs" ]]; then
  while IFS= read -r skill_name; do
    [[ -n "$skill_name" ]] || continue
    fail "catalog skill references unknown wave: $skill_name"
  done <<<"$unknown_wave_refs"
fi

missing_from_catalog="$(comm -23 "$source_skills_file" "$manifest_skills_file" || true)"
extra_in_catalog="$(comm -13 "$source_skills_file" "$manifest_skills_file" || true)"

if [[ -n "$missing_from_catalog" ]]; then
  while IFS= read -r skill_name; do
    [[ -n "$skill_name" ]] || continue
    fail "source skill missing from Codex catalog: $skill_name"
  done <<<"$missing_from_catalog"
fi

if [[ -n "$extra_in_catalog" ]]; then
  while IFS= read -r skill_name; do
    [[ -n "$skill_name" ]] || continue
    fail "catalog contains unknown skill: $skill_name"
  done <<<"$extra_in_catalog"
fi

if [[ -n "$WAVE_FILTER" ]]; then
  if ! grep -Fxq "$WAVE_FILTER" "$wave_ids_file"; then
    echo "Unknown wave id in Codex override catalog: $WAVE_FILTER" >&2
    exit 1
  fi
  jq -c --arg wave "$WAVE_FILTER" '
    .skills[]
    | select(.wave == $wave)
    | {
        name,
        treatment,
        wave,
        reason,
        operator_contract_required: (.operator_contract_required // false),
        operator_contract: (.operator_contract // null)
      }
  ' "$CATALOG_PATH" > "$selected_entries_file"
else
  jq -c '
    .skills[]
    | {
        name,
        treatment,
        wave,
        reason,
        operator_contract_required: (.operator_contract_required // false),
        operator_contract: (.operator_contract // null)
      }
  ' "$CATALOG_PATH" > "$selected_entries_file"
fi

jq -r '.name' "$selected_entries_file" | LC_ALL=C sort -u > "$selected_skills_file"
jq -r 'select(.treatment == "bespoke") | .name' "$selected_entries_file" | LC_ALL=C sort -u > "$selected_bespoke_file"

selected_count="$(wc -l < "$selected_skills_file" | tr -d ' ')"
if [[ "$selected_count" == "0" ]]; then
  fail "selected catalog scope is empty"
fi

while IFS= read -r entry; do
  [[ -n "$entry" ]] || continue
  skill="$(jq -r '.name' <<<"$entry")"
  treatment="$(jq -r '.treatment' <<<"$entry")"
  [[ -n "$skill" ]] || continue
  generated_prompt="$GENERATED_DIR/$skill/prompt.md"
  override_dir="$OVERRIDES_DIR/$skill"
  override_prompt="$override_dir/prompt.md"

  [[ -f "$generated_prompt" ]] || fail "missing generated Codex prompt for $skill"

  case "$treatment" in
    bespoke)
      [[ -f "$override_prompt" ]] || fail "missing Codex override prompt for bespoke skill $skill"
      if jq -e '.operator_contract_required == true' <<<"$entry" >/dev/null; then
        if ! jq -e '.operator_contract != null' <<<"$entry" >/dev/null; then
          fail "catalog missing operator_contract for required Codex operator-contract skill: $skill"
        fi
      fi
      if jq -e '.operator_contract_required == true and .operator_contract != null' <<<"$entry" >/dev/null; then
        expected_prompt="$(mktemp "$tmpdir/operator-contract-expected.XXXXXX")"
        synthesize_expected_prompt "$entry" "$override_prompt" "$expected_prompt"
        if ! cmp -s "$expected_prompt" "$generated_prompt"; then
          fail "checked-in Codex prompt for $skill does not match synthesized override + catalog contract output; update skills-codex/$skill/prompt.md or the override inputs"
        fi
        rm -f "$expected_prompt"
      else
        if [[ -f "$override_prompt" ]] && ! contains_regex '^## Codex Execution Profile$' "$override_prompt"; then
          fail "override prompt for $skill lacks '## Codex Execution Profile'"
        fi
        if [[ -f "$override_prompt" && -f "$generated_prompt" ]] && ! cmp -s "$override_prompt" "$generated_prompt"; then
          fail "checked-in Codex prompt for $skill does not match override source; update skills-codex/$skill/prompt.md or the override prompt"
        fi
      fi
      ;;
    parity_only)
      if jq -e '.operator_contract_required == true' <<<"$entry" >/dev/null; then
        fail "parity-only skill cannot require operator-contract governance: $skill"
      fi
      if [[ -d "$override_dir" ]]; then
        fail "parity-only skill has unexpected Codex override directory: $skill"
      fi
      ;;
    *)
      fail "skill has unsupported treatment '$treatment': $skill"
      ;;
  esac
done < "$selected_entries_file"

find "$OVERRIDES_DIR" -mindepth 1 -maxdepth 1 -type d \
  | while IFS= read -r d; do
      basename "$d"
    done \
  | LC_ALL=C sort -u > "$actual_override_dirs_file"

cp "$selected_bespoke_file" "$expected_override_dirs_file"

if [[ -n "$WAVE_FILTER" ]]; then
  grep -Fx -f "$selected_skills_file" "$actual_override_dirs_file" > "$tmpdir/actual-selected-override-dirs.txt" || true
  mv "$tmpdir/actual-selected-override-dirs.txt" "$actual_override_dirs_file"
fi

unexpected_override_dirs="$(comm -13 "$expected_override_dirs_file" "$actual_override_dirs_file" || true)"
missing_override_dirs="$(comm -23 "$expected_override_dirs_file" "$actual_override_dirs_file" || true)"

if [[ -n "$unexpected_override_dirs" ]]; then
  while IFS= read -r skill_name; do
    [[ -n "$skill_name" ]] || continue
    fail "override directory exists but catalog does not mark it bespoke in selected scope: $skill_name"
  done <<<"$unexpected_override_dirs"
fi

if [[ -n "$missing_override_dirs" ]]; then
  while IFS= read -r skill_name; do
    [[ -n "$skill_name" ]] || continue
    fail "catalog marks skill bespoke but no override directory exists in selected scope: $skill_name"
  done <<<"$missing_override_dirs"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "Codex override coverage validation FAILED ($failures finding(s))." >&2
  exit 1
fi

if [[ -n "$WAVE_FILTER" ]]; then
  echo "Codex override coverage validation passed for $selected_count skill(s) in wave '$WAVE_FILTER'."
else
  echo "Codex override coverage validation passed for $selected_count cataloged skill(s)."
fi
