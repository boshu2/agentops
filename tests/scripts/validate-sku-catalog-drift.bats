#!/usr/bin/env bats
#
# Tests for the SKU capability catalog (ag-cbm): the extractor primitive
# (scripts/lib/sku_extract.py), the join + status logic (scripts/lib/sku_catalog.py),
# and the drift gate (scripts/validate-sku-catalog-drift.sh). The gate-level test
# builds `ao` once; the logic tests run the Python predicates directly so they are
# fast and deterministic regardless of the live command tree.
#
# Heredocs are quoted (<<'PY') so bash performs no interpolation — REPO_ROOT and
# AO_BIN reach the Python via the environment (os.environ), avoiding backtick /
# $-expansion hazards in test bodies.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
}

# require_ao echoes a path to a usable `ao` binary, or `skip`s the test. The
# status/coverage logic tests need no binary and never call this; the
# extractor/linkage/gate tests do. The authoritative full check is the
# validate-sku-catalog-drift CI job (which always builds ao); this bats job has
# no Go setup, so ao-dependent cases skip gracefully when the toolchain is absent.
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

@test "status derivation: alias detected, plain merge-review stays active" {
    run python3 - <<'PY'
import os, sys
sys.path.insert(0, os.path.join(os.environ["REPO_ROOT"], "scripts", "lib"))
import sku_catalog
assert sku_catalog.compute_status("x", "deprecated", "update", "") == "deprecated"
assert sku_catalog.compute_status("x", "planned", "update", "") == "planned"
s = sku_catalog.compute_status(
    "expert-council", "", "merge-review",
    "Absorbed into council as --mode=debate; thin alias kept one release",
)
assert s == "alias-of:skill:council", s
assert sku_catalog.compute_status("crank", "", "merge-review", "Overlaps compile/forge") == "active"
print("ok")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "linkage integrity flags a fabricated stale drives_commands edge" {
    AO_BIN="$(require_ao)"; export AO_BIN
    run python3 - <<'PY'
import os, sys
sys.path.insert(0, os.path.join(os.environ["REPO_ROOT"], "scripts", "lib"))
import sku_catalog, sku_extract
valid, _ = sku_extract.scan_command_tree(os.environ["AO_BIN"])
cat = {"capabilities": [
    {"sku": "skill:fake", "type": "skill", "drives_commands": ["ao schedule"]},
]}
fails = sku_catalog.check_linkage_integrity(cat, valid)
assert len(fails) == 1 and "ao schedule" in fails[0], fails
cat2 = {"capabilities": [
    {"sku": "skill:ok", "type": "skill", "drives_commands": ["ao status"]},
]}
assert sku_catalog.check_linkage_integrity(cat2, valid) == []
print("ok")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "coverage flags a BC with no active skill" {
    run python3 - <<'PY'
import os, sys
sys.path.insert(0, os.path.join(os.environ["REPO_ROOT"], "scripts", "lib"))
import sku_catalog
caps = []
for bc, names in {"BC1": ["forge"], "BC2": ["validation"], "BC3": ["plan"],
                  "BC4": ["scaffold"], "BC5": ["push"]}.items():
    for n in names:
        caps.append({"type": "skill", "name": n, "bounded_context": bc, "status": "active"})
    caps.append({"type": "cli-command", "name": "c-" + bc, "bounded_context": bc})
for move, names in sku_catalog.LOOP_MOVES.items():
    caps.append({"type": "skill", "name": names[0], "bounded_context": "BC1", "status": "active"})
broken = {"capabilities": [
    c for c in caps if not (c.get("type") == "skill" and c.get("bounded_context") == "BC4")
]}
fails = sku_catalog.check_coverage(broken)
assert any("BC4" in f and "active skill" in f for f in fails), fails
print("ok")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "the committed registry.json passes the full drift gate" {
    AO_BIN="$(require_ao)"
    run env AGENTOPS_AO_BIN="$AO_BIN" bash "$REPO_ROOT/scripts/validate-sku-catalog-drift.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"drift — OK"* ]]
    [[ "$output" == *"linkage integrity — OK"* ]]
    [[ "$output" == *"coverage — OK"* ]]
}
