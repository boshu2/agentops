#!/usr/bin/env python3
"""Phase 1 — audit report + reversibility plan (Gate A).

Runs the scanner (bead .7) and classifier (bead .8) and synthesizes a single
human-review artifact: current state per layer, the KEEP/DROP summary, estimated
disk reclaimed, and an explicit reversibility plan naming every later destructive
action and how to undo it. This report is the go/no-go GATE before any Phase-2/3
mutation. Read-only.

Usage:
    audit_report.py [--home DIR] [--out audit-report.md] [--max-sample N]
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import audit_scan  # noqa: E402
import classify  # noqa: E402

# Destructive actions introduced by later phases, each with its rollback. Kept here
# so the gate report always names the full blast radius before it is approved.
REVERSIBILITY = [
    (
        "Phase 2 import (bead .10)",
        "writes salvaged keepers into bd",
        "non-destructive; re-runnable. Rollback: `bd forget <key>` per import-ledger entry.",
    ),
    (
        "Phase 2 MEMORY.md regen (bead .14)",
        "overwrites the Claude MEMORY.md index",
        "back up MEMORY.md -> MEMORY.md.bak first; restore from backup.",
    ),
    (
        "Phase 3 GC/evict (bead .16)",
        "moves stale low-utility memories to a cold archive",
        "archive-not-delete; restore by moving the archived memory back.",
    ),
    (
        "Phase 3 hard-retire (bead .21/.31)",
        "decommissions ao/openclaw daemons + deletes the dup pile",
        "tar the dup pile to backup first; daemons re-enable via systemctl; "
        "gated on rollback test .26.",
    ),
]


def render(home: Path, sample_cap: int) -> str:
    """Build the markdown audit report from a fresh scan + manifest."""
    scan = audit_scan.run_scan(home, sample_cap)
    man = classify.build_manifest(home, sample_cap)
    s = man.summary

    lines: list[str] = []
    lines.append("# Memory migration — Phase 1 audit report (Gate A)")
    lines.append("")
    lines.append(f"- Target: `{home}`")
    lines.append(f"- Layers present: **{scan['totals']['layers_present']}**")
    lines.append(f"- On-disk total: **{scan['totals']['mib']} MiB**")
    lines.append("")

    lines.append("## Current state")
    lines.append("")
    lines.append("| Layer | Files | MiB | Notes |")
    lines.append("|---|---|---|---|")
    for layer in scan["layers"]:
        if not layer["present"]:
            note = layer["notes"][0] if layer["notes"] else "absent"
            lines.append(f"| {layer['name']} | — | — | {note} |")
            continue
        mib = round(layer["bytes"] / (1024 * 1024), 1)
        note = "; ".join(layer["notes"]) if layer["notes"] else ""
        lines.append(f"| {layer['name']} | {layer['file_count']} | {mib} | {note} |")
    lines.append("")

    lines.append("## KEEP / DROP manifest (dry-run)")
    lines.append("")
    keep_n, drop_n = s["keep_files"], s["drop_files"]
    ao_k, ao_d = s["ao_keep_nodes"], s["ao_drop_nodes"]
    lines.append(f"- **Keep** (import to bd): {keep_n} files + {ao_k} ao nodes")
    lines.append(f"- **Drop** (retire): {drop_n} files + {ao_d} ao provisional nodes")
    reclaim = s["est_disk_reclaimed_mib"]
    lines.append(f"- **Est. disk reclaimed:** ~{reclaim} MiB (sampled lower bound)")
    lines.append(f"- **Est. memories after migration:** {s['est_memories_after']}")
    if s.get("sampled"):
        lines.append(f"- Sampled at {s['sampled']} files/pile — full run will exceed these counts.")
    lines.append("")

    lines.append("## Reversibility plan")
    lines.append("")
    lines.append("| Later action | Effect | Rollback |")
    lines.append("|---|---|---|")
    for action, effect, rollback in REVERSIBILITY:
        lines.append(f"| {action} | {effect} | {rollback} |")
    lines.append("")
    lines.append("> ⛔ **Gate A:** review this manifest before Phase 2 writes anything. "
                 "No salvage import runs until the keep/drop split is approved.")
    lines.append("")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Generate the Phase 1 audit report (Gate A).")
    parser.add_argument("--home", default=str(Path.home()), help="target home dir (default: $HOME)")
    parser.add_argument("--out", default="", help="write report to this path (default: stdout)")
    parser.add_argument("--max-sample", type=int, default=200000, help="cap files sampled per pile")
    args = parser.parse_args(argv)

    report = render(Path(args.home).expanduser(), args.max_sample)
    if args.out:
        Path(args.out).expanduser().write_text(report)
        print(f"audit report -> {args.out} ({report.count(chr(10)) + 1} lines)")
    else:
        print(report)
    return 0


if __name__ == "__main__":
    sys.exit(main())
