#!/usr/bin/env bash
# land-lane-flaky-retry.sh — classify a failed land-lane Go gate as flake or deterministic.
#
# The land lane calls this only after the gate has already failed and its output
# has been captured. This helper extracts the failing Go package(s), reruns only
# those packages under -race, and files a quarantine record when retry proves the
# failure was flaky.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/land-lane-flaky-retry.sh parse <gate-log>
  scripts/land-lane-flaky-retry.sh retry <gate-log> <bead-id>

Environment:
  LAND_LANE_FLAKY_RETRY_MAX    retry attempts per package (default 2)
  LAND_LANE_QUARANTINE_FILE    JSONL record path (default .agents/land-queue/quarantine.jsonl)
  BR_BIN                       tracker binary for quarantine bead filing (default br in PATH)
  BEADS_DIR                    forwarded to br when set by the caller
EOF
}

die() {
  echo "land-lane-flaky-retry: ERROR: $*" >&2
  exit 2
}

json_escape() {
  local s="${1:-}"
  s="${s//\\/\\\\}"; s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"; s="${s//$'\r'/\\r}"; s="${s//$'\t'/\\t}"
  printf '%s' "$s"
}

extract_seed() {
  local log_file="$1"
  awk '
    match($0, /-test\.shuffle[[:space:]]+[^"\\[:space:]]+/) {
      seed = substr($0, RSTART, RLENGTH)
      sub(/^-test\.shuffle[[:space:]]+/, "", seed)
    }
    match(tolower($0), /shuffle[ _-]?seed[[:space:]:=]+[0-9]+/) {
      seed = substr($0, RSTART, RLENGTH)
      sub(/^.*[[:space:]:=]/, "", seed)
    }
    END { if (seed != "") print seed; else print "unknown" }
  ' "$log_file"
}

extract_failed_packages() {
  local log_file="$1"
  awk '
    /^[[:space:]]*FAIL[[:space:]]+[^[:space:]]+/ {
      pkg = $2
      if (pkg != "" && pkg != "FAIL") seen[pkg] = 1
    }
    END {
      for (pkg in seen) print pkg
    }
  ' "$log_file" | sort -u
}

parse_log() {
  local log_file="$1" seed pkg
  [[ -f "$log_file" ]] || die "missing gate log: $log_file"
  seed="$(extract_seed "$log_file")"
  while IFS= read -r pkg; do
    [[ -n "$pkg" ]] || continue
    printf '%s\t%s\n' "$pkg" "$seed"
  done < <(extract_failed_packages "$log_file")
}

go_mod_files() {
  find . -path './.git' -prune -o -name go.mod -type f -print | sort
}

resolve_package_target() {
  local pkg="$1" mod_file mod_dir module suffix

  if [[ "$pkg" == ./* || "$pkg" == "." ]]; then
    printf '.\t%s\n' "$pkg"
    return 0
  fi

  while IFS= read -r mod_file; do
    mod_dir="$(dirname "$mod_file")"
    module="$(awk '$1 == "module" { print $2; exit }' "$mod_file")"
    [[ -n "$module" ]] || continue

    if [[ "$pkg" == "$module" ]]; then
      printf '%s\t.\n' "$mod_dir"
      return 0
    fi
    if [[ "$pkg" == "$module/"* ]]; then
      suffix="${pkg#"$module/"}"
      printf '%s\t./%s\n' "$mod_dir" "$suffix"
      return 0
    fi
  done < <(go_mod_files)

  if [[ -d "$pkg" ]]; then
    printf '.\t./%s\n' "${pkg#./}"
    return 0
  fi

  return 1
}

rerun_package() {
  local pkg="$1" max="$2" module_dir pattern attempt target
  if ! target="$(resolve_package_target "$pkg")"; then
    echo "land-lane-flaky-retry: could not resolve failed package: $pkg" >&2
    return 1
  fi
  module_dir="$(printf '%s' "$target" | cut -f1)"
  pattern="$(printf '%s' "$target" | cut -f2)"

  attempt=1
  while [[ "$attempt" -le "$max" ]]; do
    echo "land-lane-flaky-retry: retry $attempt/$max: (cd $module_dir && go test -race -count=1 $pattern)" >&2
    if (cd "$module_dir" && go test -race -count=1 "$pattern"); then
      echo "land-lane-flaky-retry: retry passed for $pkg" >&2
      return 0
    fi
    attempt=$((attempt + 1))
  done

  echo "land-lane-flaky-retry: retry still failing for $pkg after $max attempt(s)" >&2
  return 1
}

append_quarantine_record() {
  local file="$1" bead="$2" pkg="$3" seed="$4" log_file="$5" tracker_id="${6:-}"
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  mkdir -p "$(dirname "$file")"
  printf '{"timestamp":"%s","bead":"%s","package":"%s","shuffle_seed":"%s","gate_log":"%s","tracker_id":"%s","status":"quarantine-filed"}\n' \
    "$(json_escape "$ts")" "$(json_escape "$bead")" "$(json_escape "$pkg")" \
    "$(json_escape "$seed")" "$(json_escape "$log_file")" "$(json_escape "$tracker_id")" \
    >>"$file"
}

file_quarantine() {
  local bead="$1" pkg="$2" seed="$3" log_file="$4"
  local title body br_bin tracker_id="" qfile
  title="Quarantine flaky Go race test in $pkg"
  body="Land-lane flaky retry classified a gate failure as a flake.

Source bead: $bead
Failed package: $pkg
Shuffle seed: $seed
Gate log: $log_file

Action: keep this as tracked quarantine work; do not park executable tests under tests/_quarantine/."

  br_bin="${BR_BIN:-$(command -v br 2>/dev/null || true)}"
  if [[ -n "$br_bin" ]]; then
    if tracker_id="$("$br_bin" create "$title" -t bug -p 2 \
        -l quarantine,flake,land-lane \
        --body "$body" --silent 2>/dev/null)"; then
      echo "land-lane-flaky-retry: quarantine bead filed: $tracker_id ($pkg seed=$seed)" >&2
    else
      tracker_id=""
      echo "land-lane-flaky-retry: WARN: could not file quarantine bead via $br_bin" >&2
    fi
  else
    echo "land-lane-flaky-retry: WARN: br not found; writing quarantine record only" >&2
  fi

  qfile="${LAND_LANE_QUARANTINE_FILE:-.agents/land-queue/quarantine.jsonl}"
  append_quarantine_record "$qfile" "$bead" "$pkg" "$seed" "$log_file" "$tracker_id"
}

retry_failed_packages() {
  local log_file="$1" bead="$2" max seed pkgs pkg failed=0
  [[ -f "$log_file" ]] || die "missing gate log: $log_file"
  [[ -n "$bead" ]] || die "missing bead id"

  max="${LAND_LANE_FLAKY_RETRY_MAX:-2}"
  [[ "$max" =~ ^[0-9]+$ ]] || die "LAND_LANE_FLAKY_RETRY_MAX must be numeric"
  [[ "$max" -gt 0 ]] || die "LAND_LANE_FLAKY_RETRY_MAX must be > 0"

  seed="$(extract_seed "$log_file")"
  mapfile -t pkgs < <(extract_failed_packages "$log_file")
  if [[ "${#pkgs[@]}" -eq 0 ]]; then
    echo "land-lane-flaky-retry: no failing Go package found in gate log (seed=$seed)" >&2
    return 1
  fi

  for pkg in "${pkgs[@]}"; do
    if ! rerun_package "$pkg" "$max"; then
      failed=1
    fi
  done

  if [[ "$failed" -ne 0 ]]; then
    echo "land-lane-flaky-retry: deterministic gate failure: package(s)=${pkgs[*]} seed=$seed" >&2
    return 1
  fi

  for pkg in "${pkgs[@]}"; do
    file_quarantine "$bead" "$pkg" "$seed" "$log_file"
  done
  echo "land-lane-flaky-retry: FLAKE package(s)=${pkgs[*]} seed=$seed" >&2
  return 0
}

main() {
  local mode="${1:-}"
  case "$mode" in
    parse)
      [[ $# -eq 2 ]] || die "parse requires <gate-log>"
      parse_log "$2"
      ;;
    retry)
      [[ $# -eq 3 ]] || die "retry requires <gate-log> <bead-id>"
      retry_failed_packages "$2" "$3"
      ;;
    -h|--help|"")
      usage
      ;;
    *)
      die "unknown mode: $mode"
      ;;
  esac
}

main "$@"
