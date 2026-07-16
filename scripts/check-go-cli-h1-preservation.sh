#!/usr/bin/env bash

# shellcheck source=scripts/lib/preamble.sh
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/check-go-cli-h1-preservation.sh <receipt-directory>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import hashlib
import json
import os
import pathlib
import re
import stat
import sys

receipt = pathlib.Path(sys.argv[1])


def fail(message: str) -> None:
    print(f"go-cli H1 preservation FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


if not receipt.is_dir():
    fail(f"receipt directory does not exist: {receipt}")
if stat.S_IMODE(receipt.stat().st_mode) != 0o700:
    fail("receipt directory mode must be 0700")

required = {
    "metadata.json",
    "working-tree.patch",
    "status-z.bin",
    "untracked-z.bin",
    "payload-digests.json",
    "restore-drill.json",
    "disposal.json",
    "sha256sums",
}
files = {
    path.relative_to(receipt).as_posix()
    for path in receipt.rglob("*")
    if path.is_file()
}
missing = sorted(required - files)
if missing:
    fail("missing required files: " + ", ".join(missing))

try:
    metadata = json.loads((receipt / "metadata.json").read_text())
    restore = json.loads((receipt / "restore-drill.json").read_text())
    disposal = json.loads((receipt / "disposal.json").read_text())
except (OSError, UnicodeError, json.JSONDecodeError) as error:
    fail(f"invalid receipt JSON: {error}")

if metadata.get("schema_version") != 1 or metadata.get("transaction") != "preserve-h1-before-wip-reconcile.v1":
    fail("metadata transaction is not preserve-h1-before-wip-reconcile.v1")
for field in ("original_head", "frozen_remote_base"):
    if not re.fullmatch(r"[0-9a-f]{40}", str(metadata.get(field, ""))):
        fail(f"metadata {field} is not a complete Git SHA")
if not re.fullmatch(r"[0-9a-f]{40}", str(metadata.get("original_patch_id_stable", ""))):
    fail("metadata original_patch_id_stable is not complete")

expected_comparisons = {
    "head",
    "stable_patch_id",
    "status_z_bytes",
    "status_z_sha256",
    "untracked_payload_digests",
    "untracked_z_bytes",
    "untracked_z_sha256",
    "working_tree_patch_bytes",
    "working_tree_patch_sha256",
}
comparisons = restore.get("comparisons", {})
if restore.get("schema_version") != 1 or restore.get("status") != "VERIFIED":
    fail("restore drill is not VERIFIED")
if any(comparisons.get(name) is not True for name in expected_comparisons):
    fail("restore drill comparisons are incomplete or false")
if disposal.get("schema_version") != 1 or disposal.get("status") != "REMOVED":
    fail("disposable restore worktree was not recorded as REMOVED")
disposed_path = pathlib.Path(str(disposal.get("path", "")))
if not disposed_path.is_absolute() or disposed_path.exists():
    fail("disposable restore worktree path is not absent")

raw_sums = (receipt / "sha256sums").read_bytes()
records = raw_sums.split(b"\0")
if not records or records[-1] != b"":
    fail("sha256sums must be NUL terminated")
records.pop()
seen: set[str] = set()
for record in records:
    match = re.fullmatch(rb"([0-9a-f]{64})  ([^\0]+)", record)
    if match is None:
        fail("sha256sums contains a malformed record")
    expected = match.group(1).decode()
    relative = match.group(2).decode()
    path = pathlib.PurePosixPath(relative)
    if path.is_absolute() or ".." in path.parts or relative in seen or relative == "sha256sums":
        fail(f"sha256sums contains an unsafe or duplicate path: {relative}")
    seen.add(relative)
    target = receipt.joinpath(*path.parts)
    if not target.is_file() or target.is_symlink():
        fail(f"sha256sums target is missing or a symlink: {relative}")
    actual = hashlib.sha256(target.read_bytes()).hexdigest()
    if actual != expected:
        fail(f"digest mismatch: {relative}")

expected_files = files - {"sha256sums"}
if seen != expected_files:
    fail("sha256sums inventory does not exactly cover receipt files")

print(f"go-cli H1 preservation PASS: {receipt}")
PY
