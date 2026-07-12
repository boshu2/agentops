#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

docs_check_validators() {
  local makefile="$1"
  awk '
    /^docs-check:/ { inside = 1; next }
    inside && /^\t/ { print; next }
    inside { exit }
  ' "$makefile" | while IFS= read -r recipe; do
    recipe="${recipe#"${recipe%%[![:space:]]*}"}"
    set -- $recipe
    case "${1:-}" in
      ./*) printf '%s\n' "${1#./}" ;;
      bash) [[ "${2:-}" == scripts/* || "${2:-}" == tests/* ]] && printf '%s\n' "$2" ;;
    esac
  done
}

validate_docs_check_target() {
  local makefile="$1" root="$2" validator failed=0
  while IFS= read -r validator; do
    [[ -n "$validator" ]] || continue
    if [[ ! -f "$root/$validator" || ! -x "$root/$validator" ]]; then
      echo "$validator is missing or not executable" >&2
      failed=1
    fi
  done < <(docs_check_validators "$makefile")
  [[ "$failed" -eq 0 ]]
}

@test "docs-check recipe validators exist and are executable" {
  run validate_docs_check_target "$REPO_ROOT/Makefile" "$REPO_ROOT"
  if [ "$status" -ne 0 ]; then
    echo "$output"
  fi
  [ "$status" -eq 0 ]
}

@test "an induced missing docs-check validator is rejected by exact path" {
  local fixture="$BATS_TEST_TMPDIR/Makefile"
  awk '{ print; if ($0 ~ /^docs-check:/) print "\t./scripts/missing-docs-validator.sh" }' \
    "$REPO_ROOT/Makefile" > "$fixture"

  run validate_docs_check_target "$fixture" "$REPO_ROOT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"scripts/missing-docs-validator.sh"* ]]
}
