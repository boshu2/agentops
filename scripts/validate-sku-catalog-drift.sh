#!/usr/bin/env bash
# validate-sku-catalog-drift.sh — CI gate for the SKU capability catalog.
#
# The SKU catalog lives in registry.json (schema_version 2) as a DERIVED
# projection joining SKILL.md frontmatter + skill-dispositions.yaml +
# SKILL-TIERS.md + the live `ao` cobra tree + validate.yml gate jobs +
# packs/agentops. It must never be hand-edited. This gate enforces three things:
#
#   (a) DRIFT — regenerate the SKU block and diff against committed registry.json
#       (mirrors registry-check / validate-skill-domain-map-golden).
#   (b) LINKAGE INTEGRITY — every skill `drives_commands` entry resolves to a
#       REAL ao command in the live cobra tree (closes oracle gaps #1/#2/#3:
#       no more stale `ao schedule`-style references shipping undetected).
#   (c) COVERAGE — every bounded context (BC1-BC5) and every operating-loop
#       move (1-7) has at least one ACTIVE skill, and every BC has a cli-command.
#
# Exit codes:
#   0 — all checks pass
#   1 — drift, linkage, or coverage failure
#   2 — wrapper/tool error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REGISTRY="${REPO_ROOT}/registry.json"
AO_BIN="${AGENTOPS_AO_BIN:-}"

if [[ ! -f "$REGISTRY" ]]; then
  echo "SKU_CATALOG: registry.json not found at $REGISTRY (run scripts/generate-registry.sh)" >&2
  exit 2
fi

if [[ -z "$AO_BIN" ]]; then
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  AO_BIN="$TMP_DIR/ao"
  ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao ) >&2
fi

[[ -x "$AO_BIN" ]] || { echo "SKU_CATALOG: ao binary missing/non-executable: $AO_BIN" >&2; exit 2; }

export REPO_ROOT AO_BIN REGISTRY

python3 - <<'PY'
import json
import os
import pathlib
import sys

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
ao_bin = os.environ["AO_BIN"]
registry_path = pathlib.Path(os.environ["REGISTRY"])

sys.path.insert(0, str(repo_root / "scripts" / "lib"))
import sku_catalog  # noqa: E402
import sku_extract  # noqa: E402

valid_commands, _flags = sku_extract.scan_command_tree(ao_bin)
fresh = sku_catalog.build_catalog(repo_root, ao_bin)

try:
    committed = json.loads(registry_path.read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as exc:
    print(f"SKU_CATALOG: cannot read registry.json: {exc}", file=sys.stderr)
    sys.exit(2)

failed = False

# (a) DRIFT — the committed capabilities block must match a fresh regeneration.
committed_block = {
    "capabilities": committed.get("capabilities"),
    "capability_summary": committed.get("capability_summary"),
    "cli_top_level_commands": committed.get("cli_top_level_commands"),
}
fresh_block = {
    "capabilities": fresh["capabilities"],
    "capability_summary": fresh["capability_summary"],
    "cli_top_level_commands": fresh["cli_top_level_commands"],
}
if committed.get("schema_version") != 2:
    print("SKU_CATALOG: (a) DRIFT — registry.json schema_version is not 2", file=sys.stderr)
    failed = True
if json.dumps(committed_block, sort_keys=True) != json.dumps(fresh_block, sort_keys=True):
    print(
        "SKU_CATALOG: (a) DRIFT — committed SKU catalog differs from regeneration. "
        "Run: bash scripts/generate-registry.sh",
        file=sys.stderr,
    )
    # Surface a compact diff of capability sets.
    committed_skus = {c.get("sku") for c in (committed.get("capabilities") or [])}
    fresh_skus = {c["sku"] for c in fresh["capabilities"]}
    added = sorted(fresh_skus - committed_skus)
    removed = sorted(committed_skus - fresh_skus)
    if added:
        print(f"  + would add: {', '.join(added[:20])}", file=sys.stderr)
    if removed:
        print(f"  - would remove: {', '.join(removed[:20])}", file=sys.stderr)
    if not added and not removed:
        print("  (same SKU ids — a field changed; regenerate to inspect)", file=sys.stderr)
    failed = True
else:
    print(f"SKU_CATALOG: (a) drift — OK ({fresh['capability_summary']['total']} capabilities)")

# (b) LINKAGE INTEGRITY — every drives_commands edge resolves to a real command.
linkage_failures = sku_catalog.check_linkage_integrity(fresh, valid_commands)
if linkage_failures:
    print("SKU_CATALOG: (b) LINKAGE INTEGRITY — FAIL:", file=sys.stderr)
    for f in linkage_failures:
        print(f"  {f}", file=sys.stderr)
    failed = True
else:
    print("SKU_CATALOG: (b) linkage integrity — OK (all drives_commands resolve)")

# (c) COVERAGE — every BC + every loop move has an active skill.
coverage_failures = sku_catalog.check_coverage(fresh)
if coverage_failures:
    print("SKU_CATALOG: (c) COVERAGE — FAIL:", file=sys.stderr)
    for f in coverage_failures:
        print(f"  {f}", file=sys.stderr)
    failed = True
else:
    print("SKU_CATALOG: (c) coverage — OK (all BCs + loop moves have an active skill)")

sys.exit(1 if failed else 0)
PY
