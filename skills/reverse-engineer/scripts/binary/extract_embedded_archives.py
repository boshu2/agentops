#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import PurePosixPath
import stat
import sys
import zipfile
from dataclasses import dataclass
from io import BytesIO
from pathlib import Path


@dataclass(frozen=True)
class Candidate:
    offset: int
    file_count: int
    score: int


def _sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def _find_offsets(data: bytes, max_hits: int = 5000) -> list[int]:
    sig = b"PK\x03\x04"
    hits: list[int] = []
    start = 0
    while len(hits) < max_hits:
        i = data.find(sig, start)
        if i < 0:
            break
        hits.append(i)
        start = i + 1
    return hits


def _score_names(names: list[str]) -> int:
    exts = {".py": 5, ".js": 4, ".ts": 4, ".go": 4, ".md": 2, ".yaml": 2, ".yml": 2, ".json": 2, ".toml": 2}
    score = 0
    for n in names:
        for ext, w in exts.items():
            if n.endswith(ext):
                score += w
                break
    # Reward file count lightly.
    score += min(len(names), 200)
    return score


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--binary", required=True)
    ap.add_argument("--out-dir", required=True, help="Directory to extract archives into.")
    ap.add_argument("--max-candidates", type=int, default=200)
    ap.add_argument("--output-root", required=True)
    ap.add_argument("--max-binary-bytes", required=True, type=int)
    args = ap.parse_args()

    if not 0 < args.max_candidates <= 200:
        ap.error("--max-candidates must be in [1,200]")
    if not 0 < args.max_binary_bytes <= 256 * 1024 * 1024:
        ap.error("--max-binary-bytes must be in [1,268435456]")
    binary = Path(args.binary)
    if binary.is_symlink():
        ap.error("binary must not be a symlink")
    info = binary.stat()
    if not stat.S_ISREG(info.st_mode) or info.st_size > args.max_binary_bytes:
        ap.error("binary is not a bounded regular file")
    root_arg = Path(args.output_root)
    if not root_arg.is_absolute() or root_arg.is_symlink():
        ap.error("--output-root must be an absolute non-symlink directory")
    output_root = root_arg.resolve(strict=True)
    out_dir = Path(args.out_dir)
    if out_dir.is_symlink():
        ap.error("out-dir must not be a symlink")
    try:
        out_dir.parent.resolve(strict=True).relative_to(output_root)
    except (OSError, ValueError):
        ap.error("out-dir must stay beneath --output-root")

    data = binary.read_bytes()
    offsets = _find_offsets(data)

    cands: list[Candidate] = []
    opened = 0
    for off in offsets[: args.max_candidates]:
        try:
            with zipfile.ZipFile(BytesIO(data[off:])) as zf:
                names = zf.namelist()
                cands.append(Candidate(offset=off, file_count=len(names), score=_score_names(names)))
                opened += 1
        except Exception:
            continue

    if not cands:
        out_dir.mkdir(parents=True, exist_ok=True)
        (out_dir / "extract.NOOP.md").write_text(
            f"# Extract Embedded Archives (No-Op)\n\nNo embedded ZIP archives could be opened.\n\nBinary: `{binary.name}`\n",
            encoding="utf-8",
        )
        return 0

    best = sorted(cands, key=lambda c: (-c.score, -c.file_count, c.offset))[0]
    with zipfile.ZipFile(BytesIO(data[best.offset:])) as zf:
        # Bound decompression against zip bombs: this extracts an archive carved
        # from attacker-controlled binary bytes. Refuse an oversized member or
        # total uncompressed size before writing anything to disk.
        max_member = 16 * 1024 * 1024
        max_total = 64 * 1024 * 1024
        total = 0
        members = zf.infolist()
        if len(members) > 10_000:
            print("refusing embedded archive with more than 10000 members", file=sys.stderr)
            return 1
        safe_members: list[tuple[zipfile.ZipInfo, PurePosixPath]] = []
        for member in members:
            total += member.file_size
            name = member.filename.replace("\\", "/")
            relative = PurePosixPath(name)
            unix_mode = (member.external_attr >> 16) & 0xFFFF
            if (
                member.file_size > max_member
                or total > max_total
                or len(name.encode("utf-8", errors="replace")) > 4096
                or relative.is_absolute()
                or ".." in relative.parts
                or not relative.parts
                or stat.S_ISLNK(unix_mode)
                or stat.S_IFMT(unix_mode) not in (0, stat.S_IFREG, stat.S_IFDIR)
            ):
                print(
                    f"refusing to extract embedded archive at offset {best.offset}: "
                    "member path, type, or size exceeds bounds",
                    file=sys.stderr,
                )
                return 1
            safe_members.append((member, relative))

        out_dir.mkdir(parents=True, exist_ok=True)
        dest = out_dir / f"zip@{best.offset}"
        dest.mkdir()
        for member, relative in safe_members:
            target = dest.joinpath(*relative.parts)
            if member.is_dir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            target.parent.mkdir(parents=True, exist_ok=True)
            with zf.open(member) as source, target.open("xb") as sink:
                remaining = member.file_size
                while remaining:
                    chunk = source.read(min(1024 * 1024, remaining))
                    if not chunk:
                        break
                    sink.write(chunk)
                    remaining -= len(chunk)
                if remaining != 0:
                    print("refusing truncated embedded member", file=sys.stderr)
                    return 1

    manifest = {
        "binary": binary.name,
        "binary_sha256": _sha256_file(binary),
        "selected_offset": best.offset,
        "selected_file_count": best.file_count,
        "selected_score": best.score,
        "filename_sha256": [hashlib.sha256(member.filename.encode("utf-8", errors="replace")).hexdigest() for member, _relative in safe_members[:500]],
        "note": "Do not paste or commit extracted content. Reports must reference paths/hashes only.",
    }
    (dest / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    # Convenience pointer for downstream scripts.
    (out_dir / "PRIMARY.txt").write_text(dest.name + "\n", encoding="utf-8")

    print(f"OK: extracted {best.file_count} bounded members")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
