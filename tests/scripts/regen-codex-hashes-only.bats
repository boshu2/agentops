#!/usr/bin/env bats
# regen-codex-hashes-only.bats — the --only scoping flag (ag-fbe9).
#
# Behavior under test: `regen-codex-hashes.sh --only <skill>` rewrites ONLY the
# named skill's generated_hash and leaves every other (possibly pre-existing-
# drifted) skill's manifest entry + marker untouched, so a single-skill PR no
# longer sweeps unrelated codex drift into its diff. No --only = regenerate all.
#
# Hermetic: the script honors a SKILLS_ROOT env override, so we point it at a
# throwaway fixture tree with no source twins (skills/<name>/ absent), which
# keeps source_hash empty and isolates the generated_hash behavior.

setup() {
  source "$(git rev-parse --show-toplevel)/lib/bats-common.bash"
  SCRIPT="$(bats_repo_root)/scripts/regen-codex-hashes.sh"
  TMP="$(mktemp -d)"
  export SKILLS_ROOT="$TMP/skills-codex"
  mkdir -p "$SKILLS_ROOT"

  # Two codex skills, each with a deliberately WRONG (drifted) generated_hash.
  _make_skill foo
  _make_skill bar

  cat >"$SKILLS_ROOT/.agentops-manifest.json" <<'JSON'
{
  "skills": [
    { "name": "foo", "generated_hash": "STALE_foo", "source_hash": "" },
    { "name": "bar", "generated_hash": "STALE_bar", "source_hash": "" }
  ]
}
JSON
}

teardown() { rm -rf "$TMP"; }

# _make_skill <name> — a codex skill dir with content + a stale marker.
_make_skill() {
  local name="$1"
  mkdir -p "$SKILLS_ROOT/$name"
  printf 'content for %s\n' "$name" >"$SKILLS_ROOT/$name/SKILL.md"
  cat >"$SKILLS_ROOT/$name/.agentops-generated.json" <<JSON
{ "generated_hash": "STALE_${name}", "source_hash": "" }
JSON
}

# _hash_of <name> — the JSON value of "generated_hash" for a manifest entry.
_manifest_hash() {
  python3 - "$1" <<'PY'
import json, os, sys
m = json.load(open(os.path.join(os.environ["SKILLS_ROOT"], ".agentops-manifest.json")))
name = sys.argv[1]
print(next(e["generated_hash"] for e in m["skills"] if e["name"] == name))
PY
}

_marker_hash() {
  python3 - "$1" <<'PY'
import json, os, sys
p = os.path.join(os.environ["SKILLS_ROOT"], sys.argv[1], ".agentops-generated.json")
print(json.load(open(p))["generated_hash"])
PY
}

@test "--only foo regenerates only foo; bar stays at its pre-existing drift" {
  run bash "$SCRIPT" --only foo
  [ "$status" -eq 0 ]

  # foo updated away from the stale value...
  local foo; foo="$(_manifest_hash foo)"
  [ "$foo" != "STALE_foo" ]
  [ -n "$foo" ]
  # ...and its marker matches the manifest entry.
  [ "$(_marker_hash foo)" = "$foo" ]

  # bar left completely untouched (manifest + marker still the stale value).
  [ "$(_manifest_hash bar)" = "STALE_bar" ]
  [ "$(_marker_hash bar)" = "STALE_bar" ]
}

@test "no --only regenerates every skill" {
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]

  [ "$(_manifest_hash foo)" != "STALE_foo" ]
  [ "$(_manifest_hash bar)" != "STALE_bar" ]
  # foo and bar have distinct content, so distinct hashes.
  [ "$(_manifest_hash foo)" != "$(_manifest_hash bar)" ]
}

@test "--only bar with --check reports drift for bar only and exits 1" {
  run bash "$SCRIPT" --check --only bar
  [ "$status" -eq 1 ]
  [[ "$output" == *"bar"* ]]
  [[ "$output" != *"foo"* ]]
  # --check must not mutate anything.
  [ "$(_manifest_hash bar)" = "STALE_bar" ]
  [ "$(_manifest_hash foo)" = "STALE_foo" ]
}

@test "--only with a comma list scopes to all listed skills" {
  run bash "$SCRIPT" --only foo,bar
  [ "$status" -eq 0 ]
  [ "$(_manifest_hash foo)" != "STALE_foo" ]
  [ "$(_manifest_hash bar)" != "STALE_bar" ]
}

@test "--only requires an argument" {
  run bash "$SCRIPT" --only
  [ "$status" -eq 2 ]
  [[ "$output" == *"requires a skill list"* ]]
}
