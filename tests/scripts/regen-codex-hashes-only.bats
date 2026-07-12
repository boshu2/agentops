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

_make_source_skill() {
  local name="$1"
  mkdir -p "$TMP/skills/$name"
  printf '%s\n' "---" "name: $name" "description: fixture" "---" "source for $name" \
    >"$TMP/skills/$name/SKILL.md"
}

_set_treatment() {
  local name="$1" treatment="$2"
  mkdir -p "$TMP/skills-codex-overrides"
  cat >"$TMP/skills-codex-overrides/catalog.json" <<JSON
{"skills":[{"name":"$name","treatment":"$treatment"}]}
JSON
}

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

@test "bespoke twin refreshes source provenance when its maintained source changes" {
  _make_source_skill foo
  _set_treatment foo bespoke

  run bash "$SCRIPT" --only foo
  [ "$status" -eq 0 ]

  local first
  first="$(python3 -c 'import json,os; d=json.load(open(os.path.join(os.environ["SKILLS_ROOT"], ".agentops-manifest.json"))); print(next(e["source_hash"] for e in d["skills"] if e["name"]=="foo"))')"
  [ -n "$first" ]
  [ "$first" != "STALE_foo" ]
  [ "$(python3 -c 'import json,os; print(json.load(open(os.path.join(os.environ["SKILLS_ROOT"], "foo", ".agentops-generated.json")))["source_hash"])')" = "$first" ]

  printf '\nchanged source\n' >>"$TMP/skills/foo/SKILL.md"
  run bash "$SCRIPT" --check --only foo
  [ "$status" -eq 1 ]
  [[ "$output" == *"foo"* ]]

  bash "$SCRIPT" --only foo
  local second
  second="$(python3 -c 'import json,os; d=json.load(open(os.path.join(os.environ["SKILLS_ROOT"], ".agentops-manifest.json"))); print(next(e["source_hash"] for e in d["skills"] if e["name"]=="foo"))')"
  [ "$second" != "$first" ]
}

@test "ambient parity twin keeps its source provenance frozen" {
  _make_source_skill foo
  _set_treatment foo parity_only

  run bash "$SCRIPT" --only foo
  [ "$status" -eq 0 ]
  [ "$(_manifest_hash foo)" != "STALE_foo" ]
  [ "$(python3 -c 'import json,os; d=json.load(open(os.path.join(os.environ["SKILLS_ROOT"], ".agentops-manifest.json"))); print(next(e["source_hash"] for e in d["skills"] if e["name"]=="foo"))')" = "" ]
}

# --- Manifest dedupe (one row per skill name) --------------------------------
# Historical syncs appended duplicate skills[] rows and updated only one of a
# pair in place, so drift was masked or misreported depending on which row a
# reader's name-keyed dict kept. The writer now keys entries by name.

# _count_rows <name> — how many manifest skills[] rows carry this name.
_count_rows() {
  python3 - "$1" <<'PY'
import json, os, sys
m = json.load(open(os.path.join(os.environ["SKILLS_ROOT"], ".agentops-manifest.json")))
print(sum(1 for e in m["skills"] if e.get("name") == sys.argv[1]))
PY
}

# _add_dup_row <name> <hash> — append a duplicate manifest row for <name>.
_add_dup_row() {
  python3 - "$1" "$2" <<'PY'
import json, os, sys
p = os.path.join(os.environ["SKILLS_ROOT"], ".agentops-manifest.json")
m = json.load(open(p))
m["skills"].append({"name": sys.argv[1], "generated_hash": sys.argv[2], "source_hash": ""})
json.dump(m, open(p, "w"), indent=2)
PY
}

@test "duplicate manifest rows collapse to one row per name (last row wins, then regen fixes it)" {
  _add_dup_row foo "DUP_STALE_foo"
  [ "$(_count_rows foo)" -eq 2 ]

  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"duplicate row(s)"* ]]

  [ "$(_count_rows foo)" -eq 1 ]
  # The surviving row carries the freshly regenerated hash and agrees with the marker.
  local foo; foo="$(_manifest_hash foo)"
  [ "$foo" != "STALE_foo" ]
  [ "$foo" != "DUP_STALE_foo" ]
  [ "$(_marker_hash foo)" = "$foo" ]
}

@test "--check reports duplicate rows as drift (exit 1) without mutating the manifest" {
  # Make all hashes current first so duplication is the ONLY drift.
  bash "$SCRIPT"
  local good; good="$(_manifest_hash foo)"
  _add_dup_row foo "$good"
  [ "$(_count_rows foo)" -eq 2 ]

  run bash "$SCRIPT" --check
  [ "$status" -eq 1 ]
  [[ "$output" == *"duplicate row(s)"* ]]
  # --check must not mutate: the duplicate row is still on disk.
  [ "$(_count_rows foo)" -eq 2 ]
}

@test "dupes-only drift (current hashes, no hash update) still persists the dedupe to DISK" {
  # age-p2c7 review probe: when duplicate rows are the ONLY drift — the kept row's
  # hashes are already current, so updated[] stays empty — the write must not be
  # skipped: the manifest write is unconditional, not gated on the updated path.
  bash "$SCRIPT"
  local good; good="$(_manifest_hash foo)"
  _add_dup_row foo "$good"
  [ "$(_count_rows foo)" -eq 2 ]

  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"duplicate row(s)"* ]]
  [[ "$output" != *"Updated hashes"* ]]
  # The dedupe reached disk: exactly one row per name, hash unchanged.
  [ "$(_count_rows foo)" -eq 1 ]
  [ "$(_manifest_hash foo)" = "$good" ]
}
