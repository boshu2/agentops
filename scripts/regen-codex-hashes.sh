#!/usr/bin/env bash
set -euo pipefail

# Regenerate generated_hash values in skills-codex manifest and markers.
# Run after any change to skills-codex/ files to fix artifact metadata drift.
#
# Usage:
#   scripts/regen-codex-hashes.sh                    # update all drifted hashes
#   scripts/regen-codex-hashes.sh --check            # dry-run: report drift without fixing
#   scripts/regen-codex-hashes.sh --only foo,bar     # only touch skills foo and bar
#
# --only scopes the per-skill loop to the named skills (comma- and/or
# repeat-separated). Skills outside the set are skipped entirely, so a PR that
# changes one skill no longer sweeps unrelated pre-existing hash drift into its
# diff. Combine with --check to scope the drift report the same way.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_ROOT="${SKILLS_ROOT:-$REPO_ROOT/skills-codex}"
CHECK_ONLY=false
ONLY_SKILLS=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) CHECK_ONLY=true ;;
    --only)
      shift
      [[ $# -gt 0 ]] || { echo "--only requires a skill list" >&2; exit 2; }
      ONLY_SKILLS="${ONLY_SKILLS:+$ONLY_SKILLS,}$1"
      ;;
    --only=*) ONLY_SKILLS="${ONLY_SKILLS:+$ONLY_SKILLS,}${1#--only=}" ;;
    -h|--help)
      echo "Usage: scripts/regen-codex-hashes.sh [--check] [--only <skill[,skill,...]>]"
      echo "  --check               Report drift without fixing"
      echo "  --only <skill,...>    Limit to the named skills (scope a single-skill PR)"
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 2
      ;;
  esac
  shift
done

[[ -d "$SKILLS_ROOT" ]] || {
  echo "skills-codex root not found: $SKILLS_ROOT" >&2
  exit 1
}

export SKILLS_ROOT CHECK_ONLY ONLY_SKILLS
python3 - <<'PY'
import hashlib
import json
import os
import pathlib
import sys

skills_root = pathlib.Path(os.environ["SKILLS_ROOT"]).resolve()
check_only = os.environ.get("CHECK_ONLY") == "true"
scope = {s for s in os.environ.get("ONLY_SKILLS", "").split(",") if s.strip()}
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


def source_is_spine(source_dir: pathlib.Path) -> bool:
    """True iff the SOURCE skill declares top-level ``spine: true`` in its
    SKILL.md frontmatter. Ambient (non-spine) skills are FROZEN
    (age-focus-membrane-bookkeeper-m1wg.18): their recorded source_hash is
    authoritative and is NOT recomputed from a (possibly edited) source, so
    editing an ambient skill never restains its twin's hash record. Detection
    mirrors scripts/check-spine-integrity.sh — a bare top-level ``spine: true``
    line inside the leading frontmatter block."""
    skill_md = source_dir / "SKILL.md"
    if not skill_md.is_file():
        return False
    in_frontmatter = False
    for line in skill_md.read_text(encoding="utf-8").splitlines():
        if line.strip() == "---":
            if not in_frontmatter:
                in_frontmatter = True
                continue
            break  # end of the leading frontmatter block
        if in_frontmatter and line.strip() == "spine: true":
            return True
    return False


repo_root = skills_root.parent
source_root = repo_root / "skills"

updated = []
for skill_dir in sorted(p for p in skills_root.iterdir() if p.is_dir()):
    if not (skill_dir / "SKILL.md").exists():
        continue

    name = skill_dir.name
    if scope and name not in scope:
        continue
    new_hash = hash_tree(skill_dir)

    # Source-side hash: tree-hash skills/<name>/ if it exists. A codex skill
    # without a source twin (rare; pure-codex skill) keeps source_hash empty.
    source_dir = source_root / name
    new_source_hash = hash_tree(source_dir) if source_dir.is_dir() and (source_dir / "SKILL.md").exists() else ""

    # Freeze ambient (non-spine) source_hash: do NOT recompute it from a
    # (possibly edited) source. The stored source_hash stays authoritative so an
    # ambient skill edit never restains its twin's hash record. generated_hash is
    # still recomputed from the twin below, so a real twin-content change is still
    # caught (age-focus-membrane-bookkeeper-m1wg.18).
    if new_source_hash and not source_is_spine(source_dir):
        new_source_hash = ""

    changed = False

    # Check/update manifest entry (both generated_hash AND source_hash)
    if name in entry_by_name:
        entry = entry_by_name[name]
        if entry.get("generated_hash") != new_hash:
            changed = True
            if not check_only:
                entry["generated_hash"] = new_hash
        if new_source_hash and entry.get("source_hash") != new_source_hash:
            changed = True
            if not check_only:
                entry["source_hash"] = new_source_hash

    # Check/update marker (both generated_hash AND source_hash)
    marker_path = skill_dir / marker_name
    if marker_path.exists():
        marker = json.loads(marker_path.read_text(encoding="utf-8"))
        marker_changed = False
        if marker.get("generated_hash") != new_hash:
            marker_changed = True
            if not check_only:
                marker["generated_hash"] = new_hash
        if new_source_hash and marker.get("source_hash") != new_source_hash:
            marker_changed = True
            if not check_only:
                marker["source_hash"] = new_source_hash
        if marker_changed:
            changed = True
            if not check_only:
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
