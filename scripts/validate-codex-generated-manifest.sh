#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_ROOT="${1:-$REPO_ROOT/skills-codex}"

if [[ "$SKILLS_ROOT" != /* ]]; then
  SKILLS_ROOT="$(cd "$SKILLS_ROOT" && pwd)"
fi

[[ -d "$SKILLS_ROOT" ]] || {
  echo "skills-codex root not found: $SKILLS_ROOT" >&2
  exit 1
}

export SKILLS_ROOT
python3 - <<'PY'
import hashlib
import json
import os
import pathlib
import sys

skills_root = pathlib.Path(os.environ["SKILLS_ROOT"]).resolve()
manifest_path = skills_root / ".agentops-manifest.json"
marker_name = ".agentops-generated.json"

if not manifest_path.exists():
    print(f"Codex artifact manifest missing: {manifest_path}", file=sys.stderr)
    sys.exit(1)

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
entries = manifest.get("skills", [])
entry_by_name = {entry.get("name"): entry for entry in entries if entry.get("name")}
failures = []

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

skill_dirs = []
for skill_dir in sorted(p for p in skills_root.iterdir() if p.is_dir()):
    if (skill_dir / "SKILL.md").exists():
        skill_dirs.append(skill_dir)

if len(skill_dirs) != len(entry_by_name):
    failures.append(
        f"Codex artifact manifest drift detected: {len(skill_dirs)} skill directories, {len(entry_by_name)} manifest entries"
    )

skill_names = {skill_dir.name for skill_dir in skill_dirs}
manifest_names = set(entry_by_name)
for missing in sorted(skill_names - manifest_names):
    failures.append(f"Missing manifest entry for Codex skill artifact: {missing}")
for extra in sorted(manifest_names - skill_names):
    failures.append(f"Manifest references unknown Codex skill artifact: {extra}")

for skill_dir in skill_dirs:
    marker_path = skill_dir / marker_name
    if not marker_path.exists():
        failures.append(f"Missing Codex artifact marker: {skill_dir.relative_to(skills_root).as_posix()}/{marker_name}")
        continue

    entry = entry_by_name.get(skill_dir.name)
    if entry is None:
        continue

    marker = json.loads(marker_path.read_text(encoding="utf-8"))
    generated_hash = hash_tree(skill_dir)
    expected_source_skill = f"skills/{skill_dir.name}"

    if entry.get("source_skill") != expected_source_skill:
        failures.append(f"{skill_dir.name}: manifest source_skill mismatch ({entry.get('source_skill')} != {expected_source_skill})")
    if marker.get("source_skill") != expected_source_skill:
        failures.append(f"{skill_dir.name}: marker source_skill mismatch ({marker.get('source_skill')} != {expected_source_skill})")
    if entry.get("generated_hash") != generated_hash:
        failures.append(f"{skill_dir.name}: manifest generated_hash drift detected")
    if marker.get("generated_hash") != generated_hash:
        failures.append(f"{skill_dir.name}: marker generated_hash drift detected")
    if marker.get("source_hash") != entry.get("source_hash"):
        failures.append(f"{skill_dir.name}: marker/source hash mismatch")

if failures:
    for failure in failures:
        print(failure, file=sys.stderr)
    sys.exit(1)

print(f"Codex artifact manifest OK: {len(skill_dirs)} skill(s).")
PY
