#!/usr/bin/env bash
# tests/e2e/membrane-governor-conformance.sh
#   SPC.TEST — bead age-wy3.4
#
# Conformance of the SPC governor (ao governor budget / noise-band) END-TO-END
# through the real built ao binary against a real yield ledger built via the
# production CLI writer (ao yield emit gate-verdict). Complements the in-package
# conformance test (cli/internal/governor/conformance_test.go) which proves the
# §6.1 determinism + §6.3 read-only clauses at the unit level.
#
# Asserts:
#   - DETERMINISM (control-loop-model.md §6.1): the same ledger yields a
#     byte-identical budget verdict on repeated invocations of the real binary.
#   - SHIP-VS-HARDEN exit semantics: a burned budget exits 3 (harden, stop-the-line);
#     a clean ledger exits 0 (ship).
#   - noise-band runs over the real ledger and emits a decision.
#
# SUT: the repo-built ao (forced fresh). Real binary, isolated mktemp sandbox,
# real ledger via the production emitter. jq REQUIRED (no skip-green).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=../lib/e2e-factory.sh
source "$REPO_ROOT/tests/lib/e2e-factory.sh"

SANDBOX="$(mktemp -d)"
cleanup() { local rc=$?; chmod -R u+w "$SANDBOX" 2>/dev/null || true; rm -rf "$SANDBOX" 2>/dev/null || true; exit "$rc"; }
trap cleanup EXIT

PASS=0
pass() { PASS=$((PASS + 1)); printf 'PASS: %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq is required for SPC.TEST (verdict oracle); do NOT skip-green"
export PROOF_FORCE_BUILD=1
AO_BIN="$(e2e_factory_ao_bin "$SANDBOX/bin" "$REPO_ROOT")"

# emit one gate-verdict into the sandbox ledger (cwd = sandbox so ao writes here).
emit() { # bead run disposition attempt
  local bead="$1" run="$2" disp="$3" attempt="$4" sha="${1}-headsha0"
  ( cd "$SANDBOX" && "$AO_BIN" yield emit gate-verdict --bead "$bead" --run "$run" --json \
    "{\"difficulty\":1,\"pawl_verdict_ref\":{\"bead_id\":\"$bead\",\"head_sha\":\"$sha\"},\"disposition\":\"$disp\",\"head_sha\":\"$sha\",\"attempt\":$attempt,\"author_context_id\":\"ctx-$bead\",\"author_family\":\"claude\",\"refuter_families\":[\"gpt\"]}" >/dev/null )
}

# --- Build a BURNED-budget ledger: 8 clean confirms + 2 escape pairs ---------
# (10 confirmed, 2 escapes -> rate 0.20 > tolerance 0.10 -> burn 2.0 -> harden).
for i in 1 2 3 4 5 6 7 8; do emit "clean-$i" r CONFIRMED 1; done
for b in esc-1 esc-2; do emit "$b" r CONFIRMED 1; emit "$b" r REFUTED 2; done

# --- DETERMINISM (§6.1): two invocations -> byte-identical verdict ------------
( cd "$SANDBOX" && "$AO_BIN" governor budget --json > "$SANDBOX/v1.json" 2>/dev/null ) || true
( cd "$SANDBOX" && "$AO_BIN" governor budget --json > "$SANDBOX/v2.json" 2>/dev/null ) || true
if ! diff -q "$SANDBOX/v1.json" "$SANDBOX/v2.json" >/dev/null; then
  fail "governor budget not deterministic: two runs differ"
fi
pass "determinism (§6.1): repeated budget verdicts are byte-identical"

DEC=$(jq -r '.decision' "$SANDBOX/v1.json")
[[ "$DEC" == "harden" ]] || fail "burned ledger decision = $DEC, want harden"
pass "ship-vs-harden: a burned budget decides harden"

# --- HARDEN exit semantics: exit 3 (stop-the-line) ---------------------------
set +e
( cd "$SANDBOX" && "$AO_BIN" governor budget >/dev/null 2>&1 ); rc=$?
set -e
[[ "$rc" == "3" ]] || fail "burned budget exit = $rc, want 3 (harden / stop-the-line)"
pass "ship-vs-harden: harden exits 3 (mechanical stop-the-line)"

# --- noise-band runs over the real ledger ------------------------------------
( cd "$SANDBOX" && "$AO_BIN" governor noise-band --json > "$SANDBOX/nb.json" 2>/dev/null )
jq -e '.decision == "adjust" or .decision == "hold"' "$SANDBOX/nb.json" >/dev/null \
  || fail "noise-band did not emit a valid decision"
pass "noise-band: emits a valid adjust|hold decision over the real ledger"

# --- A CLEAN ledger ships (exit 0) -------------------------------------------
CLEAN="$(mktemp -d)"
for i in 1 2 3 4 5 6; do
  ( cd "$CLEAN" && "$AO_BIN" yield emit gate-verdict --bead "ok-$i" --run r --json \
    "{\"difficulty\":1,\"pawl_verdict_ref\":{\"bead_id\":\"ok-$i\",\"head_sha\":\"ok${i}headsha\"},\"disposition\":\"CONFIRMED\",\"head_sha\":\"ok${i}headsha\",\"attempt\":1,\"author_context_id\":\"ctx\",\"author_family\":\"claude\",\"refuter_families\":[\"gpt\"]}" >/dev/null )
done
set +e
( cd "$CLEAN" && "$AO_BIN" governor budget >/dev/null 2>&1 ); rc=$?
set -e
chmod -R u+w "$CLEAN" 2>/dev/null || true; rm -rf "$CLEAN" 2>/dev/null || true
[[ "$rc" == "0" ]] || fail "clean ledger exit = $rc, want 0 (ship)"
pass "ship-vs-harden: a clean ledger ships (exit 0)"

echo "OK: SPC.TEST — $PASS assertions passed (determinism, ship-vs-harden, noise-band)"
