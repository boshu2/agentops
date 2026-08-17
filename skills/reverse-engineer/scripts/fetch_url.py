#!/usr/bin/env python3
"""Fetch one HTTPS document into an explicitly confined bounded output."""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import stat
import tempfile
import urllib.error
import urllib.parse
import urllib.request


MAX_DOWNLOAD_BYTES = 16 * 1024 * 1024


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # type: ignore[no-untyped-def]
        return None


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("url")
    parser.add_argument("out_path")
    parser.add_argument("--output-root", required=True)
    parser.add_argument(
        "--input-root",
        help="Required authorization root for a local file:// input",
    )
    parser.add_argument("--max-bytes", required=True, type=int)
    args = parser.parse_args(argv)
    if not 0 < args.max_bytes <= MAX_DOWNLOAD_BYTES:
        parser.error(f"--max-bytes must be in [1, {MAX_DOWNLOAD_BYTES}]")
    if len(args.url.encode("utf-8")) > 2048:
        parser.error("URL exceeds 2048 bytes")
    parsed = urllib.parse.urlparse(args.url)
    is_https = parsed.scheme == "https" and bool(parsed.hostname) and not parsed.username and not parsed.password
    is_local = parsed.scheme == "file" and not parsed.netloc and not parsed.query and not parsed.fragment
    if not is_https and not is_local:
        parser.error("only credential-free HTTPS or explicitly rooted local URLs are allowed")

    root_arg = Path(args.output_root)
    if not root_arg.is_absolute() or root_arg.is_symlink():
        parser.error("--output-root must be an absolute non-symlink directory")
    root = root_arg.resolve(strict=True)
    if root_arg != root:
        parser.error("--output-root must use its canonical spelling")
    out_arg = Path(args.out_path)
    if out_arg.is_absolute():
        candidate = out_arg
    else:
        candidate = root / out_arg
    if candidate.is_symlink():
        parser.error("output must not be a symlink")
    try:
        parent = candidate.parent.resolve(strict=True)
        parent.relative_to(root)
    except (OSError, ValueError):
        parser.error("output must stay beneath --output-root")

    if is_local:
        if not args.input_root:
            parser.error("local URLs require --input-root")
        input_arg = Path(args.input_root)
        if not input_arg.is_absolute() or input_arg.is_symlink():
            parser.error("--input-root must be an absolute non-symlink directory")
        input_root = input_arg.resolve(strict=True)
        if input_arg != input_root:
            parser.error("--input-root must use its canonical spelling")
        local_path = Path(urllib.parse.unquote(parsed.path))
        if not local_path.is_absolute() or ".." in local_path.parts:
            parser.error("local URL path must be canonical and absolute")
        cursor = local_path
        while cursor != input_root:
            if cursor.is_symlink():
                parser.error("local URL path must not traverse symlinks")
            if cursor.parent == cursor:
                parser.error("local URL path is outside --input-root")
            cursor = cursor.parent
        try:
            info = local_path.stat(follow_symlinks=False)
        except OSError:
            parser.error("local URL source is unavailable")
        if not stat.S_ISREG(info.st_mode):
            parser.error("local URL source must be a regular file")
        if info.st_size > args.max_bytes:
            parser.error("local URL source exceeds --max-bytes")
        with local_path.open("rb") as handle:
            data = handle.read(args.max_bytes + 1)
    else:
        if args.input_root:
            parser.error("--input-root is only valid with file:// URLs")
        request = urllib.request.Request(args.url, headers={"User-Agent": "reverse-engineer/2.0"})
        opener = urllib.request.build_opener(NoRedirect)
        try:
            with opener.open(request, timeout=10) as response:
                declared = response.headers.get("Content-Length")
                if declared and declared.isdigit() and int(declared) > args.max_bytes:
                    parser.error("response Content-Length exceeds --max-bytes")
                data = response.read(args.max_bytes + 1)
        except urllib.error.HTTPError as exc:
            parser.error(f"HTTPS fetch failed with status {exc.code}")
        except (OSError, urllib.error.URLError) as exc:
            parser.error(f"HTTPS fetch failed: {type(exc).__name__}")
    if len(data) > args.max_bytes:
        parser.error("response exceeds --max-bytes")

    fd, temp_name = tempfile.mkstemp(prefix=".fetch-url.", dir=parent)
    try:
        with os.fdopen(fd, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temp_name, candidate)
    finally:
        try:
            os.unlink(temp_name)
        except FileNotFoundError:
            pass
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
