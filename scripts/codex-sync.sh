#!/usr/bin/env bash
# codex-sync.sh — generate parity_only Codex twins from their source skills.
#
# A parity_only twin is a pure FUNCTION of its source skill: its SKILL.md is a
# slim frontmatter (name + description copied from source) plus a fixed pointer
# body that names skills/<name>/SKILL.md as the source of truth; its prompt.md
# is the standard pointer template. Because the twin carries no source *content*
# beyond name+description, source body edits never require a hand-edit to the
# twin — which is what kills the "add/touch a skill -> chase ~5 codex gates
# serially" whack-a-mole (regen-all.sh historically only rehashed EXISTING
# twins; it could not author a missing one).
#
# This generator authors the COMPLETE twin for any source skill that lacks one:
# the two body files, the per-skill marker, and all three catalog surfaces
# (manifest .skills[], manifest .codex_override_catalog.skills[], and
# skills-codex-overrides/catalog.json .skills[]), then fixes every hash. It is
# idempotent: a source skill that already has a complete, registered twin is
# left untouched.
#
# bespoke twins (hand-authored Codex profiles) are the opt-out: they are never
# generated or overwritten.
#
# Usage:
#   scripts/codex-sync.sh                 # generate any missing parity twin (writes)
#   scripts/codex-sync.sh --check         # report missing/incomplete twins; exit 1 on drift (no writes)
#   scripts/codex-sync.sh --only a,b      # scope to skills a and b
#   scripts/codex-sync.sh <name>          # scope to a single skill
#
# Wired into scripts/regen-all.sh ahead of the codex-hash step.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK_ONLY=false
ONLY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) CHECK_ONLY=true; shift ;;
    --only) ONLY="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    -*) echo "Unknown flag: $1" >&2; exit 2 ;;
    *) ONLY="$1"; shift ;;
  esac
done

export ROOT CHECK_ONLY ONLY

python3 - <<'PY'
import hashlib
import json
import os
import pathlib
import sys

import yaml

root = pathlib.Path(os.environ["ROOT"]).resolve()
check_only = os.environ.get("CHECK_ONLY") == "true"
scope = {s.strip() for s in os.environ.get("ONLY", "").split(",") if s.strip()}

source_root = root / "skills"
codex_root = root / "skills-codex"
manifest_path = codex_root / ".agentops-manifest.json"
overrides_catalog_path = root / "skills-codex-overrides" / "catalog.json"
marker_name = ".agentops-generated.json"

if not manifest_path.exists():
    print(f"FATAL: missing manifest {manifest_path}", file=sys.stderr)
    sys.exit(1)
if not overrides_catalog_path.exists():
    print(f"FATAL: missing overrides catalog {overrides_catalog_path}", file=sys.stderr)
    sys.exit(1)

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
overrides_catalog = json.loads(overrides_catalog_path.read_text(encoding="utf-8"))

manifest_skills = manifest.setdefault("skills", [])
manifest_catalog = manifest.setdefault("codex_override_catalog", {})
manifest_catalog_skills = manifest_catalog.setdefault("skills", [])
overrides_skills = overrides_catalog.setdefault("skills", [])

# bespoke = the opt-out set (never generate/overwrite). Read from both catalogs.
bespoke = {
    e.get("name")
    for e in (manifest_catalog_skills + overrides_skills)
    if e.get("treatment") == "bespoke"
}


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def hash_tree_with(root_dir: pathlib.Path, overlay: dict[str, bytes]) -> str:
    """Tree-hash of a skill dir, with `overlay` (relpath -> bytes) substituted
    in. Mirrors regen-codex-hashes.sh hash_tree exactly (excludes manifest,
    marker, .DS_Store, __pycache__, *.pyc)."""
    files: dict[str, bytes] = {}
    if root_dir.is_dir():
        for path in root_dir.rglob("*"):
            if not path.is_file():
                continue
            if path.name in {".agentops-manifest.json", marker_name, ".DS_Store"}:
                continue
            if "__pycache__" in path.parts or path.suffix == ".pyc":
                continue
            files[path.relative_to(root_dir).as_posix()] = path.read_bytes()
    files.update(overlay)
    rows = [f"{rel}\t{sha256_bytes(data)}\n" for rel, data in sorted(files.items())]
    return sha256_bytes("".join(rows).encode("utf-8"))


def parse_frontmatter(skill_md: pathlib.Path) -> dict:
    text = skill_md.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return {}
    parts = text.split("---", 2)
    if len(parts) < 3:
        return {}
    return yaml.safe_load(parts[1]) or {}


def twin_skill_md(name: str, description: str) -> bytes:
    fm = {"name": name, "description": description}
    front = yaml.safe_dump(fm, sort_keys=False, allow_unicode=True, width=10_000).strip()
    body = (
        f"# {name} (Codex twin)\n\n"
        f"The canonical skill is `skills/{name}/SKILL.md` — that is the source of "
        f"truth. Read it there and follow it.\n\n"
        f"Run via Codex with `$`-prefixed skill invocations and shell. Use "
        f"`~/.codex/` paths. Return evidence for each step.\n"
    )
    return f"---\n{front}\n---\n{body}".encode("utf-8")


def twin_prompt_md(name: str, description: str) -> bytes:
    return (
        f"# {name}\n\n"
        f"{description}\n\n"
        f"## Instructions\n\n"
        f"Load and follow the skill instructions from the sibling `SKILL.md` file "
        f"for this skill.\n"
        f"Then read local files in `references/` and `scripts/` when needed.\n"
    ).encode("utf-8")


def upsert(entries: list, name: str, entry: dict) -> bool:
    """Insert or replace by name. Returns True if the list changed. Appends in
    place (no global re-sort) to keep the diff minimal — matches the append-only
    behavior of register-new-codex-skill.sh and avoids reordering the existing
    catalog on every new skill."""
    for i, e in enumerate(entries):
        if e.get("name") == name:
            if e == entry:
                return False
            entries[i] = entry
            return True
    entries.append(entry)
    return True


# Discover source skills (ground truth), skip non-skill dirs and bespoke.
source_skills = sorted(
    p.name
    for p in source_root.iterdir()
    if p.is_dir() and not p.name.startswith("_") and (p / "SKILL.md").exists()
)

drift = []
generated = []

for name in source_skills:
    if name in bespoke:
        continue
    if scope and name not in scope:
        continue

    twin_dir = codex_root / name
    skill_md = twin_dir / "SKILL.md"
    prompt_md = twin_dir / "prompt.md"
    marker_path = twin_dir / marker_name

    fm = parse_frontmatter(source_root / name / "SKILL.md")
    description = str(fm.get("description", "")).strip()

    desired_skill = twin_skill_md(name, description)
    desired_prompt = twin_prompt_md(name, description)

    # A twin is "complete" iff its body files + marker exist AND it is registered
    # in the gate-enforced 1:1 surface (skills-codex-overrides/catalog.json — the
    # surface validate-codex-override-coverage.sh holds to source skills exactly).
    # If complete, leave it ENTIRELY untouched: never clobber an existing,
    # possibly-richer hand-tended twin body or relabel its marker. The bloated
    # manifest .codex_override_catalog (which also carries stale phantom entries)
    # is downstream and is NOT a generation trigger.
    in_ocat = any(e.get("name") == name for e in overrides_skills)
    complete = (
        skill_md.exists()
        and prompt_md.exists()
        and marker_path.exists()
        and in_ocat
    )
    if complete:
        continue

    drift.append(name)
    if check_only:
        continue

    # --- Author the missing twin ---
    twin_dir.mkdir(parents=True, exist_ok=True)
    if not skill_md.exists():
        skill_md.write_bytes(desired_skill)
    if not prompt_md.exists():
        prompt_md.write_bytes(desired_prompt)

    source_hash = hash_tree_with(source_root / name, {})
    generated_hash = hash_tree_with(twin_dir, {})

    marker_path.write_text(
        json.dumps(
            {
                "generator": "codex-sync",
                "source_skill": f"skills/{name}",
                "layout": "modular",
                "source_hash": source_hash,
                "generated_hash": generated_hash,
            },
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )

    upsert(
        manifest_skills,
        name,
        {
            "name": name,
            "source_skill": f"skills/{name}",
            "source_hash": source_hash,
            "generated_hash": generated_hash,
        },
    )
    catalog_entry = {
        "name": name,
        "treatment": "parity_only",
        "wave": "catalog-parity",
        "reason": (
            f"Auto-generated parity twin (codex-sync): skills/{name} is the source "
            f"of truth; no durable Codex-specific divergence yet."
        ),
    }
    upsert(manifest_catalog_skills, name, catalog_entry)
    upsert(overrides_skills, name, catalog_entry)
    generated.append(name)

if check_only:
    if drift:
        print(f"codex-sync drift: {len(drift)} source skill(s) lack a complete twin:")
        for n in drift:
            print(f"  - {n}")
        sys.exit(1)
    print("codex-sync: all parity twins present and registered.")
    sys.exit(0)

if generated:
    # Recompute the embedded catalog hash (same algorithm as
    # register-new-codex-skill.sh) so the manifest catalog stays self-consistent.
    catalog_for_hash = json.dumps(
        {k: v for k, v in manifest_catalog.items() if k != "skills"}
        | {"skills": manifest_catalog_skills},
        sort_keys=True,
    ).encode("utf-8")
    manifest["codex_override_catalog_hash"] = sha256_bytes(catalog_for_hash)

    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    overrides_catalog_path.write_text(
        json.dumps(overrides_catalog, indent=2) + "\n", encoding="utf-8"
    )
    print(f"codex-sync: generated {len(generated)} twin(s): {', '.join(generated)}")
else:
    print("codex-sync: nothing to generate (all parity twins present).")
PY
