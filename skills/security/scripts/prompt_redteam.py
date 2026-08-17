#!/usr/bin/env python3
from __future__ import annotations

import argparse
import glob
import hashlib
import json
import re
import signal
import stat
import sys
import time
from pathlib import Path
from typing import Any


FAIL_EXIT_CODE = 3
SCHEMA_VERSION = 1
MAX_PACK_BYTES = 1024 * 1024
MAX_SOURCE_BYTES = 4 * 1024 * 1024
MAX_SOURCE_FILES = 10000
MAX_REGEX_BYTES = 512


def _now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def _write_json(path: Path, data: dict[str, Any]) -> None:
    _ensure_dir(path.parent)
    path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def _write_text(path: Path, text: str) -> None:
    _ensure_dir(path.parent)
    path.write_text(text, encoding="utf-8")


def _load_pack(path: Path) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file() or path.stat().st_size > MAX_PACK_BYTES:
        raise ValueError("pack must be a bounded regular non-symlink file")
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise ValueError(f"pack file not found: {path}") from exc
    except json.JSONDecodeError as exc:
        raise ValueError(f"pack file is not valid JSON: {path}: {exc}") from exc

    if data.get("schema_version") != SCHEMA_VERSION:
        raise ValueError(f"unsupported schema_version in {path}: {data.get('schema_version')!r}")

    cases = data.get("cases")
    if not isinstance(cases, list) or not cases or len(cases) > 128:
        raise ValueError(f"pack file must contain a non-empty cases array: {path}")

    for idx, case in enumerate(cases, start=1):
        if not isinstance(case, dict):
            raise ValueError(f"case #{idx} is not an object")
        for field in ("id", "title", "attack_prompt", "severity", "targets"):
            if not case.get(field):
                raise ValueError(f"case #{idx} missing required field: {field}")
        if case["severity"] not in {"fail", "warn"}:
            raise ValueError(f"case {case['id']} has unsupported severity: {case['severity']}")
        if not isinstance(case["targets"], list) or not case["targets"]:
            raise ValueError(f"case {case['id']} must define at least one target")
        for target in case["targets"]:
            if not isinstance(target, dict):
                raise ValueError(f"case {case['id']} contains a non-object target")
            if not target.get("globs"):
                raise ValueError(f"case {case['id']} target missing globs")
            if not target.get("require_groups") and not target.get("forbidden_any"):
                raise ValueError(
                    f"case {case['id']} target must define require_groups and/or forbidden_any",
                )
            patterns = list(target.get("forbidden_any", []))
            patterns.extend(target.get("applies_if_any", []))
            for group in target.get("require_groups", []):
                patterns.extend(group.get("patterns", []))
            if len(patterns) > 128 or any(not isinstance(pattern, str) or len(pattern.encode()) > MAX_REGEX_BYTES for pattern in patterns):
                raise ValueError(f"case {case['id']} exceeds regex count or size bounds")
    return data


def _compile_regex(pattern: str) -> re.Pattern[str]:
    return re.compile(pattern, re.IGNORECASE | re.MULTILINE)


def _match_excerpt(text: str, pattern: str) -> str | None:
    class RegexTimedOut(Exception):
        pass

    def timed_out(_signum: int, _frame: Any) -> None:
        raise RegexTimedOut()

    previous = signal.signal(signal.SIGALRM, timed_out)
    signal.setitimer(signal.ITIMER_REAL, 0.1)
    try:
        match = _compile_regex(pattern).search(text)
    except RegexTimedOut as exc:
        raise ValueError("regex evaluation exceeded 100ms") from exc
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous)
    if not match:
        return None
    line_start = text.rfind("\n", 0, match.start()) + 1
    line_end = text.find("\n", match.end())
    if line_end == -1:
        line_end = len(text)
    excerpt = text[line_start:line_end].strip()
    return "sha256:" + hashlib.sha256(excerpt.encode("utf-8", errors="replace")).hexdigest()


def _expand_globs(repo_root: Path, patterns: list[str]) -> list[str]:
    matches: set[str] = set()
    for pattern in patterns:
        for rel in glob.glob(pattern, root_dir=str(repo_root), recursive=True):
            candidate = Path(rel)
            full = repo_root / candidate
            if full.is_symlink():
                continue
            try:
                resolved = full.resolve(strict=True)
                resolved.relative_to(repo_root)
            except (OSError, ValueError):
                continue
            if resolved.is_file() and resolved.stat().st_size <= MAX_SOURCE_BYTES:
                matches.add(candidate.as_posix())
            if len(matches) > MAX_SOURCE_FILES:
                raise ValueError("matched source file count exceeds 10000")
    return sorted(matches)


def _evaluate_file(rel_path: str, text: str, target: dict[str, Any]) -> dict[str, Any]:
    applies_if_any = target.get("applies_if_any", [])
    if applies_if_any and not any(_match_excerpt(text, pattern) for pattern in applies_if_any):
        return {
            "path": rel_path,
            "status": "SKIP",
            "missing_groups": [],
            "forbidden_matches": [],
            "evidence": [],
            "reason": "target did not meet applies_if_any conditions",
        }

    evidence: list[dict[str, str]] = []
    missing_groups: list[dict[str, Any]] = []
    for group in target.get("require_groups", []):
        label = group.get("label", "unnamed requirement")
        matched = None
        for pattern in group.get("patterns", []):
            excerpt = _match_excerpt(text, pattern)
            if excerpt:
                matched = {
                    "label": label,
                    "pattern_sha256": hashlib.sha256(pattern.encode()).hexdigest(),
                    "evidence_line_sha256": excerpt.removeprefix("sha256:"),
                }
                break
        if matched:
            evidence.append(matched)
        else:
            missing_groups.append({"label": label})

    forbidden_matches: list[dict[str, str]] = []
    for pattern in target.get("forbidden_any", []):
        excerpt = _match_excerpt(text, pattern)
        if excerpt:
            forbidden_matches.append({
                "pattern_sha256": hashlib.sha256(pattern.encode()).hexdigest(),
                "evidence_line_sha256": excerpt.removeprefix("sha256:"),
            })

    status = "PASS" if not missing_groups and not forbidden_matches else "FAIL"
    return {
        "path": rel_path,
        "status": status,
        "missing_groups": missing_groups,
        "forbidden_matches": forbidden_matches,
        "evidence": evidence,
    }


def _target_label(target: dict[str, Any]) -> str:
    label = target.get("label")
    if isinstance(label, str) and label.strip():
        return label.strip()
    globs = target.get("globs", [])
    return ", ".join(globs[:2]) if globs else "unnamed target"


def _aggregate_case_status(severity: str, target_results: list[dict[str, Any]]) -> str:
    failed = any(target["status"] == "FAIL" for target in target_results)
    if failed:
        return "FAIL" if severity == "fail" else "WARN"
    warned = any(target["status"] == "WARN" for target in target_results)
    if warned:
        return "WARN"
    return "PASS"


def _evaluate_case(repo_root: Path, case: dict[str, Any]) -> dict[str, Any]:
    target_results: list[dict[str, Any]] = []
    for target in case["targets"]:
        matched_files = _expand_globs(repo_root, list(target.get("globs", [])))
        file_results: list[dict[str, Any]] = []
        if not matched_files:
            target_results.append(
                {
                    "label": _target_label(target),
                    "globs": target.get("globs", []),
                    "matched_files": [],
                    "status": "FAIL",
                    "files": [],
                    "reason": "no files matched target globs",
                },
            )
            continue

        for rel_path in matched_files:
            source = repo_root / rel_path
            if source.is_symlink() or source.stat().st_size > MAX_SOURCE_BYTES:
                raise ValueError("source changed to an unsafe type or size during scan")
            text = source.read_text(encoding="utf-8", errors="ignore")
            file_results.append(_evaluate_file(rel_path, text, target))

        target_status = "PASS"
        if any(result["status"] == "FAIL" for result in file_results):
            target_status = "FAIL"
        elif any(result["status"] == "WARN" for result in file_results):
            target_status = "WARN"

        target_results.append(
            {
                "label": _target_label(target),
                "globs": target.get("globs", []),
                "matched_files": matched_files,
                "status": target_status,
                "files": file_results,
            },
        )

    case_status = _aggregate_case_status(case["severity"], target_results)
    return {
        "id": case["id"],
        "title": case["title"],
        "severity": case["severity"],
        "attack_prompt_sha256": hashlib.sha256(case["attack_prompt"].encode()).hexdigest(),
        "status": case_status,
        "targets": target_results,
    }


def _build_report(repo_root: Path, pack_path: Path, pack: dict[str, Any]) -> dict[str, Any]:
    case_results = [_evaluate_case(repo_root, case) for case in pack["cases"]]
    verdict = "PASS"
    if any(case["status"] == "FAIL" for case in case_results):
        verdict = "FAIL"
    elif any(case["status"] == "WARN" for case in case_results):
        verdict = "WARN"

    matched_files = sorted(
        {
            rel_path
            for case in case_results
            for target in case["targets"]
            for rel_path in target.get("matched_files", [])
        },
    )
    return {
        "schema_version": SCHEMA_VERSION,
        "generated_at": _now_iso(),
        "repo_root_sha256": hashlib.sha256(str(repo_root).encode()).hexdigest(),
        "pack_file": pack_path.name,
        "pack_name": pack.get("name", pack_path.name),
        "verdict": verdict,
        "case_count": len(case_results),
        "files_scanned": matched_files,
        "failed_cases": [case["id"] for case in case_results if case["status"] == "FAIL"],
        "warn_cases": [case["id"] for case in case_results if case["status"] == "WARN"],
        "results": case_results,
    }


def _write_report(out_dir: Path, report: dict[str, Any]) -> None:
    redteam_dir = out_dir / "redteam"
    _write_json(redteam_dir / "redteam-results.json", report)

    lines = [
        "# Prompt Redteam Report",
        "",
        f"- Generated: {report['generated_at']}",
        f"- Repo root SHA256: `{report['repo_root_sha256']}`",
        f"- Pack: `{report['pack_name']}`",
        f"- Verdict: **{report['verdict']}**",
        f"- Cases: `{report['case_count']}`",
        f"- Files scanned: `{len(report['files_scanned'])}`",
        "",
        "## Case Results",
        "",
    ]

    for case in report["results"]:
        lines.extend(
            [
                f"### {case['id']} — {case['status']}",
                "",
                f"- Severity: `{case['severity']}`",
                f"- Attack prompt SHA256: `{case['attack_prompt_sha256']}`",
            ],
        )
        for target in case["targets"]:
            lines.append(f"- Target `{target['label']}`: `{target['status']}`")
            if target.get("reason"):
                lines.append(f"  reason: {target['reason']}")
            for file_result in target.get("files", []):
                lines.append(f"  file `{file_result['path']}`: `{file_result['status']}`")
                for missing in file_result.get("missing_groups", []):
                    lines.append(f"    missing `{missing['label']}`")
                for forbidden in file_result.get("forbidden_matches", []):
                    lines.append(
                        f"    forbidden pattern `{forbidden['pattern_sha256']}` -> evidence line `{forbidden['evidence_line_sha256']}`"
                    )
        lines.append("")

    _write_text(redteam_dir / "redteam-results.md", "\n".join(lines).rstrip() + "\n")


def scan(repo_root: Path, pack_file: Path, out_dir: Path) -> int:
    pack = _load_pack(pack_file)
    report = _build_report(repo_root, pack_file, pack)
    _write_report(out_dir, report)
    return FAIL_EXIT_CODE if report["verdict"] == "FAIL" else 0


def main() -> int:
    parser = argparse.ArgumentParser(prog="prompt_redteam.py")
    sub = parser.add_subparsers(dest="cmd", required=True)

    scan_parser = sub.add_parser("scan")
    scan_parser.add_argument("--repo-root", required=True, help="Repository root to scan")
    scan_parser.add_argument("--pack-root", required=True)
    scan_parser.add_argument("--pack-file", required=True, help="Pack-root-relative JSON attack pack")
    scan_parser.add_argument("--output-root", required=True)
    scan_parser.add_argument("--out-dir", required=True, help="Output-root-relative artifact directory")

    args = parser.parse_args()

    if args.cmd == "scan":
        repo_arg = Path(args.repo_root)
        if not repo_arg.is_absolute() or repo_arg.is_symlink():
            print("error: repo root must be an absolute non-symlink directory", file=sys.stderr)
            return 2
        try:
            repo_root = repo_arg.resolve(strict=True)
            pack_root_arg = Path(args.pack_root)
            output_root_arg = Path(args.output_root)
            if (
                not pack_root_arg.is_absolute()
                or pack_root_arg.is_symlink()
                or not output_root_arg.is_absolute()
                or output_root_arg.is_symlink()
            ):
                raise ValueError("pack/output roots must be absolute non-symlink directories")
            pack_root = pack_root_arg.resolve(strict=True)
            output_root = output_root_arg.resolve(strict=True)
            pack_rel = Path(args.pack_file)
            out_rel = Path(args.out_dir)
            if (
                pack_rel.is_absolute()
                or ".." in pack_rel.parts
                or pack_rel.as_posix() != args.pack_file
                or out_rel.is_absolute()
                or ".." in out_rel.parts
                or out_rel.as_posix() != args.out_dir
            ):
                raise ValueError("pack and output paths must be canonical root-relative paths")
            pack_file = pack_root / pack_rel
            out_dir = output_root / out_rel
            if pack_file.is_symlink() or out_dir.is_symlink():
                raise ValueError("pack and output paths must not be symlinks")
            pack_file.resolve(strict=True).relative_to(pack_root)
            ancestor = out_dir
            while not ancestor.exists() and ancestor != output_root:
                ancestor = ancestor.parent
            ancestor.resolve(strict=True).relative_to(output_root)
            return scan(repo_root, pack_file, out_dir)
        except (OSError, ValueError) as exc:
            print(f"error: {exc}", file=sys.stderr)
            return 2

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
