#!/usr/bin/env bash
# ingest-ci-deterministic-catches.sh — fold CI deterministic-catch artifacts into
# the LOCAL yield gauge ledger (age-activate-deterministic-catch-default-on-i4m,
# part 2: CI direct-persist).
#
# Hooks do NOT fire in CI, so the local pre-push deterministic-catch emitter
# (scripts/emit-deterministic-catch.sh, wired via scripts/hooks/pre-push.local)
# never runs there. Instead, CI's go-gate-shadow job emits a REFUTED /
# mode=deterministic gate-verdict on a gate FAIL and uploads the (ephemeral,
# gitignored) yield ledger as the `ci-deterministic-catch` artifact.
#
# The yield ledger (.agents/yield/yield-ledger.jsonl) is gitignored / LOCAL — so
# there is NO commit-back. A maintainer downloads the artifact(s) and runs this
# to APPEND the CI catches to their local ledger, where `ao yield gauge` reads
# them. Idempotent: a catch already present (by run_id + head_sha + mode +
# disposition) is not re-appended, so re-ingesting the same artifacts is safe.
#
# Usage:
#   ingest-ci-deterministic-catches.sh <artifact-dir> [--ledger <path>]
#     <artifact-dir>  directory holding downloaded yield-ledger.jsonl file(s)
#                     (searched recursively)
#     --ledger <path> target local yield ledger (default:
#                     .agents/yield/yield-ledger.jsonl under the repo root)
set -euo pipefail

usage() { echo "usage: $0 <artifact-dir> [--ledger <path>]" >&2; exit 2; }

ART_DIR="${1:-}"
[ -n "$ART_DIR" ] || usage
shift || true

LEDGER=""
while [ $# -gt 0 ]; do
  case "$1" in
    --ledger) LEDGER="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[ -d "$ART_DIR" ] || { echo "ingest: artifact dir not found: $ART_DIR" >&2; exit 2; }

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
if [ -z "$LEDGER" ]; then
  LEDGER="$root/.agents/yield/yield-ledger.jsonl"
fi
mkdir -p "$(dirname "$LEDGER")"
touch "$LEDGER"

# Resolve ao — REQUIRED: candidates are validated by round-tripping through the
# real yield-ledger reader (`ao yield gauge`, which load-validates every line
# under the closed DisallowUnknownFields schema). We refuse to append anything
# the real reader would reject, so a malformed/hostile artifact can never corrupt
# the local gauge ledger.
AO="${AO_BIN:-}"
if [ -z "$AO" ] && [ -x "$root/cli/bin/ao" ]; then AO="$root/cli/bin/ao"; fi
if [ -z "$AO" ] || [ ! -x "$AO" ]; then
  echo "ingest: ao binary not found (set AO_BIN or build cli/bin/ao) — required to validate candidates; refusing to append unvalidated" >&2
  exit 2
fi

CAND="$(mktemp)"
trap 'rm -f "$CAND"' EXIT

# Stage 1 (structural pre-filter): collect NEW, well-shaped deterministic catches.
# Enforce event=gate-verdict, mode=deterministic, disposition=REFUTED, a string
# head_sha + run_id, and dedup by (run_id, head_sha, mode, disposition). os.walk
# (NOT glob '**', which skips dot-dirs) so artifacts under .agents/ are found.
ART_DIR="$ART_DIR" LEDGER="$LEDGER" CAND="$CAND" python3 - <<'PY'
import os, json

art_dir = os.environ["ART_DIR"]; ledger = os.environ["LEDGER"]; cand = os.environ["CAND"]

def key(ev):
    b = ev.get("body", {}) or {}
    return (ev.get("run_id"), b.get("head_sha"), b.get("mode"), b.get("disposition"))

def is_str(x):
    return isinstance(x, str) and x != ""

seen = set()
with open(ledger) as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        try:
            ev = json.loads(line)
        except Exception:
            continue
        if isinstance(ev, dict) and ev.get("event") == "gate-verdict":
            seen.add(key(ev))

art_paths = []
for dirpath, _dn, filenames in os.walk(art_dir):
    if "yield-ledger.jsonl" in filenames:
        art_paths.append(os.path.join(dirpath, "yield-ledger.jsonl"))

out = open(cand, "w")
n = 0
for path in sorted(art_paths):
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                ev = json.loads(line)
            except Exception:
                continue
            if not isinstance(ev, dict) or ev.get("event") != "gate-verdict":
                continue
            if not is_str(ev.get("run_id")):
                continue
            body = ev.get("body")
            if not isinstance(body, dict):
                continue
            if body.get("mode") != "deterministic" or body.get("disposition") != "REFUTED":
                continue
            if not is_str(body.get("head_sha")):
                continue
            k = key(ev)
            if k in seen:
                continue
            seen.add(k)
            out.write(json.dumps(ev, separators=(",", ":")) + "\n")
            n += 1
out.close()
print("ingest: %d candidate(s) after structural filter" % n)
PY

if [ ! -s "$CAND" ]; then
  echo "ingest: 0 new CI deterministic catch(es) appended to $LEDGER"
  exit 0
fi

# Stage 2 (authoritative validate): the proposed ledger (existing + candidates)
# MUST load via the real reader, or we append nothing (fail-closed, no corruption).
VTMP="$(mktemp -d)"
trap 'rm -f "$CAND"; rm -rf "$VTMP"' EXIT
# `ao yield gauge` resolves the ledger by project context, so the validation dir
# must look like a repo (mirrors the emit test harness). Read-only — never the
# real repo.
( cd "$VTMP" && git init -q && git config user.email ingest@local && git config user.name ingest ) >/dev/null 2>&1
mkdir -p "$VTMP/.agents/yield"
cat "$LEDGER" "$CAND" > "$VTMP/.agents/yield/yield-ledger.jsonl"
# `gauge` requires --run, but load-validation of EVERY line happens before the
# run filter, so any --run validates the whole proposed ledger.
if ! ( cd "$VTMP" && "$AO" yield gauge --run ci-deterministic --json >/dev/null 2>&1 ); then
  echo "ingest: ERROR — proposed ledger fails to load via 'ao yield gauge' (a candidate line is invalid under the closed schema); refusing to append. Inspect the artifact." >&2
  exit 1
fi

cat "$CAND" >> "$LEDGER"
echo "ingest: $(wc -l < "$CAND" | tr -d ' ') new CI deterministic catch(es) appended to $LEDGER (validated via ao yield gauge)"
