#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'materialize-toolchain: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: materialize-toolchain.sh --output DIR [options]

Build one exact gc/bd/ao toolchain from deploy/gc/toolchain.lock.json.

Options:
  --output DIR  New or empty directory that will receive bin/gc, bin/bd, bin/ao,
                and toolchain.json
  --pair ID     Accepted pair id (default: first pair with status=qualified)
  --lock PATH   Alternate lock for isolated testing (default: adjacent lock)
  --describe    Print the selected lock entry and exit without building
  -h, --help    Show this help
EOF
}

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
agentops_root="$(CDPATH='' cd -- "$script_dir/../.." && pwd)"
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
import re
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
    for field in ("repository", "ref", "version", "source_commit", "runtime_commit", "release_checksum_asset", "release_checksum_sha256"):
        item = value.get(field)
        if not isinstance(item, str) or not item or "\n" in item:
            raise SystemExit(f"selected pair has invalid {component}.{field}")
    if not re.fullmatch(r"[0-9a-f]{64}", value["release_checksum_sha256"]):
        raise SystemExit(f"selected pair has invalid {component}.release_checksum_sha256")
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

# The AO receipt may claim only the exact committed CLI tree that was built.
# A changed or untracked CLI source would make that claim unverifiable, while
# writing the build into cli/bin would mutate the source checkout.
git -C "$agentops_root" rev-parse --is-inside-work-tree >/dev/null 2>&1 || \
  die "AgentOps checkout is not a Git worktree: $agentops_root"
git -C "$agentops_root" diff --quiet HEAD -- cli || \
  die "committed CLI subtree does not match working CLI subtree"
if [ -n "$(git -C "$agentops_root" ls-files --others --exclude-standard -- cli)" ]; then
  die "untracked CLI source could affect the ao build"
fi
ao_source_commit="$(git -C "$agentops_root" rev-parse HEAD)"
ao_cli_tree="$(git -C "$agentops_root" rev-parse HEAD:cli)"
ao_build_version="$(git -C "$agentops_root" describe --tags --always "$ao_source_commit")"

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
bd_repository="${fields[7]}"
bd_ref="${fields[8]}"
bd_version="${fields[9]}"
bd_source_commit="${fields[10]}"

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
  local version="$3"
  local commit="$4"
  local destination="$5"
  git init -q "$destination"
  git -C "$destination" remote add origin "$repository"
  # Fetch the immutable lock identity directly. A qualified pair may pin a
  # commit behind a moving branch; a depth-one fetch of that branch stops
  # materializing the pair as soon as the branch advances.
  git -C "$destination" fetch -q --depth=1 origin "$commit" || \
    die "cannot fetch exact commit $commit locked from $ref"
  # Official release builds derive their runtime version from an exact local
  # tag. Fetch that immutable tag ref explicitly, then prove it dereferences
  # to the separately locked commit before allowing Make to observe it.
  if [ "$ref" = "v$version" ]; then
    git -C "$destination" fetch -q --depth=1 origin \
      "refs/tags/$ref:refs/tags/$ref" || \
      die "cannot fetch locked release tag $ref"
    [ "$(git -C "$destination" rev-parse "$ref^{}")" = "$commit" ] || \
      die "locked release tag $ref does not dereference to $commit"
    # A commit-first shallow fetch can leave the verified annotated tag outside
    # `git describe`'s shallow history walk. Recreate the local tag only after
    # dereference verification so the upstream Makefile sees the proven release
    # identity without consulting a mutable remote ref.
    git -C "$destination" tag -f "$ref" "$commit"
  fi
  git -C "$destination" checkout -q --detach "$commit"
  [ "$(git -C "$destination" rev-parse HEAD)" = "$commit" ] || \
    die "source checkout did not resolve exact commit $commit"
}

checkout_exact "$gc_repository" "$gc_ref" "$gc_version" "$gc_source_commit" "$sources/gc"
checkout_exact "$bd_repository" "$bd_ref" "$bd_version" "$bd_source_commit" "$sources/bd"

make -s -C "$sources/gc" build
make -s -C "$sources/bd" build
mkdir -p "$stage/bin"
install -m 0755 "$sources/gc/bin/gc" "$stage/bin/gc"
install -m 0755 "$sources/bd/bd" "$stage/bin/bd"
go -C "$agentops_root/cli" build -ldflags "-X main.version=$ao_build_version" -o "$stage/bin/ao" ./cmd/ao

python3 - "$stage/bin/gc" "$stage/bin/bd" "$stage/bin/ao" "$selection" "$stage/toolchain.json" "$ao_source_commit" "$ao_cli_tree" <<'PY'
import hashlib
import json
import re
import subprocess
import sys

gc_path, bd_path, ao_path, selection_json, receipt_path, ao_source_commit, ao_cli_tree = sys.argv[1:]
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

ao = json.loads(run([ao_path, "-o", "json", "version"]))
ao_version = ao.get("version")
if not isinstance(ao_version, str) or not ao_version:
    raise SystemExit(f"cannot parse built ao version: {ao}")


def digest(path):
    value = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


receipt = {
    "schema_version": 2,
    "pair": selected,
    "runtime": {
        "gc": {"path": "bin/gc", "version": gc["version"], "commit": gc["commit"], "sha256": digest(gc_path)},
        "bd": {"path": "bin/bd", "version": match.group(1), "commit": match.group(2), "sha256": digest(bd_path)},
        "ao": {"path": "bin/ao", "sha256": digest(ao_path), "source_commit": ao_source_commit, "cli_tree": ao_cli_tree, "build_version": ao_version},
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
