#!/usr/bin/env bash
# scripts/check-bounded-contexts-drift.sh
#
# Verify the BC1-BC6 definitions in docs/contracts/bounded-contexts.yaml
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
#   2. Every BC id+name pair appears in docs/architecture/component-map.md.
#   3. Each BC's responsibility (canonical sentence) appears verbatim in
#      both of the above docs.
#   4. Each BC's product_layer string appears in skill-domain-map.md.
#   5. Each BC's implemented and target port names appear in component-map.md.
#   6. implemented_ports is an exact, one-owner inventory of every Go interface
#      declared under cli/internal/ports (including three legacy unsuffixed names).
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
COMPONENT_DOC="${REPO_ROOT}/docs/architecture/component-map.md"
PORTS_DIR="${PORTS_DIR:-${REPO_ROOT}/cli/internal/ports}"

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

for f in "${BC_YAML}" "${MAP_DOC}" "${COMPONENT_DOC}"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: required file missing: $f" >&2
    exit 2
  fi
done
if [[ ! -d "${PORTS_DIR}" ]]; then
  echo "ERROR: required directory missing: ${PORTS_DIR}" >&2
  exit 2
fi

export BC_YAML MAP_DOC COMPONENT_DOC PORTS_DIR JSON_OUT

exec python3 - <<'PY'
import json
import os
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed; install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

BC_YAML  = Path(os.environ["BC_YAML"])
MAP_DOC  = Path(os.environ["MAP_DOC"])
COMPONENT_DOC = Path(os.environ["COMPONENT_DOC"])
PORTS_DIR = Path(os.environ["PORTS_DIR"])
JSON_OUT = os.environ.get("JSON_OUT") == "1"

data = yaml.safe_load(BC_YAML.read_text())
bcs = data.get("bounded_contexts", [])
if len(bcs) != 6:
    print(f"ERROR: expected 6 bounded contexts in {BC_YAML.name}, got {len(bcs)}", file=sys.stderr)
    sys.exit(2)

map_text = MAP_DOC.read_text()
component_text = COMPONENT_DOC.read_text()

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

    # Check 2: title in the stable component map
    if title not in component_text:
        add("fail", "BC_TITLE_MISSING_FROM_COMPONENT_MAP",
            f"`{title}` not found in {COMPONENT_DOC.name} — every BC must appear in component-map")

    # Check 3: responsibility in both
    resp = bc["responsibility"]
    if resp not in map_text:
        add("fail", "BC_RESP_DRIFT_MAP",
            f"`{title}` responsibility in {MAP_DOC.name} drifts from yaml canonical: \"{resp}\"")
    if resp not in component_text:
        add("fail", "BC_RESP_DRIFT_COMPONENT_MAP",
            f"`{title}` responsibility in {COMPONENT_DOC.name} drifts from yaml canonical: \"{resp}\"")

    # Check 4: product_layer in map doc
    pl = bc.get("product_layer", "")
    if pl and pl not in map_text:
        add("fail", "BC_PRODUCT_LAYER_DRIFT",
            f"`{title}` product_layer in {MAP_DOC.name} drifts from yaml canonical: \"{pl}\"")

    # Check 5: implemented and target ports are explicitly classified.
    for field in ("implemented_ports", "target_ports"):
        for port in bc.get(field, []):
            if port not in component_text:
                add("fail", "BC_PORT_MISSING_FROM_COMPONENT_MAP",
                    f"`{title}` {field} entry `{port}` is absent from {COMPONENT_DOC.name}")

# Check 6: the declared implemented inventory is exactly the Go port surface.
declared_owners = {}
for bc in bcs:
    for port in bc.get("implemented_ports", []):
        declared_owners.setdefault(port, []).append(bc["id"])

for port, owners in sorted(declared_owners.items()):
    if len(owners) != 1:
        add("fail", "IMPLEMENTED_PORT_MULTIPLE_OWNERS",
            f"`{port}` is assigned to multiple bounded contexts: {', '.join(owners)}")

go_ports = set()
port_pattern = re.compile(r"^type\s+([A-Za-z0-9_]+)\s+interface\s*\{", re.MULTILINE)
for go_file in PORTS_DIR.glob("*.go"):
    go_ports.update(port_pattern.findall(go_file.read_text()))

declared_ports = set(declared_owners)
for port in sorted(go_ports - declared_ports):
    add("fail", "GO_PORT_MISSING_FROM_CONTRACT",
        f"`{port}` exists under cli/internal/ports but has no implemented_ports owner")
for port in sorted(declared_ports - go_ports):
    add("fail", "CONTRACT_PORT_MISSING_FROM_GO",
        f"`{port}` is declared implemented but no matching Go interface exists")


fails = [f for f in findings if f["severity"] == "fail"]
warns = [f for f in findings if f["severity"] == "warn"]

if JSON_OUT:
    print(json.dumps({
        "bounded_contexts_checked": len(bcs),
        "implemented_ports_checked": len(go_ports),
        "findings": findings,
        "verdict": "FAIL" if fails else ("WARN" if warns else "PASS"),
    }, indent=2))
else:
    print(f"Bounded-context drift check: {len(bcs)} BCs in {BC_YAML.name}")
    print(f"  cross-checked against {MAP_DOC.name} + {COMPONENT_DOC.name}")
    print(f"  exact Go port inventory: {len(go_ports)} interfaces")
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
