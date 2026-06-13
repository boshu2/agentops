#!/usr/bin/env bats
#
# S1 re-bin + BC6 population + role re-grade contract (ag-j3ge0).
#
# Asserts the corrected dispositions on the CANONICAL ledger
# (docs/contracts/skill-dispositions.yaml) after the S0 BC6 foundation
# (ag-4akl8) landed. These are the bead's acceptance scenarios made executable:
#   - mis-binned skills land in their correct bounded context
#   - the loop spine (rpi/evolve) is graded `domain`, not `supporting`
#   - the lone `generic` skill (converter) is given a real hexagonal role
#   - every bounded context BC1-BC6 is non-empty
#
# The assertions read the production ledger with the same yaml loader the
# validators use (round-trip fidelity), so a green here means the real
# generators + drift gates see the same corrected shape.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    DISP_YAML="$REPO_ROOT/docs/contracts/skill-dispositions.yaml"
    export DISP_YAML
}

# Helper: print "<domain>|<hexagonal_role>" for an active skill row.
_row() {
    python3 - "$1" <<'PY'
import sys, yaml, pathlib
disp = pathlib.Path(__import__("os").environ["DISP_YAML"])
data = yaml.safe_load(disp.read_text(encoding="utf-8")) or {}
rows = {r["skill"]: r for r in data.get("dispositions", []) if isinstance(r, dict) and "skill" in r}
name = sys.argv[1]
r = rows.get(name)
if r is None:
    print("MISSING")
else:
    print(f"{r.get('domain')}|{r.get('hexagonal_role')}")
PY
}

@test "refactor is re-binned BC2 Validation -> BC3 Loop" {
    run _row refactor
    [ "$status" -eq 0 ]
    [[ "$output" == "BC3 Loop|"* ]]
}

@test "perf is re-binned BC2 Validation -> BC3 Loop" {
    run _row perf
    [ "$status" -eq 0 ]
    [[ "$output" == "BC3 Loop|"* ]]
}

@test "cass is re-binned BC5 Runtime -> BC1 Corpus" {
    run _row cass
    [ "$status" -eq 0 ]
    [[ "$output" == "BC1 Corpus|"* ]]
}

@test "rpi is graded domain (loop spine), not supporting" {
    run _row rpi
    [ "$status" -eq 0 ]
    [ "$output" = "BC3 Loop|domain" ]
}

@test "evolve is graded domain (loop spine), not supporting" {
    run _row evolve
    [ "$status" -eq 0 ]
    [ "$output" = "BC3 Loop|domain" ]
}

@test "converter is no longer the lone generic; it has a real hexagonal role" {
    run _row converter
    [ "$status" -eq 0 ]
    [[ "$output" != *"|generic" ]]
}

@test "no active skill row remains tagged generic" {
    run python3 - <<'PY'
import yaml, pathlib, os
disp = pathlib.Path(os.environ["DISP_YAML"])
data = yaml.safe_load(disp.read_text(encoding="utf-8")) or {}
generic = [r["skill"] for r in data.get("dispositions", [])
           if isinstance(r, dict) and r.get("hexagonal_role") == "generic"]
if generic:
    print("GENERIC:" + ",".join(generic))
    raise SystemExit(1)
print("OK")
PY
    [ "$status" -eq 0 ]
    [ "$output" = "OK" ]
}

@test "every bounded context BC1-BC6 is non-empty" {
    run python3 - <<'PY'
import yaml, pathlib, os
disp = pathlib.Path(os.environ["DISP_YAML"])
data = yaml.safe_load(disp.read_text(encoding="utf-8")) or {}
seen = set()
for r in data.get("dispositions", []):
    if isinstance(r, dict) and r.get("domain"):
        seen.add(str(r["domain"]).split()[0])  # "BC3 Loop" -> "BC3"
missing = [f"BC{n}" for n in range(1, 7) if f"BC{n}" not in seen]
if missing:
    print("EMPTY:" + ",".join(missing))
    raise SystemExit(1)
print("OK")
PY
    [ "$status" -eq 0 ]
    [ "$output" = "OK" ]
}
