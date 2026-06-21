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
  # age-obae: pawl-verdict.sh `write` has side-effects — `ao provenance
  # emit-verdict` (resolveLedgerPath walks up for .git or docs/+schemas/) and
  # `ao yield emit` (cwd-walk). Run from an isolated git workspace so those
  # ledgers resolve into TMP, never the real repo's TRACKED docs/provenance/.
  git -C "$TMP" init -q
  git -C "$TMP" config user.email t@t
  git -C "$TMP" config user.name t
  cd "$TMP"
  # The provenance emit isolates via cwd (resolveLedgerPath walks up to $TMP/.git);
  # the yield emit is forced to cd to the script's root, so it needs the
  # AGENTOPS_REPO_ROOT override to land in TMP too. (age-obae)
  export AGENTOPS_REPO_ROOT="$TMP"
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

@test "pawl-verdict write does not pollute the repo's real provenance OR yield ledgers (age-obae)" {
  prov="$REPO_ROOT/docs/provenance/ledger.jsonl"
  yield="$REPO_ROOT/.agents/yield/yield-ledger.jsonl"
  prov_before="$(shasum -a 256 "$prov" 2>/dev/null | awk '{print $1}')"
  yield_before="$(shasum -a 256 "$yield" 2>/dev/null | awk '{print $1}')"
  run bash "$SCRIPT" write age-obae-iso 0 \
    --disposition CONFIRMED --head "$SHA" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$TMP/evidence.txt" \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  # Core guarantee — holds whether or not `ao` is installed (the Bats CI job runs
  # before `ao` is built; both emits are best-effort): the real repo's TRACKED
  # provenance ledger AND its (gitignored) yield ledger are byte-identical.
  [ "$prov_before" = "$(shasum -a 256 "$prov" 2>/dev/null | awk '{print $1}')" ]
  [ "$yield_before" = "$(shasum -a 256 "$yield" 2>/dev/null | awk '{print $1}')" ]
  # When `ao` IS present the emits fire — assert they landed in the ISOLATED temp
  # repo (provenance via cwd, yield via AGENTOPS_REPO_ROOT). Conditional so a thin
  # env (no `ao`) doesn't false-fail; the byte-identical checks are the invariant.
  if command -v ao >/dev/null 2>&1; then
    [ -f "$TMP/docs/provenance/ledger.jsonl" ]
    [ -f "$TMP/.agents/yield/yield-ledger.jsonl" ]
  fi
}

@test "jq schema-fallback rejects invalid disposition when jsonschema validators are absent" {
  cat > "$TMP/verdicts/age-anc-bad.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"age-anc-bad","pr":0,"head_sha":"$SHA","disposition":"MAYBE","generated_at":"2026-01-01T00:00:00Z","author_context_id":"author-ctx","refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-reviewer-ctx","evidence":"$TMP/evidence.txt"}]}
EOF
  run bash "$SCRIPT" check age-anc-bad 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -ne 0 ]
}
