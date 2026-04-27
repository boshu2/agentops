#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCE_SCRIPT="$ROOT/tests/cli/test-json-flag-consistency.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

[[ -f "$SOURCE_SCRIPT" ]] || {
  echo "FAIL: missing script: $SOURCE_SCRIPT" >&2
  exit 1
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

FIXTURE="$TMP_DIR/fixture"
BIN_DIR="$TMP_DIR/bin"
TMP_ROOT="$TMP_DIR/custom-tmp"
MKTRACE="$TMP_DIR/mktemp-args.log"

mkdir -p "$FIXTURE/tests/cli" "$FIXTURE/cli/bin" "$BIN_DIR"
cp "$SOURCE_SCRIPT" "$FIXTURE/tests/cli/test-json-flag-consistency.sh"
chmod +x "$FIXTURE/tests/cli/test-json-flag-consistency.sh"

cat > "$FIXTURE/cli/bin/ao" <<'EOF'
#!/usr/bin/env bash
for ((i = 1; i <= $#; i++)); do
  if [[ "${!i}" == "--json" ]]; then
    printf '{}\n'
    exit 0
  fi
  next=$((i + 1))
  if [[ "${!i}" == "-o" ]] && [[ $next -le $# ]] && [[ "${!next}" == "json" ]]; then
    printf '{}\n'
    exit 0
  fi
done

printf '{}\n'
EOF
chmod +x "$FIXTURE/cli/bin/ao"

cat > "$BIN_DIR/mktemp" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >> "$MKTRACE"
target="$TMP_DIR/fake-work"
mkdir -p "\$target"
printf '%s\n' "\$target"
EOF
chmod +x "$BIN_DIR/mktemp"

mkdir -p "$TMP_ROOT"

if (cd "$FIXTURE" && PATH="$BIN_DIR:$PATH" TMPDIR="$TMP_ROOT" bash tests/cli/test-json-flag-consistency.sh >/dev/null); then
  pass "script runs with stubbed temp workspace"
else
  fail "script should pass with stubbed temp workspace"
fi

mktemp_args="$(cat "$MKTRACE")"
expected_arg="$TMP_ROOT/ao-json-flag-consistency.XXXXXX"

if [[ "$mktemp_args" == "-d $expected_arg" ]]; then
  pass "script requests mktemp workspace under TMPDIR"
else
  fail "expected mktemp args '-d $expected_arg', got '$mktemp_args'"
fi

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
