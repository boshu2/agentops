#!/usr/bin/env bash
set -euo pipefail

# Regenerate generated_hash values in skills-codex manifest and markers.
# Run after any change to skills-codex/ files to fix artifact metadata drift.
#
# Usage:
#   scripts/regen-codex-hashes.sh              # update all drifted hashes
#   scripts/regen-codex-hashes.sh --check      # dry-run: report drift without fixing

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_ROOT="$REPO_ROOT/skills-codex"
CHECK_ONLY=false

for arg in "$@"; do
  case "$arg" in
    --check) CHECK_ONLY=true ;;
    -h|--help)
      echo "Usage: scripts/regen-codex-hashes.sh [--check]"
      echo "  --check  Report drift without fixing"
      exit 0
      ;;
    *)
      echo "Unknown arg: $arg" >&2
      exit 2
      ;;
  esac
done

[[ -d "$SKILLS_ROOT" ]] || {
  echo "skills-codex root not found: $SKILLS_ROOT" >&2
  exit 1
}

export SKILLS_ROOT CHECK_ONLY
python3 - <<'PY'
import hashlib
import json
import os
import pathlib
import sys

skills_root = pathlib.Path(os.environ["SKILLS_ROOT"]).resolve()
check_only = os.environ.get("CHECK_ONLY") == "true"
manifest_path = skills_root / ".agentops-manifest.json"
marker_name = ".agentops-generated.json"

if not manifest_path.exists():
    print(f"Codex artifact manifest missing: {manifest_path}", file=sys.stderr)
    sys.exit(1)

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
entries = manifest.get("skills", [])
entry_by_name = {entry.get("name"): entry for entry in entries if entry.get("name")}


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    return sha256_bytes(path.read_bytes())


def hash_tree(root: pathlib.Path) -> str:
    rows = []
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        if path.name in {".agentops-manifest.json", marker_name, ".DS_Store"}:
            continue
        if "__pycache__" in path.parts:
            continue
        if path.suffix == ".pyc":
            continue
        rel = path.relative_to(root).as_posix()
        rows.append(f"{rel}\t{sha256_file(path)}\n")
    return sha256_bytes("".join(rows).encode("utf-8"))


updated = []
for skill_dir in sorted(p for p in skills_root.iterdir() if p.is_dir()):
    if not (skill_dir / "SKILL.md").exists():
        continue

    name = skill_dir.name
    new_hash = hash_tree(skill_dir)
    changed = False

    # Check/update manifest entry
    if name in entry_by_name and entry_by_name[name].get("generated_hash") != new_hash:
        changed = True
        if not check_only:
            entry_by_name[name]["generated_hash"] = new_hash

    # Check/update marker
    marker_path = skill_dir / marker_name
    if marker_path.exists():
        marker = json.loads(marker_path.read_text(encoding="utf-8"))
        if marker.get("generated_hash") != new_hash:
            changed = True
            if not check_only:
                marker["generated_hash"] = new_hash
                marker_path.write_text(json.dumps(marker, indent=2) + "\n", encoding="utf-8")

    if changed:
        updated.append(name)

if not check_only:
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")

if updated:
    verb = "Drifted" if check_only else "Updated"
    print(f"{verb} hashes for {len(updated)} skill(s): {', '.join(updated)}")
    if check_only:
        sys.exit(1)
else:
    print("All hashes up to date.")
PY
