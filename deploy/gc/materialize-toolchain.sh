#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'materialize-toolchain: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: materialize-toolchain.sh --output DIR [options]

Build one exact gc/bd pair from deploy/gc/toolchain.lock.json.

Options:
  --output DIR  New or empty directory that will receive bin/gc, bin/bd,
                and toolchain.json
  --pair ID     Accepted pair id (default: first pair with status=qualified)
  --lock PATH   Alternate lock for isolated testing (default: adjacent lock)
  --describe    Print the selected lock entry and exit without building
  -h, --help    Show this help
EOF
}

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
lock="$script_dir/toolchain.lock.json"
output=""
pair_id=""
describe=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --output)
      [ "$#" -ge 2 ] || die "--output requires a path"
      output="$2"
      shift 2
      ;;
    --pair)
      [ "$#" -ge 2 ] || die "--pair requires an id"
      pair_id="$2"
      shift 2
      ;;
    --lock)
      [ "$#" -ge 2 ] || die "--lock requires a path"
      lock="$2"
      shift 2
      ;;
    --describe)
      describe=1
      shift
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

command -v python3 >/dev/null 2>&1 || die "python3 is required"
[ -f "$lock" ] || die "toolchain lock not found: $lock"

selection=""
if ! selection="$(python3 - "$lock" "$pair_id" <<'PY'
import json
import sys

path, requested = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    lock = json.load(handle)
if lock.get("schema_version") != 1:
    raise SystemExit("toolchain lock schema_version must be 1")
entries = lock.get("accepted_pairs")
if not isinstance(entries, list) or not entries:
    raise SystemExit("toolchain lock must contain accepted_pairs")
selected = None
if requested:
    selected = next((entry for entry in entries if entry.get("id") == requested), None)
    if selected is None:
        raise SystemExit(f"unknown pair id: {requested}")
else:
    selected = next((entry for entry in entries if entry.get("status") == "qualified"), None)
    if selected is None:
        raise SystemExit("toolchain lock contains no qualified pair")
for component in ("gc", "bd"):
    value = selected.get(component)
    if not isinstance(value, dict):
        raise SystemExit(f"selected pair has no {component} object")
    for field in ("repository", "ref", "version", "source_commit", "runtime_commit"):
        item = value.get(field)
        if not isinstance(item, str) or not item or "\n" in item:
            raise SystemExit(f"selected pair has invalid {component}.{field}")
print(json.dumps(selected, sort_keys=True, separators=(",", ":")))
PY
)"; then
  die "$selection"
fi

if [ "$describe" -eq 1 ]; then
  python3 -m json.tool <<<"$selection"
  exit 0
fi

[ -n "$output" ] || die "--output is required unless --describe is used"
for command_name in git go make install; do
  command -v "$command_name" >/dev/null 2>&1 || die "$command_name is required"
done

output="$(python3 - "$output" <<'PY'
import os
import sys
print(os.path.realpath(os.path.abspath(os.path.expanduser(sys.argv[1]))))
PY
)"
if [ -e "$output" ]; then
  [ -d "$output" ] || die "output exists and is not a directory: $output"
  [ -z "$(find "$output" -mindepth 1 -maxdepth 1 -print -quit)" ] || \
    die "output directory is not empty: $output"
fi

fields=()
while IFS= read -r field; do
  fields+=("$field")
done < <(python3 - "$selection" <<'PY'
import json
import sys
selected = json.loads(sys.argv[1])
for value in (
    selected["id"],
    selected["status"],
    selected["gc"]["repository"],
    selected["gc"]["ref"],
    selected["gc"]["version"],
    selected["gc"]["source_commit"],
    selected["gc"]["runtime_commit"],
    selected["bd"]["repository"],
    selected["bd"]["ref"],
    selected["bd"]["version"],
    selected["bd"]["source_commit"],
    selected["bd"]["runtime_commit"],
):
    print(value)
PY
)
[ "${#fields[@]}" -eq 12 ] || die "selected pair projection is incomplete"
selected_id="${fields[0]}"
selected_status="${fields[1]}"
gc_repository="${fields[2]}"
gc_ref="${fields[3]}"
gc_version="${fields[4]}"
gc_source_commit="${fields[5]}"
gc_runtime_commit="${fields[6]}"
bd_repository="${fields[7]}"
bd_ref="${fields[8]}"
bd_version="${fields[9]}"
bd_source_commit="${fields[10]}"
bd_runtime_commit="${fields[11]}"

parent="$(dirname "$output")"
mkdir -p "$parent"
sources="$(mktemp -d "$parent/.gc-toolchain-sources.XXXXXX")"
stage="$(mktemp -d "$parent/.gc-toolchain-stage.XXXXXX")"
cleanup() {
  rm -rf "$sources" "$stage"
}
trap cleanup EXIT

checkout_exact() {
  local repository="$1"
  local ref="$2"
  local commit="$3"
  local destination="$4"
  git init -q "$destination"
  git -C "$destination" remote add origin "$repository"
  git -C "$destination" fetch -q --depth=1 origin "$ref"
  git -C "$destination" checkout -q --detach "$commit"
  [ "$(git -C "$destination" rev-parse HEAD)" = "$commit" ] || \
    die "source checkout did not resolve exact commit $commit"
}

checkout_exact "$gc_repository" "$gc_ref" "$gc_source_commit" "$sources/gc"
checkout_exact "$bd_repository" "$bd_ref" "$bd_source_commit" "$sources/bd"

make -s -C "$sources/gc" build
make -s -C "$sources/bd" build
mkdir -p "$stage/bin"
install -m 0755 "$sources/gc/bin/gc" "$stage/bin/gc"
install -m 0755 "$sources/bd/bd" "$stage/bin/bd"

python3 - "$stage/bin/gc" "$stage/bin/bd" "$selection" "$stage/toolchain.json" <<'PY'
import hashlib
import json
import re
import subprocess
import sys

gc_path, bd_path, selection_json, receipt_path = sys.argv[1:]
selected = json.loads(selection_json)


def run(command):
    result = subprocess.run(command, check=False, capture_output=True, text=True)
    if result.returncode != 0:
        raise SystemExit(f"{' '.join(command)} failed ({result.returncode}): {(result.stderr or result.stdout).strip()}")
    return result.stdout


gc = json.loads(run([gc_path, "version", "--json"]))
if gc.get("ok") is not True:
    raise SystemExit("gc version --json did not report ok=true")
expected_gc = selected["gc"]
if gc.get("version") != expected_gc["version"] or not expected_gc["source_commit"].startswith(str(gc.get("commit", ""))):
    raise SystemExit(f"built gc identity mismatch: {gc}")

bd_output = run([bd_path, "version"])
match = re.search(r"^bd version (\S+) \(([^:)]+)", bd_output, re.MULTILINE)
if match is None:
    raise SystemExit(f"cannot parse built bd identity: {bd_output.strip()!r}")
expected_bd = selected["bd"]
if match.group(1) != expected_bd["version"] or not expected_bd["source_commit"].startswith(match.group(2)):
    raise SystemExit(f"built bd identity mismatch: {bd_output.strip()}")


def digest(path):
    value = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


receipt = {
    "schema_version": 1,
    "pair": selected,
    "runtime": {
        "gc": {"version": gc["version"], "commit": gc["commit"], "sha256": digest(gc_path)},
        "bd": {"version": match.group(1), "commit": match.group(2), "sha256": digest(bd_path)},
    },
}
with open(receipt_path, "w", encoding="utf-8") as handle:
    json.dump(receipt, handle, indent=2, sort_keys=True)
    handle.write("\n")
PY

if [ -d "$output" ]; then
  rmdir "$output"
fi
mv "$stage" "$output"
stage="$parent/.gc-toolchain-stage.consumed"
trap - EXIT
rm -rf "$sources"

printf 'materialized %s (%s) at %s\n' "$selected_id" "$selected_status" "$output"
printf 'use: %s/bootstrap.sh --gc-bin %s/bin/gc ...\n' "$script_dir" "$output"
