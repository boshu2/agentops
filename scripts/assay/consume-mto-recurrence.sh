#!/usr/bin/env bash
# consume-mto-recurrence: the AgentOps PULL side of the MTO->AgentOps recurrence
# loop (ag-qjccl). Reads a tracker-INDEPENDENT JSON FILE handoff written by the
# MTO bridge — NOT the retired `bd memories` store.
#
# Handoff file contract (.agents/mto-handoff/recurrence.json):
#   {"recurred_classes": <int>, "max_class_recurrence": <int|null>,
#    "recurred_dimensions": <int>, "total_classes": <int>,
#    "recurred_class_names": ["<class> [<dim>]x<n>", ...],
#    "date": "<str>", "source": "<str>", "verdict": "<TRIPWIRE|clean>"}
#
# Four states, branched explicitly — NONE is a silent fail-open (the D1 class the
# MTO gate keeps catching):
#   1. file ABSENT       -> benign no-op, exit 0 ("run the MTO bridge first")
#   2. file UNPARSEABLE  -> FAIL-CLOSED, exit non-zero (jq type checks; never no-op)
#   3. CLEAN  (recurred_classes==0) -> no-op, write NO file, exit 0
#   4. TRIPWIRE (recurred_classes>0) -> idempotently materialize a finding +
#      planning-rule under STABLE ids (finding-id-keyed paths) so re-runs update
#      in place rather than duplicating.
#
# No wall-clock: the finding's date is resolved from --date / $EFFICACY_DATE_STAMP
# if set, ELSE the handoff file's own `.date` (written by the MTO producer). The
# resolved date must pass BOTH a YYYY-MM-DD shape regex AND a real calendar-validity
# parse (python3 date.fromisoformat, BSD `date -j -f` fallback) or we FAIL-CLOSED.
# A shape-valid-but-bogus date ("UNDATED"/"?"/absent, but also 2026-99-99 / a
# non-leap 2026-02-29) is rejected by the runtime schema (finding-artifact.schema.json:
# date has format "date", enforced via FormatChecker), so we refuse to emit one.
# Door 9: never `claude -p`/`--print`; only jq/mkdir/printf.

set -euo pipefail

# ---- defaults ----
HANDOFF_FILE=".agents/mto-handoff/recurrence.json"
PLANNING_DIR=".agents/planning-rules"
FINDINGS_DIR=".agents/findings"
DATE_STAMP="${EFFICACY_DATE_STAMP:-}"
DRY_RUN=0

# Stable finding id. Per finding-compiler.md the planning-rule path is keyed by
# the SAME id (.agents/planning-rules/<finding-id>.md), so the runtime can look
# the rule up by finding id. Both artifacts share this one id.
FINDING_ID="f-mto-recurrence-handoff"

usage() {
  cat <<'EOF'
consume-mto-recurrence — AgentOps PULL side of the MTO recurrence file handoff (ag-qjccl)

Usage:
  scripts/assay/consume-mto-recurrence.sh [flags]

Flags:
  --handoff-file <path>  JSON handoff to read. Default: .agents/mto-handoff/recurrence.json
  --dry-run              Print what WOULD be written; touch nothing. Hermetic.
  --date <YYYY-MM-DD>    Date stamp for materialized artifacts (no wall-clock).
                         Falls back to $EFFICACY_DATE_STAMP, then the handoff
                         file's own .date. Must be YYYY-MM-DD or the tripwire
                         path FAILS-CLOSED (no schema-conformant date, no finding).
  --planning-dir <path>  Planning-rules output dir. Default: .agents/planning-rules
  --findings-dir <path>  Findings output dir.       Default: .agents/findings
  -h, --help             This help.

Four states (each handled, none fail-open):
  absent      -> exit 0, "run the MTO bridge first"
  unparseable -> exit non-zero (FAIL-CLOSED)
  clean       -> exit 0, NO file written
  tripwire    -> materialize finding + planning-rule (idempotent, update-in-place)
EOF
}

log() { printf '%s\n' "$*" >&2; }
die() { log "consume-mto-recurrence: ERROR: $*"; exit 1; }

# ---- arg parse ----
while [ $# -gt 0 ]; do
  case "$1" in
    --handoff-file) HANDOFF_FILE="${2:?--handoff-file needs a path}"; shift 2 ;;
    --handoff-file=*) HANDOFF_FILE="${1#*=}"; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --date) DATE_STAMP="${2:?--date needs a value}"; shift 2 ;;
    --date=*) DATE_STAMP="${1#*=}"; shift ;;
    --planning-dir) PLANNING_DIR="${2:?--planning-dir needs a path}"; shift 2 ;;
    --planning-dir=*) PLANNING_DIR="${1#*=}"; shift ;;
    --findings-dir) FINDINGS_DIR="${2:?--findings-dir needs a path}"; shift 2 ;;
    --findings-dir=*) FINDINGS_DIR="${1#*=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1 (try --help)" ;;
  esac
done

# NOTE: do NOT default DATE_STAMP here. An explicit --date / $EFFICACY_DATE_STAMP
# wins; otherwise the finding's date comes from the handoff's own `.date` (resolved
# + validated in STATE 4). A finding can't be schema-conformant without a real date.
command -v jq >/dev/null 2>&1 || die "jq not found on PATH (required to parse the handoff file)"

# ---- STATE 1: file ABSENT (benign — nothing to consume is not an error) ----
if [ ! -e "$HANDOFF_FILE" ]; then
  printf 'no MTO handoff file yet — run the MTO bridge first (%s)\n' "$HANDOFF_FILE"
  exit 0
fi

# ---- STATE 2: present but UNPARSEABLE -> FAIL-CLOSED ----
# Unparseable = not valid JSON, OR .recurred_classes absent / not a non-negative
# integer. jq -e drives the exit code; the type guard rejects strings like "0",
# malformed tokens like 0abc (invalid JSON -> `jq empty` fails), and negatives.
# `.recurred_classes|type=="number" and .>=0 and .==floor` is the load-bearing
# fail-closed check — a string "0" fails type=="number"; -1 fails .>=0; 1.5 fails
# .==floor. NEVER silently no-op on malformed input.
if ! jq empty "$HANDOFF_FILE" 2>/dev/null; then
  die "handoff file is not valid JSON — FAIL-CLOSED (refusing to silently no-op): $HANDOFF_FILE"
fi
if ! jq -e '.recurred_classes | (type=="number" and . >= 0 and . == floor)' \
     "$HANDOFF_FILE" >/dev/null 2>&1; then
  die ".recurred_classes is absent or not a non-negative integer — FAIL-CLOSED: $HANDOFF_FILE"
fi

RECURRED_CLASSES="$(jq -r '.recurred_classes' "$HANDOFF_FILE")"

# ---- STATE 3: CLEAN (recurred_classes==0) -> no-op, write NO file ----
if [ "$RECURRED_CLASSES" -eq 0 ]; then
  printf 'clean signal — no recurring finding-class (recurred_classes=0), nothing to materialize\n'
  exit 0
fi

# ---- STATE 4: TRIPWIRE -> materialize finding + planning-rule (idempotent) ----
# Read the rest of the payload (best-effort; tripwire requires only the count).
RECURRED_DIMS="$(jq -r '.recurred_dimensions // "?"' "$HANDOFF_FILE")"
MAX_RECUR="$(jq -r '.max_class_recurrence // "?"' "$HANDOFF_FILE")"
TOTAL_CLASSES="$(jq -r '.total_classes // "?"' "$HANDOFF_FILE")"
SIGNAL_DATE="$(jq -r '.date // "?"' "$HANDOFF_FILE")"
SIGNAL_SOURCE="$(jq -r '.source // "MTO gate ASSAY recurrence handoff"' "$HANDOFF_FILE")"

# ---- resolve + VALIDATE the finding date (fail-closed) ----
# Priority: explicit --date / $EFFICACY_DATE_STAMP (DATE_STAMP set above), ELSE the
# handoff's own .date. The runtime schema requires `date` to be format "date"
# (YYYY-MM-DD); "UNDATED"/"?"/absent would be REJECTED — so refuse to emit a
# non-conformant finding rather than write one the runtime will throw out.
if [ -z "$DATE_STAMP" ]; then
  DATE_STAMP="$SIGNAL_DATE"
fi
# (a) cheap SHAPE pre-filter: must look like YYYY-MM-DD before we bother parsing.
case "$DATE_STAMP" in
  [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) : ;;  # YYYY-MM-DD shape only
  *) die "no valid finding date — got '${DATE_STAMP:-<empty>}' (need YYYY-MM-DD from --date, \$EFFICACY_DATE_STAMP, or the handoff's .date). FAIL-CLOSED: a finding cannot be schema-conformant without a valid date." ;;
esac
# (b) AUTHORITATIVE calendar-validity gate. The shape regex above ACCEPTS bogus
# dates (2026-99-99, 2026-02-29 in a non-leap year) that the runtime schema's
# `format: "date"` (a real Draft202012Validator + FormatChecker) REJECTS — so a
# regex-only check is fail-OPEN against the schema contract. Verify it is a REAL
# calendar date and FAIL-CLOSED if not. Prefer python3 (date.fromisoformat rejects
# both 2026-99-99 and 2026-02-29, matching the schema validator); fall back to a
# strict BSD `date -j -f` parse only if python3 is absent.
if command -v python3 >/dev/null 2>&1; then
  python3 -c 'import sys,datetime; datetime.date.fromisoformat(sys.argv[1])' "$DATE_STAMP" 2>/dev/null \
    || die "resolved date '$DATE_STAMP' is not a real calendar date — FAIL-CLOSED; schema format:date would reject it."
elif command -v date >/dev/null 2>&1; then
  # BSD (macOS) parse fallback (python3 absent). `date -j -f` rejects out-of-range
  # months/days (2026-99-99) but, unlike python3, SILENTLY ROLLS OVER a non-leap
  # Feb 29 -> Mar 01 instead of erroring. So we round-trip: the reformatted output
  # must equal the input, or it was not a real calendar date. (GNU date lacks -j.)
  _rt="$(date -j -f '%Y-%m-%d' "$DATE_STAMP" '+%Y-%m-%d' 2>/dev/null || true)"
  [ "$_rt" = "$DATE_STAMP" ] \
    || die "resolved date '$DATE_STAMP' is not a real calendar date — FAIL-CLOSED; schema format:date would reject it."
else
  die "cannot validate calendar date '$DATE_STAMP' — neither python3 nor date(1) on PATH. FAIL-CLOSED."
fi
# Join recurred_class_names into a single readable line, then SANITIZE it.
# The class names originate from MTO's manifest (semi-trusted): the gate's whole
# point is robustness, so we never trust this string into a markdown body. A name
# containing a newline + `---` could otherwise be misread as a SECOND frontmatter
# fence by a downstream YAML/frontmatter reader, and embedded newlines break the
# intended single-line Pattern. Defensive sanitation, layered:
#   (1) jq gsub: collapse CR/LF/tab to a single space INSIDE the join, so the
#       whole value is forced onto one physical line before it leaves jq.
#   (2) sed pass: neutralize a literal `---` run (-> an em-dash "—" so it can
#       never read as a frontmatter delimiter at column 0) and strip ASCII
#       control chars (incl. any stray CR/LF/tab jq's class missed). Unicode
#       printables are preserved — names stay human-readable.
#   (3) length cap (500 chars) with an explicit truncation marker, so a huge
#       recurred_class_names array cannot bloat the artifact.
CLASS_NAMES="$(jq -r '
  (.recurred_class_names // [])
  | map(gsub("[\\n\\r\\t]"; " "))
  | join(", ")
' "$HANDOFF_FILE")"
# Neutralize any literal `---` (3+ dashes collapse to an em-dash) and delete ASCII
# control chars (0x00-0x1F and 0x7F). LC_ALL=C keeps sed byte-oriented so multibyte
# UTF-8 sequences are left intact (their bytes are all >= 0x80, outside the class).
CLASS_NAMES="$(printf '%s' "$CLASS_NAMES" \
  | LC_ALL=C sed -e 's/---*/—/g' -e 's/[[:cntrl:]]//g')"
# Cap length so an oversized array can't bloat the artifact (UTF-8-safe truncation
# via cut -c, which counts characters under a UTF-8 locale).
if [ "$(printf '%s' "$CLASS_NAMES" | wc -m | tr -d ' ')" -gt 500 ]; then
  CLASS_NAMES="$(printf '%s' "$CLASS_NAMES" | cut -c1-500) …[truncated]"
fi
[ -n "$CLASS_NAMES" ] || CLASS_NAMES="(class names not present in signal)"

FINDING_PATH="$FINDINGS_DIR/$FINDING_ID.md"
# Finding-id-keyed planning-rule path: the runtime looks rules up BY finding id.
RULE_PATH="$PLANNING_DIR/$FINDING_ID.md"

# schema-conformant frontmatter (finding-artifact.schema.json). detectability=
# advisory -> no `compiler` block required; compiler_targets are the advisory two
# (plan, pre-mortem); applicable_when uses only enum members; additionalProperties
# is false so we emit ONLY schema-known keys.
read -r -d '' FINDING_BODY <<EOF || true
---
id: "$FINDING_ID"
type: "finding"
date: "$DATE_STAMP"
source_skill: "consume-mto-recurrence"
source_artifact: "scripts/assay/consume-mto-recurrence.sh"
title: "MTO seeded-guard finding-class recurred across beads"
summary: "The MTO gate ASSAY reported $RECURRED_CLASSES finding-class(es) recurring on more than one distinct bead, meaning a seeded guard is not sticking and the same defect class is reaching new work."
pattern: "A finding-class that already has a seeded guard recurs on a fresh bead in the MTO gate's seeded-corpus assay — the guard is dead, scoped-out, or never fired on the real failing case."
detection_question: "For each recurring finding-class, does its mechanical guard actually FIRE on the failing case today (proven red->green), or is it dead/scoped-out?"
checklist_item: "Before planning work that could trip a recurring MTO class, re-verify its seeded guard against a fixture and repair it FIRST if it does not catch the case."
severity: "significant"
detectability: "advisory"
status: "active"
compiler_targets: ["plan", "pre-mortem"]
scope_tags: ["mto-gate", "seeded-guard", "recurrence"]
dedup_key: "validation-gap|mto-seeded-guard-recurred-across-beads|validation-gap"
applicable_when: ["validation-gap", "plan-shape"]
applicable_languages: ["markdown", "shell", "go"]
---
# Finding: MTO seeded-guard finding-class recurred across beads

The MTO gate ASSAY recurrence handoff reported \`recurred_classes=$RECURRED_CLASSES\`
(max_class_recurrence=$MAX_RECUR, recurred_dimensions=$RECURRED_DIMS,
total_classes=$TOTAL_CLASSES). A class recurring across distinct beads means a
seeded guard is NOT sticking: the gate caught the same defect class again on new
work.

Recurring class(es): $CLASS_NAMES

Signal date: $SIGNAL_DATE
Source: $SIGNAL_SOURCE
EOF

read -r -d '' RULE_BODY <<EOF || true
---
id: "$FINDING_ID"
type: "planning-rule"
finding_id: "$FINDING_ID"
source_artifact: "$FINDING_PATH"
status: "active"
applicable_when: ["validation-gap", "plan-shape"]
applicable_languages: ["markdown", "shell", "go"]
---
# Planning Rule: re-verify a recurring MTO finding-class's seeded guard before planning similar work

Prevent this known failure mode:
- Pattern: finding-class(es) $CLASS_NAMES recurred on >1 distinct bead in the MTO gate (recurred_classes=$RECURRED_CLASSES) — a seeded guard is not sticking.
- Ask: For each recurring class, does a mechanical guard actually FIRE on the failing case today (proven red->green), or is it dead/scoped-out?
- Do: Before planning work that could trip a recurring class, re-verify its seeded guard against a fixture; if it does not catch the case, repair the guard FIRST — do not rely on it.
- Source: $FINDING_PATH
EOF

if [ "$DRY_RUN" -eq 1 ]; then
  printf 'DRY-RUN (no write)\n'
  printf 'WOULD write finding:       %s\n' "$FINDING_PATH"
  printf 'WOULD write planning-rule: %s\n' "$RULE_PATH"
  exit 0
fi

mkdir -p "$PLANNING_DIR" "$FINDINGS_DIR"
# Stable finding-id-keyed paths => idempotent: re-runs OVERWRITE in place.
printf '%s\n' "$FINDING_BODY" > "$FINDING_PATH"
printf '%s\n' "$RULE_BODY" > "$RULE_PATH"

printf 'tripwire consumed — materialized finding + planning-rule (recurred_classes=%s)\n' "$RECURRED_CLASSES"
printf 'finding:       %s\n' "$FINDING_PATH"
printf 'planning-rule: %s\n' "$RULE_PATH"
printf '(idempotent: stable finding id %s — re-runs update in place)\n' "$FINDING_ID"
