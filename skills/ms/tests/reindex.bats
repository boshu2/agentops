#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/reindex.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/data/index" "$FIX/skills/example"
  printf '%s\n' '---' 'name: example' 'description: example' '---' '# Example' >"$FIX/skills/example/SKILL.md"
  cat >"$FIX/ms.c" <<'C'
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
int main(int argc, char **argv) {
  if (argc == 2 && strcmp(argv[1], "-V") == 0) { puts("ms 9.9.9"); return 0; }
  if (argc == 2 && strcmp(argv[1], "--help") == 0) {
    puts(getenv("MOCK_BAD_HELP") ? "index load" : "index mcp load"); return 0;
  }
  if (argc >= 2 && strcmp(argv[1], "index") == 0) {
    FILE *f = fopen(getenv("MOCK_ACTIONS"), "a"); fputs("index\n", f); fclose(f);
    if (getenv("MOCK_DIRTY_LOCK")) {
      char path[4096]; snprintf(path, sizeof(path), "%s/.agentops-reindex.lock/unexpected", getenv("MOCK_DATA"));
      FILE *dirty = fopen(path, "w"); fputs("foreign\n", dirty); fclose(dirty);
    }
    puts("{\"indexed\":1,\"errors\":[],\"package_summary\":{\"skills_discovered\":1}}"); return 0;
  }
  if (argc >= 3 && strcmp(argv[1], "mcp") == 0 && strcmp(argv[2], "serve") == 0) {
    if (getenv("MOCK_PERSIST")) { while (1) pause(); }
    char buf[4096]; while (fgets(buf, sizeof(buf), stdin)) {}
    puts("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}");
    puts("{\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"query\\\":\\\"account rotation\\\",\\\"count\\\":1,\\\"results\\\":[{\\\"id\\\":\\\"example\\\"}]}\"}]}}");
    return 0;
  }
  return 2;
}
C
  cc "$FIX/ms.c" -o "$FIX/ms"
  export MS_BIN="$FIX/ms"
  export MOCK_ACTIONS="$FIX/actions"
  export MOCK_DATA="$FIX/data"
}

teardown() {
  if [[ -n "${SERVER_PID:-}" ]]; then kill -KILL "$SERVER_PID" 2>/dev/null || true; fi
  rm -rf "$FIX"
}

@test "normal reindex reaps an exact server and probes fresh state" {
  MOCK_PERSIST=1 "$MS_BIN" mcp serve &
  SERVER_PID=$!
  sleep 0.1
  run "$RUN" --data-dir "$FIX/data" --skills-root "$FIX/skills" --approve "ms:reindex:$FIX/data:$FIX/skills" --deadline 30
  [ "$status" -eq 0 ]
  run kill -0 "$SERVER_PID"
  [ "$status" -ne 0 ]
  SERVER_PID=''
  [ "$(grep -c '^index$' "$MOCK_ACTIONS")" -eq 1 ]
  [ ! -e "$FIX/data/.agentops-reindex.lock" ]
}

@test "raw baseline indexes while missing approval stops before index" {
  run "$MS_BIN" index -O json
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" --data-dir "$FIX/data" --skills-root "$FIX/skills"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "live writer lock stops before rebuild" {
  printf '{"pid":%s}\n' "$$" >"$FIX/data/ms.lock"
  run "$RUN" --data-dir "$FIX/data" --skills-root "$FIX/skills" --approve "ms:reindex:$FIX/data:$FIX/skills" --deadline 30
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "missing MCP capability stops before process cleanup or rebuild" {
  export MOCK_BAD_HELP=1
  run "$RUN" --data-dir "$FIX/data" --skills-root "$FIX/skills" --approve "ms:reindex:$FIX/data:$FIX/skills" --deadline 30
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "failed lock cleanup cannot be reported as a successful reindex" {
  export MOCK_DIRTY_LOCK=1
  run "$RUN" --data-dir "$FIX/data" --skills-root "$FIX/skills" --approve "ms:reindex:$FIX/data:$FIX/skills" --deadline 30
  [ "$status" -eq 125 ]
  [ -e "$FIX/data/.agentops-reindex.lock/unexpected" ]
}
