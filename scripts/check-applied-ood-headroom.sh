#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SCENARIO_DIR="evals/scenarios/applied-ood"
OUT_DIR=".agents/evals/applied-ood-headroom/latest"
AO_BIN="${AO_BIN:-$ROOT/cli/bin/ao}"

usage() {
  cat <<'USAGE'
Usage: scripts/check-applied-ood-headroom.sh [--scenario-dir DIR] [--out-dir DIR] [--ao-bin PATH]

Run the applied-OOD campaign admission preflight. Each scenario is executed with:
  ao eval scenario-ab --control-only

This is intentionally CI/campaign-only model work; do not wire it into fast pre-push.
USAGE
}

die() {
  echo "FAIL applied-OOD headroom: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scenario-dir)
      [[ -n "${2:-}" ]] || die "--scenario-dir requires a directory"
      SCENARIO_DIR="$2"
      shift 2
      ;;
    --out-dir)
      [[ -n "${2:-}" ]] || die "--out-dir requires a directory"
      OUT_DIR="$2"
      shift 2
      ;;
    --ao-bin)
      [[ -n "${2:-}" ]] || die "--ao-bin requires a path"
      AO_BIN="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

if [[ ! -d "$SCENARIO_DIR" ]]; then
  die "scenario directory not found: $SCENARIO_DIR"
fi

tmp_dir=""
cleanup() {
  if [[ -n "$tmp_dir" ]]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

if [[ ! -x "$AO_BIN" ]]; then
  if [[ -x "$ROOT/cli/bin/ao" ]]; then
    AO_BIN="$ROOT/cli/bin/ao"
  else
    tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/agentops-applied-ood-headroom.XXXXXX")"
    AO_BIN="$tmp_dir/ao"
    (cd cli && env -u AGENTOPS_RPI_RUNTIME go build -o "$AO_BIN" ./cmd/ao)
  fi
fi

mkdir -p "$OUT_DIR"

shopt -s nullglob
scenarios=("$SCENARIO_DIR"/*.json)
if [[ "${#scenarios[@]}" -eq 0 ]]; then
  die "no applied-OOD scenarios found under $SCENARIO_DIR"
fi

failures=0
for scenario_path in "${scenarios[@]}"; do
  scenario_id="$(basename "$scenario_path" .json)"
  scorecard_path="$OUT_DIR/$scenario_id.scorecard.json"
  echo "applied-OOD headroom: $scenario_path"
  if "$AO_BIN" eval scenario-ab --control-only --scenario "$scenario_path" --output "$scorecard_path"; then
    echo "  PASS headroom scorecard: $scorecard_path"
  else
    echo "  FAIL headroom preflight: $scenario_path" >&2
    failures=$((failures + 1))
  fi
done

if [[ "$failures" -gt 0 ]]; then
  die "$failures scenario(s) failed control-only headroom; block campaign/live A/B spend"
fi

echo "PASS applied-OOD headroom: ${#scenarios[@]} scenario(s) cleared control-only preflight; scorecards=$OUT_DIR"
