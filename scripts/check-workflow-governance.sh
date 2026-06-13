#!/usr/bin/env bash
# check-workflow-governance.sh — kind-aware workflow governance gate (ag-km74w).
#
# Asserts every repo-tracked .claude/workflows/*.js has a matching entry in the
# top-level `workflows:` section of docs/contracts/skill-dispositions.yaml, and
# that each such entry declares `kind: workflow`. This is the thin
# workflow-only sliver of the S0 artifact-dispositions schema: it gives
# Claude Workflow scripts a governed home in the ledger WITHOUT teaching the
# `- skill:` line-parsers (sku_catalog.py / generate-skill-domain-map.sh) to
# miscount them as skills (the `workflows:` mapping is top-level, skipped like
# `historical:`).
#
# Exit 0: every workflow .js is registered with kind: workflow.
# Exit 1: a workflow .js has no ledger entry, or an entry lacks kind: workflow.
#
# Repo root from cwd git. No hardcoded user paths.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
disp_yaml="$repo_root/docs/contracts/skill-dispositions.yaml"

if [ ! -f "$disp_yaml" ]; then
  echo "FAIL: ledger not found: $disp_yaml" >&2
  exit 1
fi

status=0

# For each tracked workflow .js, derive its id from meta.name (authoritative),
# then assert the ledger has a `workflows.<id>` entry with kind: workflow.
while IFS= read -r tracked; do
  rel="$tracked"
  abs="$repo_root/$tracked"
  [ -f "$abs" ] || continue
  # id = meta.name literal from the .js (single- or double-quoted).
  id="$(grep -oE "name:[[:space:]]*['\"][^'\"]+['\"]" "$abs" | head -1 | sed -E "s/.*['\"]([^'\"]+)['\"].*/\1/")"
  if [ -z "$id" ]; then
    echo "FAIL: $rel has no parseable meta.name (cannot match to ledger)" >&2
    status=1
    continue
  fi
  # Verify the ledger has this workflow id with kind: workflow.
  if ! python3 - "$disp_yaml" "$id" "$rel" <<'PY'
import sys
import yaml
disp_yaml, wf_id, rel = sys.argv[1], sys.argv[2], sys.argv[3]
data = yaml.safe_load(open(disp_yaml, encoding="utf-8")) or {}
workflows = data.get("workflows") or {}
entry = workflows.get(wf_id)
if entry is None:
    print(f"FAIL: workflow '{wf_id}' ({rel}) has no `workflows:` ledger entry", file=sys.stderr)
    sys.exit(1)
if entry.get("kind") != "workflow":
    print(f"FAIL: workflow '{wf_id}' ledger entry kind={entry.get('kind')!r}, expected 'workflow'", file=sys.stderr)
    sys.exit(1)
sys.exit(0)
PY
  then
    status=1
    continue
  fi
  echo "OK: $id ($rel) registered with kind: workflow"
done < <(git -C "$repo_root" ls-files '.claude/workflows/*.js')

if [ "$status" -eq 0 ]; then
  echo "OK: all repo-tracked .claude/workflows/*.js are governed (kind: workflow)"
fi
exit "$status"
