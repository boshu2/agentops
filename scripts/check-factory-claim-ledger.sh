#!/usr/bin/env bash
# Validate the AgentOps factory claim ledger and marker discipline.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LEDGER="docs/contracts/factory-claim-ledger.example.json"
STRICT=0
FIXTURE_ROOT=""
RUN_FIXTURES=1

usage() {
  cat <<'EOF'
Usage: scripts/check-factory-claim-ledger.sh [--strict] [--ledger PATH] [--fixture DIR] [--no-fixtures]

Validates factory claim markers against the machine-readable claim ledger.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --strict)
      STRICT=1
      shift
      ;;
    --ledger)
      LEDGER="${2:?missing ledger path}"
      shift 2
      ;;
    --fixture)
      FIXTURE_ROOT="${2:?missing fixture dir}"
      RUN_FIXTURES=0
      shift 2
      ;;
    --no-fixtures)
      RUN_FIXTURES=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 2
  }
}

require_cmd jq
require_cmd python3

abs_path() {
  local base="$1"
  local path="$2"
  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
  else
    printf '%s/%s\n' "$base" "$path"
  fi
}

collect_repo_sources() {
  local root="$1"
  shopt -s nullglob globstar
  local sources=(
    "README.md"
    "PRODUCT.md"
    "GOALS.md"
    "docs/index.md"
    "docs/agentops-brief.md"
    "docs/assurance-profile.md"
    "docs/software-factory.md"
    "docs/trust-factory.md"
    "docs/wiki-for-agents.md"
  )

  local f
  for f in "$root"/docs/comparisons/**/*.md "$root"/docs/comparisons/*.md; do
    [[ -f "$f" ]] && sources+=("${f#"$root"/}")
  done
  for f in "$root"/docs/contracts/factory-*.md; do
    [[ -f "$f" ]] && sources+=("${f#"$root"/}")
  done

  printf '%s\n' "${sources[@]}" | awk 'NF && !seen[$0]++'
}

collect_fixture_sources() {
  local root="$1"
  find "$root" -type f -name '*.md' -printf '%P\n' | sort
}

validate_ledger_shape() {
  local ledger_abs="$1"

  jq -e '
    .schema_version == "factory-claim-ledger/v1"
    and (.ledger_id | type == "string" and length > 0)
    and (.updated_at | type == "string" and length > 0)
    and (.claims | type == "array" and length > 0)
  ' "$ledger_abs" >/dev/null || return 1

  jq -e '
    all(.claims[];
      (.claim_id | type == "string" and length > 0)
      and (.claim_text | type == "string" and length > 0)
      and (.source.file | type == "string" and length > 0)
      and (.source.marker | type == "string" and length > 0)
      and (.current_evidence | type == "string" and length > 0)
      and (.missing_proof | type == "string" and length > 0)
      and (.owner_issue | type == "string" and length > 0)
      and (.closure_gate | type == "string" and length > 0)
      and (.anti_overclaim_wording | type == "string" and length > 0)
      and (.evidence_artifacts | type == "array")
    )
  ' "$ledger_abs" >/dev/null || return 1

  jq -e '
    all(.claims[];
      (.validation_level as $v | ["L0","L1","L2","L3"] | index($v) != null)
      and (.release_posture as $v | ["roadmap","contracted_l0","locally_checked_l1","integrated_l2","pilot_observed_l3","advisory_gate","blocking_gate","release_gate"] | index($v) != null)
      and (.evidence_status as $v | ["none","planned","partial","present","stale","blocked"] | index($v) != null)
      and (.authority_state as $v | ["agentops_owned","operator_owned","external_authority_required","not_claimed"] | index($v) != null)
      and (.promotion_state as $v | ["not_promoted","eligible","promoted","demoted","blocked"] | index($v) != null)
    )
  ' "$ledger_abs" >/dev/null || return 1

  jq -e '
    all(.claims[];
      if (.validation_level == "L2" or .validation_level == "L3") then
        (.evidence_artifacts | type == "array" and length > 0 and all(.[]; (.path | type == "string" and length > 0)))
      else
        true
      end
    )
  ' "$ledger_abs" >/dev/null || return 1

  local duplicates
  duplicates="$(jq -r '.claims[].claim_id' "$ledger_abs" | sort | uniq -d)"
  if [[ -n "$duplicates" ]]; then
    echo "duplicate claim_id(s):" >&2
    echo "$duplicates" >&2
    return 1
  fi
}

validate_sources_and_markers() {
  local root="$1"
  local ledger_abs="$2"
  shift 2
  local sources=("$@")

  local src missing=0
  for src in "${sources[@]}"; do
    if [[ ! -f "$root/$src" ]]; then
      echo "missing claim source: $src" >&2
      missing=1
    fi
  done
  [[ "$missing" -eq 0 ]] || return 1

  python3 - "$root" "$ledger_abs" "${sources[@]}" <<'PY'
import json
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
ledger_path = pathlib.Path(sys.argv[2])
sources = sys.argv[3:]

ledger = json.loads(ledger_path.read_text())
ledger_ids = {claim["claim_id"] for claim in ledger["claims"]}
marker_to_file = {claim["source"]["marker"]: claim["source"]["file"] for claim in ledger["claims"]}

term_re = re.compile(r"\b(validated|factory-grade|improves|throughput|high-assurance|autonomous)\b", re.IGNORECASE)
marker_re = re.compile(r"agentops:claim:([A-Za-z0-9_.-]+)")

def strip_fenced_blocks(text):
    lines = []
    in_fence = False
    for line in text.splitlines():
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
            lines.append("")
            continue
        lines.append("" if in_fence else line)
    return "\n".join(lines)

def structural_paragraph(paragraph):
    raw_lines = [line for line in paragraph.splitlines() if line.strip()]
    if raw_lines and raw_lines[0].startswith(("    ", "\t")):
        return True
    lines = [
        line.strip()
        for line in paragraph.strip().splitlines()
        if line.strip() and "agentops:claim:" not in line
    ]
    if not lines:
        return True
    first = lines[0]
    return (
        first.startswith("|")
        or first.startswith(("- ", "* "))
        or first.startswith(">")
        or re.match(r"^\d+\.\s", first) is not None
        or re.match(r"^#{1,6}\s", first) is not None
    )

source_ids = {}
errors = []

for source in sources:
    path = root / source
    text = path.read_text(errors="replace")
    scan_text = strip_fenced_blocks(text)
    for match in marker_re.finditer(scan_text):
        claim_id = match.group(1)
        source_ids[claim_id] = source

    offset = 0
    for para in re.split(r"\n\s*\n", scan_text):
        line_no = scan_text.count("\n", 0, offset) + 1
        offset += len(para) + 2
        if structural_paragraph(para):
            continue
        if term_re.search(para) and not marker_re.search(para):
            snippet = " ".join(para.strip().split())[:160]
            errors.append(f"{source}:{line_no}: high-claim paragraph lacks marker: {snippet}")

for claim_id, source in sorted(source_ids.items()):
    if claim_id not in ledger_ids:
        errors.append(f"{source}: marker {claim_id} missing from ledger")

for claim in ledger["claims"]:
    claim_id = claim["claim_id"]
    if claim_id not in source_ids:
        errors.append(f"{claim['source']['file']}: ledger row {claim_id} has no matching source marker")
    elif claim["source"]["file"] != source_ids[claim_id]:
        errors.append(
            f"{claim['source']['file']}: ledger row {claim_id} points to {claim['source']['file']} "
            f"but marker is in {source_ids[claim_id]}"
        )
    if claim["source"]["marker"] != claim_id:
        errors.append(f"{claim['source']['file']}: source.marker must equal claim_id for {claim_id}")

if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
PY
  return $?
}

validate_once() {
  local root="$1"
  local ledger_rel="$2"
  local fixture_mode="${3:-0}"
  local ledger_abs
  ledger_abs="$(abs_path "$root" "$ledger_rel")"

  [[ -f "$ledger_abs" ]] || {
    echo "missing ledger: $ledger_rel" >&2
    return 1
  }

  validate_ledger_shape "$ledger_abs" || return 1

  mapfile -t sources < <(
    if [[ "$fixture_mode" == "1" ]]; then
      collect_fixture_sources "$root"
    else
      collect_repo_sources "$root"
    fi
  )

  validate_sources_and_markers "$root" "$ledger_abs" "${sources[@]}" || return 1
}

run_fixture_suite() {
  local fixtures="$ROOT/tests/fixtures/factory-claim-ledger"
  [[ -d "$fixtures" ]] || {
    echo "missing fixtures: $fixtures" >&2
    return 1
  }

  validate_once "$fixtures/positive" "ledger.json" 1

  local name
  for name in negative-missing-marker negative-orphan-row negative-unknown-enum negative-l2-no-evidence; do
    if validate_once "$fixtures/$name" "ledger.json" 1 >/tmp/factory-claim-ledger-negative.out 2>&1; then
      echo "negative fixture unexpectedly passed: $name" >&2
      cat /tmp/factory-claim-ledger-negative.out >&2
      return 1
    fi
  done
}

if [[ -n "$FIXTURE_ROOT" ]]; then
  validate_once "$FIXTURE_ROOT" "$LEDGER" 1
else
  validate_once "$ROOT" "$LEDGER" 0
  if [[ "$STRICT" -eq 1 && "$RUN_FIXTURES" -eq 1 ]]; then
    run_fixture_suite
  fi
fi

echo "factory claim ledger: PASS"
