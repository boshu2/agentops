#!/usr/bin/env python3
"""Phase 1 — salvage-candidate classifier.

Consumes the layer inventory and partitions every memory item into KEEP (import
to bd) vs DROP (retire), with a per-item reason. Near-duplicates are collapsed to
one keeper. Output is a dry-run manifest: what imports, what retires, estimated
disk reclaimed, estimated memory count after migration. Writes nothing destructive.

Classification rules (bead .8):
  KEEP  - .agents/knowledge authored files; Claude topic files (not MEMORY.md);
          ao Established/Candidate/Anti-Pattern nodes (by count); existing bd memories.
  DROP  - ao Provisional dump; N×-duplicated .agents/learnings ``-pend-*`` extracts
          and dup-group members; the bloated/pasted MEMORY.md session blobs.
  DEDUP - within a layer, keep one file per content-hash group; the rest are DROP.

Usage:
    classify.py [--home DIR] [--out manifest.json] [--max-sample N]
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path

_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

import audit_scan  # noqa: E402  (sibling module; path injected above)


@dataclass
class Item:
    """One classified memory item."""

    layer: str
    path: str
    verdict: str          # "keep" | "drop"
    reason: str
    bytes: int = 0


@dataclass
class Manifest:
    """Dry-run salvage manifest produced by the classifier."""

    schema: str = "bd-first-memory.manifest.v1"
    home: str = ""
    keep: list[Item] = field(default_factory=list)
    drop: list[Item] = field(default_factory=list)
    summary: dict[str, object] = field(default_factory=dict)


def _file_sha1(p: Path) -> str | None:
    try:
        return hashlib.sha1(p.read_bytes()).hexdigest()  # noqa: S324 - content id
    except OSError:
        return None


def _safe_size(p: Path) -> int:
    try:
        return p.stat().st_size
    except OSError:
        return 0


def classify_learnings(root: Path, sample_cap: int) -> tuple[list[Item], list[Item]]:
    """Classify .agents/learnings: drop ``-pend-*`` extracts and dup-group members."""
    keep: list[Item] = []
    drop: list[Item] = []
    if not root.exists():
        return keep, drop
    seen_hashes: set[str] = set()
    for p in audit_scan._iter_files(root)[:sample_cap]:
        size = _safe_size(p)
        if "-pend-" in p.name:
            drop.append(Item("agents_learnings", str(p), "drop", "pending-extract dump", size))
            continue
        digest = _file_sha1(p)
        if digest is not None and digest in seen_hashes:
            drop.append(Item("agents_learnings", str(p), "drop", "duplicate content", size))
            continue
        if digest is not None:
            seen_hashes.add(digest)
        keep.append(Item("agents_learnings", str(p), "keep", "unique authored learning", size))
    return keep, drop


def classify_knowledge(root: Path) -> list[Item]:
    """Classify .agents/knowledge: all authored content is a keeper."""
    if not root.exists():
        return []
    return [
        Item("agents_knowledge", str(p), "keep", "authored knowledge", _safe_size(p))
        for p in audit_scan._iter_files(root)
    ]


def classify_claude_memory(home: Path) -> tuple[list[Item], list[Item]]:
    """Classify Claude auto-memory: topic files keep; bloated MEMORY.md indexes drop."""
    keep: list[Item] = []
    drop: list[Item] = []
    base = home / ".claude" / "projects"
    if not base.exists():
        return keep, drop
    for mem in base.glob("*/memory"):
        if not mem.is_dir():
            continue
        for p in audit_scan._iter_files(mem):
            size = _safe_size(p)
            if p.name == "MEMORY.md":
                lines = p.read_text(errors="replace").count("\n") + 1
                if lines > 200:
                    reason = f"bloated index ({lines} lines)"
                    drop.append(Item("claude_memory", str(p), "drop", reason, size))
                # a healthy MEMORY.md is a derived cache — neither keep nor drop.
                continue
            keep.append(Item("claude_memory", str(p), "keep", "authored topic file", size))
    return keep, drop


def build_manifest(home: Path, sample_cap: int) -> Manifest:
    """Run all classifiers + fold in ao/bd counts into a single manifest."""
    man = Manifest(home=str(home))

    lk = classify_knowledge(home / ".agents" / "knowledge")
    man.keep.extend(lk)

    keep_learn, drop_learn = classify_learnings(home / ".agents" / "learnings", sample_cap)
    man.keep.extend(keep_learn)
    man.drop.extend(drop_learn)

    keep_cm, drop_cm = classify_claude_memory(home)
    man.keep.extend(keep_cm)
    man.drop.extend(drop_cm)

    # ao: classify by maturity counts (individual nodes not enumerable via CLI here).
    ao = audit_scan.scan_ao(home)
    ao_keep = sum(ao.maturity.get(k, 0) for k in ("established", "candidate", "anti-pattern"))
    ao_drop = ao.maturity.get("provisional", 0)

    # bd: existing memories persist after migration.
    bd = audit_scan.scan_bd(home)
    bd_existing = bd.file_count

    drop_bytes = sum(i.bytes for i in man.drop)
    man.summary = {
        "keep_files": len(man.keep),
        "drop_files": len(man.drop),
        "ao_keep_nodes": ao_keep,
        "ao_drop_nodes": ao_drop,
        "est_disk_reclaimed_mib": round(drop_bytes / (1024 * 1024), 1),
        "est_memories_after": bd_existing + len(man.keep) + ao_keep,
        "sampled": sample_cap,
        "dry_run": True,
    }
    return man


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. Returns process exit code."""
    parser = argparse.ArgumentParser(description="Classify memory items keep-vs-drop (dry-run).")
    parser.add_argument("--home", default=str(Path.home()), help="target home dir (default: $HOME)")
    parser.add_argument("--out", default="", help="write manifest JSON here (default: stdout)")
    parser.add_argument("--max-sample", type=int, default=200000, help="cap files classified/pile")
    args = parser.parse_args(argv)

    man = build_manifest(Path(args.home).expanduser(), args.max_sample)
    payload = json.dumps(asdict(man), indent=2)
    if args.out:
        Path(args.out).expanduser().write_text(payload)
        s = man.summary
        print(
            f"manifest -> {args.out}: keep={s['keep_files']} drop={s['drop_files']} "
            f"ao_keep={s['ao_keep_nodes']} ao_drop={s['ao_drop_nodes']} "
            f"reclaim~{s['est_disk_reclaimed_mib']}MiB"
        )
    else:
        print(payload)
    return 0


if __name__ == "__main__":
    sys.exit(main())
