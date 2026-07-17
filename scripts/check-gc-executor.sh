#!/usr/bin/env bash
# Static contract gate for the optional Gas City execution adapter.
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

cd "$REPO_ROOT" || exit 2
require_cmd python3
require_cmd bats
export PYTHONDONTWRITEBYTECODE=1

python3 scripts/sync-gc-pack.py --check
python3 -m unittest discover -s tests/python -p 'test_gc_packet.py' -v
python3 -m unittest discover -s tests/python -p 'test_gc_factory.py' -v
python3 -m unittest discover -s tests/python -p 'test_sync_gc_pack.py' -v
bats tests/scripts/gc-agentops-bootstrap.bats

PACK="$REPO_ROOT/packs/agentops-executor"
FACTORY="$REPO_ROOT/packs/agentops-factory"
AGENTOPS_GC_SKIP_VERSION_CHECK=1 GC_PACK_DIR="$PACK" \
  python3 "$PACK/assets/scripts/packet.py" doctor-contract
GC_PACK_DIR="$PACK" python3 "$PACK/assets/scripts/packet.py" doctor-roles
GC_PACK_DIR="$PACK" python3 "$PACK/assets/scripts/packet.py" doctor-projection
python3 "$FACTORY/assets/scripts/factory.py" doctor

python3 -m json.tool "$PACK/assets/schemas/gc-execution-envelope.v1.schema.json" >/dev/null
python3 -m json.tool "$PACK/commands/run-packet/schemas/result.schema.json" >/dev/null
python3 -m json.tool "$PACK/commands/run-packet/schemas/failure.schema.json" >/dev/null
for schema in "$FACTORY"/assets/schemas/*.json; do
  python3 -m json.tool "$schema" >/dev/null
done

echo "Gas City executor and bead-native factory static contract: PASS"
