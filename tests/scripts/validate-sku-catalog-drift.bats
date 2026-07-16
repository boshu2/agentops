#!/usr/bin/env bats
#
# Tests for the surviving SKU command extractor primitive
# (scripts/lib/sku_extract.py). The joined SKU catalog and its drift gate were
# removed by the Cathedral Cut.
#
# Heredocs are quoted (<<'PY') so bash performs no interpolation — REPO_ROOT and
# AO_BIN reach the Python via the environment (os.environ), avoiding backtick /
# $-expansion hazards in test bodies.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
}

# require_ao echoes a path to a usable `ao` binary, or `skip`s the test. The
# extractor test builds `ao` when needed.
require_ao() {
    if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
        echo "$REPO_ROOT/cli/bin/ao"
        return
    fi
    if command -v go >/dev/null 2>&1; then
        local bin="$BATS_TEST_TMPDIR/ao"
        if ( cd "$REPO_ROOT/cli" && go build -o "$bin" ./cmd/ao ) >/dev/null 2>&1; then
            echo "$bin"
            return
        fi
    fi
    skip "no ao binary and no Go toolchain to build one (covered by validate-sku-catalog-drift CI job)"
}

@test "extractor resolves real commands and drops stale ones" {
    AO_BIN="$(require_ao)"; export AO_BIN
    run python3 - <<'PY'
import os, sys
sys.path.insert(0, os.path.join(os.environ["REPO_ROOT"], "scripts", "lib"))
import sku_extract
valid, _ = sku_extract.scan_command_tree(os.environ["AO_BIN"])
assert sku_extract.resolve_command_path("ao status --json", valid) == ("status",)
assert sku_extract.resolve_command_path("ao schedule", valid) is None
if ("goals", "measure") in valid:
    assert sku_extract.resolve_command_path("ao goals measure", valid) == ("goals", "measure")
print("ok")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}
