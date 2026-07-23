#!/usr/bin/env bash
set -euo pipefail

die() { printf 'materialize-toolchain: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'EOF'
Usage: materialize-toolchain.sh --output DIR [--pair ID] [--lock FILE] [--describe]

Fetch only the exact official Gas City and Beads pair selected from the lock.
Official prebuilt release archives are downloaded, verified against the pinned
official checksums, and installed as bin/gc, bin/bd, and toolchain.json. No
source is compiled and no fork or custom binary is produced.

Set AGENTOPS_GC_ARCHIVE_CACHE=DIR to install pre-placed release assets from DIR
(each asset named exactly as its upstream release asset) instead of downloading.
The checksum verification chain is identical in both modes.
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

for tool in python3 curl tar; do command -v "$tool" >/dev/null || die "$tool is required"; done
if command -v shasum >/dev/null; then
  sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }
elif command -v sha256sum >/dev/null; then
  sha256_of() { sha256sum "$1" | awk '{print $1}'; }
else
  die "shasum or sha256sum is required"
fi
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
    for field in ("repository", "ref", "version", "source_commit", "runtime_commit", "release_checksum_asset", "release_checksum_sha256"):
        value = item.get(field) if isinstance(item, dict) else None
        if not isinstance(value, str) or not value:
            raise SystemExit(f"invalid {name}.{field}")
    if not re.fullmatch(r"[0-9a-f]{40}", item["source_commit"]):
        raise SystemExit(f"invalid {name}.source_commit")
    if not re.fullmatch(r"[0-9a-f]{7,40}", item["runtime_commit"]):
        raise SystemExit(f"invalid {name}.runtime_commit")
    if not item["source_commit"].startswith(item["runtime_commit"]):
        raise SystemExit(f"{name}.runtime_commit is not a prefix of source_commit")
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

# Detect the running platform and map it to the official release archive tag.
os_raw="$(uname -s)"; arch_raw="$(uname -m)"
case "$os_raw" in
  Darwin) os_tag="darwin" ;;
  Linux) os_tag="linux" ;;
  *) die "unsupported operating system: $os_raw (supported: Darwin, Linux)" ;;
esac
case "$arch_raw" in
  arm64|aarch64) arch_tag="arm64" ;;
  x86_64|amd64) arch_tag="amd64" ;;
  *) die "unsupported architecture: $arch_raw (supported: arm64/aarch64, x86_64/amd64)" ;;
esac
platform="${os_tag}_${arch_tag}"

# Project the exact download coordinates for each tool from the verified lock.
meta=()
while IFS= read -r field; do
  meta[${#meta[@]}]="$field"
done < <(python3 - "$selection" <<'PY'
import json, sys
s = json.loads(sys.argv[1])
def coords(item):
    repo = item["repository"]
    slug = repo[len("https://github.com/"):] if repo.startswith("https://github.com/") else repo
    if slug.endswith(".git"):
        slug = slug[:-len(".git")]
    name = slug.rsplit("/", 1)[-1]
    return [slug, item["ref"], item["version"], name, item["release_checksum_asset"], item["release_checksum_sha256"]]
for value in coords(s["gc"]) + coords(s["bd"]):
    print(value)
PY
)
[ "${#meta[@]}" -eq 12 ] || die "selected pair projection is incomplete"

parent="$(dirname "$output")"
mkdir -p "$parent"
sources="$(mktemp -d "$parent/.gc-sources.XXXXXX")"
stage="$(mktemp -d "$parent/.gc-stage.XXXXXX")"
cleanup() { rm -rf "$sources" "$stage"; }
trap cleanup EXIT
mkdir -p "$stage/bin"

fetch_asset() {
  local slug="$1" ref="$2" asset="$3" dest="$4"
  if [ -n "${AGENTOPS_GC_ARCHIVE_CACHE:-}" ]; then
    local cached="$AGENTOPS_GC_ARCHIVE_CACHE/$asset"
    [ -f "$cached" ] || die "cached asset not found: $cached"
    cp "$cached" "$dest"
  else
    curl -fsSL -o "$dest" "https://github.com/$slug/releases/download/$ref/$asset" || die "cannot download $asset from $slug $ref"
  fi
}

install_tool() {
  local slug="$1" ref="$2" version="$3" name="$4" ck_asset="$5" ck_sha="$6" binname="$7"
  local archive="${name}_${version}_${platform}.tar.gz"

  # 1. Fetch the official checksums asset and verify it against the lock.
  local ck_file="$sources/${name}.checksums"
  fetch_asset "$slug" "$ref" "$ck_asset" "$ck_file"
  local got_ck_sha; got_ck_sha="$(sha256_of "$ck_file")"
  [ "$got_ck_sha" = "$ck_sha" ] || die "$name checksums asset $ck_asset sha256 mismatch: got $got_ck_sha want $ck_sha"

  # 2. Fetch the platform archive and verify it against the verified checksums.
  local archive_path="$sources/$archive"
  fetch_asset "$slug" "$ref" "$archive" "$archive_path"
  local expected_sha
  expected_sha="$(awk -v want="$archive" '{f=$2; sub(/^\*/,"",f); if (f==want) print $1}' "$ck_file" | head -n1)"
  [ -n "$expected_sha" ] || die "$name archive $archive is not listed in verified checksums $ck_asset (unsupported platform?)"
  local got_sha; got_sha="$(sha256_of "$archive_path")"
  [ "$got_sha" = "$expected_sha" ] || die "$name archive $archive sha256 mismatch: got $got_sha want $expected_sha"

  # 3. Extract and install the exact binary.
  local extract="$sources/${name}.extract"
  mkdir -p "$extract"
  tar -xzf "$archive_path" -C "$extract" || die "cannot extract $archive"
  local bin_src
  bin_src="$(find "$extract" -type f -name "$binname" | LC_ALL=C sort | head -n1)"
  [ -n "$bin_src" ] || die "$binname binary not found inside $archive"
  cp "$bin_src" "$stage/bin/$binname"
  chmod 0755 "$stage/bin/$binname"
}

install_tool "${meta[0]}" "${meta[1]}" "${meta[2]}" "${meta[3]}" "${meta[4]}" "${meta[5]}" gc
install_tool "${meta[6]}" "${meta[7]}" "${meta[8]}" "${meta[9]}" "${meta[10]}" "${meta[11]}" bd

# 4. Verify each installed binary reports the locked version and commit, then
#    write the toolchain receipt. Any identity drift is a hard error.
python3 - "$stage/bin/gc" "$stage/bin/bd" "$selection" "$stage/toolchain.json" <<'PY'
import hashlib, json, re, subprocess, sys
gc_path, bd_path, selection, receipt_path = sys.argv[1:]
selected = json.loads(selection)
def run(argv):
    result = subprocess.run(argv, check=False, capture_output=True, text=True)
    if result.returncode:
        raise SystemExit((result.stderr or result.stdout).strip() or f"{argv[0]} version failed")
    return result.stdout
def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()
def commit_hex(text):
    match = re.match(r"[0-9a-f]+", text.strip().lower())
    return match.group(0) if match else ""
def verify(name, version, reported_commit):
    locked = selected[name]
    if version != locked["version"]:
        raise SystemExit(f"installed {name} version {version!r} differs from locked {locked['version']!r}")
    if not reported_commit:
        raise SystemExit(f"installed {name} binary reports no commit")
    if not locked["source_commit"].startswith(reported_commit):
        raise SystemExit(f"installed {name} commit {reported_commit!r} is not the locked source commit {locked['source_commit']!r}")
    if not reported_commit.startswith(locked["runtime_commit"]):
        raise SystemExit(f"installed {name} commit {reported_commit!r} does not match locked runtime commit {locked['runtime_commit']!r}")

gc = json.loads(run([gc_path, "version", "--json"]))
gc_version = str(gc.get("version", ""))
gc_commit = commit_hex(str(gc.get("commit", "")))
verify("gc", gc_version, gc_commit)

bd_text = run([bd_path, "version"])
match = re.search(r"^bd version (\S+) \(([0-9a-f]+)", bd_text, re.M)
if match is None:
    raise SystemExit("cannot parse Beads version output")
bd_version = match.group(1)
bd_commit = commit_hex(match.group(2))
verify("bd", bd_version, bd_commit)

receipt = {"schema_version": 2, "pair": selected, "runtime": {"gc": {"path": "bin/gc", "version": gc_version, "commit": gc_commit, "sha256": sha(gc_path)}, "bd": {"path": "bin/bd", "version": bd_version, "commit": bd_commit, "sha256": sha(bd_path)}}}
with open(receipt_path, "w", encoding="utf-8") as f:
    json.dump(receipt, f, indent=2, sort_keys=True); f.write("\n")
PY

if [ -d "$output" ]; then rmdir "$output"; fi
mv "$stage" "$output"
stage="$parent/.gc-stage.consumed"
trap - EXIT
rm -rf "$sources"
printf 'materialized %s (%s) at %s\n' "${meta[3]}" "$platform" "$output"
