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
SRC_DIR="$ROOT/skills/$PROBE"
TWIN_DIR="$ROOT/skills-codex/$PROBE"
MANIFEST="$ROOT/skills-codex/.agentops-manifest.json"
OVERRIDES="$ROOT/skills-codex-overrides/catalog.json"

PASS=0
FAIL=0
pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  rm -f "$SRC_DIR/SKILL.md" "$TWIN_DIR/SKILL.md" "$TWIN_DIR/prompt.md" \
        "$TWIN_DIR/.agentops-generated.json" 2>/dev/null || true
  rmdir "$SRC_DIR" "$TWIN_DIR" 2>/dev/null || true
  PROBE="$PROBE" python3 - "$MANIFEST" "$OVERRIDES" <<'PY' 2>/dev/null || true
import hashlib, json, os, sys
probe = os.environ["PROBE"]
for path in sys.argv[1:]:
    data = json.loads(open(path, encoding="utf-8").read())
    if "skills" in data:
        data["skills"] = [e for e in data["skills"] if e.get("name") != probe]
    cat = data.get("codex_override_catalog")
    if isinstance(cat, dict) and "skills" in cat:
        cat["skills"] = [e for e in cat["skills"] if e.get("name") != probe]
        # Recompute the embedded catalog hash so removing the probe leaves the
        # manifest byte-identical to its pre-test baseline (no stale-hash drift).
        blob = json.dumps(
            {k: v for k, v in cat.items() if k != "skills"} | {"skills": cat["skills"]},
            sort_keys=True,
        ).encode("utf-8")
        data["codex_override_catalog_hash"] = hashlib.sha256(blob).hexdigest()
    open(path, "w", encoding="utf-8").write(json.dumps(data, indent=2) + "\n")
PY
}
trap cleanup EXIT

# Precondition: probe must not already exist.
if [[ -e "$SRC_DIR" || -e "$TWIN_DIR" ]]; then
  echo "FATAL: probe '$PROBE' already exists — aborting to avoid clobber." >&2
  exit 1
fi

echo "== 1. create throwaway SOURCE skill (no twin authored) =="
mkdir -p "$SRC_DIR"
cat > "$SRC_DIR/SKILL.md" <<'EOF'
---
name: zzz-codex-sync-accept-probe
description: 'Throwaway probe for the codex-sync acceptance test. Triggers: "zzz codex sync accept probe".'
---
# ZZZ Codex Sync Accept Probe

Disposable skill that exists only to exercise the codex-twin generator.
EOF

echo "== 2. generate the twin (the only action — zero hand-edits) =="
bash "$ROOT/scripts/codex-sync.sh" --only "$PROBE"

echo "== 3. assert the twin is complete + correct =="
[[ -f "$TWIN_DIR/SKILL.md" ]] && pass "twin SKILL.md generated" || fail "twin SKILL.md missing"
[[ -f "$TWIN_DIR/prompt.md" ]] && pass "twin prompt.md generated" || fail "twin prompt.md missing"
[[ -f "$TWIN_DIR/.agentops-generated.json" ]] && pass "twin marker generated" || fail "twin marker missing"

if grep -q "source of truth" "$TWIN_DIR/SKILL.md" 2>/dev/null; then
  pass "twin body is a pointer to source"
else
  fail "twin body is not the expected pointer"
fi

# Frontmatter must be name + description only (validate-codex-api-conformance rule).
fm_fields="$(awk 'NR==1&&/^---$/{f=1;next} f&&/^---$/{exit} f{print}' "$TWIN_DIR/SKILL.md" \
  | grep -oE '^[a-z_-]+:' | sed 's/:$//' | grep -vE '^(name|description)$' || true)"
[[ -z "$fm_fields" ]] && pass "twin frontmatter is name+description only" \
  || fail "twin frontmatter has stray fields: $fm_fields"

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
rm -f /tmp/codex-sync-accept.$$.log

echo "== 5. assert idempotency: --check reports no drift =="
if bash "$ROOT/scripts/codex-sync.sh" --check --only "$PROBE" >/dev/null 2>&1; then
  pass "codex-sync --check is clean after generation"
else
  fail "codex-sync --check still reports drift"
fi

echo
echo "Results: $PASS PASS, $FAIL FAIL"
[[ "$FAIL" -eq 0 ]] || exit 1
exit 0
