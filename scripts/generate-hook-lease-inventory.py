#!/usr/bin/env python3
# practices: [hexagonal-architecture, data-contracts, design-by-contract]
"""Generate the AgentOps hook lease inventory from the live hook manifest."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any


EVENT_ORDER = [
    "SessionStart",
    "SessionEnd",
    "Stop",
    "UserPromptSubmit",
    "PreToolUse",
    "PostToolUse",
    "TaskCompleted",
    "PreCompact",
    "SubagentStop",
    "WorktreeCreate",
    "WorktreeRemove",
    "ConfigChange",
]

OWNER_BUBBLES = {
    "Work Lifecycle",
    "Context Compiler",
    "Evidence and Trust",
    "Knowledge Flywheel",
    "Skill Catalog",
    "Runtime Shell",
}

DISPOSITIONS = {
    "remove",
    "gate",
    "event-subscriber",
    "explicit-command",
    "optional-adapter",
}

EVIDENCE_STATUSES = {
    "deterministic-safety",
    "deterministic-maintenance",
    "needs-eval",
    "token-risk-needs-remeasure",
    "unproven",
}


def load_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        value = json.load(handle)
    if not isinstance(value, dict):
        raise ValueError(f"{path} must contain a JSON object")
    return value


def hook_file_from_command(command: str) -> str:
    return command.rsplit("/", 1)[-1].replace("${CLAUDE_PLUGIN_ROOT}", "").strip()


def first_header_summary(path: Path) -> str:
    if not path.exists():
        return "missing hook file"

    for raw in path.read_text(encoding="utf-8", errors="replace").splitlines()[:20]:
        line = raw.strip()
        if not line or line.startswith("#!"):
            continue
        if line.startswith("#"):
            summary = line.lstrip("#").strip()
            if summary and not summary.startswith("practices:"):
                return summary
    return "no summary header"


def flatten_manifest(manifest: dict[str, Any]) -> list[dict[str, Any]]:
    hooks = manifest.get("hooks", {})
    if not isinstance(hooks, dict):
        raise ValueError("hooks manifest must contain object field: hooks")

    entries: list[dict[str, Any]] = []
    ordered_events = [event for event in EVENT_ORDER if event in hooks]
    ordered_events.extend(sorted(event for event in hooks if event not in EVENT_ORDER))

    for event in ordered_events:
        rules = hooks.get(event) or []
        if not isinstance(rules, list):
            raise ValueError(f"hooks.{event} must be an array")
        for rule_index, rule in enumerate(rules):
            if not isinstance(rule, dict):
                raise ValueError(f"hooks.{event}[{rule_index}] must be an object")
            matcher = str(rule.get("matcher", "*"))
            commands = rule.get("hooks") or []
            if not isinstance(commands, list):
                raise ValueError(f"hooks.{event}[{rule_index}].hooks must be an array")
            for hook_index, item in enumerate(commands):
                if not isinstance(item, dict):
                    raise ValueError(
                        f"hooks.{event}[{rule_index}].hooks[{hook_index}] must be an object"
                    )
                command = str(item.get("command", ""))
                if not command:
                    raise ValueError(
                        f"hooks.{event}[{rule_index}].hooks[{hook_index}] missing command"
                    )
                entries.append(
                    {
                        "event": event,
                        "matcher": matcher,
                        "command": command,
                        "hook_file": hook_file_from_command(command),
                        "timeout": int(item.get("timeout", 0) or 0),
                    }
                )
    return entries


def validate_classification(name: str, data: dict[str, Any]) -> None:
    required = {
        "owner_bubble",
        "disposition",
        "replacement",
        "evidence_status",
        "context_injection",
        "side_effects",
        "rationale",
    }
    missing = sorted(required - set(data))
    if missing:
        raise ValueError(f"classification for {name} missing fields: {', '.join(missing)}")
    if data["owner_bubble"] not in OWNER_BUBBLES:
        raise ValueError(f"classification for {name} has invalid owner_bubble")
    if data["disposition"] not in DISPOSITIONS:
        raise ValueError(f"classification for {name} has invalid disposition")
    if data["evidence_status"] not in EVIDENCE_STATUSES:
        raise ValueError(f"classification for {name} has invalid evidence_status")
    if not isinstance(data["context_injection"], bool):
        raise ValueError(f"classification for {name} context_injection must be boolean")
    if not isinstance(data["side_effects"], list) or not data["side_effects"]:
        raise ValueError(f"classification for {name} side_effects must be non-empty array")


def build_inventory(repo_root: Path) -> dict[str, Any]:
    manifest_path = repo_root / "hooks" / "hooks.json"
    classification_path = repo_root / "docs" / "contracts" / "hook-lease-classification.v1.json"
    manifest = load_json(manifest_path)
    classification = load_json(classification_path)
    classifications = classification.get("entries", {})
    if not isinstance(classifications, dict):
        raise ValueError("hook lease classification must contain object field: entries")
    additional_classifications = classification.get("additional_surfaces", {})
    if not isinstance(additional_classifications, dict):
        raise ValueError("hook lease classification additional_surfaces must be an object")

    manifest_entries = flatten_manifest(manifest)
    manifest_hook_files = {entry["hook_file"] for entry in manifest_entries}
    classification_files = set(classifications)
    missing = sorted(manifest_hook_files - classification_files)
    stale = sorted(classification_files - manifest_hook_files)
    if missing:
        raise ValueError("missing hook lease classifications: " + ", ".join(missing))
    if stale:
        raise ValueError("stale hook lease classifications: " + ", ".join(stale))

    entries: list[dict[str, Any]] = []
    for entry in manifest_entries:
        hook_file = entry["hook_file"]
        class_data = classifications[hook_file]
        validate_classification(hook_file, class_data)
        merged = {
            **entry,
            "owner_bubble": class_data["owner_bubble"],
            "disposition": class_data["disposition"],
            "replacement": class_data["replacement"],
            "evidence_status": class_data["evidence_status"],
            "context_injection": class_data["context_injection"],
            "side_effects": class_data["side_effects"],
            "summary": first_header_summary(repo_root / "hooks" / hook_file),
            "rationale": class_data["rationale"],
        }
        entries.append(merged)

    additional_surfaces: list[dict[str, Any]] = []
    for hook_file in sorted(additional_classifications):
        class_data = additional_classifications[hook_file]
        validate_classification(hook_file, class_data)
        additional_surfaces.append(
            {
                "hook_file": hook_file,
                "runtime_status": str(class_data.get("runtime_status", "unspecified")),
                "owner_bubble": class_data["owner_bubble"],
                "disposition": class_data["disposition"],
                "replacement": class_data["replacement"],
                "evidence_status": class_data["evidence_status"],
                "context_injection": class_data["context_injection"],
                "side_effects": class_data["side_effects"],
                "summary": first_header_summary(repo_root / "hooks" / hook_file),
                "rationale": class_data["rationale"],
            }
        )

    return {
        "schema_version": 1,
        "generated_from": "hooks/hooks.json",
        "classification_source": "docs/contracts/hook-lease-classification.v1.json",
        "hook_count": len(entries),
        "unique_hook_count": len(manifest_hook_files),
        "entries": entries,
        "additional_surfaces": additional_surfaces,
    }


def markdown_escape(value: str) -> str:
    return value.replace("|", "\\|").replace("\n", " ")


def render_markdown(inventory: dict[str, Any]) -> str:
    entries = inventory["entries"]
    counts: dict[str, int] = {}
    for entry in entries:
        counts[entry["disposition"]] = counts.get(entry["disposition"], 0) + 1

    lines = [
        "# Hook Lease Inventory",
        "",
        "> **Status:** Draft",
        "> **Decision:** Runtime hooks are lease-bound adapter candidates for",
        "> AgentOps 3.0. No hook is product core by default.",
        "> **Generated by:** `python3 scripts/generate-hook-lease-inventory.py`",
        f"> **Source:** `{inventory['generated_from']}` + `{inventory['classification_source']}`",
        "",
        "This inventory classifies every hook entry in the live hook manifest.",
        "It does not delete hooks. It assigns each hook to the 3.0 replacement",
        "path required before hook defaults can be removed.",
        "",
        "## Summary",
        "",
        f"- Manifest hook entries: {inventory['hook_count']}",
        f"- Unique hook files in manifest: {inventory['unique_hook_count']}",
        f"- Additional hook surfaces outside main manifest: {len(inventory['additional_surfaces'])}",
        f"- `remove`: {counts.get('remove', 0)}",
        f"- `gate`: {counts.get('gate', 0)}",
        f"- `event-subscriber`: {counts.get('event-subscriber', 0)}",
        f"- `explicit-command`: {counts.get('explicit-command', 0)}",
        f"- `optional-adapter`: {counts.get('optional-adapter', 0)}",
        "",
        "## Dispositions",
        "",
        "| Disposition | Meaning |",
        "|-------------|---------|",
        "| `remove` | No proven behavior delta or mainly prompt/context bloat. |",
        "| `gate` | Deterministic safety or validation behavior; move to a validation lane. |",
        "| `event-subscriber` | Useful side effect; move to typed event subscription. |",
        "| `explicit-command` | Useful only when the operator or lifecycle explicitly asks. |",
        "| `optional-adapter` | Runtime-specific and eval-proven; disabled by default. |",
        "",
        "## Bead Classification Mapping",
        "",
        "This inventory uses concrete migration dispositions. For the",
        "`soc-m6v5.9.9.6` hook-classification bead:",
        "",
        "- `gate` maps to guard adapter.",
        "- `optional-adapter` maps to optional runtime adapter.",
        "- Any row with `Context = yes` is a context-injection candidate until",
        "  token-value evidence proves it should remain default-on.",
        "- `event-subscriber` and `explicit-command` are demotion paths for useful",
        "  behavior that should not stay as hidden resident context.",
        "",
        "## Inventory",
        "",
        "| Event | Matcher | Hook | Timeout | Bubble | Disposition | Evidence | Context | Replacement |",
        "|-------|---------|------|---------|--------|-------------|----------|---------|-------------|",
    ]

    for entry in entries:
        lines.append(
            "| {event} | {matcher} | `{hook_file}` | {timeout} | {owner_bubble} | `{disposition}` | `{evidence_status}` | {context} | {replacement} |".format(
                event=markdown_escape(entry["event"]),
                matcher=markdown_escape(entry["matcher"]),
                hook_file=markdown_escape(entry["hook_file"]),
                timeout=entry["timeout"],
                owner_bubble=markdown_escape(entry["owner_bubble"]),
                disposition=entry["disposition"],
                evidence_status=entry["evidence_status"],
                context="yes" if entry["context_injection"] else "no",
                replacement=markdown_escape(entry["replacement"]),
            )
        )

    if inventory["additional_surfaces"]:
        lines.extend(
            [
                "",
                "## Runtime Asymmetry",
                "",
                "The main inventory above is generated from `hooks/hooks.json`.",
                "The surfaces below are deliberately tracked outside that manifest",
                "because they are Codex-only active hooks or script-tested hook",
                "surfaces that are not wired by the active manifests.",
                "",
                "| Runtime status | Hook | Bubble | Disposition | Evidence | Context | Replacement |",
                "|----------------|------|--------|-------------|----------|---------|-------------|",
            ]
        )
        for surface in inventory["additional_surfaces"]:
            lines.append(
                "| {runtime_status} | `{hook_file}` | {owner_bubble} | `{disposition}` | `{evidence_status}` | {context} | {replacement} |".format(
                    runtime_status=markdown_escape(surface["runtime_status"]),
                    hook_file=markdown_escape(surface["hook_file"]),
                    owner_bubble=markdown_escape(surface["owner_bubble"]),
                    disposition=surface["disposition"],
                    evidence_status=surface["evidence_status"],
                    context="yes" if surface["context_injection"] else "no",
                    replacement=markdown_escape(surface["replacement"]),
                )
            )

    lines.extend(
        [
            "",
            "## Hook Notes",
            "",
        ]
    )
    seen: set[str] = set()
    for entry in entries:
        hook_file = entry["hook_file"]
        if hook_file in seen:
            continue
        seen.add(hook_file)
        side_effects = "; ".join(entry["side_effects"])
        lines.extend(
            [
                f"### `{hook_file}`",
                "",
                f"- **Summary:** {entry['summary']}",
                f"- **Side effects:** {side_effects}",
                f"- **Rationale:** {entry['rationale']}",
                "",
            ]
        )

    for surface in inventory["additional_surfaces"]:
        hook_file = surface["hook_file"]
        side_effects = "; ".join(surface["side_effects"])
        lines.extend(
            [
                f"### `{hook_file}`",
                "",
                f"- **Runtime status:** {surface['runtime_status']}",
                f"- **Summary:** {surface['summary']}",
                f"- **Side effects:** {side_effects}",
                f"- **Rationale:** {surface['rationale']}",
                "",
            ]
        )

    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=Path(__file__).resolve().parents[1])
    parser.add_argument("--format", choices=["markdown", "json"], default="markdown")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    try:
        inventory = build_inventory(repo_root)
        if args.format == "json":
            rendered = json.dumps(inventory, indent=2, sort_keys=False) + "\n"
        else:
            rendered = render_markdown(inventory)
    except Exception as exc:  # noqa: BLE001 - script should report compact CLI errors.
        print(f"generate-hook-lease-inventory: FAIL - {exc}", file=sys.stderr)
        return 1

    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
