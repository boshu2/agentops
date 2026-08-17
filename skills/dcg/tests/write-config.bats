#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/write-config.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/repo/.git"
  printf '[packs]\nenabled = []\n' >"$FIX/candidate.toml"
  MOCK="$FIX/dcg"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version) printf '9.9.9\n'; exit 0 ;;
  --help) printf 'test doctor\n'; exit 0 ;;
  test)
    printf '%s\n' "$*" >>"$MOCK_ACTIONS"
    [[ "$*" == *'git reset --hard HEAD'* ]] && exit 1
    exit 0
    ;;
esac
SH
  chmod +x "$MOCK"
  export DCG_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
  NEW=$(shasum -a 256 "$FIX/candidate.toml" | awk '{print $1}')
  TARGET="$FIX/repo/.dcg.toml"
}

teardown() { rm -rf "$FIX"; }

@test "validated candidate installs atomically with exact approval" {
  run "$RUN" --root "$FIX/repo" --kind project --input "$FIX/candidate.toml" --approve "dcg:write:$TARGET:$NEW:absent"
  [ "$status" -eq 0 ]
  [ "$(shasum -a 256 "$TARGET" | awk '{print $1}')" = "$NEW" ]
}

@test "raw copy baseline overwrites while wrapper refuses missing approval" {
  printf 'old\n' >"$TARGET"
  cp "$FIX/candidate.toml" "$TARGET"
  [ "$(shasum -a 256 "$TARGET" | awk '{print $1}')" = "$NEW" ]
  rm -f "$TARGET"
  run "$RUN" --root "$FIX/repo" --kind project --input "$FIX/candidate.toml"
  [ "$status" -ne 0 ]
  [ ! -e "$TARGET" ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "candidate that allows destructive probe is rejected before write" {
  sed -i.bak 's/\[\[ "$\*" == \*'"'"'git reset --hard HEAD'"'"'\* \]\] && exit 1/false/' "$MOCK"
  run "$RUN" --root "$FIX/repo" --kind project --input "$FIX/candidate.toml" --approve "dcg:write:$TARGET:$NEW:absent"
  [ "$status" -ne 0 ]
  [ ! -e "$TARGET" ]
}

@test "symlink target and filesystem root are rejected" {
  ln -s "$FIX/candidate.toml" "$TARGET"
  run "$RUN" --root "$FIX/repo" --kind project --input "$FIX/candidate.toml" --approve anything
  [ "$status" -ne 0 ]
  run "$RUN" --root / --kind project --input "$FIX/candidate.toml" --approve anything
  [ "$status" -ne 0 ]
}
