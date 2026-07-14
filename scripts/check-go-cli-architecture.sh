#!/usr/bin/env bash

# shellcheck source=scripts/lib/preamble.sh
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

check_retired_source_references() {
  local retired live path hit failures=0
  local -a retired_paths=(
    "cli/cmd/ao/doctor.go"
    "cli/cmd/ao/doctor_surface.go"
  )
  local -a live_surfaces=(
    "$REPO_ROOT/GOALS.md"
    "$REPO_ROOT/PROGRAM.md"
    "$REPO_ROOT/cli/internal/quality/AGENTS.md"
  )

  for live in "${live_surfaces[@]}"; do
    [[ -f "$live" ]] || continue
    for retired in "${retired_paths[@]}"; do
      if hit="$(grep -nF -- "$retired" "$live" 2>/dev/null)" && [[ -n "$hit" ]]; then
        path="${live#"$REPO_ROOT"/}"
        printf 'go-cli architecture FAIL: live authority %s references retired source %s\n%s\n' \
          "$path" "$retired" "$hit" >&2
        failures=$((failures + 1))
      fi
    done
  done

  [[ "$failures" -eq 0 ]]
}

check_retired_source_references || exit 1

candidate_sha="$(git -C "$REPO_ROOT" rev-parse HEAD^{commit})" || exit 1
"$REPO_ROOT/scripts/check-go-cli-semantic-seals.sh" --production --candidate-sha "$candidate_sha" || exit 1

args=(--root "$REPO_ROOT" --candidate-sha "$candidate_sha")
while [[ $# -gt 0 ]]; do
  case "$1" in
    --self-test)
      args+=(--self-test)
      shift
      ;;
    --family)
      [[ $# -ge 2 ]] || { echo "--family requires a value" >&2; exit 2; }
      args+=(--family "$2")
      shift 2
      ;;
    --all-migrated|--inventory)
      args+=("$1")
      shift
      ;;
    --out|--verify-scope)
      [[ $# -ge 2 ]] || { echo "$1 requires a value" >&2; exit 2; }
      args+=("$1" "$2")
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

cd "$REPO_ROOT/cli" || exit 1
exec go run ./internal/archcheck/cmd "${args[@]}"
