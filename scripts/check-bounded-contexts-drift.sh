#!/usr/bin/env bash
# scripts/check-bounded-contexts-drift.sh
#
# Verify the BC1-BC5 definitions in docs/contracts/bounded-contexts.yaml
# (canonical) match the prose used in the registry docs that classify
# skills against them.
#
# Encodes Phase 2 of the registries-drift remediation (soc-zxia.2):
# extract BC1-BC5 definitions to a single yaml source-of-truth so that
# the same five concepts cannot be restated with drift in 3 places.
#
# Checks:
#   1. Every BC id+name pair in the yaml appears verbatim as a row prefix
#      in docs/reference/agentops-skill-domain-map.md Domain Taxonomy table.
#   2. Every BC id+name pair appears in docs/reference/agentops-hexagonal-
#      architecture-map.md Bounded Contexts table.
#   3. Each BC's responsibility (canonical sentence) appears verbatim in
#      both of the above docs.
#   4. Each BC's product_layer string appears in skill-domain-map.md.
#   5. Each BC's port names appear in hexagonal-architecture-map.md.
#
# Exit codes:
#   0 = no drift
#   1 = drift detected
#   2 = usage / missing input
#
# Modes:
#   --check  (default) report drift
#   --json   machine-readable report
#
# Lesson:  .agents/learnings/2026-05-17-registries-drift.md
# Phase:   soc-zxia.2 (after soc-zxia.1 schema-gate, before soc-zxia.3 generators)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BC_YAML="${REPO_ROOT}/docs/contracts/bounded-contexts.yaml"
MAP_DOC="${REPO_ROOT}/docs/reference/agentops-skill-domain-map.md"
HEX_DOC="${REPO_ROOT}/docs/reference/agentops-hexagonal-architecture-map.md"

JSON_OUT=0
for arg in "$@"; do
  case "$arg" in
    --check) ;;
    --json)  JSON_OUT=1 ;;
    -h|--help)
      sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "ERROR: unknown arg: $arg (try --help)" >&2
      exit 2
      ;;
  esac
done

for f in "${BC_YAML}" "${MAP_DOC}" "${HEX_DOC}"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: required file missing: $f" >&2
    exit 2
  fi
done

export BC_YAML MAP_DOC HEX_DOC JSON_OUT

exec python3 - <<'PY'
import json
import os
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed; install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

BC_YAML  = Path(os.environ["BC_YAML"])
MAP_DOC  = Path(os.environ["MAP_DOC"])
HEX_DOC  = Path(os.environ["HEX_DOC"])
JSON_OUT = os.environ.get("JSON_OUT") == "1"

data = yaml.safe_load(BC_YAML.read_text())
bcs = data.get("bounded_contexts", [])
if len(bcs) != 5:
    print(f"ERROR: expected 5 bounded contexts in {BC_YAML.name}, got {len(bcs)}", file=sys.stderr)
    sys.exit(2)

map_text = MAP_DOC.read_text()
hex_text = HEX_DOC.read_text()

findings = []


def add(severity, code, msg):
    findings.append({"severity": severity, "code": code, "msg": msg})


for bc in bcs:
    bc_id   = bc["id"]
    bc_name = bc["name"]
    title   = f"{bc_id} {bc_name}"  # e.g. "BC1 Corpus"

    # Check 1: title appears in map doc
    if title not in map_text:
        add("fail", "BC_TITLE_MISSING_FROM_MAP",
            f"`{title}` not found in {MAP_DOC.name} — every BC must appear in skill-domain-map")

    # Check 2: title in hex doc
    if title not in hex_text:
        add("fail", "BC_TITLE_MISSING_FROM_HEX",
            f"`{title}` not found in {HEX_DOC.name} — every BC must appear in hexagonal-architecture-map")

    # Check 3: responsibility in both
    resp = bc["responsibility"]
    if resp not in map_text:
        add("fail", "BC_RESP_DRIFT_MAP",
            f"`{title}` responsibility in {MAP_DOC.name} drifts from yaml canonical: \"{resp}\"")
    if resp not in hex_text:
        add("fail", "BC_RESP_DRIFT_HEX",
            f"`{title}` responsibility in {HEX_DOC.name} drifts from yaml canonical: \"{resp}\"")

    # Check 4: product_layer in map doc
    pl = bc.get("product_layer", "")
    if pl and pl not in map_text:
        add("fail", "BC_PRODUCT_LAYER_DRIFT",
            f"`{title}` product_layer in {MAP_DOC.name} drifts from yaml canonical: \"{pl}\"")

    # Check 5: ports in hex doc
    for port in bc.get("ports", []):
        if port not in hex_text:
            add("warn", "BC_PORT_MISSING_FROM_HEX",
                f"`{title}` port `{port}` declared in yaml but not mentioned in {HEX_DOC.name}")


fails = [f for f in findings if f["severity"] == "fail"]
warns = [f for f in findings if f["severity"] == "warn"]

if JSON_OUT:
    print(json.dumps({
        "bounded_contexts_checked": len(bcs),
        "findings": findings,
        "verdict": "FAIL" if fails else ("WARN" if warns else "PASS"),
    }, indent=2))
else:
    print(f"Bounded-context drift check: {len(bcs)} BCs in {BC_YAML.name}")
    print(f"  cross-checked against {MAP_DOC.name} + {HEX_DOC.name}")
    print()
    for f in findings:
        tag = {"fail": "FAIL", "warn": "WARN"}[f["severity"]]
        print(f"[{tag}] {f['code']}: {f['msg']}")
    print()
    if not findings:
        print("PASS — registry docs match yaml canonical.")
    elif fails:
        print(f"FAIL — {len(fails)} drift finding(s), {len(warns)} warning(s)")
    else:
        print(f"WARN — {len(warns)} warning(s)")

sys.exit(1 if fails else 0)
PY
