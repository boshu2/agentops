#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  MATERIALIZE="$REPO_ROOT/deploy/gc/materialize-toolchain.sh"
  LOCK="$REPO_ROOT/deploy/gc/toolchain.lock.json"
}

@test "materializer describes the official release pair by default" {
  run "$MATERIALIZE" --describe

  [ "$status" -eq 0 ]
  python3 -c 'import json,sys; value=json.load(sys.stdin); assert value["id"] == "gascity-v1.3.5-beads-v1.1.0"; assert value["status"] == "qualified"; assert value["gc"]["source_commit"] == "8ffc009ded781a2ada2077f3a29bd712b2def0bf"; assert value["bd"]["source_commit"] == "8e4e59d39f3459a43cf21a3236a13eca4dd874f7"; assert value["gc"]["release_checksum_sha256"] == "aefe5b1defa6ab383e27cd933c76e5118f3c920dab911ba19d807f1f89053d3f"; assert value["bd"]["release_checksum_sha256"] == "b3684128bf6af985ee2148ca50dc22ff19e29b43227aefef48219fdbad8cbe52"' <<<"$output"
}

@test "materializer refuses the stale PR 3985 exception identity" {
  run "$MATERIALIZE" --pair agentops-gc-pr3985-347c66b1-bd-v1.1.0 --describe

  [ "$status" -ne 0 ]
  [[ "$output" == *"unknown pair id"* ]]
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

@test "materializer fetches and verifies the locked release tag after its branch advances" {
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
tag = subprocess.run(
    ["git", "describe", "--tags", "--exact-match"],
    check=False,
    capture_output=True,
    text=True,
).stdout.strip()
version = tag.removeprefix("v") if tag == "v1.0.0" else "dev"
pathlib.Path("bin").mkdir(exist_ok=True)
pathlib.Path("bin/gc").write_text(
    "#!/bin/sh\nprintf '%s\\n' '" +
    '{"ok":true,"version":"' + version + '","commit":"' + commit + '"}' +
    "'\n",
    encoding="utf-8",
)
pathlib.Path("bd").write_text(
    "#!/bin/sh\nprintf '%s\\n' 'bd version " + version + " (" + commit + ")'\n",
    encoding="utf-8",
)
os.chmod("bin/gc", 0o755)
os.chmod("bd", 0o755)
PY
  git -C "$source_repo" add Makefile build.py
  git -C "$source_repo" commit -qm initial
  locked_commit="$(git -C "$source_repo" rev-parse HEAD)"
  git -C "$source_repo" tag v1.0.0 "$locked_commit"
  printf '%s\n' moved >"$source_repo/branch-moved"
  git -C "$source_repo" add branch-moved
  git -C "$source_repo" commit -qm moved
  python3 - "$lock" "$source_repo" "$locked_commit" <<'PY'
import json
import sys

path, repository, commit = sys.argv[1:]
component = {
    "repository": repository,
    "ref": "v1.0.0",
    "version": "1.0.0",
    "source_commit": commit,
    "runtime_commit": commit,
    "release_checksum_asset": "fixture-checksums.txt",
    "release_checksum_sha256": "0" * 64,
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
  python3 - "$output_dir/toolchain.json" "$locked_commit" "$REPO_ROOT" <<'PY'
import json
import os
import subprocess
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    receipt = json.load(handle)
root = sys.argv[3]
assert receipt["pair"]["gc"]["source_commit"] == sys.argv[2]
assert receipt["schema_version"] == 2
assert receipt["runtime"]["gc"]["commit"] == sys.argv[2][:7]
assert receipt["runtime"]["bd"]["commit"] == sys.argv[2][:7]
assert receipt["runtime"]["gc"]["path"] == "bin/gc"
assert receipt["runtime"]["bd"]["path"] == "bin/bd"
assert receipt["runtime"]["ao"]["path"] == "bin/ao"
assert os.path.isfile(os.path.join(os.path.dirname(sys.argv[1]), "bin", "ao"))
assert receipt["runtime"]["ao"]["source_commit"] == subprocess.check_output(["git", "-C", root, "rev-parse", "HEAD"], text=True).strip()
assert receipt["runtime"]["ao"]["cli_tree"] == subprocess.check_output(["git", "-C", root, "rev-parse", "HEAD:cli"], text=True).strip()
assert receipt["runtime"]["ao"]["build_version"]
PY
}
