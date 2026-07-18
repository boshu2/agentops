#!/usr/bin/env bash
# check-honest-voice.sh — honest-voice claims gate (bead age-5qjyn / FU3).
#
# Scans user-facing string content for CLAIMS of proven / automatic knowledge
# compounding and for hookless-3.0-violating "session hooks" language, per the
# lexicon docs/contracts/forbidden-claims.yaml. Fails (exit 1) naming the
# phrase + file + line + rationale so a dev who reintroduces "knowledge compounds
# automatically" gets told exactly what, where, and why.
#
# WHY: the README's honest version demotes corpus-compounding to "still
# measuring" (ADR-0004, ADR-0011) and 3.0 is hookless (docs/3.0.md, ADR-0009),
# yet CLI strings kept regrowing the claim (#907, FU4 diet commit) because
# nothing gated it. This is that gate.
#
# SCOPE (scanned): cli/cmd + cli/internal Go source (excluding *_test.go) and the
# seed/template assets.
# NOT docs/** — narrative docs discuss the unproven hypothesis honestly.
#
# SUPPRESSION: a line with `honest-voice:allow` is skipped; per-entry
# allowed_contexts globs in the lexicon also exempt a file.
#
# Exit: 0 pass, 1 fail (offenders found), 2 structural error (missing lexicon /
# no PyYAML).
set -euo pipefail

# ROOT is the tree scanned. It defaults to the repo root (script's ../) and is
# overridable via HONEST_VOICE_ROOT so the bats test can scan a throwaway fixture
# tree while pointing HONEST_VOICE_LEXICON at the real lexicon.
ROOT="${HONEST_VOICE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
LEXICON="${HONEST_VOICE_LEXICON:-$ROOT/docs/contracts/forbidden-claims.yaml}"

if [ ! -f "$LEXICON" ]; then
  echo "check-honest-voice: ERROR — lexicon not found: $LEXICON" >&2
  exit 2
fi

export ROOT LEXICON

exec python3 - <<'PY'
import fnmatch
import os
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("check-honest-voice: ERROR — PyYAML not installed (pip install pyyaml)", file=sys.stderr)
    sys.exit(2)

ROOT = Path(os.environ["ROOT"])
LEXICON = Path(os.environ["LEXICON"])

data = yaml.safe_load(LEXICON.read_text()) or {}
claims = data.get("claims", [])
if not claims:
    print("check-honest-voice: ERROR — lexicon has no claims", file=sys.stderr)
    sys.exit(2)

# Compile each claim's pattern once.
compiled = []
for c in claims:
    pat = c.get("pattern")
    if not pat:
        print(f"check-honest-voice: ERROR — claim {c.get('id')!r} has no pattern", file=sys.stderr)
        sys.exit(2)
    try:
        rx = re.compile(pat, re.IGNORECASE)
    except re.error as e:
        print(f"check-honest-voice: ERROR — bad regex for {c.get('id')!r}: {e}", file=sys.stderr)
        sys.exit(2)
    compiled.append({
        "id": c.get("id", "?"),
        "rx": rx,
        "rationale": " ".join((c.get("rationale") or "").split()),
        "allowed": c.get("allowed_contexts") or [],
    })

# Scan set: glob roots relative to repo root. Go source excludes *_test.go;
# template assets are the seed/template YAMLs.
SCAN_GLOBS = [
    ("cli/cmd", "*.go"),
    ("cli/internal", "*.go"),
    ("cli/internal/extract/templates", "*.yaml"),
]

SUPPRESS = "honest-voice:allow"


def rel(p: Path) -> str:
    return p.relative_to(ROOT).as_posix()


def is_allowed(relpath: str, globs) -> bool:
    return any(fnmatch.fnmatch(relpath, g) for g in globs)


files = []
for subdir, pat in SCAN_GLOBS:
    base = ROOT / subdir
    if not base.exists():
        continue
    for p in sorted(base.rglob(pat)):
        if not p.is_file():
            continue
        name = p.name
        if pat == "*.go" and name.endswith("_test.go"):
            continue  # tests assert ABSENCE and may legitimately name a phrase
        files.append(p)

offenders = []
for p in files:
    relpath = rel(p)
    try:
        text = p.read_text(encoding="utf-8", errors="replace")
    except OSError as e:
        print(f"check-honest-voice: ERROR — cannot read {relpath}: {e}", file=sys.stderr)
        sys.exit(2)
    for lineno, line in enumerate(text.splitlines(), start=1):
        if SUPPRESS in line:
            continue
        for c in compiled:
            if c["rx"].search(line):
                if is_allowed(relpath, c["allowed"]):
                    continue
                offenders.append((relpath, lineno, c["id"], line.strip(), c["rationale"]))
    # Second pass: fuse Go string concatenation so a claim split across
    # adjacent literals ("Knowledge compounds " + \n "automatically") cannot
    # evade the line scan. Fusing `" + ... "` (incl. across newlines) joins
    # the literals; matches are attributed to the line where they start.
    # Suppressed lines are honored by checking the starting line's text.
    if p.suffix == ".go":
        # Raw-string pass: a claim wrapped across lines inside a backtick
        # literal has its newlines collapsed so phrase patterns still match.
        for rawm in re.finditer(r"`[^`]*`", text, re.DOTALL):
            raw = rawm.group(0)
            if "\n" not in raw:
                continue  # single-line raw strings were covered by the line scan
            collapsed = re.sub(r"\s+", " ", raw)
            start_line = text.count("\n", 0, rawm.start()) + 1
            src_lines_r = text.splitlines()
            line_txt = src_lines_r[start_line - 1] if start_line <= len(src_lines_r) else ""
            if SUPPRESS in line_txt:
                continue
            for c in compiled:
                if c["rx"].search(collapsed) and not is_allowed(relpath, c["allowed"]):
                    key = (relpath, start_line, c["id"])
                    if not any((o[0], o[1], o[2]) == key for o in offenders):
                        offenders.append((relpath, start_line, c["id"],
                                          "(multiline raw string) " + collapsed.strip()[:120],
                                          c["rationale"]))
        fused = re.sub(r'"\s*\+\s*(?:\r?\n\s*)?(?:/[/*][^\n]*\n\s*)?"', "", text)
        if fused != text:
            src_lines = text.splitlines()
            for c in compiled:
                for m in c["rx"].finditer(fused):
                    if is_allowed(relpath, c["allowed"]):
                        continue
                    approx = fused.count("\n", 0, m.start()) + 1
                    line_txt = src_lines[approx - 1] if approx <= len(src_lines) else ""
                    if SUPPRESS in line_txt:
                        continue
                    key = (relpath, approx, c["id"])
                    if any((o[0], o[1], o[2]) == key for o in offenders):
                        continue  # already caught by the line scan
                    offenders.append((relpath, approx, c["id"],
                                      m.group(0).strip() + "  (fused string concatenation)",
                                      c["rationale"]))

if offenders:
    print("check-honest-voice: FAIL — forbidden claim(s) in user-facing content:", file=sys.stderr)
    for relpath, lineno, cid, snippet, rationale in offenders:
        print(f"  {relpath}:{lineno}  [{cid}]", file=sys.stderr)
        print(f"    > {snippet}", file=sys.stderr)
        print(f"    rationale: {rationale}", file=sys.stderr)
    print("", file=sys.stderr)
    print("  fix: rewrite to honest phrasing (context accrues in .agents/ — whether it", file=sys.stderr)
    print("       compounds is still being measured; 3.0 is hookless), or add a reviewed", file=sys.stderr)
    print("       `honest-voice:allow` on the line. Lexicon: docs/contracts/forbidden-claims.yaml", file=sys.stderr)
    sys.exit(1)

print(f"check-honest-voice: ok ({len(files)} file(s) scanned, no forbidden claims)")
PY
