#!/usr/bin/env bash
# Acceptance test for scripts/codex-sync.sh (age-codex-twin-generator-qlj).
#
# Proves the bead's runnable acceptance criterion: create a throwaway SOURCE
# skill -> run the generator -> a complete, lint-clean, fully-registered Codex
# twin exists with ZERO hand-edits to skills-codex/. Before the generator this
# path failed the codex gates serially (override-coverage first, then the
# cascade as each was hand-fixed).
#
# Self-cleaning: a trap removes the probe skill + twin and surgically drops the
# probe's entries from both catalogs on exit (pass OR fail), so the dev tree is
# left exactly as found. Uses no `git checkout`/`git stash` (would disturb other
# pending work).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROBE="zzz-codex-sync-accept-probe"
ORPHAN="zzz-codex-sync-orphan-probe"
SRC_DIR="$ROOT/skills/$PROBE"
TWIN_DIR="$ROOT/skills-codex/$PROBE"
MANIFEST="$ROOT/skills-codex/.agentops-manifest.json"
OVERRIDES="$ROOT/skills-codex-overrides/catalog.json"

PASS=0
FAIL=0
pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  rm -f "$SRC_DIR/SKILL.md" "$SRC_DIR/references/deep-dive.md" \
        "$TWIN_DIR/SKILL.md" "$TWIN_DIR/prompt.md" \
        "$TWIN_DIR/.agentops-generated.json" "$TWIN_DIR/references/deep-dive.md" 2>/dev/null || true
  rmdir "$SRC_DIR/references" "$SRC_DIR" "$TWIN_DIR/references" "$TWIN_DIR" 2>/dev/null || true
  PROBE="$PROBE" ORPHAN="$ORPHAN" python3 - "$MANIFEST" "$OVERRIDES" <<'PY' 2>/dev/null || true
import hashlib, json, os, pathlib, sys
probe = os.environ["PROBE"]
orphan = os.environ["ORPHAN"]
for path in sys.argv[1:]:
    data = json.loads(open(path, encoding="utf-8").read())
    if "skills" in data:
        data["skills"] = [e for e in data["skills"] if e.get("name") not in {probe, orphan}]
    cat = data.get("codex_override_catalog")
    if isinstance(cat, dict) and "skills" in cat:
        cat["skills"] = [e for e in cat["skills"] if e.get("name") not in {probe, orphan}]
        # Recompute the embedded catalog hash so removing the probe leaves the
        # manifest byte-identical to its pre-test baseline (no stale-hash drift).
        blob = json.dumps(
            {k: v for k, v in cat.items() if k != "skills"} | {"skills": cat["skills"]},
            sort_keys=True,
        ).encode("utf-8")
        data["codex_override_catalog_hash"] = hashlib.sha256(blob).hexdigest()
    if "package_count" in data:
        root = pathlib.Path(path).parent
        data["package_count"] = sum(
            1 for child in root.iterdir()
            if child.is_dir() and (child / "SKILL.md").is_file()
        )
    open(path, "w", encoding="utf-8").write(json.dumps(data, indent=2) + "\n")
PY
}
trap cleanup EXIT

# Precondition: probe must not already exist.
if [[ -e "$SRC_DIR" || -e "$TWIN_DIR" ]]; then
  echo "FATAL: probe '$PROBE' already exists — aborting to avoid clobber." >&2
  exit 1
fi

echo "== 1. create throwaway SOURCE skill WITH a reference + transform cases (no twin authored) =="
mkdir -p "$SRC_DIR/references"
cat > "$SRC_DIR/SKILL.md" <<'EOF'
---
name: zzz-codex-sync-accept-probe
description: 'Throwaway probe for the codex-sync acceptance test. SENTINEL_SECOND_SENTENCE must not reach the twin. Triggers: "zzz codex sync accept probe".'
practices:
- some-practice
hexagonal_role: supporting
---
# ZZZ Codex Sync Accept Probe

Disposable skill that exercises the codex-twin generator. SENTINEL_BODY_TOKEN.
Use Claude Code to run this; first invoke /research, then /forge.
Config lives at ~/.claude/probe.json. See [deep dive](references/deep-dive.md).
EOF
echo "SENTINEL_REF_TOKEN: reference content the twin must carry." > "$SRC_DIR/references/deep-dive.md"

echo "== 2. generate the twin (the only action — zero hand-edits) =="
bash "$ROOT/scripts/codex-sync.sh" --only "$PROBE"

echo "== 3. assert the twin is complete + correct =="
[[ -f "$TWIN_DIR/SKILL.md" ]] && pass "twin SKILL.md generated" || fail "twin SKILL.md missing"
[[ -f "$TWIN_DIR/prompt.md" ]] && pass "twin prompt.md generated" || fail "twin prompt.md missing"
[[ -f "$TWIN_DIR/.agentops-generated.json" ]] && pass "twin marker generated" || fail "twin marker missing"

# Self-contained: the twin carries the source body content + its reference,
# because the Codex runtime ships skills-codex/ only (never skills/ source).
grep -q "SENTINEL_BODY_TOKEN" "$TWIN_DIR/SKILL.md" 2>/dev/null \
  && pass "twin body is self-contained (carries source body)" \
  || fail "twin body did not carry source content"
grep -q "SENTINEL_REF_TOKEN" "$TWIN_DIR/references/deep-dive.md" 2>/dev/null \
  && pass "twin carries its reference (runtime-native, self-contained)" \
  || fail "twin did not carry source reference"

# Runtime-native transforms: slash→$, ~/.claude→~/.codex, no "Claude Code".
grep -q '\$research' "$TWIN_DIR/SKILL.md" 2>/dev/null \
  && pass "slash-command transformed (/research → \$research)" \
  || fail "slash-command not transformed"
grep -q '/[.]codex/probe.json' "$TWIN_DIR/SKILL.md" 2>/dev/null \
  && ! grep -q '/[.]claude/probe.json' "$TWIN_DIR/SKILL.md" 2>/dev/null \
  && pass "Claude path transformed (.claude → .codex)" \
  || fail "Claude path not transformed"
grep -qi 'claude code' "$TWIN_DIR/SKILL.md" 2>/dev/null \
  && fail "'Claude Code' runtime reference remains" \
  || pass "'Claude Code' → Codex"

# Frontmatter must be name + description only (validate-codex-api-conformance rule).
fm_fields="$(awk 'NR==1&&/^---$/{f=1;next} f&&/^---$/{exit} f{print}' "$TWIN_DIR/SKILL.md" \
  | grep -oE '^[a-z_-]+:' | sed 's/:$//' | grep -vE '^(name|description)$' || true)"
[[ -z "$fm_fields" ]] && pass "twin frontmatter is name+description only" \
  || fail "twin frontmatter has stray fields: $fm_fields"

grep -q 'Triggers: "zzz codex sync accept probe"' "$TWIN_DIR/SKILL.md" 2>/dev/null \
  && pass "twin catalog preserves the source activation trigger" \
  || fail "twin catalog discarded or truncated the source activation trigger"

# The frozen catalog-projection contract: the twin description is the FIRST
# SENTENCE of the source prose plus the FULL Triggers clause, verbatim, and
# nothing else. The probe's source prose is deliberately two sentences, so this
# discriminates: the first must survive whole, the second must be gone, and the
# clause must be untouched. The rule this replaced cut prose at 44 chars on a
# word boundary and shipped 51 of 56 catalog entries as mid-clause fragments.
generated_description="$(awk '/^description:/{print; exit}' "$TWIN_DIR/SKILL.md")"
expected_description="description: 'Throwaway probe for the codex-sync acceptance test. Triggers: \"zzz codex sync accept probe\".'"

[[ "$generated_description" == *"Throwaway probe for the codex-sync acceptance test."* ]] \
  && pass "twin catalog keeps the source's first sentence WHOLE" \
  || fail "twin catalog cut inside the first sentence: $generated_description"

[[ "$generated_description" != *"SENTINEL_SECOND_SENTENCE"* ]] \
  && pass "twin catalog drops prose after the first sentence" \
  || fail "twin catalog kept prose past the first sentence: $generated_description"

[[ "$generated_description" == "$expected_description" ]] \
  && pass "twin catalog is exactly first-sentence + full Triggers clause" \
  || fail "twin catalog text is not the frozen projection
      expected: $expected_description
      actual:   $generated_description"

echo "== 3b. projection edge cases, end-to-end against the real generator =="
# LITERAL input -> LITERAL output, driven through codex-sync.sh itself. These
# pin the parts of the contract a boundary rule alone cannot state:
#   - an abbreviation's period ("e.g.", "i.e.", "vs.", "etc.", "cf.") is not a
#     sentence end;
#   - a terminator followed by a closing quote ends the sentence AFTER the
#     quote;
#   - the description value is a YAML scalar, so a value that legitimately ENDS
#     in a quote keeps it. The generator used to .strip(" '\"") the value and
#     ate that final character.
# Comparison is on the twin's parsed VALUE, not on YAML quoting style.
canonical_description="$(awk '/^description:/{sub(/^description:[[:space:]]*/,""); print; exit}' "$SRC_DIR/SKILL.md")"

set_source_description() {
  DESC_LINE="$1" python3 - "$SRC_DIR/SKILL.md" <<'PY'
import os, pathlib, sys
path = pathlib.Path(sys.argv[1])
out = []
for line in path.read_text(encoding="utf-8").splitlines(keepends=True):
    if line.startswith("description:"):
        line = "description: " + os.environ["DESC_LINE"] + "\n"
    out.append(line)
path.write_text("".join(out), encoding="utf-8")
PY
}

twin_description_value() {
  python3 - "$TWIN_DIR/SKILL.md" <<'PY'
import pathlib, sys, yaml
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
print(yaml.safe_load(text.split("---", 2)[1])["description"])
PY
}

project_case() { # project_case <label> <source description scalar> <expected VALUE>
  local label="$1" scalar="$2" want="$3" got
  set_source_description "$scalar"
  bash "$ROOT/scripts/codex-sync.sh" --only "$PROBE" >/dev/null 2>&1
  got="$(twin_description_value)"
  if [[ "$got" == "$want" ]]; then
    pass "projection: $label"
  else
    fail "projection: $label
      expected: $want
      actual:   $got"
  fi
}

project_case "abbreviation 'e.g.' does not end the sentence" \
  "'Use tools, e.g. shell. Then stop. Triggers: \"x\".'" \
  'Use tools, e.g. shell. Triggers: "x".'

project_case "abbreviations i.e./vs./etc./cf. do not end the sentence" \
  "'Weigh i.e. this vs. that, etc. and cf. the notes. Then stop. Triggers: \"x\".'" \
  'Weigh i.e. this vs. that, etc. and cf. the notes. Triggers: "x".'

project_case "closing quote after the terminator ends the sentence after the quote" \
  "'Say \"done.\" Then stop. Triggers: \"x\".'" \
  'Say "done." Triggers: "x".'

project_case "a description value ending in a quote keeps its final character" \
  "'Emit the sentinel \"ready.\" Triggers: \"x\"'" \
  'Emit the sentinel "ready." Triggers: "x"'

# Restore the canonical probe description so sections 4-6 judge the real shape.
set_source_description "$canonical_description"
bash "$ROOT/scripts/codex-sync.sh" --only "$PROBE" >/dev/null 2>&1
[[ "$(twin_description_value)" == 'Throwaway probe for the codex-sync acceptance test. Triggers: "zzz codex sync accept probe".' ]] \
  && pass "canonical probe description restored for the gate section" \
  || fail "failed to restore the canonical probe description"

# Registered in the gate-enforced 1:1 surface.
if jq -e --arg n "$PROBE" '.skills[]|select(.name==$n)' "$OVERRIDES" >/dev/null; then
  pass "registered in skills-codex-overrides/catalog.json"
else
  fail "not registered in overrides catalog.json"
fi

echo "== 4. assert the codex gates that used to fail serially now PASS =="
for v in validate-codex-override-coverage lint-codex-native validate-codex-api-conformance; do
  if bash "$ROOT/scripts/$v.sh" >/tmp/codex-sync-accept.$$.log 2>&1; then
    pass "$v"
  else
    fail "$v"; sed 's/^/      /' /tmp/codex-sync-accept.$$.log | tail -4
  fi
done
if bash "$ROOT/scripts/validate-codex-generated-artifacts.sh" --scope worktree \
     >/tmp/codex-sync-accept.$$.log 2>&1; then
  pass "validate-codex-generated-artifacts (content divergence)"
else
  fail "validate-codex-generated-artifacts"; sed 's/^/      /' /tmp/codex-sync-accept.$$.log | tail -4
fi
rm -f /tmp/codex-sync-accept.$$.log

echo "== 5. assert idempotency: --check reports no drift =="
if bash "$ROOT/scripts/codex-sync.sh" --check --only "$PROBE" >/dev/null 2>&1; then
  pass "codex-sync --check is clean after generation"
else
  fail "codex-sync --check still reports drift"
fi

echo "== 6. assert stale manifest-only skills are pruned =="
PROBE="$PROBE" ORPHAN="$ORPHAN" python3 - "$MANIFEST" <<'PY'
import json, os, sys
path = sys.argv[1]
data = json.loads(open(path, encoding="utf-8").read())
orphan = os.environ["ORPHAN"]
data.setdefault("skills", []).append({
    "name": orphan,
    "source_skill": f"skills/{orphan}",
    "source_hash": "stale",
    "generated_hash": "stale",
})
data.setdefault("codex_override_catalog", {}).setdefault("skills", []).append({
    "name": orphan,
    "treatment": "parity_only",
    "wave": "catalog-parity",
    "reason": "stale probe",
})
open(path, "w", encoding="utf-8").write(json.dumps(data, indent=2) + "\n")
PY
bash "$ROOT/scripts/codex-sync.sh" --only "$PROBE" >/dev/null
if ! jq -e --arg n "$ORPHAN" '([.skills[].name, .codex_override_catalog.skills[].name] | flatten | index($n)) == null' "$MANIFEST" >/dev/null; then
  fail "codex-sync retained a stale manifest-only skill"
else
  pass "codex-sync prunes stale manifest-only skills"
fi

echo
echo "Results: $PASS PASS, $FAIL FAIL"
[[ "$FAIL" -eq 0 ]] || exit 1
exit 0
