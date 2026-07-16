#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-go-cli-h1-preservation.sh"
  RECEIPT="$BATS_TEST_TMPDIR/receipt"
  python3 - "$RECEIPT" <<'PY'
import hashlib
import json
import pathlib
import sys

receipt = pathlib.Path(sys.argv[1])
receipt.mkdir(mode=0o700)
metadata = {
    "schema_version": 1,
    "transaction": "preserve-h1-before-wip-reconcile.v1",
    "original_head": "a" * 40,
    "frozen_remote_base": "b" * 40,
    "original_patch_id_stable": "c" * 40,
}
comparisons = {
    "head": True,
    "stable_patch_id": True,
    "status_z_bytes": True,
    "status_z_sha256": True,
    "untracked_payload_digests": True,
    "untracked_z_bytes": True,
    "untracked_z_sha256": True,
    "working_tree_patch_bytes": True,
    "working_tree_patch_sha256": True,
}
(receipt / "metadata.json").write_text(json.dumps(metadata))
(receipt / "restore-drill.json").write_text(json.dumps({"schema_version": 1, "status": "VERIFIED", "comparisons": comparisons}))
(receipt / "disposal.json").write_text(json.dumps({"schema_version": 1, "status": "REMOVED", "path": str(receipt.parent / "disposed-worktree")}))
(receipt / "payload-digests.json").write_text("{}")
(receipt / "working-tree.patch").write_bytes(b"patch\n")
(receipt / "status-z.bin").write_bytes(b"status\0")
(receipt / "untracked-z.bin").write_bytes(b"")
records = []
for path in sorted(receipt.iterdir()):
    if path.name == "sha256sums":
        continue
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    records.append(f"{digest}  {path.name}".encode() + b"\0")
(receipt / "sha256sums").write_bytes(b"".join(records))
PY
}

@test "H1 preservation checker accepts a complete restorable receipt" {
  run "$CHECKER" "$RECEIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"H1 preservation PASS"* ]]
}

@test "H1 preservation checker rejects receipt tampering" {
  printf 'tampered\n' >>"$RECEIPT/working-tree.patch"
  run "$CHECKER" "$RECEIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"digest mismatch: working-tree.patch"* ]]
}
