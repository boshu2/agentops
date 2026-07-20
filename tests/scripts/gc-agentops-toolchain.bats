#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  MATERIALIZE="$REPO_ROOT/deploy/gc/materialize-toolchain.sh"
  LOCK="$REPO_ROOT/deploy/gc/toolchain.lock.json"
}

@test "materializer describes the qualified pair by default" {
  run "$MATERIALIZE" --describe

  [ "$status" -eq 0 ]
  python3 -c 'import json,sys; value=json.load(sys.stdin); assert value["id"] == "agentops-gc-v16-20260719"; assert value["status"] == "qualified"' <<<"$output"
}

@test "materializer can select the compatible official release" {
  run "$MATERIALIZE" --pair gascity-v1.3.5-sdk-release --describe

  [ "$status" -eq 0 ]
  python3 -c 'import json,sys; value=json.load(sys.stdin); assert value["gc"]["ref"] == "v1.3.5"; assert value["bd"]["ref"] == "v1.1.0"' <<<"$output"
}

@test "materializer fails closed for an unknown pair" {
  run "$MATERIALIZE" --pair does-not-exist --describe

  [ "$status" -ne 0 ]
  [[ "$output" == *"unknown pair id"* ]]
}

@test "materializer refuses a nonempty destination before cloning" {
  destination="$BATS_TEST_TMPDIR/existing"
  mkdir -p "$destination"
  printf '%s\n' preserve >"$destination/user-file"

  run "$MATERIALIZE" --output "$destination"

  [ "$status" -ne 0 ]
  [[ "$output" == *"output directory is not empty"* ]]
  [ "$(cat "$destination/user-file")" = preserve ]
}

@test "materializer fetches the locked commit after its branch advances" {
  source_repo="$BATS_TEST_TMPDIR/source"
  output_dir="$BATS_TEST_TMPDIR/output"
  lock="$BATS_TEST_TMPDIR/toolchain.lock.json"
  mkdir -p "$source_repo"
  git -C "$source_repo" init -q -b main
  git -C "$source_repo" config user.name test
  git -C "$source_repo" config user.email test@example.invalid
  cat >"$source_repo/Makefile" <<'EOF'
build:
	@python3 build.py
EOF
  cat >"$source_repo/build.py" <<'PY'
import os
import pathlib
import subprocess

commit = subprocess.check_output(["git", "rev-parse", "--short", "HEAD"], text=True).strip()
pathlib.Path("bin").mkdir(exist_ok=True)
pathlib.Path("bin/gc").write_text(
    "#!/bin/sh\nprintf '%s\\n' '" +
    '{"ok":true,"version":"test","commit":"' + commit + '"}' +
    "'\n",
    encoding="utf-8",
)
pathlib.Path("bd").write_text(
    "#!/bin/sh\nprintf '%s\\n' 'bd version test (" + commit + ")'\n",
    encoding="utf-8",
)
os.chmod("bin/gc", 0o755)
os.chmod("bd", 0o755)
PY
  git -C "$source_repo" add Makefile build.py
  git -C "$source_repo" commit -qm initial
  locked_commit="$(git -C "$source_repo" rev-parse HEAD)"
  printf '%s\n' moved >"$source_repo/branch-moved"
  git -C "$source_repo" add branch-moved
  git -C "$source_repo" commit -qm moved
  python3 - "$lock" "$source_repo" "$locked_commit" <<'PY'
import json
import sys

path, repository, commit = sys.argv[1:]
component = {
    "repository": repository,
    "ref": "main",
    "version": "test",
    "source_commit": commit,
    "runtime_commit": commit,
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump({
        "schema_version": 1,
        "accepted_pairs": [{
            "id": "moving-branch",
            "status": "qualified",
            "gc": component,
            "bd": component,
        }],
    }, handle)
PY

  run "$MATERIALIZE" --lock "$lock" --output "$output_dir"

  [ "$status" -eq 0 ]
  python3 - "$output_dir/toolchain.json" "$locked_commit" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    receipt = json.load(handle)
assert receipt["pair"]["gc"]["source_commit"] == sys.argv[2]
assert receipt["runtime"]["gc"]["commit"] == sys.argv[2][:7]
assert receipt["runtime"]["bd"]["commit"] == sys.argv[2][:7]
PY
}
