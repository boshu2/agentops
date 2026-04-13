#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/bd-cluster.sh"

    TMP_DIR="$(mktemp -d)"
    MOCK_BIN="$TMP_DIR/bin"
    mkdir -p "$MOCK_BIN"

    cat >"$MOCK_BIN/bd" <<'BD'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "list" ]]; then
  cat <<'JSON'
[
  {"id":"na-aaa","title":"Swarm cluster docs","labels":["skill:swarm"]},
  {"id":"na-bbb","title":"Swarm cluster runtime","labels":["skill:swarm"]}
]
JSON
  exit 0
fi

if [[ "$1" == "show" ]]; then
  case "$2" in
    na-aaa)
      cat <<'JSON'
[
  {"id":"na-aaa","title":"Swarm cluster docs","description":"Update skills/swarm/SKILL.md","issue_type":"chore","labels":["skill:swarm"]}
]
JSON
      ;;
    na-bbb)
      cat <<'JSON'
[
  {"id":"na-bbb","title":"Swarm cluster runtime","description":"Update skills/swarm/SKILL.md","issue_type":"chore","labels":["skill:swarm"]}
]
JSON
      ;;
    *)
      exit 1
      ;;
  esac
  exit 0
fi

exit 1
BD
    chmod +x "$MOCK_BIN/bd"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "bd-cluster --json accepts bd show array output" {
    run env PATH="$MOCK_BIN:$PATH" bash "$SCRIPT" --json

    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.clusters | type == "array"'
}
