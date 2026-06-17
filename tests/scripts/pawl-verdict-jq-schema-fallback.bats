#!/usr/bin/env bats
# age-anc: jq schema-fallback must accept valid verdicts when check-jsonschema
# and python jsonschema are absent (minimal Codex / unattended host).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  REAL_PYTHON="$(command -v python3)"
  cat >"$TMP/bin/python3" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == "-c" && "\$2" == *jsonschema* ]]; then
  exit 1
fi
exec "$REAL_PYTHON" "\$@"
EOF
  chmod +x "$TMP/bin/python3"
  export PATH="$TMP/bin:$PATH"
  SHA="cafef00dbabe1234cafef00dbabe1234cafef00d"
  printf 'fresh-context review evidence\n' > "$TMP/evidence.txt"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

@test "jq schema-fallback accepts a valid CONFIRMED verdict when jsonschema validators are absent" {
  run bash "$SCRIPT" write age-anc-jq 0 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]

  run bash "$SCRIPT" check age-anc-jq 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

@test "jq schema-fallback rejects invalid disposition when jsonschema validators are absent" {
  cat > "$TMP/verdicts/age-anc-bad.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"age-anc-bad","pr":0,"head_sha":"$SHA","disposition":"MAYBE","generated_at":"2026-01-01T00:00:00Z","author_context_id":"author-ctx","refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-reviewer-ctx","evidence":"$TMP/evidence.txt"}]}
EOF
  run bash "$SCRIPT" check age-anc-bad 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -ne 0 ]
}
