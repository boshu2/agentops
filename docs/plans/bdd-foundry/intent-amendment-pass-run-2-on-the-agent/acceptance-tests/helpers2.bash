#!/usr/bin/env bash
# helpers2.bash — run-2 (amendment pass) harness, layered over the run-1 contract.
# bdd-foundry Phase 2 (ATDD): every test in this directory is the executable
# definition of done for one scenario B74–B94 in ../behaviors.md (frozen
# 2026-06-12; B94 appended at drift-guard repair as the split of B85's
# rollout-evidence concern). The suite is RED by design until the
# amendment-pass work is built.
#
# ──────────────────── PINNED RUN-2 CONTRACT ────────────────────
# Everything the amendment pass must expose to be observable by this suite is
# pinned HERE (one place). The spec phase may renegotiate a pin by editing this
# file only — never by weakening a scenario (that re-opens Phase 1). The run-1
# contract in ../../acceptance-tests/helpers.bash still applies verbatim.
#
# Checked-in verifier scripts (repo-root-relative unless absolute):
#   docs/plans/bdd-foundry/acceptance-tests/audit-red.sh          (B75/B76)
#     — runs the base suite entry point with the SUT forced to the red
#       placeholder via the LAND_BIN seam; exits 0 iff the run is
#       red-ON-ASSERTION (zero ok, not-ok count == the coverage-manifest-derived
#       enumerated count, zero harness "No such file or directory" crashes,
#       every failure trace pointing at a test-body assertion line) and the
#       suite contains no if/else with byte-identical branches; nonzero naming
#       every offending test otherwise. References the B91 coverage manifest.
#   scripts/check-regen-manifest.sh [--repo <dir>] [--manifest <file>] (B78)
#     — strict-format + reality-parity checker for scripts/regen-manifest.txt.
#   scripts/land.sh --check-counts                                  (B79/B80)
#     — the run-1 B48 count checker, generalized to the real repo; reads
#       scripts/count-docs.txt; repo-wide out-of-marker sweep included.
#   scripts/check-gate-parity.sh [--repo <dir>] [--workflow <file>]  (B81)
#     — STRUCTURAL (YAML-parse) land-gate-families checker + B49 parity.
#   scripts/check-doctrine-docs.sh [--repo <dir>]                    (B86)
#     — operator-doc sweep over the pinned doc list; recognizes the
#       historical/superseded section marker convention (documented in its
#       header).
#   scripts/check-rollout-evidence.sh [--manifest <file>]       (B85/B94)
#     — validates the rollout evidence manifest (below) against the current
#       cutover commit; stale guard version / repo SHA ⇒ nonzero.
#   scripts/sweep-bead-acceptance.sh                                 (B90)
#     — sweeps ALL landing-redesign bead bodies via live `br show`, rejects
#       shorthand-only criteria, EXECUTES every extracted command, fails
#       closed on missing br / ledger / bead id. Env: BR_MAIN_CHECKOUT,
#       BR_BEADS_DIR (defaults below).
#   scripts/with-hermetic-check.sh <cmd...>                          (B92)
#     — wraps a verifier: captures `git status --porcelain` + HEAD SHA before
#       and after, exits nonzero with "verifier residue" naming every leftover
#       path / SHA mismatch; otherwise propagates the wrapped exit status.
# Checked-in data artifacts:
#   <run-2 plan dir>/coverage-manifest.txt                           (B91)
#     — one line per appended behavior: "B<n> <kind>:<ref>", kind ∈
#       {bats,script,cmd}; checker below.
#   <run-2 tests dir>/check-coverage-manifest.sh [--manifest <file>] (B91)
#   <run-2 plan dir>/rollout-evidence.jsonl                          (B94)
#     — one JSON record per live clone with keys: host, repo_sha,
#       guard_version, command, timestamp (ISO-8601), verify (raw JSON).
# Hook-chain pins (B82–B85, B93):
#   beads segment markers: "# BEGIN BEADS INTEGRATION" / "# END BEADS INTEGRATION"
#   guard segment markers: "# BEGIN LAND GUARD v<semver>" / "# END LAND GUARD"
#   foreign-hook policy:   refuse, exit nonzero, message contains
#                          "not recognized" and "chain manually"
#   install backup path:   .git/hooks/pre-push.pre-land-install.bak
#   --install --verify JSON keys: guard_present, guard_version, chain, defects
#   defect tokens (one per injected fault): stale guard version, duplicate
#     guard segment, unpaired marker, missing executable bit, chain order
# Install fault seams (B93; active only when LAND_TEST_MODE=1):
#   LAND_TEST_INSTALL_FAIL=<step>, step ∈ {write,rename,chmod} — the install
#   fails AT that step (write = kill-mid-write semantics: the temp file is
#   abandoned before the atomic rename). Error output names the step.
# Substrate seam (B88):
#   LAND_BIN — every SUT invocation in BOTH suites routes through the run-1
#   helpers' land()/start_land()/status_json(), which honor LAND_BIN
#   (default: the lane's scripts/land.sh). Installed guard dispatch consults
#   the same configured land command, never a hardcoded path.
# Lock-dir default (B89):
#   ${XDG_STATE_HOME:-$HOME/.local/state}/land/<digest-of-CANONICALIZED-origin>
#   — canonicalization pinned in --help; equivalent ssh/scp/https/.git
#   spellings of one remote digest identically.
# br access (B77, B87, B90) — read from the MAIN checkout, never a worktree:
#   BR_MAIN_CHECKOUT (default /Users/bo/dev/agentops)
#   BR_BEADS_DIR     (default $BR_MAIN_CHECKOUT/_beads)
#   Missing br / ledger / bead id is FAIL-CLOSED (hard failure, never a skip).
# ─────────────────────────────────────────────────────────────────

RUN2_TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN2_PLAN_DIR="$(cd "$RUN2_TESTS_DIR/.." && pwd)"
BASE_SUITE_DIR="$(cd "$RUN2_PLAN_DIR/../acceptance-tests" && pwd)"
RUN1_SPEC="$RUN2_PLAN_DIR/../spec.md"

# Run-1 harness (sandbox, lanes, SUT runner, lock fabrication, polling).
source "$BASE_SUITE_DIR/helpers.bash"

# Run-2 tests live one directory deeper than run 1 — repoint root discovery.
repo_under_test() { cd "$RUN2_TESTS_DIR/../../../../.." && pwd; }
REAL_REPO_ROOT="$(repo_under_test)"

# ── pinned artifact paths ────────────────────────────────────────────
AUDIT_RED="$BASE_SUITE_DIR/audit-red.sh"
COVERAGE_MANIFEST="$RUN2_PLAN_DIR/coverage-manifest.txt"
COVERAGE_CHECKER="$RUN2_TESTS_DIR/check-coverage-manifest.sh"
ROLLOUT_EVIDENCE="$RUN2_PLAN_DIR/rollout-evidence.jsonl"
V_REGEN_MANIFEST="scripts/check-regen-manifest.sh"
V_GATE_PARITY="scripts/check-gate-parity.sh"
V_DOCTRINE="scripts/check-doctrine-docs.sh"
V_ROLLOUT="scripts/check-rollout-evidence.sh"
V_BEAD_SWEEP="scripts/sweep-bead-acceptance.sh"
V_HERMETIC="scripts/with-hermetic-check.sh"
BEADS_BEGIN_RE='# BEGIN BEADS INTEGRATION'
BEADS_END_RE='# END BEADS INTEGRATION'
GUARD_BEGIN_RE='# BEGIN LAND GUARD'
GUARD_END_RE='# END LAND GUARD'

# ── br access (fail-closed; main checkout only) ──────────────────────
BR_MAIN_CHECKOUT="${BR_MAIN_CHECKOUT:-/Users/bo/dev/agentops}"
BR_BEADS_DIR="${BR_BEADS_DIR:-$BR_MAIN_CHECKOUT/_beads}"

br_guard() {
  command -v br >/dev/null 2>&1 || { echo "FAIL-CLOSED: br not on PATH" >&2; return 90; }
  [ -d "$BR_MAIN_CHECKOUT/.git" ] || { echo "FAIL-CLOSED: main checkout missing: $BR_MAIN_CHECKOUT" >&2; return 90; }
  [ -d "$BR_BEADS_DIR" ] || { echo "FAIL-CLOSED: beads ledger missing: $BR_BEADS_DIR" >&2; return 90; }
}

br_show() { # id → bead body on stdout; hard failure on any missing dependency
  br_guard || return $?
  ( cd "$BR_MAIN_CHECKOUT" && BEADS_DIR="$BR_BEADS_DIR" br show "$1" )
}

br_ready_all() {
  br_guard || return $?
  ( cd "$BR_MAIN_CHECKOUT" && BEADS_DIR="$BR_BEADS_DIR" br ready --limit 0 )
}

bead_section() { # <name> ← br-show output on stdin → that ## section's body
  awk -v sec="$1" '
    $0 ~ ("^## " sec) { on=1; next }
    on && (/^## / || /^Dependents:/ || /^Blockers:/) { on=0 }
    on { print }
  '
}

# ── real-repo (cutover) helpers — hermetic by construction (B92) ─────
real_repo_clone() { # → echoes a disposable clone of the operator checkout's HEAD
  local d
  d="$(mktemp -d "${BATS_TEST_TMPDIR:-${TMPDIR:-/tmp}}/land-real.XXXXXX")"
  git clone -q "$REAL_REPO_ROOT" "$d/repo"
  echo "$d/repo"
}

manifest_covers() { # <manifest> <path> → 0 iff path is declared (exact or dir/ prefix)
  local manifest="$1" p="$2" l
  while IFS= read -r l; do
    case "$l" in \#*|"") continue ;; esac
    l="${l%"${l##*[![:space:]]}"}"
    [ "$p" = "$l" ] && return 0
    case "$l" in */) case "$p" in "$l"*) return 0 ;; esac ;; esac
  done < "$manifest"
  return 1
}

# ── checksums ────────────────────────────────────────────────────────
sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1"; else shasum -a 256 "$1"; fi | awk '{print $1}'
}
sha256_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum; else shasum -a 256; fi | awk '{print $1}'
}
extract_segment() { # <file> <begin-re> <end-re> → segment incl. marker lines
  sed -n "/$2/,/$3/p" "$1"
}

# ── hook fabrication (reproduces the real repo's pre-push shape) ─────
make_chained_hook() { # <lane> — beads marker block + pre-push.local cockpit gate,
  # each instrumented to append its name to $PROBE_LOG when executed (B82).
  local lane="$1" hooks="$1/.git/hooks"
  PROBE_LOG="$SANDBOX/probe.log"
  : > "$PROBE_LOG"
  mkdir -p "$hooks"
  cat > "$hooks/pre-push.local" <<EOF
#!/usr/bin/env bash
echo "cockpit-gate" >> "$PROBE_LOG"
exit 0
EOF
  cat > "$hooks/pre-push" <<EOF
#!/usr/bin/env bash
# BEGIN BEADS INTEGRATION v1.0.5
echo "beads-segment" >> "$PROBE_LOG"
# END BEADS INTEGRATION v1.0.5
hookdir="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
if [ -x "\$hookdir/pre-push.local" ]; then "\$hookdir/pre-push.local" "\$@" || exit \$?; fi
EOF
  chmod +x "$hooks/pre-push" "$hooks/pre-push.local"
}

write_foreign_hook() { # <lane> <variant ∈ exit0|exectrap|noshebang|noexec> (B84)
  local lane="$1" variant="$2" hook="$1/.git/hooks/pre-push"
  mkdir -p "$1/.git/hooks"
  case "$variant" in
    exit0)
      printf '#!/bin/sh\nexit 0\necho unreachable\n' > "$hook"; chmod +x "$hook" ;;
    exectrap)
      printf '#!/bin/sh\necho foreign-hook\nexec /usr/bin/true\n' > "$hook"; chmod +x "$hook" ;;
    noshebang)
      printf 'echo foreign-no-shebang\nexit 0\n' > "$hook"; chmod +x "$hook" ;;
    noexec)
      printf '#!/bin/sh\necho foreign-not-executable\nexit 0\n' > "$hook"; chmod -x "$hook" ;;
    *) echo "unknown foreign-hook variant: $variant" >&2; return 1 ;;
  esac
}

inject_guard_defect() { # <lane> <defect ∈ stale|duplicate|unpaired|noexec|order> (B85)
  local lane="$1" defect="$2" hook="$1/.git/hooks/pre-push"
  case "$defect" in
    stale)
      sed -i.b85 -E "s/(# BEGIN LAND GUARD v)[0-9][^[:space:]]*/\10.0.1/" "$hook"
      rm -f "$hook.b85" ;;
    duplicate)
      extract_segment "$hook" "$GUARD_BEGIN_RE" "$GUARD_END_RE" >> "$hook" ;;
    unpaired)
      sed -i.b85 "/$GUARD_END_RE/d" "$hook"
      rm -f "$hook.b85" ;;
    noexec)
      chmod -x "$hook" ;;
    order)
      python3 - "$hook" <<'PYEOF'
import re, sys
p = sys.argv[1]
src = open(p).read().splitlines(keepends=True)
beg = next(i for i, l in enumerate(src) if '# BEGIN LAND GUARD' in l)
end = next(i for i, l in enumerate(src) if '# END LAND GUARD' in l)
seg = src[beg:end + 1]
rest = src[:beg] + src[end + 1:]
insert = 1 if rest and rest[0].startswith('#!') else 0
open(p, 'w').write(''.join(rest[:insert] + seg + rest[insert:]))
PYEOF
      ;;
    *) echo "unknown guard defect: $defect" >&2; return 1 ;;
  esac
}

# ── structural lints over the suite itself ───────────────────────────
find_identical_if_else() { # <file...> → prints "file:line" per if/else whose
  # branches are byte-identical after comment/whitespace stripping (B76).
  python3 - "$@" <<'PYEOF'
import re, sys
def norm(body):
    out = []
    for l in body:
        s = l.strip()
        if not s or s.startswith('#'):
            continue
        out.append(s)
    return out
for path in sys.argv[1:]:
    lines = open(path).read().splitlines()
    stack = []
    for i, raw in enumerate(lines, 1):
        s = raw.strip()
        if re.match(r'^if\b', s):
            stack.append({'line': i, 'then': [], 'else': [], 'in': 'then', 'elif': False})
            continue
        if not stack:
            continue
        top = stack[-1]
        if re.match(r'^then\b\s*$', s):
            continue
        if re.match(r'^elif\b', s):
            top['elif'] = True
            top['in'] = 'then'
            continue
        if re.match(r'^else\b\s*$', s):
            top['in'] = 'else'
            continue
        if re.match(r'^fi\b', s):
            f = stack.pop()
            if not f['elif'] and f['else'] and norm(f['then']) and norm(f['then']) == norm(f['else']):
                print(f"{path}:{f['line']}")
            if stack:
                stack[-1][stack[-1]['in']].append(f"<if@{f['line']}>")
            continue
        top[top['in']].append(s)
PYEOF
}

find_direct_sut_invocations() { # <bats-file...> → prints offenders: lines that
  # INVOKE scripts/land.sh directly (heredoc bodies and comments excluded) (B88).
  python3 - "$@" <<'PYEOF'
import re, sys
pat = re.compile(r'(?:^|&&|[;|(`])\s*(?:\.\./)*(?:\./)?scripts/land\.sh\b')
hd = re.compile(r"<<-?\s*['\"]?(\w+)['\"]?")
for path in sys.argv[1:]:
    in_h, tag = False, None
    for i, line in enumerate(open(path).read().splitlines(), 1):
        if in_h:
            if line.strip() == tag:
                in_h = False
            continue
        m = hd.search(line)
        if m:
            in_h, tag = True, m.group(1)
        if line.strip().startswith('#'):
            continue
        if pat.search(line):
            print(f"{path}:{i}:{line.strip()}")
PYEOF
}

hold_overlap_check() { # <audit.jsonl> → 0 iff acquire/release never overlap (B89)
  python3 - "$1" <<'PYEOF'
import sys
held = 0
for line in open(sys.argv[1]):
    if '"acquire"' in line or '"stale-takeover"' in line:
        held += 1
        if held > 1:
            print("OVERLAP: second acquire while held")
            sys.exit(1)
    elif '"release"' in line:
        held = max(0, held - 1)
sys.exit(0)
PYEOF
}
