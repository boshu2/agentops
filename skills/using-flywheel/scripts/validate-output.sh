#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" || -L "$1" ]]; then
  echo "usage: $0 <flywheel-run-receipt.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import json
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 131072:
    raise SystemExit("flywheel receipt: output exceeds 131072 bytes")
try:
    value = json.loads(raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"flywheel receipt: unreadable JSON: {exc}")
required = {"schema_version", "mode", "authorization_id", "target", "target_allowlist", "limits", "provisioning", "dispatch", "effects", "status"}
if not isinstance(value, dict) or set(value) != required:
    raise SystemExit("flywheel receipt: unexpected or missing fields")
def text(item, maximum):
    return isinstance(item, str) and 0 < len(item) <= maximum
if value["schema_version"] != "flywheel-run-receipt.v1" or value["mode"] not in {"observation", "skill-installation", "provisioning", "dispatch"} or not text(value["authorization_id"], 256):
    raise SystemExit("flywheel receipt: invalid identity, mode, or authorization")
allowlist = value["target_allowlist"]
if not isinstance(allowlist, list) or not 1 <= len(allowlist) <= 5 or len(set(allowlist)) != len(allowlist) or not all(text(item, 253) for item in allowlist) or value["target"] not in allowlist:
    raise SystemExit("flywheel receipt: target is not in the declared allowlist")
limits = value["limits"]
bounds = {
    "command_timeout_seconds": (1, 900),
    "overall_timeout_seconds": (1, 3600),
    "max_output_bytes": (1, 16777216),
    "max_download_bytes": (0, 268435456),
    "max_workers": (0, 8),
    "max_rounds": (0, 2),
}
if not isinstance(limits, dict) or set(limits) != set(bounds):
    raise SystemExit("flywheel receipt: malformed limits")
for field, (minimum, maximum) in bounds.items():
    item = limits[field]
    if not isinstance(item, int) or isinstance(item, bool) or not minimum <= item <= maximum:
        raise SystemExit(f"flywheel receipt: {field} exceeds bound")
provisioning = value["provisioning"]
if not isinstance(provisioning, dict) or set(provisioning) != {"requested", "upstream_version", "upstream_sha256", "download_domains"} or not isinstance(provisioning["requested"], bool):
    raise SystemExit("flywheel receipt: malformed provisioning declaration")
domains = provisioning["download_domains"]
if not isinstance(domains, list) or len(domains) > 10 or len(set(domains)) != len(domains) or not all(text(item, 253) for item in domains):
    raise SystemExit("flywheel receipt: malformed provisioning domains")
dispatch = value["dispatch"]
if not isinstance(dispatch, dict) or set(dispatch) != {"requested", "coordinator", "source_intent_id"} or not isinstance(dispatch["requested"], bool):
    raise SystemExit("flywheel receipt: malformed dispatch declaration")
effects = value["effects"]
expected_effects = {"network_domains", "bytes_downloaded", "credential_identity", "workers_dispatched", "rounds_completed", "output_bytes", "writes", "supervisor_cleanup_verified"}
if not isinstance(effects, dict) or set(effects) != expected_effects:
    raise SystemExit("flywheel receipt: malformed effects")
network_domains = effects["network_domains"]
writes = effects["writes"]
if (
    not isinstance(network_domains, list) or len(network_domains) > 10 or len(set(network_domains)) != len(network_domains)
    or not all(text(item, 253) for item in network_domains)
    or not isinstance(writes, list) or len(writes) > 20 or not all(text(item, 1024) for item in writes)
):
    raise SystemExit("flywheel receipt: effect lists exceed bounds")
for field, maximum in (("bytes_downloaded", limits["max_download_bytes"]), ("workers_dispatched", limits["max_workers"]), ("rounds_completed", limits["max_rounds"]), ("output_bytes", limits["max_output_bytes"])):
    item = effects[field]
    if not isinstance(item, int) or isinstance(item, bool) or not 0 <= item <= maximum:
        raise SystemExit(f"flywheel receipt: observed {field} exceeds declaration")
credential = effects["credential_identity"]
if credential is not None and not text(credential, 128):
    raise SystemExit("flywheel receipt: invalid credential identity")
if not isinstance(effects["supervisor_cleanup_verified"], bool) or value["status"] not in {"complete", "incomplete", "stopped-before-effect"}:
    raise SystemExit("flywheel receipt: invalid status or cleanup fact")
if value["status"] in {"complete", "incomplete"} and effects["supervisor_cleanup_verified"] is not True:
    raise SystemExit("flywheel receipt: effected run lacks verified cleanup")
if value["status"] == "stopped-before-effect" and (
    network_domains or effects["bytes_downloaded"] or credential is not None
    or effects["workers_dispatched"] or effects["rounds_completed"]
    or effects["output_bytes"] or writes
):
    raise SystemExit("flywheel receipt: stopped-before-effect run leaked effects")

mode = value["mode"]
if mode in {"provisioning", "skill-installation"}:
    valid_digest = isinstance(provisioning["upstream_sha256"], str) and re.fullmatch(r"[a-f0-9]{64}", provisioning["upstream_sha256"])
    missing_credential = value["status"] != "stopped-before-effect" and credential is None
    if not provisioning["requested"] or dispatch["requested"] or not text(provisioning["upstream_version"], 128) or not valid_digest or not domains or missing_credential or not set(network_domains).issubset(domains):
        raise SystemExit("flywheel receipt: incomplete or out-of-allowlist provisioning declaration")
elif mode == "dispatch":
    missing_effect = value["status"] != "stopped-before-effect" and (effects["workers_dispatched"] < 1 or effects["rounds_completed"] < 1)
    missing_credential = value["status"] != "stopped-before-effect" and credential is None
    if provisioning["requested"] or not dispatch["requested"] or not text(dispatch["coordinator"], 256) or not text(dispatch["source_intent_id"], 256) or missing_credential or missing_effect:
        raise SystemExit("flywheel receipt: incomplete dispatch declaration")
else:
    if provisioning["requested"] or dispatch["requested"] or network_domains or effects["bytes_downloaded"] or credential is not None or effects["workers_dispatched"] or effects["rounds_completed"] or writes:
        raise SystemExit("flywheel receipt: observation-only run declared mutation")
print(f"valid flywheel-run-receipt.v1: {path}")
PY
