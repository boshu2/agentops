#!/usr/bin/env python3
import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OFFICIAL = ROOT / "docs/audits/manifests/external-skill-official-2026-07-09.txt"
COMPANIONS = ROOT / "docs/audits/manifests/external-skill-companions-2026-07-09.txt"
EXPECTED = {
    "official": "7642c6f23753f01d305eafb73bc4abd6b60ef01cc138de01ba20fb77bf31f909",
    "companions": "f64f66ae0291accd42e95e93f0b1b553fe7917e191e7e092dcc3be3bdcd20232",
}
LEAVES = ["idea-genie", "dueling-idea-genies", "codebase-recon", "pattern-mining"]
CONSUMERS = ["discovery", "plan", "research", "refactor", "operationalize", "validate"]


def files(root: Path):
    return sorted(path for path in root.rglob("*") if path.is_file() and path.suffix in {".md", ".feature", ".sh"})


def words(path: Path):
    return re.findall(r"[a-z0-9][a-z0-9_-]*", path.read_text(encoding="utf-8", errors="ignore").lower())


def longest_common(left, right):
    previous = [0] * (len(right) + 1)
    best = 0
    for token in left:
        current = [0]
        for j, other in enumerate(right, 1):
            value = previous[j - 1] + 1 if token == other else 0
            current.append(value)
            best = max(best, value)
        previous = current
    return best


def compare(left_root: Path, right_root: Path, minimum: int):
    findings = []
    for left in files(left_root):
        lw = words(left)
        for right in files(right_root):
            run = longest_common(lw, words(right))
            if run >= minimum:
                findings.append((run, left, right))
    return sorted(findings, reverse=True, key=lambda item: item[0])


def sha(path: Path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def validate_receipt(path: Path):
    data = json.loads(path.read_text(encoding="utf-8"))
    required = ["implementation_context_id", "allowed_path_prompt_sha256", "manifest_sha256", "compared_source_paths", "external_root", "longest_shared_sequence_words", "similarity_disposition", "ci_denylist", "independent_review"]
    missing = [key for key in required if key not in data]
    if missing:
        raise ValueError("receipt missing fields: " + ", ".join(missing))
    if not data["implementation_context_id"] or data["implementation_context_id"] in {"unknown", "placeholder"}:
        raise ValueError("receipt implementation context is missing")
    if not re.fullmatch(r"[0-9a-f]{64}", data["allowed_path_prompt_sha256"]):
        raise ValueError("receipt prompt digest invalid")
    if data["manifest_sha256"] != EXPECTED:
        raise ValueError("receipt manifest digests mismatch")
    if not data["compared_source_paths"]:
        raise ValueError("receipt compared paths empty")
    if data["ci_denylist"] != "PASS" or data["independent_review"] != "PASS":
        raise ValueError("receipt gates are not PASS")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--external-root", type=Path)
    parser.add_argument("--agentops-root", type=Path)
    parser.add_argument("--min-words", type=int, default=24)
    parser.add_argument("--mesh-only", action="store_true")
    parser.add_argument("--check-manifests", action="store_true")
    parser.add_argument("--official-manifest", type=Path, default=OFFICIAL)
    parser.add_argument("--companions-manifest", type=Path, default=COMPANIONS)
    parser.add_argument("--check-receipt", type=Path)
    parser.add_argument("--ci-denylist", action="store_true")
    args = parser.parse_args()
    failures = []

    if args.external_root and args.agentops_root:
        findings = compare(args.external_root, args.agentops_root, args.min_words)
        if findings:
            run, left, right = findings[0]
            failures.append(f"shared sequence {run} words: {left} <> {right}")

    if args.mesh_only:
        leaf_root = ROOT / "skills"
        for leaf in LEAVES:
            for consumer in CONSUMERS:
                findings = compare(leaf_root / leaf, leaf_root / consumer, 36)
                if findings:
                    failures.append(f"consumer duplicates leaf workflow: {consumer} <> {leaf} ({findings[0][0]} words)")

    if args.check_manifests:
        if sha(args.official_manifest) != EXPECTED["official"]:
            failures.append("official manifest digest mismatch")
        if sha(args.companions_manifest) != EXPECTED["companions"]:
            failures.append("companion manifest digest mismatch")

    if args.check_receipt:
        try:
            validate_receipt(args.check_receipt)
        except Exception as exc:
            failures.append(str(exc))

    if args.ci_denylist:
        deny = re.compile(r"\b(jeffreys?|idea-wizard|dueling-idea-wizards|gemini -p|claude --print|claude -p)\b", re.I)
        for leaf in LEAVES:
            for path in files(ROOT / "skills" / leaf):
                if deny.search(path.read_text(encoding="utf-8", errors="ignore")):
                    failures.append(f"forbidden external/provider expression: {path}")

    if failures:
        for failure in failures:
            print("FAIL: " + failure, file=sys.stderr)
        return 1
    print("clean-room checks: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
