#!/usr/bin/env bash
# codex-sync.sh — generate parity_only Codex twins from their source skills.
#
# A parity_only twin is a SELF-CONTAINED runtime artifact derived from its
# source skill. The Codex runtime ships skills-codex/ ONLY (never skills/ source
# Codex may still consume a generated skills-codex projection for archive/
# marketplace artifacts. Live installs use `ao skills link` into runtime skill
# roots — not a plugin-cache installer.
# twin must carry its own body + references; a bare pointer to skills/<name>
# would dangle at runtime (docs/contracts/codex-skill-api.md). The generated twin is therefore:
#   - SKILL.md: slim (name + description) frontmatter + the source body
#     transformed runtime-native (slash-command invocations of known skills ->
#     `$` prefix, ~/.claude -> ~/.codex, "Claude Code" -> "Codex");
#   - references/ + scripts/: copied byte-identical (lint scans only SKILL.md);
#   - prompt.md: the standard codex pointer-to-sibling-SKILL.md template,
#     optionally plus catalog-declared operator-contract markers.
# Because the twin is GENERATED from source, source edits never require a hand
# mirror — re-running this (via regen-all) reproduces a correct twin, killing the
# "add/touch a skill -> chase ~5 codex gates serially" whack-a-mole (regen-all.sh
# historically only rehashed EXISTING twins; it could not author one).
#
# This generator authors the COMPLETE twin for any source skill that lacks one:
# the body files + references, the per-skill marker, and all three catalog
# surfaces (manifest .skills[], manifest .codex_override_catalog.skills[], and
# skills-codex-overrides/catalog.json .skills[]), then fixes every hash. It is
# idempotent: a source skill that already matches the generated parity form is
# left untouched; a complete but stale parity twin is refreshed from source.
#
# bespoke twins (hand-authored Codex profiles) are the opt-out: they are never
# generated or overwritten — body AND references/scripts are hand-maintained, so
# even --force skips them. Source reference/body edits do NOT auto-propagate to a
# bespoke twin (many bespoke references are deliberate Codex rewrites of source);
# refreshing one is a deliberate human edit. Auto-mirroring source over a bespoke
# twin would clobber the hand-authored copy (age-0js4). Accidental drift is the
# divergence gate's job (age-odv), not this generator's.
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
FORCE=false
ONLY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) CHECK_ONLY=true; shift ;;
    --force) FORCE=true; shift ;;
    --only) ONLY="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,36p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    -*) echo "Unknown flag: $1" >&2; exit 2 ;;
    *) ONLY="$1"; shift ;;
  esac
done

# --force regenerates EXISTING twins (overwrite body + exact-mirror references/
# scripts from source). It must be scoped (--only / a skill name) so it cannot
# silently clobber the ~75 existing hand-tended twins in one shot.
if [[ "$FORCE" == "true" && -z "$ONLY" ]]; then
  echo "Refusing --force without scope: pass --only <skill[,...]> or a skill name." >&2
  echo "(--force rewrites existing twins from source; an unscoped run would clobber all of them.)" >&2
  exit 2
fi

export ROOT CHECK_ONLY FORCE ONLY

python3 - <<'PY'
import hashlib
import json
import os
import pathlib
import shutil
import sys

import yaml

root = pathlib.Path(os.environ["ROOT"]).resolve()
check_only = os.environ.get("CHECK_ONLY") == "true"
force = os.environ.get("FORCE") == "true"
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

# excluded = the drop-the-twin set (age-focus-membrane-bookkeeper-m1wg.19). A
# spine-excluded source skill (e.g. a corpus-flywheel skill demoted to the
# experimental tier) ships NO Codex twin: the skills-codex/<name>/ dir is deleted
# and MUST NOT be regenerated. Like bespoke it is skipped entirely — never
# generated, never checked, never restained — but unlike bespoke there is no twin
# on disk at all. The catalog keeps the entry (treatment: excluded) so
# validate-codex-override-coverage.sh treats the source skill as covered-by-
# exclusion rather than "missing from Codex catalog". Read from both catalogs.
excluded = {
    e.get("name")
    for e in (manifest_catalog_skills + overrides_skills)
    if e.get("treatment") == "excluded"
}

# Cross-runtime skills: exempt from the Claude->Codex / ~/.claude->~/.codex body
# rewrites (they legitimately document non-Codex runtimes). Single source of truth
# shared with the gates: scripts/lint/codex-cross-runtime-skills.txt.
cross_runtime_path = root / "scripts" / "lint" / "codex-cross-runtime-skills.txt"
cross_runtime = set()
if cross_runtime_path.exists():
    for line in cross_runtime_path.read_text(encoding="utf-8").splitlines():
        s = line.strip()
        if s and not s.startswith("#"):
            cross_runtime.add(s)


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


def split_frontmatter(skill_md: pathlib.Path) -> str:
    """Return the markdown BODY of a SKILL.md (everything after the leading
    --- ... --- frontmatter block)."""
    text = skill_md.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return text
    parts = text.split("---", 2)
    return parts[2].lstrip("\n") if len(parts) >= 3 else text


def transform_body(body: str, known_skills: set[str], exempt: bool = False) -> str:
    """Make a source skill body runtime-native for Codex (the lint-codex-native
    contract): slash-command invocations of KNOWN skills -> `$` prefix, Claude
    paths -> Codex paths, "Claude Code" -> "Codex". References are copied
    byte-identical (lint scans only SKILL.md), so only the body is transformed.

    exempt=True (cross-runtime skill, see scripts/lint/codex-cross-runtime-skills.txt):
    apply ONLY the slash->$ rewrite (Codex execution syntax is universal) and
    PRESERVE runtime names/paths verbatim — the twin legitimately documents
    Claude/AGY/etc., so rewriting "Claude Code"->"Codex" or ~/.claude->~/.codex
    would make it inaccurate."""
    import re

    # Skill(skill="known", args="...") -> $known ... for declarative skill
    # invocations in source skills. Preserve args when present so inline examples
    # remain actionable in Codex.
    known_alt = "|".join(re.escape(skill) for skill in sorted(known_skills, key=len, reverse=True))
    if known_alt:
        def repl_skill_call(match: re.Match) -> str:
            skill = match.group(1)
            args = match.group(2)
            return f"${skill}{(' ' + args) if args else ''}"

        body = re.sub(
            rf'Skill\(skill="({known_alt})"(?:,\s*args="([^"]*)")?\)',
            repl_skill_call,
            body,
        )

    # /<known-skill> -> $<known-skill> for slash-COMMAND invocations only — never
    # a path segment. Longest names first (so /premortem wins over /pre). Exclude
    # when preceded by a path char (word/./-/_/slash, e.g. ../research/, foo/plan)
    # or followed by '/' (a path like /research/SKILL.md), so markdown links and
    # file paths are left intact (the bug that turned ../foo/ into ..$foo/).
    for skill in sorted(known_skills, key=len, reverse=True):
        body = re.sub(rf"(?<![\w./_-])/{re.escape(skill)}\b(?!/)", f"${skill}", body)
    body = re.sub(r"(?<![\w./_-])/skill\b(?!/)", "$skill", body)

    if exempt:
        return body

    body = body.replace("~/.claude/", "~/.codex/").replace("~/.claude", "~/.codex")
    body = body.replace(".claude/", ".codex/")
    body = body.replace("Claude Code", "Codex")
    return body


def render_operator_contract_block(name: str, operator_contract: dict | None) -> str:
    if not operator_contract:
        return ""
    sections = operator_contract.get("required_sections") or []
    markers = operator_contract.get("required_markers") or []
    if not sections or not markers:
        return ""

    out: list[str] = [
        "<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->",
        f"<!-- Generated from skills-codex-overrides/catalog.json for {name}. -->",
        "",
    ]
    marker_index = 0
    remaining_markers = len(markers)
    section_count = len(sections)

    for section_index, section in enumerate(sections):
        out.extend([str(section), ""])
        sections_left = section_count - section_index
        if remaining_markers == 0:
            count = 0
        elif sections_left == 1:
            count = remaining_markers
        else:
            count = remaining_markers - (sections_left - 1)
            if count < 1:
                count = 1

        for bullet_index in range(count):
            out.append(f"{bullet_index + 1}. {markers[marker_index]}")
            marker_index += 1
            remaining_markers -= 1

        if section_index < section_count - 1:
            out.append("")

    out.extend(["", "<!-- END AGENTOPS OPERATOR CONTRACT -->"])
    return "\n".join(out)


def needs_codex_start_guard(operator_contract: dict | None) -> bool:
    markers = (operator_contract or {}).get("required_markers") or []
    return any("ao codex ensure-start" in str(marker) for marker in markers)


def insert_after_h1(body: str, block: str) -> str:
    if not block:
        return body
    first_newline = body.find("\n")
    if body.startswith("# ") and first_newline != -1:
        title = body[: first_newline + 1]
        rest = body[first_newline + 1 :].lstrip("\n")
        return f"{title}\n{block.rstrip()}\n\n{rest}"
    return f"{block.rstrip()}\n\n{body}"


def codex_start_guard_block() -> str:
    return """## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), run:

```bash
ao codex ensure-start 2>/dev/null || true
```

The CLI records startup once per thread and skips duplicates automatically."""


def codex_catalog_description(name: str, source_description: str) -> str:
    """Terse but skill-specific always-loaded Codex catalog text.

    The full source description remains available in prompt.md; SKILL.md
    frontmatter is loaded as an activation catalog, so keep enough routing
    signal to choose the skill while staying under the average token budget.
    """
    import re

    desc = re.sub(r"\s+[Tt]riggers?:.*", "", source_description).strip()
    desc = re.sub(r"\s+", " ", desc).strip(" '\"")
    if not desc:
        return f"Run {name}."

    max_chars = 44
    if len(desc) <= max_chars:
        return desc

    shortened = desc[: max_chars + 1].rsplit(" ", 1)[0].strip(" ,;:-")
    if len(shortened) < 18:
        shortened = desc[:max_chars].rstrip(" ,;:-")
    return shortened


def twin_skill_md(
    name: str,
    description: str,
    source_body: str,
    known_skills: set[str],
    exempt: bool = False,
    operator_contract: dict | None = None,
) -> bytes:
    """A self-contained Codex twin: slim (name + terse catalog description)
    frontmatter + the source body transformed runtime-native. Self-contained
    because the Codex runtime ships skills-codex/ ONLY (never skills/ source) —
    a twin must carry its own body + references (docs/contracts/codex-skill-api.md)."""
    fm = {"name": name, "description": description}
    front = yaml.safe_dump(fm, sort_keys=False, allow_unicode=True, width=10_000).strip()
    body = transform_body(source_body, known_skills, exempt)
    if needs_codex_start_guard(operator_contract) and "ao codex ensure-start" not in body:
        body = insert_after_h1(body, codex_start_guard_block())
    return f"---\n{front}\n---\n{body.rstrip()}\n".encode("utf-8")


def twin_prompt_md(
    name: str, description: str, operator_contract: dict | None = None
) -> bytes:
    prompt = (
        f"# {name}\n\n"
        f"{description}\n\n"
        f"## Instructions\n\n"
        f"Load and follow the skill instructions from the sibling `SKILL.md` file "
        f"for this skill.\n"
        f"Then read local files in `references/` and `scripts/` when needed.\n"
    )
    contract = render_operator_contract_block(name, operator_contract)
    if contract:
        prompt = f"{prompt.rstrip()}\n\n\n{contract}\n"
    return prompt.encode("utf-8")


def upsert(entries: list, name: str, entry: dict) -> bool:
    """Insert or replace by name, keeping EXACTLY ONE row per name. Returns True
    if the list changed. Replaces in place (no global re-sort) to keep the diff
    minimal — matches the append-only behavior of register-new-codex-skill.sh
    and avoids reordering the existing catalog on every new skill. Any later
    duplicate rows for the same name are dropped: historical syncs updated one
    row of a duplicated pair in place, so drift was masked or misreported
    depending on which row a reader's name-keyed dict happened to keep."""
    replaced_at = None
    removed = False
    for i in range(len(entries) - 1, -1, -1):
        if entries[i].get("name") != name:
            continue
        if replaced_at is None:
            replaced_at = i
        else:
            # entries[i] is an EARLIER duplicate (we scan backwards): keep the
            # first position for the row, drop the later one.
            entries[replaced_at : replaced_at + 1] = []
            replaced_at = i
            removed = True
    if replaced_at is None:
        entries.append(entry)
        return True
    if not removed and entries[replaced_at] == entry:
        return False
    entries[replaced_at] = entry
    return True


# Discover source skills (ground truth), skip non-skill dirs and bespoke.
def mirror_reasons(src_dir: pathlib.Path, twin_dir: pathlib.Path) -> list[str]:
    """Drift reasons for a twin's mirrored content (references/scripts/fixtures/
    etc.) vs source — missing, stale (content mismatch), or extra files. SKILL.md
    is transformed (verified separately); prompt.md + marker are twin-only."""
    twin_only = {"SKILL.md", "prompt.md", marker_name, ".agentops-manifest.json", ".DS_Store"}

    def tree(root_dir: pathlib.Path) -> dict[str, bytes]:
        out: dict[str, bytes] = {}
        if not root_dir.is_dir():
            return out
        for p in root_dir.rglob("*"):
            if not p.is_file() or p.name in twin_only:
                continue
            if "__pycache__" in p.parts or p.suffix == ".pyc":
                continue
            out[p.relative_to(root_dir).as_posix()] = p.read_bytes()
        return out

    src = tree(src_dir)
    twin = tree(twin_dir)
    reasons = [f"missing {r}" for r in src if r not in twin]
    reasons += [f"stale {r}" for r in src if r in twin and twin[r] != src[r]]
    reasons += [f"extra {r}" for r in twin if r not in src]
    return reasons


def exact_mirror_source_payload(src_dir: pathlib.Path, twin_dir: pathlib.Path) -> None:
    """Exact-copy source sibling content into a parity twin, excluding SKILL.md
    and twin-only bookkeeping. This keeps references/scripts/fixtures generated
    from source and removes stale copied files when source deletes them."""
    twin_only = {"SKILL.md", "prompt.md", marker_name, ".agentops-manifest.json", ".DS_Store"}
    source_names = {
        entry.name
        for entry in src_dir.iterdir()
        if entry.name not in {"SKILL.md", ".agentops-manifest.json", marker_name, ".DS_Store"}
    }

    for entry in sorted(twin_dir.iterdir()):
        if entry.name in twin_only:
            continue
        if entry.name not in source_names:
            shutil.rmtree(entry) if entry.is_dir() else entry.unlink()

    for entry in sorted(src_dir.iterdir()):
        if entry.name in {"SKILL.md", ".agentops-manifest.json", marker_name, ".DS_Store"}:
            continue
        dst_entry = twin_dir / entry.name
        if dst_entry.exists():
            shutil.rmtree(dst_entry) if dst_entry.is_dir() else dst_entry.unlink()
        shutil.copytree(entry, dst_entry) if entry.is_dir() else shutil.copy2(entry, dst_entry)


source_skills = sorted(
    p.name
    for p in source_root.iterdir()
    if p.is_dir()
    and not p.name.startswith("_")
    and (p / "SKILL.md").exists()
)
known_skills = set(source_skills)

drift = []
generated = []

# Source metadata owns the installed set. Retired source roots must not leave
# empty directories, stale generated twins, or override rows that continue to
# advertise removed skills.
retired_twin_dirs = sorted(
    p for p in codex_root.iterdir()
    if p.is_dir() and not p.name.startswith("_") and p.name not in known_skills
)
if check_only:
    drift.extend((p.name, ["retired twin directory remains"]) for p in retired_twin_dirs)
else:
    for path in retired_twin_dirs:
        shutil.rmtree(path)

overrides_skills[:] = [
    entry for entry in overrides_skills if entry.get("name") in known_skills
]

for name in source_skills:
    if name in bespoke:
        continue
    if name in excluded:
        # Spine-excluded: the twin was intentionally dropped; never regenerate it.
        continue
    if scope and name not in scope:
        continue

    twin_dir = codex_root / name
    skill_md = twin_dir / "SKILL.md"
    prompt_md = twin_dir / "prompt.md"
    marker_path = twin_dir / marker_name

    catalog_source_entry = next((e for e in overrides_skills if e.get("name") == name), {})
    operator_contract = catalog_source_entry.get("operator_contract")

    fm = parse_frontmatter(source_root / name / "SKILL.md")
    source_description = str(fm.get("description", "")).strip()
    codex_description = codex_catalog_description(name, source_description)
    source_body = split_frontmatter(source_root / name / "SKILL.md")

    desired_skill = twin_skill_md(
        name,
        codex_description,
        source_body,
        known_skills,
        name in cross_runtime,
        operator_contract,
    )
    desired_prompt = twin_prompt_md(name, source_description, operator_contract)

    # A twin is "complete" iff its body files + marker exist AND it is registered
    # in the gate-enforced 1:1 surface (skills-codex-overrides/catalog.json — the
    # surface validate-codex-override-coverage.sh holds to source skills exactly).
    # Complete parity twins are still checked against the generated shape below;
    # stale generated bodies, prompts, or mirrored references are refreshed.
    # Bespoke twins opt out via the catalog and are skipped before this point.
    # The bloated manifest .codex_override_catalog is downstream and is NOT a
    # generation trigger.
    in_ocat = any(e.get("name") == name for e in overrides_skills)

    if check_only:
        # THE single drift gate for parity twins: the on-disk twin must EXACTLY
        # match what the generator would emit — presence + registration +
        # byte-identical transformed SKILL.md + generated prompt.md + mirrored
        # references. Any drift forces a regen. This guarantee is what lets the
        # content validators (lint-native, api-conformance, runtime-sections,
        # audit-parity) skip parity twins entirely: a generated artifact is
        # verified by regenerate-and-diff, not by re-checking content rules.
        reasons = []
        if not in_ocat:
            reasons.append("unregistered in catalog.json")
        if not marker_path.exists():
            reasons.append("missing marker")
        if not skill_md.exists() or skill_md.read_bytes() != desired_skill:
            reasons.append("SKILL.md")
        if not prompt_md.exists() or prompt_md.read_bytes() != desired_prompt:
            reasons.append("prompt.md")
        reasons += mirror_reasons(source_root / name, twin_dir)
        if reasons:
            drift.append((name, reasons))
        continue

    regen_reasons = []
    if not in_ocat:
        regen_reasons.append("unregistered in catalog.json")
    if not marker_path.exists():
        regen_reasons.append("missing marker")
    if not skill_md.exists() or skill_md.read_bytes() != desired_skill:
        regen_reasons.append("SKILL.md")
    if not prompt_md.exists() or prompt_md.read_bytes() != desired_prompt:
        regen_reasons.append("prompt.md")
    regen_reasons += mirror_reasons(source_root / name, twin_dir)

    if not force and not regen_reasons:
        continue

    # --- Author/refresh the parity twin (self-contained: body + references) ---
    twin_dir.mkdir(parents=True, exist_ok=True)
    if force or not skill_md.exists() or skill_md.read_bytes() != desired_skill:
        skill_md.write_bytes(desired_skill)
    if force or not prompt_md.exists() or prompt_md.read_bytes() != desired_prompt:
        prompt_md.write_bytes(desired_prompt)

    # Mirror ALL source content (references/, scripts/, fixtures/, templates/,
    # agents/, any sibling files) EXCEPT SKILL.md — byte-identical, so every link
    # in the body resolves and the Codex runtime artifact is fully self-contained.
    # lint scans only SKILL.md, so copied content needs no transform.
    exact_mirror_source_payload(source_root / name, twin_dir)

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
    # Catalog entries are ADD-ONLY: an already-registered skill keeps its existing
    # (often hand-written) reason/wave in the authoritative
    # skills-codex-overrides/catalog.json. The manifest embeds that catalog for
    # the shipped runtime artifact, so mirror the authoritative entry for skills
    # touched by this generator.
    if catalog_source_entry:
        upsert(manifest_catalog_skills, name, catalog_source_entry)
    elif not any(e.get("name") == name for e in manifest_catalog_skills):
        manifest_catalog_skills.append(catalog_entry)
    if not any(e.get("name") == name for e in overrides_skills):
        overrides_skills.append(catalog_entry)
    generated.append(name)

# Rebuild the runtime artifact inventory from the actual generated directories.
# Source retirement can delete a twin before this generator runs; an append-only
# manifest would otherwise retain a ghost row forever. The embedded treatment
# catalog is likewise a projection of the authoritative overrides catalog, not
# a second hand-maintained store.
desired_manifest_skills = []
existing_manifest_by_name = {
    entry.get("name"): entry for entry in manifest_skills if entry.get("name")
}
package_count = sum(
    1
    for package_dir in codex_root.iterdir()
    if package_dir.is_dir() and (package_dir / "SKILL.md").exists()
)
for twin_dir in sorted(
    p
    for p in codex_root.iterdir()
    if p.is_dir()
    and (p / "SKILL.md").exists()
):
    marker_path = twin_dir / marker_name
    if not marker_path.exists():
        continue  # the manifest validator reports the missing marker fail-closed
    marker = json.loads(marker_path.read_text(encoding="utf-8"))
    entry = dict(existing_manifest_by_name.get(twin_dir.name, {}))
    entry.update(
        name=twin_dir.name,
        source_skill=marker.get("source_skill", f"skills/{twin_dir.name}"),
        source_hash=marker.get("source_hash", ""),
        generated_hash=marker.get("generated_hash", ""),
    )
    desired_manifest_skills.append(entry)

manifest_inventory_drift = manifest_skills != desired_manifest_skills
embedded_catalog_drift = manifest_catalog_skills != overrides_skills
package_count_drift = manifest.get("package_count") != package_count

if check_only:
    if manifest_inventory_drift:
        drift.append(("<manifest>", ["artifact inventory differs from skills-codex directories"]))
    if embedded_catalog_drift:
        drift.append(("<manifest-catalog>", ["embedded treatment catalog differs from authoritative overrides catalog"]))
    if package_count_drift:
        drift.append(("<manifest>", [f"package_count differs from {package_count} installable skill directories"]))
    if drift:
        print(f"codex-sync drift: {len(drift)} parity twin(s) differ from generator output:")
        for n, reasons in drift:
            print(f"  - {n}: {', '.join(reasons)}")
        print("Fix: scripts/codex-sync.sh --force --only <name> (then regen hashes).")
        sys.exit(1)
    print("codex-sync: all parity twins match generator output.")
    sys.exit(0)

manifest_skills[:] = desired_manifest_skills
manifest_catalog_skills[:] = [dict(entry) for entry in overrides_skills]
manifest["package_count"] = package_count

if generated or manifest_inventory_drift or embedded_catalog_drift or package_count_drift:
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
    if generated:
        print(f"codex-sync: generated {len(generated)} twin(s): {', '.join(generated)}")
    else:
        print("codex-sync: refreshed manifest projections")
else:
    print("codex-sync: nothing to generate (all parity twins present).")
PY
