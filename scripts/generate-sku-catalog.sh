#!/usr/bin/env bash
# generate-sku-catalog.sh — emit the SKU capability block (schema v2) as JSON.
#
# The SKU catalog is a DERIVED projection (4th axis after DDD-vocab, Hex-
# frontmatter, Gherkin-acceptance). It JOINs existing sources — never hand-
# authored. This script prints the `capabilities` + `capability_summary` +
# `cli_top_level_commands` block to stdout; scripts/generate-registry.sh folds
# it into registry.json (schema_version 2). It is also the engine the drift gate
# (scripts/validate-sku-catalog-drift.sh) regenerates from.
#
# Usage:
#   bash scripts/generate-sku-catalog.sh            # print SKU block JSON to stdout
#   AGENTOPS_AO_BIN=/path/to/ao bash scripts/generate-sku-catalog.sh
#
# Sources joined (all canonical + gated):
#   docs/contracts/skill-dispositions.yaml  (BC, hex_role, disposition)
#   skills/<name>/SKILL.md frontmatter      (purpose, consumes, produces, status?)
#   skills/SKILL-TIERS.md                   (tier)
#   the live `ao capabilities` cobra tree   (cli-command SKUs, drives_commands)
#   .github/workflows/validate.yml          (gate SKUs)
#   packs/agentops/                         (reference-impl SKUs)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AO_BIN="${AGENTOPS_AO_BIN:-}"

if [[ -z "$AO_BIN" ]]; then
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  AO_BIN="$TMP_DIR/ao"
  (
    cd "$REPO_ROOT/cli"
    go build -o "$AO_BIN" ./cmd/ao
  )
fi

[[ -x "$AO_BIN" ]] || {
  echo "Missing or non-executable ao binary: $AO_BIN" >&2
  exit 1
}

export REPO_ROOT AO_BIN

python3 - <<'PY'
import json
import os
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(os.environ["REPO_ROOT"]) / "scripts" / "lib"))
import sku_catalog  # noqa: E402

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
ao_bin = os.environ["AO_BIN"]
catalog = sku_catalog.build_catalog(repo_root, ao_bin)
json.dump(catalog, sys.stdout, indent=2, sort_keys=False)
sys.stdout.write("\n")
PY
