#!/usr/bin/env bash

# shellcheck source=scripts/lib/preamble.sh
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

phase="inventory"
root="."
manifest="docs/contracts/agents-documentation-authority.yaml"

for arg in "$@"; do
  case "$arg" in
    --phase=*) phase="${arg#*=}" ;;
    --root=*) root="${arg#*=}" ;;
    --manifest=*) manifest="${arg#*=}" ;;
    *) echo "check-agents-doc-authority: unknown argument: $arg" >&2; exit 2 ;;
  esac
done

case "$phase" in
  inventory|pre-cutover|cutover) ;;
  *) echo "check-agents-doc-authority: invalid phase: $phase" >&2; exit 2 ;;
esac

python3 - "$root" "$manifest" "$phase" <<'PY'
from __future__ import annotations

import fnmatch
import os
from pathlib import Path
import subprocess
import sys

try:
    import yaml
except ImportError:
    print("check-agents-doc-authority: PyYAML is required", file=sys.stderr)
    raise SystemExit(2)


repo = Path(sys.argv[1]).resolve()
manifest_arg = Path(sys.argv[2])
manifest_path = manifest_arg if manifest_arg.is_absolute() else repo / manifest_arg
phase = sys.argv[3]
errors: list[str] = []


def error(message: str) -> None:
    errors.append(message)


class UniqueKeyLoader(yaml.SafeLoader):
    pass


def construct_unique_mapping(loader, node, deep=False):
    mapping = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in mapping:
            raise yaml.constructor.ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                f"duplicate mapping key: {key}",
                key_node.start_mark,
            )
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    construct_unique_mapping,
)


try:
    data = yaml.load(manifest_path.read_text(encoding="utf-8"), Loader=UniqueKeyLoader) or {}
except (OSError, yaml.YAMLError) as exc:
    print(f"check-agents-doc-authority: cannot read manifest: {exc}", file=sys.stderr)
    raise SystemExit(2)

def reject_unknown(mapping, allowed: set[str], location: str) -> None:
    if not isinstance(mapping, dict):
        return
    for key in mapping:
        if key not in allowed:
            error(f"{location}: unknown key: {key}")


reject_unknown(data, {"schema_version", "status", "scope", "documents"}, "manifest")
if data.get("schema_version") != "agentops-documentation-authority.v1":
    error("schema_version must be agentops-documentation-authority.v1")
if data.get("status") != "active":
    error("status must be active")

scope = data.get("scope")
if not isinstance(scope, dict):
    scope = {}
    error("scope must be a mapping")
else:
    reject_unknown(scope, {"root_markdown_count", "reference_excludes"}, "scope")

exclusions = scope.get("reference_excludes", [])
exclude_globs: list[str] = []
if not isinstance(exclusions, list):
    error("scope.reference_excludes must be a list")
else:
    for index, exclusion in enumerate(exclusions):
        if not isinstance(exclusion, dict):
            error(f"reference_excludes[{index}] must be a mapping")
            continue
        reject_unknown(exclusion, {"glob", "reason"}, f"reference_excludes[{index}]")
        glob = exclusion.get("glob")
        reason = exclusion.get("reason")
        if not isinstance(glob, str) or not glob:
            error(f"reference_excludes[{index}].glob must be non-empty")
        elif glob in exclude_globs:
            error(f"duplicate reference exclusion: {glob}")
        else:
            exclude_globs.append(glob)
        if not isinstance(reason, str) or not reason:
            error(f"reference_excludes[{index}].reason must be non-empty")

documents = data.get("documents")
if not isinstance(documents, list):
    documents = []
    error("documents must be a list")

manifest_rel = os.path.relpath(manifest_path, repo)
seen_paths: set[str] = set()
records: dict[str, dict] = {}
required = ("path", "tracked", "class", "disposition", "owner", "proof")
for index, record in enumerate(documents):
    if not isinstance(record, dict):
        error(f"documents[{index}] must be a mapping")
        continue
    reject_unknown(
        record,
        {"path", "tracked", "presence", "class", "disposition", "owner", "proof", "references"},
        f"documents[{index}]",
    )
    for field in required:
        if field not in record:
            error(f"documents[{index}] is missing {field}")
    path = record.get("path")
    if not isinstance(path, str) or not path or Path(path).is_absolute():
        error(f"documents[{index}].path must be a non-empty relative path")
        continue
    if ".." in Path(path).parts:
        error(f"documents[{index}].path must stay inside the repository")
        continue
    if path in seen_paths:
        error(f"duplicate document path: {path}")
    seen_paths.add(path)
    records[path] = record
    if not isinstance(record.get("tracked"), bool):
        error(f"{path}: tracked must be boolean")
    if record.get("tracked") is False and record.get("presence") != "optional-generated":
        error(f"{path}: untracked records require presence: optional-generated")
    for field in ("class", "disposition", "owner", "proof"):
        value = record.get(field)
        if not isinstance(value, str) or not value:
            error(f"{path}: {field} must be one non-empty string")
    references = record.get("references")
    if references is not None:
        if not isinstance(references, dict):
            error(f"{path}: references must be a mapping")
        else:
            reject_unknown(references, {"phase_policy", "consumers", "include_globs"}, f"{path}.references")
            policies = references.get("phase_policy")
            consumers = references.get("consumers")
            include_globs = references.get("include_globs")
            if not isinstance(policies, dict):
                error(f"{path}: references.phase_policy must be a mapping")
            else:
                reject_unknown(
                    policies,
                    {"inventory", "pre-cutover", "cutover"},
                    f"{path}.references.phase_policy",
                )
                for name in ("inventory", "pre-cutover", "cutover"):
                    if policies.get(name) not in ("exact", "zero", "ignore"):
                        error(f"{path}: invalid {name} reference policy")
            if not isinstance(consumers, list) or any(not isinstance(item, str) or not item for item in consumers):
                error(f"{path}: references.consumers must be a list of paths")
            elif len(consumers) != len(set(consumers)):
                error(f"{path}: duplicate declared consumer")
            if include_globs is not None:
                if not isinstance(include_globs, list) or not include_globs or any(
                    not isinstance(item, str) or not item or Path(item).is_absolute() or ".." in Path(item).parts
                    for item in include_globs
                ):
                    error(f"{path}: references.include_globs must be a non-empty list of repository globs")

if records and not any(isinstance(record.get("references"), dict) for record in records.values()):
    error("manifest must declare at least one bounded literal-reference assertion")

actual_root = sorted(item.name for item in repo.iterdir() if item.name.endswith(".md") and (item.is_file() or item.is_symlink()))
declared_root = sorted(seen_paths)
missing = sorted(set(actual_root) - set(declared_root))
required_absent = sorted(
    path for path in set(declared_root) - set(actual_root)
    if records[path].get("presence") != "optional-generated"
)
if missing:
    error("root Markdown missing from manifest: " + ", ".join(missing))
if required_absent:
    error("required manifest paths absent from root Markdown: " + ", ".join(required_absent))
if scope.get("root_markdown_count") != len(declared_root):
    error(f"scope.root_markdown_count must equal declared inventory count {len(declared_root)}")

tracked_output = subprocess.run(
    ["git", "-C", str(repo), "ls-files", "-z"],
    check=True,
    stdout=subprocess.PIPE,
).stdout.decode("utf-8", errors="surrogateescape")
tracked_files = sorted(item for item in tracked_output.split("\0") if item)
tracked_set = set(tracked_files)

for path, record in records.items():
    target = repo / path
    if not target.exists() and record.get("presence") != "optional-generated":
        error(f"{path}: document does not exist")
    expected_tracked = record.get("tracked")
    if expected_tracked is True and path not in tracked_set:
        error(f"{path}: declared tracked but absent from git index")
    if expected_tracked is False and path in tracked_set:
        error(f"{path}: declared untracked but present in git index")
    owner = record.get("owner")
    if isinstance(owner, str) and owner != "git-history":
        if Path(owner).is_absolute() or ".." in Path(owner).parts:
            error(f"{path}: owner must stay inside the repository: {owner}")
        elif not (repo / owner).exists():
            error(f"{path}: owner path does not exist: {owner}")
    if target.is_symlink():
        link_target = (target.parent / os.readlink(target)).resolve()
        if not link_target.exists():
            error(f"{path}: symlink target does not exist")

for path, record in records.items():
    references = record.get("references")
    if not isinstance(references, dict):
        continue
    consumers = references.get("consumers", [])
    include_globs = references.get("include_globs")
    for consumer in consumers:
        if Path(consumer).is_absolute() or ".." in Path(consumer).parts:
            error(f"{path}: consumer must stay inside the repository: {consumer}")
        elif not (repo / consumer).exists():
            error(f"{path}: declared consumer does not exist: {consumer}")

    policies = references.get("phase_policy", {})
    policy = policies.get(phase)
    if policy == "ignore":
        continue
    live: list[str] = []
    needle = path.encode("utf-8")
    for candidate in tracked_files:
        if candidate in (path, manifest_rel):
            continue
        if include_globs and not any(fnmatch.fnmatch(candidate, glob) for glob in include_globs):
            continue
        if any(fnmatch.fnmatch(candidate, glob) for glob in exclude_globs):
            continue
        candidate_path = repo / candidate
        if not candidate_path.is_file():
            continue
        try:
            payload = candidate_path.read_bytes()
        except OSError as exc:
            error(f"cannot read tracked path {candidate}: {exc}")
            continue
        if needle in payload:
            live.append(candidate)
    live = sorted(live)
    declared = sorted(consumers)
    if policy == "exact" and live != declared:
        undeclared = sorted(set(live) - set(declared))
        stale = sorted(set(declared) - set(live))
        if undeclared:
            error(f"{path}: undeclared live consumers: " + ", ".join(undeclared))
        if stale:
            error(f"{path}: declared consumers without literal reference: " + ", ".join(stale))
    elif policy == "zero" and live:
        error(f"{path}: cutover requires zero live consumers: " + ", ".join(live))

if errors:
    for item in errors:
        print(f"ERROR: {item}", file=sys.stderr)
    print(f"check-agents-doc-authority: FAIL ({len(errors)} errors)", file=sys.stderr)
    raise SystemExit(1)

print(
    "check-agents-doc-authority: PASS "
    f"phase={phase} root_markdown={len(actual_root)} declared={len(documents)}"
)
PY
