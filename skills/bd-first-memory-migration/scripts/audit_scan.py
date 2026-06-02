#!/usr/bin/env python3
"""Phase 1 — layer-discovery scanner for the bd-first memory migration.

Read-only inventory of every agent-memory layer on a target box. Emits a JSON
(or human) report of counts, sizes, duplicate-rate, and maturity distribution so
the classifier (bead .8) and audit gate (bead .9) can decide keep-vs-retire.

Layers detected (each is optional; absent layers are reported, never fatal):
  - claude_memory  : ~/.claude/projects/*/memory/ (MEMORY.md + topic files)
  - agents_knowledge / agents_learnings : <home>/.agents/{knowledge,learnings}
  - ao             : the ao/flywheel store (via `ao stats`/`ao maturity --scan` if present)
  - openclaw       : ~/.openclaw/wiki + memory store
  - bd             : `bd memories` count (canonical target)

Usage:
    audit_scan.py [--home DIR] [--json | --human] [--max-dup-sample N]

100% read-only. No mutation, no network.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
from collections import Counter
from dataclasses import asdict, dataclass, field
from pathlib import Path


@dataclass
class LayerReport:
    """Inventory of a single memory layer."""

    name: str
    present: bool
    file_count: int = 0
    bytes: int = 0
    dup_groups: int = 0          # number of content-hash groups with >1 member
    dup_files: int = 0           # files that are duplicates (group size - 1, summed)
    maturity: dict[str, int] = field(default_factory=dict)
    notes: list[str] = field(default_factory=list)


def _iter_files(root: Path) -> list[Path]:
    """Return all regular files under root (no symlink following)."""
    out: list[Path] = []
    for dirpath, _dirs, files in os.walk(root, followlinks=False):
        for name in files:
            p = Path(dirpath) / name
            if p.is_file() and not p.is_symlink():
                out.append(p)
    return out


def _dup_stats(files: list[Path], sample_cap: int) -> tuple[int, int]:
    """Count duplicate content groups via sha1 of file bytes.

    To stay cheap on huge piles, hash at most ``sample_cap`` files; if the pile
    is larger, the result is a lower bound and a note is added by the caller.
    Returns (dup_groups, dup_files).
    """
    hashes: Counter[str] = Counter()
    for p in files[:sample_cap]:
        try:
            # sha1 is a content identifier here, not a security primitive.
            digest = hashlib.sha1(p.read_bytes()).hexdigest()  # noqa: S324
        except OSError:
            continue
        hashes[digest] += 1
    dup_groups = sum(1 for n in hashes.values() if n > 1)
    dup_files = sum(n - 1 for n in hashes.values() if n > 1)
    return dup_groups, dup_files


def scan_dir_layer(name: str, root: Path, sample_cap: int) -> LayerReport:
    """Inventory a filesystem-backed layer (knowledge/learnings/openclaw)."""
    if not root.exists():
        return LayerReport(name=name, present=False, notes=[f"{root} absent"])
    files = _iter_files(root)
    total_bytes = 0
    for p in files:
        try:
            total_bytes += p.stat().st_size
        except OSError:
            continue
    dup_groups, dup_files = _dup_stats(files, sample_cap)
    rep = LayerReport(
        name=name,
        present=True,
        file_count=len(files),
        bytes=total_bytes,
        dup_groups=dup_groups,
        dup_files=dup_files,
    )
    if len(files) > sample_cap:
        rep.notes.append(
            f"dup-rate sampled over first {sample_cap} of {len(files)} files (lower bound)"
        )
    return rep


def scan_claude_memory(home: Path, sample_cap: int) -> LayerReport:
    """Inventory Claude auto-memory across all project memory dirs."""
    base = home / ".claude" / "projects"
    if not base.exists():
        return LayerReport(name="claude_memory", present=False, notes=[f"{base} absent"])
    mem_dirs = [p for p in base.glob("*/memory") if p.is_dir()]
    if not mem_dirs:
        return LayerReport(name="claude_memory", present=False, notes=["no */memory dirs"])
    files: list[Path] = []
    for d in mem_dirs:
        files.extend(_iter_files(d))
    total_bytes = 0
    for f in files:
        try:
            total_bytes += f.stat().st_size
        except OSError:
            continue
    dup_groups, dup_files = _dup_stats(files, sample_cap)
    rep = LayerReport(
        name="claude_memory",
        present=True,
        file_count=len(files),
        bytes=total_bytes,
        dup_groups=dup_groups,
        dup_files=dup_files,
        notes=[f"{len(mem_dirs)} project memory dir(s)"],
    )
    # MEMORY.md health: flag bloat (the known corruption mode).
    for d in mem_dirs:
        idx = d / "MEMORY.md"
        if idx.exists():
            lines = idx.read_text(errors="replace").count("\n") + 1
            if lines > 200:
                rep.notes.append(f"{idx} is {lines} lines (>200 limit — likely bloated)")
    return rep


def scan_ao(home: Path) -> LayerReport:
    """Inventory the ao/flywheel store via the ao CLI maturity scan, if installed.

    ``ao`` resolves its knowledge store relative to CWD, so the scan is run with
    ``cwd=home`` to inventory the *target* box's store, not the caller's.
    """
    if shutil.which("ao") is None:
        return LayerReport(name="ao", present=False, notes=["ao CLI not on PATH"])
    try:
        proc = subprocess.run(
            ["ao", "maturity", "--scan"],
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
            cwd=str(home),
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return LayerReport(name="ao", present=True, notes=[f"ao maturity --scan failed: {exc}"])
    maturity: dict[str, int] = {}
    total = 0
    for line in proc.stdout.splitlines():
        parts = line.split(":")
        if len(parts) == 2:
            label = parts[0].strip().lower()
            val = parts[1].strip()
            mat_labels = {"provisional", "candidate", "established", "anti-pattern"}
            if label in mat_labels and val.isdigit():
                maturity[label] = int(val)
            elif label == "total" and val.isdigit():
                total = int(val)
    rep = LayerReport(
        name="ao", present=bool(maturity or total), file_count=total, maturity=maturity
    )
    prov = maturity.get("provisional", 0)
    if total and prov / total > 0.95:
        pct = 100 * prov // total
        rep.notes.append(f"STALLED: {prov}/{total} ({pct}%) provisional — flywheel not promoting")
    return rep


def scan_bd() -> LayerReport:
    """Count canonical bd memories (the migration target)."""
    if shutil.which("bd") is None:
        return LayerReport(name="bd", present=False, notes=["bd CLI not on PATH"])
    try:
        proc = subprocess.run(
            ["bd", "memories"], capture_output=True, text=True, timeout=60, check=False
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return LayerReport(name="bd", present=True, notes=[f"bd memories failed: {exc}"])
    count = 0
    for line in proc.stdout.splitlines():
        # header form: "Memories (202):"
        if line.strip().lower().startswith("memories (") and "(" in line:
            inner = line.split("(", 1)[1].split(")", 1)[0]
            if inner.isdigit():
                count = int(inner)
                break
    return LayerReport(name="bd", present=True, file_count=count, notes=["canonical target store"])


def run_scan(home: Path, sample_cap: int) -> dict[str, object]:
    """Run all layer scans and return a serializable inventory."""
    agents = home / ".agents"
    layers = [
        scan_claude_memory(home, sample_cap),
        scan_dir_layer("agents_knowledge", agents / "knowledge", sample_cap),
        scan_dir_layer("agents_learnings", agents / "learnings", sample_cap),
        scan_ao(home),
        scan_dir_layer("openclaw", home / ".openclaw" / "wiki", sample_cap),
        scan_bd(),
    ]
    total_bytes = sum(layer.bytes for layer in layers)
    return {
        "schema": "bd-first-memory.audit.v1",
        "home": str(home),
        "layers": [asdict(layer) for layer in layers],
        "totals": {
            "layers_present": sum(1 for layer in layers if layer.present),
            "bytes": total_bytes,
            "mib": round(total_bytes / (1024 * 1024), 1),
        },
    }


def _fmt_human(report: dict[str, object]) -> str:
    lines = [f"Memory-layer audit for {report['home']}"]
    for layer in report["layers"]:  # type: ignore[index]
        if not layer["present"]:
            reason = f" ({layer['notes'][0]})" if layer["notes"] else ""
            lines.append(f"  - {layer['name']:<18} absent{reason}")
            continue
        mib = round(layer["bytes"] / (1024 * 1024), 1)
        extra = ""
        if layer["dup_files"]:
            extra = f"  dup_files={layer['dup_files']} in {layer['dup_groups']} groups"
        if layer["maturity"]:
            extra += "  maturity=" + ",".join(f"{k}:{v}" for k, v in layer["maturity"].items())
        lines.append(f"  - {layer['name']:<18} files={layer['file_count']:<7} {mib}MiB{extra}")
        for note in layer["notes"]:
            lines.append(f"      · {note}")
    t = report["totals"]  # type: ignore[index]
    lines.append(f"  TOTAL present={t['layers_present']}  {t['mib']}MiB")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Read-only inventory of agent-memory layers.")
    parser.add_argument("--home", default=str(Path.home()), help="target home dir (default: $HOME)")
    parser.add_argument(
        "--max-dup-sample", type=int, default=20000, help="cap files hashed for dup-rate"
    )
    fmt = parser.add_mutually_exclusive_group()
    fmt.add_argument("--json", action="store_true", help="emit JSON (default)")
    fmt.add_argument("--human", action="store_true", help="emit a human-readable report")
    args = parser.parse_args(argv)

    report = run_scan(Path(args.home).expanduser(), args.max_dup_sample)
    if args.human:
        print(_fmt_human(report))
    else:
        print(json.dumps(report, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
