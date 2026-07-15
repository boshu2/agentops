#!/usr/bin/env bash
# Validate scenario-to-test links in one caller-supplied behavior artifact.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN=0
JSON=0
SOURCE=""

usage() {
  echo "Usage: check-scenario-coverage.sh [--run] [--json] <feature-or-markdown|->"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --run) RUN=1; shift ;;
    --json) JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -) SOURCE="-"; shift ;;
    --*) echo "unknown flag: $1" >&2; exit 2 ;;
    *)
      if [[ -n "$SOURCE" ]]; then
        echo "only one source may be supplied" >&2
        exit 2
      fi
      SOURCE="$1"
      shift
      ;;
  esac
done

if [[ -z "$SOURCE" ]]; then
  echo "no source given" >&2
  exit 2
fi

if [[ "$SOURCE" == "-" ]]; then
  RAW="$(cat)"
elif [[ -f "$SOURCE" ]]; then
  RAW="$(<"$SOURCE")"
else
  echo "source file does not exist: $SOURCE" >&2
  exit 2
fi

run_test() {
  local path="$1" name="$2" dir
  case "$path" in
    *_test.go)
      dir="$(dirname "$path")"
      if [[ -n "$name" ]]; then
        (cd "$dir" && go test -run "^${name}$" .)
      else
        (cd "$dir" && go test .)
      fi
      ;;
    *.bats)
      if [[ -n "$name" ]]; then bats -f "$name" "$path"; else bats "$path"; fi
      ;;
    *.sh) bash "$path" ;;
    *.py)
      if [[ -n "$name" ]]; then
        python3 -m pytest -q -k "$name" "$path"
      else
        python3 -m pytest -q "$path"
      fi
      ;;
    *)
      echo "no runner for linked test: ${path#$REPO_ROOT/}" >&2
      return 2
      ;;
  esac
}

declare -A TARGET_CACHE=()
validate_target() {
  local spec="$1" path name abs rc=0
  if [[ "$spec" == *"::"* ]]; then
    path="${spec%%::*}"
    name="${spec#*::}"
  else
    path="$spec"
    name=""
  fi
  [[ "$path" = /* ]] && abs="$path" || abs="$REPO_ROOT/$path"

  if [[ ! -f "$abs" ]]; then
    printf 'test path does not exist: %s' "$path"
    return 1
  fi
  if [[ -n "$name" ]] && ! grep -qF -- "$name" "$abs"; then
    printf 'test "%s" not found in %s' "$name" "$path"
    return 1
  fi
  if [[ $RUN -eq 1 ]]; then
    if [[ -n "${TARGET_CACHE[$spec]+x}" ]]; then
      rc="${TARGET_CACHE[$spec]}"
    else
      run_test "$abs" "$name" >/dev/null 2>&1 || rc=$?
      TARGET_CACHE[$spec]="$rc"
    fi
    if [[ "$rc" -ne 0 ]]; then
      printf 'covering test did not pass: %s (exit %s)' "$path" "$rc"
      return 1
    fi
  fi
  return 0
}

has_scenarios_heading=0
if printf '%s\n' "$RAW" | awk '
  /^[[:space:]]*```/ { fence = !fence; next }
  !fence && /^[[:space:]]*##[[:space:]]+Scenarios[[:space:]]*$/ { found = 1 }
  END { exit !found }
'; then
  has_scenarios_heading=1
fi

in_scenarios=$((1 - has_scenarios_heading))
in_fence=0
pending_tags=""
file_tags=""
scenarios_total=0
scenarios_covered=0
errors=()

while IFS= read -r line || [[ -n "$line" ]]; do
  trimmed="$(printf '%s' "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
  if [[ "$trimmed" == '```'* ]]; then
    in_fence=$((1 - in_fence))
    continue
  fi
  [[ $in_fence -eq 1 ]] && continue

  if [[ $has_scenarios_heading -eq 1 ]]; then
    if [[ "$trimmed" =~ ^##[[:space:]]+Scenarios[[:space:]]*$ ]]; then
      in_scenarios=1
      pending_tags=""
      continue
    fi
    if [[ $in_scenarios -eq 1 && "$trimmed" =~ ^##[[:space:]]+ ]]; then
      in_scenarios=0
      pending_tags=""
      continue
    fi
  fi

  if [[ "$trimmed" == @* ]]; then
    for token in $trimmed; do
      [[ "$token" == @covered-by:* ]] && pending_tags+="${token#@covered-by:} "
    done
    continue
  fi
  if [[ $has_scenarios_heading -eq 0 && "$trimmed" == Feature:* ]]; then
    file_tags="$pending_tags"
    pending_tags=""
    continue
  fi
  [[ $in_scenarios -eq 0 ]] && continue

  if [[ "$trimmed" == Scenario:* || "$trimmed" == "Scenario Outline:"* ]]; then
    scenarios_total=$((scenarios_total + 1))
    name="${trimmed#Scenario: }"
    name="${name#Scenario Outline: }"
    targets="$file_tags $pending_tags"
    pending_tags=""
    valid=0
    if [[ -z "${targets//[[:space:]]/}" ]]; then
      errors+=("scenario \"$name\": no covering test")
      continue
    fi
    for target in $targets; do
      if detail="$(validate_target "$target")"; then
        valid=$((valid + 1))
      else
        errors+=("scenario \"$name\": dangling @covered-by:$target — $detail")
      fi
    done
    if [[ $valid -gt 0 ]]; then scenarios_covered=$((scenarios_covered + 1)); fi
  fi
done <<< "$RAW"

result="pass"
if [[ $scenarios_total -eq 0 ]]; then
  result="fail"
  errors+=("no scenarios found")
elif [[ $scenarios_covered -ne $scenarios_total || ${#errors[@]} -gt 0 ]]; then
  result="fail"
fi

if [[ $JSON -eq 1 ]]; then
  printf '{"result":"%s","scenarios_total":%d,"scenarios_covered":%d,"errors":%d}\n' \
    "$result" "$scenarios_total" "$scenarios_covered" "${#errors[@]}"
else
  for error in "${errors[@]}"; do echo "ERROR: $error" >&2; done
  if [[ "$result" == "pass" ]]; then
    suffix=""
    [[ $RUN -eq 1 ]] && suffix=" and passing"
    echo "check-scenario-coverage: PASS (${scenarios_covered}/${scenarios_total} scenarios covered${suffix})"
  else
    echo "check-scenario-coverage: FAIL (${scenarios_covered}/${scenarios_total} scenarios covered)" >&2
  fi
fi

[[ "$result" == "pass" ]]
