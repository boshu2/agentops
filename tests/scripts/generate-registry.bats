#!/usr/bin/env bats
#
# Tests for scripts/generate-registry.sh focused on deterministic eval-suite
# discovery. Registry generation must depend on tracked files, not local
# untracked eval artifacts from developer runs.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts"
    cp "$REPO_ROOT/scripts/generate-registry.sh" "$FAKE_REPO/scripts/generate-registry.sh"
    chmod +x "$FAKE_REPO/scripts/generate-registry.sh"
    git -C "$TMP_DIR" init repo >/dev/null 2>&1
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "eval registry ignores untracked eval suites and untracked eval files" {
    mkdir -p "$FAKE_REPO/evals/tracked-suite" "$FAKE_REPO/evals/local-suite"
    printf '{"id":"tracked.case"}\n' > "$FAKE_REPO/evals/tracked-suite/tracked.json"
    printf '{"id":"local.case"}\n' > "$FAKE_REPO/evals/tracked-suite/local-only.json"
    printf '{"id":"local.suite"}\n' > "$FAKE_REPO/evals/local-suite/noise.json"
    git -C "$FAKE_REPO" add evals/tracked-suite/tracked.json

    run bash "$FAKE_REPO/scripts/generate-registry.sh" --stdout

    [ "$status" -eq 0 ]
    printf '%s\n' "$output" | jq -e '
      .surfaces.evals == [
        {
          "suite": "tracked-suite",
          "path": "evals/tracked-suite/",
          "eval_count": 1,
          "evals": ["tracked"]
        }
      ]
    ' >/dev/null
}
