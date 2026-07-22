#!/usr/bin/env bash
set -euo pipefail

die() { printf 'materialize-toolchain: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'EOF'
Usage: materialize-toolchain.sh --output DIR [--pair ID] [--lock FILE] [--describe]

Build only the exact official Gas City and Beads pair selected from the lock.
The output contains bin/gc, bin/bd, and toolchain.json.
EOF
}

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
lock="$script_dir/toolchain.lock.json"
output=""
pair_id=""
describe=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) output="${2:?--output requires a path}"; shift 2 ;;
    --pair) pair_id="${2:?--pair requires an id}"; shift 2 ;;
    --lock) lock="${2:?--lock requires a path}"; shift 2 ;;
    --describe) describe=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

for tool in python3 git make install; do command -v "$tool" >/dev/null || die "$tool is required"; done
[ -f "$lock" ] || die "lock not found: $lock"
selection="$(python3 - "$lock" "$pair_id" <<'PY'
import json, re, sys
lock = json.load(open(sys.argv[1], encoding="utf-8"))
if lock.get("schema_version") != 1:
    raise SystemExit("toolchain lock schema_version must be 1")
entries = lock.get("accepted_pairs")
requested = sys.argv[2]
selected = next((x for x in entries if x.get("id") == requested), None) if requested else next((x for x in entries if x.get("status") == "qualified"), None)
if not isinstance(selected, dict):
    raise SystemExit("requested or qualified toolchain pair not found")
for name in ("gc", "bd"):
    item = selected.get(name)
    for field in ("repository", "ref", "version", "source_commit", "release_checksum_asset", "release_checksum_sha256"):
        value = item.get(field) if isinstance(item, dict) else None
        if not isinstance(value, str) or not value:
            raise SystemExit(f"invalid {name}.{field}")
    if not re.fullmatch(r"[0-9a-f]{40}", item["source_commit"]):
        raise SystemExit(f"invalid {name}.source_commit")
    if not re.fullmatch(r"[0-9a-f]{64}", item["release_checksum_sha256"]):
        raise SystemExit(f"invalid {name}.release_checksum_sha256")
print(json.dumps(selected, sort_keys=True, separators=(",", ":")))
PY
)" || die "cannot select toolchain"

if [ "$describe" -eq 1 ]; then python3 -m json.tool <<<"$selection"; exit 0; fi
[ -n "$output" ] || die "--output is required"
output="$(python3 - "$output" <<'PY'
import os, sys
print(os.path.realpath(os.path.abspath(os.path.expanduser(sys.argv[1]))))
PY
)"
if [ -e "$output" ]; then
  [ -d "$output" ] || die "output is not a directory: $output"
  [ -z "$(find "$output" -mindepth 1 -maxdepth 1 -print -quit)" ] || die "output is not empty: $output"
fi

fields=()
while IFS= read -r field; do
  fields[${#fields[@]}]="$field"
done < <(python3 - "$selection" <<'PY'
import json, sys
s = json.loads(sys.argv[1])
for value in (s["id"], s["status"], s["gc"]["repository"], s["gc"]["ref"], s["gc"]["version"], s["gc"]["source_commit"], s["bd"]["repository"], s["bd"]["ref"], s["bd"]["version"], s["bd"]["source_commit"]):
    print(value)
PY
)
[ "${#fields[@]}" -eq 10 ] || die "selected pair projection is incomplete"
parent="$(dirname "$output")"
mkdir -p "$parent"
sources="$(mktemp -d "$parent/.gc-sources.XXXXXX")"
stage="$(mktemp -d "$parent/.gc-stage.XXXXXX")"
cleanup() { rm -rf "$sources" "$stage"; }
trap cleanup EXIT

checkout_exact() {
  local repository="$1" ref="$2" version="$3" commit="$4" destination="$5"
  git init -q "$destination"
  git -C "$destination" remote add origin "$repository"
  git -C "$destination" fetch -q --depth=1 origin "$commit" || die "cannot fetch $commit"
  if [ "$ref" = "v$version" ]; then
    git -C "$destination" fetch -q --depth=1 origin "refs/tags/$ref:refs/tags/$ref" || die "cannot fetch $ref"
    [ "$(git -C "$destination" rev-parse "$ref^{}")" = "$commit" ] || die "$ref does not resolve to $commit"
    git -C "$destination" tag -f "$ref" "$commit" >/dev/null
  fi
  git -C "$destination" checkout -q --detach "$commit"
}
checkout_exact "${fields[2]}" "${fields[3]}" "${fields[4]}" "${fields[5]}" "$sources/gc"
checkout_exact "${fields[6]}" "${fields[7]}" "${fields[8]}" "${fields[9]}" "$sources/bd"
make -s -C "$sources/gc" build
make -s -C "$sources/bd" build
mkdir -p "$stage/bin"
install -m 0755 "$sources/gc/bin/gc" "$stage/bin/gc"
install -m 0755 "$sources/bd/bd" "$stage/bin/bd"

python3 - "$stage/bin/gc" "$stage/bin/bd" "$selection" "$stage/toolchain.json" <<'PY'
import hashlib, json, re, subprocess, sys
gc_path, bd_path, selection, receipt_path = sys.argv[1:]
selected = json.loads(selection)
def run(argv):
    result = subprocess.run(argv, check=False, capture_output=True, text=True)
    if result.returncode:
        raise SystemExit((result.stderr or result.stdout).strip())
    return result.stdout
def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()
gc = json.loads(run([gc_path, "version", "--json"]))
bd_text = run([bd_path, "version"])
match = re.search(r"^bd version (\S+) \(([^:)]+)", bd_text, re.M)
if gc.get("version") != selected["gc"]["version"] or not selected["gc"]["source_commit"].startswith(str(gc.get("commit", ""))):
    raise SystemExit("built Gas City identity differs from lock")
if match is None or match.group(1) != selected["bd"]["version"] or not selected["bd"]["source_commit"].startswith(match.group(2)):
    raise SystemExit("built Beads identity differs from lock")
receipt = {"schema_version": 2, "pair": selected, "runtime": {"gc": {"path": "bin/gc", "version": gc["version"], "commit": gc["commit"], "sha256": sha(gc_path)}, "bd": {"path": "bin/bd", "version": match.group(1), "commit": match.group(2), "sha256": sha(bd_path)}}}
with open(receipt_path, "w", encoding="utf-8") as f:
    json.dump(receipt, f, indent=2, sort_keys=True); f.write("\n")
PY

if [ -d "$output" ]; then rmdir "$output"; fi
mv "$stage" "$output"
stage="$parent/.gc-stage.consumed"
trap - EXIT
rm -rf "$sources"
printf 'materialized %s (%s) at %s\n' "${fields[0]}" "${fields[1]}" "$output"
