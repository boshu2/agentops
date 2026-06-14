#!/usr/bin/env bash
# Hermetic tests for consume-mto-recurrence.sh — all four states, idempotency,
# and schema-conformance of the generated finding. Fixtures are JSON files in a
# temp dir; the script reads them via --handoff-file. No live store, no
# wall-clock, no network.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUT="$SCRIPT_DIR/consume-mto-recurrence.sh"
SCHEMA="$(cd "$SCRIPT_DIR/../.." && pwd)/docs/contracts/finding-artifact.schema.json"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0
ok()   { printf '  ok   %s\n' "$1"; PASS=$((PASS+1)); }
bad()  { printf '  FAIL %s\n' "$1"; FAIL=$((FAIL+1)); }

# run_sut <handoff-json-path> <extra args...> ; captures rc + stdout into globals
# Passes an explicit --date so this is the deterministic stamp source (no wall-clock).
run_sut() {
  local hf="$1"; shift
  SUT_OUT=""
  SUT_RC=0
  # Clear the env date so --date is the only stamp source (no wall-clock).
  SUT_OUT="$(env -u EFFICACY_DATE_STAMP "$SUT" --handoff-file "$hf" \
               --planning-dir "$TMP/planning" --findings-dir "$TMP/findings" \
               --date 2026-06-13 "$@" 2>/dev/null)" || SUT_RC=$?
}

# run_sut_nodate <handoff-json-path> <extra args...> ; like run_sut but with NO
# --date flag and a cleared $EFFICACY_DATE_STAMP, so the finding's date can only
# come from the handoff file's own .date (the no-flag tripwire path).
run_sut_nodate() {
  local hf="$1"; shift
  SUT_OUT=""
  SUT_RC=0
  SUT_OUT="$(env -u EFFICACY_DATE_STAMP "$SUT" --handoff-file "$hf" \
               --planning-dir "$TMP/planning" --findings-dir "$TMP/findings" \
               "$@" 2>/dev/null)" || SUT_RC=$?
}

# ---------------------------------------------------------------------------
# STATE 1: absent -> exit 0, "run the MTO bridge first"
# ---------------------------------------------------------------------------
run_sut "$TMP/does-not-exist.json"
if [ "$SUT_RC" -eq 0 ] && printf '%s' "$SUT_OUT" | grep -q "run the MTO bridge first"; then
  ok "absent -> exit 0 + bridge-first message"
else
  bad "absent (rc=$SUT_RC out=$SUT_OUT)"
fi

# ---------------------------------------------------------------------------
# STATE 2: unparseable -> exit non-zero (FAIL-CLOSED), three flavors
# ---------------------------------------------------------------------------
printf '%s' '{not valid json' > "$TMP/bad-json.json"
run_sut "$TMP/bad-json.json"
[ "$SUT_RC" -ne 0 ] && ok "invalid JSON -> fail-closed (rc=$SUT_RC)" || bad "invalid JSON should fail-closed"

printf '%s' '{"recurred_classes":"0abc"}' > "$TMP/token.json"
run_sut "$TMP/token.json"
[ "$SUT_RC" -ne 0 ] && ok "0abc token -> fail-closed (rc=$SUT_RC)" || bad "0abc should fail-closed"

printf '%s' '{"recurred_classes":"0"}' > "$TMP/string-zero.json"
run_sut "$TMP/string-zero.json"
[ "$SUT_RC" -ne 0 ] && ok "string \"0\" -> fail-closed (rc=$SUT_RC)" || bad "string 0 should fail-closed"

printf '%s' '{"recurred_classes":-1}' > "$TMP/negative.json"
run_sut "$TMP/negative.json"
[ "$SUT_RC" -ne 0 ] && ok "-1 -> fail-closed (rc=$SUT_RC)" || bad "-1 should fail-closed"

printf '%s' '{"max_class_recurrence":2}' > "$TMP/missing.json"
run_sut "$TMP/missing.json"
[ "$SUT_RC" -ne 0 ] && ok "missing recurred_classes -> fail-closed (rc=$SUT_RC)" || bad "missing field should fail-closed"

# ---------------------------------------------------------------------------
# STATE 3: clean (recurred_classes==0) -> exit 0, NO file written
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
printf '%s' '{"recurred_classes":0,"recurred_dimensions":0,"total_classes":5,"recurred_class_names":[],"verdict":"clean"}' > "$TMP/clean.json"
run_sut "$TMP/clean.json"
if [ "$SUT_RC" -eq 0 ] && [ ! -d "$TMP/findings" ] && [ ! -d "$TMP/planning" ]; then
  ok "clean -> exit 0, NO file written"
else
  bad "clean wrote files or non-zero (rc=$SUT_RC, findings=$([ -d "$TMP/findings" ] && echo dir))"
fi

# ---------------------------------------------------------------------------
# STATE 4: tripwire -> emits finding + planning-rule keyed by finding id
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
TRIP='{"recurred_classes":2,"max_class_recurrence":2,"recurred_dimensions":2,"total_classes":7,"recurred_class_names":["fail-open [D1]x2","label-not-act [D5]x2"],"date":"2026-06-13","source":"mto-gate-assay","verdict":"TRIPWIRE"}'
printf '%s' "$TRIP" > "$TMP/trip.json"
run_sut "$TMP/trip.json"
FINDING_ID="f-mto-recurrence-handoff"
FPATH="$TMP/findings/$FINDING_ID.md"
RPATH="$TMP/planning/$FINDING_ID.md"
if [ "$SUT_RC" -eq 0 ] && [ -f "$FPATH" ] && [ -f "$RPATH" ]; then
  ok "tripwire -> finding + planning-rule both written (finding-id-keyed paths)"
else
  bad "tripwire missing artifacts (rc=$SUT_RC finding=$([ -f "$FPATH" ]&&echo y) rule=$([ -f "$RPATH" ]&&echo y))"
fi

# planning-rule path MUST equal the finding id (#2 fix)
[ -f "$TMP/planning/$FINDING_ID.md" ] && ok "planning-rule path == finding id" || bad "planning-rule not keyed by finding id"

# idempotency: run TWICE -> exactly 1 finding + 1 rule, no dup
run_sut "$TMP/trip.json"
NF=$(find "$TMP/findings" -name '*.md' | wc -l | tr -d ' ')
NR=$(find "$TMP/planning" -name '*.md' | wc -l | tr -d ' ')
if [ "$NF" -eq 1 ] && [ "$NR" -eq 1 ]; then
  ok "idempotent: re-run update-in-place (1 finding, 1 rule)"
else
  bad "idempotency broke (findings=$NF rules=$NR)"
fi

# class names propagate into the artifacts
grep -q "fail-open \[D1\]x2" "$FPATH" && ok "recurred_class_names propagate into finding" || bad "class names missing from finding"

# ---------------------------------------------------------------------------
# STATE 4 (injection): malicious recurred_class_names — newline + `---` + an
# `injected: true` payload + unicode + quotes + a very long element. The names
# are NON-load-bearing, so we take the EMIT-SANITIZED path: the consumer still
# materializes the finding, but the body must be inert — exactly one frontmatter
# block, no injected key, schema-valid, no raw `---` at column 0.
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
# 600-char element forces the length cap; the `\n---\ninjected: true` is the
# frontmatter-injection attempt; π/quotes/tab exercise unicode + control chars.
LONG="$(python3 -c 'print("A"*600, end="")')"
EVIL_JSON="$(python3 - "$LONG" <<'PY'
import json, sys
long_el = sys.argv[1]
payload = {
    "recurred_classes": 2,
    "max_class_recurrence": 2,
    "recurred_dimensions": 2,
    "total_classes": 7,
    "recurred_class_names": [
        "evil\n---\ninjected: true",
        "quote\"and:colon πλunicode\ttab",
        long_el,
    ],
    "date": "2026-06-13",
    "source": "mto-gate-assay",
    "verdict": "TRIPWIRE",
}
print(json.dumps(payload), end="")
PY
)"
printf '%s' "$EVIL_JSON" > "$TMP/trip-evil.json"
run_sut "$TMP/trip-evil.json"

# (a) consumer still emits (emit-sanitized path; names non-load-bearing)
if [ "$SUT_RC" -eq 0 ] && [ -f "$FPATH" ]; then
  ok "injection: consumer still emits sanitized finding (rc=$SUT_RC)"
else
  bad "injection: consumer should emit (rc=$SUT_RC finding=$([ -f "$FPATH" ]&&echo y))"
fi

# (b) EXACTLY ONE frontmatter block -> exactly two `^---$` fence lines
NDASH=$(grep -c '^---$' "$FPATH" || true)
[ "$NDASH" -eq 2 ] && ok "injection: exactly one frontmatter block (^---$ count == 2)" \
  || bad "injection: stray frontmatter fence (^---$ count=$NDASH, expected 2)"

# (c) the injected `injected: true` key must NOT appear as a frontmatter key.
# Confirm via the actual YAML parse: load frontmatter, assert no 'injected' key.
if python3 - "$FPATH" <<'PY'
import sys, yaml
text = open(sys.argv[1]).read()
assert text.startswith("---\n"), "no frontmatter fence"
_, fm, _ = text.split("---\n", 2)
data = yaml.safe_load(fm)
assert "injected" not in data, f"injected key leaked into frontmatter: {data!r}"
print("NO-INJECTED-KEY")
PY
then
  ok "injection: no 'injected:' key parsed from frontmatter"
else
  bad "injection: 'injected:' key leaked into frontmatter"
fi

# (d) the sanitized finding still passes the real format-checked jsonschema validator
if python3 - "$FPATH" "$SCHEMA" <<'PY'
import sys, yaml, json
fpath, schema_path = sys.argv[1], sys.argv[2]
text = open(fpath).read()
assert text.startswith("---\n"), "no frontmatter fence"
_, fm, _ = text.split("---\n", 2)
data = yaml.safe_load(fm)
schema = json.load(open(schema_path))
try:
    from jsonschema import Draft202012Validator, FormatChecker
    Draft202012Validator(schema, format_checker=FormatChecker()).validate(data)
    print("SCHEMA-VALID (format-checked)")
except ImportError:
    missing = [k for k in schema["required"] if k not in data]
    assert not missing, f"missing required keys: {missing}"
    print("REQUIRED-KEYS-PRESENT (jsonschema not installed)")
PY
then
  ok "injection: sanitized finding still passes format-checked jsonschema"
else
  bad "injection: sanitized finding FAILED schema validation"
fi

# (e) the BODY (everything after the second fence) has no raw `---` at column 0:
# the injected delimiter must have been neutralized, not just shifted out of the
# frontmatter. Emit only the post-frontmatter lines, then grep for a `^---` line.
if ! awk 'fences>=2{print}; /^---$/{fences++}' "$FPATH" | grep -q '^---'; then
  ok "injection: body has no raw '---' at column 0 (delimiter neutralized)"
else
  bad "injection: a raw '---' survived into the finding body"
fi

# the injected newline must not have produced a multiline Pattern either:
# grep the body for the neutralized em-dash marker proves the `---` was rewritten.
grep -q '—' "$FPATH" && ok "injection: literal '---' rewritten to em-dash in body" \
  || bad "injection: em-dash neutralization marker absent (sanitation may not have run)"

# ---------------------------------------------------------------------------
# STATE 4 (dry-run): hermetic — touches nothing
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
run_sut "$TMP/trip.json" --dry-run
if [ "$SUT_RC" -eq 0 ] && [ ! -d "$TMP/findings" ] && [ ! -d "$TMP/planning" ]; then
  ok "dry-run hermetic: nothing written"
else
  bad "dry-run wrote files (rc=$SUT_RC)"
fi

# ---------------------------------------------------------------------------
# SCHEMA CONFORMANCE: generated finding validates against the schema
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
run_sut "$TMP/trip.json"
if python3 - "$FPATH" "$SCHEMA" <<'PY'
import sys, yaml, json
fpath, schema_path = sys.argv[1], sys.argv[2]
text = open(fpath).read()
# split YAML frontmatter
assert text.startswith("---\n"), "no frontmatter fence"
_, fm, _ = text.split("---\n", 2)
data = yaml.safe_load(fm)
schema = json.load(open(schema_path))
try:
    import jsonschema
    jsonschema.validate(instance=data, schema=schema)
    print("SCHEMA-VALID")
except ImportError:
    # fallback: assert all required keys present
    missing = [k for k in schema["required"] if k not in data]
    assert not missing, f"missing required keys: {missing}"
    print("REQUIRED-KEYS-PRESENT (jsonschema not installed)")
PY
then
  ok "generated finding conforms to finding-artifact.schema.json"
else
  bad "generated finding FAILED schema validation"
fi

# ---------------------------------------------------------------------------
# NO --date FLAG: tripwire handoff carrying a valid .date -> finding date is the
# handoff's date AND the finding still passes real jsonschema validation.
# (Regression guard: the no-flag path used to stamp "UNDATED", failing the schema.)
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
HFDATE="2026-06-14"
TRIP_DATED="{\"recurred_classes\":2,\"max_class_recurrence\":2,\"recurred_dimensions\":2,\"total_classes\":7,\"recurred_class_names\":[\"fail-open [D1]x2\"],\"date\":\"$HFDATE\",\"source\":\"mto-gate-assay\",\"verdict\":\"TRIPWIRE\"}"
printf '%s' "$TRIP_DATED" > "$TMP/trip-dated.json"
run_sut_nodate "$TMP/trip-dated.json"
if [ "$SUT_RC" -eq 0 ] && [ -f "$FPATH" ] && grep -q "date: \"$HFDATE\"" "$FPATH"; then
  ok "no --date: finding date sourced from handoff's .date ($HFDATE)"
else
  bad "no --date: handoff date not used (rc=$SUT_RC, date line=$(grep '^date:' "$FPATH" 2>/dev/null))"
fi

if python3 - "$FPATH" "$SCHEMA" <<'PY'
import sys, yaml, json
fpath, schema_path = sys.argv[1], sys.argv[2]
text = open(fpath).read()
assert text.startswith("---\n"), "no frontmatter fence"
_, fm, _ = text.split("---\n", 2)
data = yaml.safe_load(fm)
schema = json.load(open(schema_path))
try:
    import jsonschema
    from jsonschema import Draft202012Validator, FormatChecker
    # FormatChecker makes `format: "date"` actually enforced — this is what
    # rejects "UNDATED"/"?" and proves the handoff-date path is conformant.
    Draft202012Validator(schema, format_checker=FormatChecker()).validate(data)
    print("SCHEMA-VALID (format-checked)")
except ImportError:
    missing = [k for k in schema["required"] if k not in data]
    assert not missing, f"missing required keys: {missing}"
    # Without jsonschema we at least assert the date is YYYY-MM-DD.
    import re
    assert re.match(r"^\d{4}-\d{2}-\d{2}$", str(data["date"])), f"bad date: {data['date']}"
    print("REQUIRED-KEYS-PRESENT + date well-formed (jsonschema not installed)")
PY
then
  ok "handoff-date finding passes real (format-checked) jsonschema validation"
else
  bad "handoff-date finding FAILED format-checked schema validation"
fi

# ---------------------------------------------------------------------------
# NO --date FLAG + handoff .date absent/invalid -> FAIL-CLOSED, no finding written.
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
# .date absent entirely
TRIP_NODATE='{"recurred_classes":2,"max_class_recurrence":2,"recurred_dimensions":2,"total_classes":7,"recurred_class_names":["fail-open [D1]x2"],"source":"mto-gate-assay","verdict":"TRIPWIRE"}'
printf '%s' "$TRIP_NODATE" > "$TMP/trip-nodate.json"
run_sut_nodate "$TMP/trip-nodate.json"
if [ "$SUT_RC" -ne 0 ] && [ ! -e "$FPATH" ] && [ ! -e "$RPATH" ]; then
  ok "no --date + .date absent -> fail-closed, no finding (rc=$SUT_RC)"
else
  bad "no --date + .date absent should fail-closed with no finding (rc=$SUT_RC finding=$([ -e "$FPATH" ]&&echo y))"
fi

rm -rf "$TMP/planning" "$TMP/findings"
# .date present but invalid (not YYYY-MM-DD)
TRIP_BADDATE='{"recurred_classes":2,"max_class_recurrence":2,"recurred_dimensions":2,"total_classes":7,"recurred_class_names":["fail-open [D1]x2"],"date":"UNDATED","source":"mto-gate-assay","verdict":"TRIPWIRE"}'
printf '%s' "$TRIP_BADDATE" > "$TMP/trip-baddate.json"
run_sut_nodate "$TMP/trip-baddate.json"
if [ "$SUT_RC" -ne 0 ] && [ ! -e "$FPATH" ] && [ ! -e "$RPATH" ]; then
  ok "no --date + .date invalid (\"UNDATED\") -> fail-closed, no finding (rc=$SUT_RC)"
else
  bad "no --date + invalid .date should fail-closed with no finding (rc=$SUT_RC finding=$([ -e "$FPATH" ]&&echo y))"
fi

# ---------------------------------------------------------------------------
# NO --date FLAG + handoff .date is regex-VALID but CALENDAR-INVALID -> FAIL-CLOSED.
# These are the dates the shape regex accepts but the schema's real
# format:date (Draft202012Validator + FormatChecker) rejects — the fail-open the
# cross-family review caught. The consumer must NOT emit a finding for them.
# ---------------------------------------------------------------------------
rm -rf "$TMP/planning" "$TMP/findings"
# month 99 / day 99: matches [0-9]{4}-[0-9]{2}-[0-9]{2} but is not a real date.
TRIP_BOGUSCAL='{"recurred_classes":2,"max_class_recurrence":2,"recurred_dimensions":2,"total_classes":7,"recurred_class_names":["fail-open [D1]x2"],"date":"2026-99-99","source":"mto-gate-assay","verdict":"TRIPWIRE"}'
printf '%s' "$TRIP_BOGUSCAL" > "$TMP/trip-boguscal.json"
run_sut_nodate "$TMP/trip-boguscal.json"
if [ "$SUT_RC" -ne 0 ] && [ ! -e "$FPATH" ] && [ ! -e "$RPATH" ]; then
  ok "no --date + .date \"2026-99-99\" (regex-valid, calendar-invalid) -> fail-closed, no finding (rc=$SUT_RC)"
else
  bad "regex-valid bogus date 2026-99-99 should fail-closed with no finding (rc=$SUT_RC finding=$([ -e "$FPATH" ]&&echo y))"
fi

rm -rf "$TMP/planning" "$TMP/findings"
# Feb 29 in 2026 (NOT a leap year): regex-valid, calendar-invalid. date.fromisoformat
# and BSD `date -j -f` both reject it, matching the schema FormatChecker.
TRIP_NONLEAP='{"recurred_classes":2,"max_class_recurrence":2,"recurred_dimensions":2,"total_classes":7,"recurred_class_names":["fail-open [D1]x2"],"date":"2026-02-29","source":"mto-gate-assay","verdict":"TRIPWIRE"}'
printf '%s' "$TRIP_NONLEAP" > "$TMP/trip-nonleap.json"
run_sut_nodate "$TMP/trip-nonleap.json"
if [ "$SUT_RC" -ne 0 ] && [ ! -e "$FPATH" ] && [ ! -e "$RPATH" ]; then
  ok "no --date + .date \"2026-02-29\" (non-leap-year Feb 29) -> fail-closed, no finding (rc=$SUT_RC)"
else
  bad "non-leap Feb 29 should fail-closed with no finding (rc=$SUT_RC finding=$([ -e "$FPATH" ]&&echo y))"
fi

# ---------------------------------------------------------------------------
printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
