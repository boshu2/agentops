#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Contract tests for the advisory skill.probe-coverage gate. A ledger label is
# never evidence by itself: a directional verdict counts only when its one
# scorecard pointer resolves to a verified v3 scorecard + self-contained fixture
# set whose response-only discriminator replay produces the same classification.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    GATE="$REPO_ROOT/scripts/check-skill-probe-coverage.sh"
    HARNESS="$REPO_ROOT/scripts/probe-skill.sh"
    META_TOOL="$REPO_ROOT/scripts/lib/probe-fixture-metadata.py"
    PREAMBLE="$REPO_ROOT/scripts/lib/preamble.sh"
    DISPATCH_HELPER="$REPO_ROOT/scripts/lib/codex-exec.sh"

    FIX="$BATS_TEST_TMPDIR/repo"
    PROBES="$FIX/evals/skill-probes"
    mkdir -p "$FIX/skills" "$PROBES" "$FIX/docs/evals/scorecards" "$FIX/scripts/lib"
    cp "$HARNESS" "$FIX/scripts/probe-skill.sh"
    cp "$META_TOOL" "$FIX/scripts/lib/probe-fixture-metadata.py"
    cp "$PREAMBLE" "$FIX/scripts/lib/preamble.sh"
    cp "$DISPATCH_HELPER" "$FIX/scripts/lib/codex-exec.sh"
    mkdir -p "$FIX/test-bin"
    # Synthetic unit-only runtime identity: it exercises the positive verifier
    # path but is neither persisted evidence nor a claim that a model was run.
    cat > "$FIX/test-bin/codex" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
    printf 'codex-cli synthetic-gate-test\n'
    exit 0
fi
printf 'synthetic identity stub is not a live producer\n' >&2
exit 70
SH
    chmod +x "$FIX/test-bin/codex"
    export PATH="$FIX/test-bin:$PATH"
    export SKILL_PROBE_SKILLS_DIR="$FIX/skills"
    export SKILL_PROBE_LEDGER_FILE="$PROBES/LEDGER.md"
    export SKILL_PROBE_EVIDENCE_ROOT="$FIX"
    # Declared denominator: fixture runs start with an EMPTY exclusion list, so
    # the repo's real exclusions (which name real skills) cannot leak in and
    # trip the stale-slug guard against a synthetic skills dir.
    EXCLUSIONS="$FIX/scripts/.skill-probe-denominator-exclusions"
    : > "$EXCLUSIONS"
    export SKILL_PROBE_EXCLUSIONS_FILE="$EXCLUSIONS"
}

# write_exclusions LINE... — replace the fixture exclusion list.
write_exclusions() {
    : > "$EXCLUSIONS"
    local line
    for line in "$@"; do
        printf '%s\n' "$line" >> "$EXCLUSIONS"
    done
}

make_skill() {
    local name="$1" tier="$2"
    mkdir -p "$SKILL_PROBE_SKILLS_DIR/$name"
    cat > "$SKILL_PROBE_SKILLS_DIR/$name/SKILL.md" <<EOF
---
name: $name
description: fixture skill $name
metadata:
  tier: $tier
---
# $name
EOF
}

make_redirect() {
    local name="$1"
    mkdir -p "$SKILL_PROBE_SKILLS_DIR/$name"
    cat > "$SKILL_PROBE_SKILLS_DIR/$name/SKILL.md" <<EOF
---
name: $name
implementation: false
---
Use the canonical skill instead.
EOF
}

write_ledger() {
    local ledger_file="${SKILL_PROBE_LEDGER_FILE:-$SKILL_PROBE_TIERS_FILE}"
    {
        echo "# Behavioral probe ledger"
        echo
        echo "## Behavioral Probe Ledger (MEASUREMENT STATUS)"
        echo
        echo "| Skill | Probe | Date | Verdict | Notes |"
        echo "|---|---|---|---|---|"
        local row
        for row in "$@"; do
            echo "| $row |"
        done
    } > "$ledger_file"
}

write_transcript() {
    local directory="$1" name="$2" body="$3"
    python3 - "$directory" "$name" "$body" <<'PY'
import json, pathlib, sys

directory = pathlib.Path(sys.argv[1])
name = sys.argv[2]
body = sys.argv[3]
arm, rep_text = name.rsplit("-", 1)
rep = int(rep_text)
contract = json.loads((directory / "capture-contract.json").read_text())
prompt = contract["prompts"][0 if arm == "control" else 1]
position = next(
    item["position"]
    for item in contract["schedule"]
    if item["arm"] == arm and item["rep"] == rep
)
events = [
    {
        "type": "agentops.probe-input.v1",
        "arm": arm,
        "rep": rep,
        "position": position,
        "prompt": prompt,
    },
    {"type": "thread.started", "thread_id": f"gate-{directory.name}-{name}"},
    {"type": "turn.started"},
    {
        "type": "item.completed",
        "item": {"id": f"item-{name}", "type": "agent_message", "text": body},
    },
    {
        "type": "turn.completed",
        "usage": {"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1},
    },
]
(directory / f"{name}.txt").write_text(
    "".join(json.dumps(event, sort_keys=True, separators=(",", ":")) + "\n" for event in events)
)
PY
}

# write_seal DIR SPEC — place the harness-shaped seal record
# (agentops-skill-probe-seal.v1) the capture stage carries into `snapshot`.
# `full` denies the checkout plus the four skill roots under the recorded real
# home (a fixture home, so the test never depends on the operator's $HOME),
# which is what a counted row must prove. `symlinked-skill-root` denies the
# checkout but not the literal ~/.claude/skills, which is a symlink INTO the
# checkout: the kernel seals the resolved path, so the row still counts.
write_seal() {
    local directory="$1" spec="$2"
    local repo_real
    repo_real="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$FIX")"
    # The fixture home lives OUTSIDE the fixture checkout so a checkout deny
    # cannot cover a skill root by ancestry.
    REAL_HOME="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$BATS_TEST_TMPDIR")/home"
    mkdir -p "$BATS_TEST_TMPDIR/home"
    local -a roots=(
        "$repo_real"
        "$REAL_HOME/.agents"
        "$REAL_HOME/.claude/skills"
        "$REAL_HOME/.gemini/skills"
        "$REAL_HOME/.codex/skills"
    )
    case "$spec" in
        absent|legacy) return 0 ;;
        none) roots=() ;;
        full) ;;
        omit-checkout) roots=("${roots[@]:1}") ;;
        omit-skill-root) roots=("${roots[@]:0:4}") ;;
        symlinked-skill-root)
            mkdir -p "$BATS_TEST_TMPDIR/home/.claude"
            ln -s "$FIX/skills" "$BATS_TEST_TMPDIR/home/.claude/skills"
            roots=("${roots[0]}" "${roots[1]}" "${roots[3]}" "${roots[4]}")
            ;;
        *) echo "unknown seal spec: $spec" >&2; return 1 ;;
    esac
    python3 - "$directory" "$spec" "$REAL_HOME" "${roots[@]}" <<'PY'
import hashlib, json, pathlib, sys

directory = pathlib.Path(sys.argv[1])
spec = sys.argv[2]
real_home = sys.argv[3]
denied = sys.argv[4:]
sealed = spec != "none"
profile = None
if sealed:
    profile = "(version 1)\n(allow default)\n(deny file-read*\n" + "".join(
        f'  (subpath "{root}")\n' for root in denied
    ) + ")"
record = {
    "schema": "agentops-skill-probe-seal.v1",
    "seal_mode": "seatbelt" if sealed else "none",
    "coverage_eligible": sealed,
    "mechanism": "sandbox-exec" if sealed else None,
    "sandbox_exec": "/usr/bin/sandbox-exec" if sealed else None,
    "platform": "Darwin",
    "wrap": ["sandbox-exec", "-p", profile] if sealed else [],
    "profile": profile,
    "profile_file": "/private/tmp/probe-seal/seal.sb" if sealed else None,
    "profile_sha256": ("sha256:" + hashlib.sha256(profile.encode()).hexdigest()) if sealed else None,
    "denied_read_roots": denied,
    "writable_roots": ["/private/tmp/probe-ws", "/private/tmp/probe-seal"] if sealed else [],
    "rep_env": {"HOME": "/private/tmp/probe-seal", "CODEX_HOME": "/private/tmp/probe-seal/.codex"},
    "real_home": real_home,
    "real_codex_home": real_home + "/.codex",
    "auth_links": ["auth.json"] if sealed else [],
}
(directory / "seal.json").write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
PY
}

# set_scorecard_seal SCORECARD_PATH SEAL_JSON_PATH [PYTHON_SNIPPET] — echo the
# seal record into the scorecard the way the live harness does, optionally
# mutating the copy first.
set_scorecard_seal() {
    python3 - "$1" "$2" "${3:-}" <<'PY'
import json, sys
scorecard_path, seal_path, snippet = sys.argv[1:]
scorecard = json.load(open(scorecard_path))
record = json.load(open(seal_path))
if snippet:
    exec(snippet, {"record": record})
scorecard["seal"] = record
open(scorecard_path, "w").write(json.dumps(scorecard, indent=2, sort_keys=True) + "\n")
PY
}

# downgrade_contract_to_v2 PATH — rewrite a fresh v3 capture contract into the
# pre-seal v2 shape (no seal block, v2-era eligibility) and rebind it.
downgrade_contract_to_v2() {
    python3 - "$1" <<'PY'
import hashlib, json, sys
path = sys.argv[1]
contract = json.load(open(path))
contract.pop("seal")
contract["schema"] = "agentops-skill-probe-capture.v2"
identity = contract["producer_request"]["identity"]
identity["coverage_eligible"] = (
    not identity["override"]
    and contract["producer_request"]["model"] is not None
    and contract["producer_request"]["effort"] is not None
)
payload = {key: value for key, value in contract.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
contract["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
open(path, "w").write(json.dumps(contract, indent=2, sort_keys=True) + "\n")
PY
}

# make_bound_result SKILL PROBE VERDICT [TREATMENT_SOURCE] [PRODUCER_OVERRIDE] [SEAL_SPEC]
# Sets BOUND_SCORECARD_REL to a safe repo-relative v3 scorecard path.
make_bound_result() {
    local skill="$1" probe="$2" verdict="$3"
    local treatment_source="${4:-canonical-skill}"
    local producer_override="${5:-}"
    # SEAL_SPEC: full (default) | none | absent | omit-checkout | omit-skill-root | legacy
    local seal_spec="${6:-full}"
    local probe_dir="$PROBES/$probe"
    local fixture_name="fixtures-test"
    local fixture_dir="$probe_dir/$fixture_name"
    local control_body="ABSENT" treatment_body="ABSENT" rep

    if [ "$verdict" = "BEHAVIORAL" ]; then treatment_body="ACTION"; fi
    mkdir -p "$fixture_dir"
    write_seal "$fixture_dir" "$seal_spec"
    cat > "$probe_dir/probe.json" <<EOF
{"id":"$probe","skill":"$skill","reps":2,"discriminator":"discriminator.sh","treatment_source":"$treatment_source"}
EOF
    printf 'QUESTION\n' > "$probe_dir/question.md"
    printf 'PRELUDE\n' > "$probe_dir/treatment-prelude.md"
    cat > "$probe_dir/discriminator.sh" <<'SH'
#!/usr/bin/env bash
if grep -q '^INFRA$' "$1"; then exit 2; fi
grep -q '^ACTION$' "$1"
SH
    chmod +x "$probe_dir/discriminator.sh"
    local -a snapshot_args=(
        snapshot
        --fixture-dir "$fixture_dir"
        --probe-dir "$probe_dir"
        --skills-dir "$SKILL_PROBE_SKILLS_DIR"
        --probe "$probe"
        --requested-model fixture-model
        --requested-effort low
    )
    if [[ -n "$producer_override" ]]; then
        snapshot_args+=(--producer-override-bin "$producer_override")
    fi
    python3 "$META_TOOL" "${snapshot_args[@]}" >/dev/null
    if [[ "$seal_spec" == "legacy" ]]; then
        downgrade_contract_to_v2 "$fixture_dir/capture-contract.json"
    fi
    for rep in 1 2; do
        write_transcript "$fixture_dir" "control-$rep" "$control_body"
        write_transcript "$fixture_dir" "treatment-$rep" "$treatment_body"
    done

    python3 "$META_TOOL" create \
        --fixture-dir "$fixture_dir" \
        --probe-dir "$probe_dir" \
        --skills-dir "$SKILL_PROBE_SKILLS_DIR" \
        --harness "$HARNESS" \
        --preamble "$PREAMBLE" \
        --dispatch-helper "$DISPATCH_HELPER" \
        --probe "$probe" \
        --reps 2 \
        --requested-model fixture-model \
        --requested-effort low >/dev/null

    BOUND_SCORECARD_REL="docs/evals/scorecards/$probe.json"
    SKILL_PROBES_DIR="$PROBES" bash "$HARNESS" \
        --probe "$probe" \
        --replay \
        --fixtures "$fixture_name" \
        --output "$FIX/$BOUND_SCORECARD_REL" >/dev/null
}

json_field() {
    python3 -c '
import json, sys
value = json.loads(sys.argv[1])
for part in sys.argv[2].split("."):
    value = value[part]
print(value)
' "$1" "$2"
}

@test "a product-tier skill absent from the ledger is named and strict fails" {
    make_skill foo product
    write_ledger

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"foo"* ]]
}

@test "default mode stays advisory and names missing coverage" {
    make_skill foo product
    write_ledger

    run bash "$GATE"

    [ "$status" -eq 0 ]
    [[ "$output" == *"foo"* ]]
    [[ "$output" == *"WARN"* ]]
}

@test "a judgment-tier skill is gated too" {
    make_skill val judgment
    write_ledger

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"val"* ]]
}

@test "a hand-written BEHAVIORAL row without v3 evidence does not count" {
    make_skill foo product
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | prose only"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"lacks exactly one scorecard"* ]]
    [[ "$output" == *"foo"* ]]
}

@test "a valid bound BEHAVIORAL scorecard counts" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 0 ]
    [[ "$output" != *"foo"* ]]
}

@test "a valid bound INERT scorecard counts" {
    make_skill foo product
    make_bound_result foo probe-foo INERT
    write_ledger "foo | probe-foo | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 0 ]
}

@test "an explicit producer override is replayable but cannot qualify as coverage" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "$FIX/test-bin/codex"
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"tier coverage requires non-overrideable native Codex runtime evidence"* ]]
    [[ "$output" == *"foo"* ]]
}

@test "an unsealed capture (seal mode none) is replayable but cannot qualify as coverage" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "" none
    write_ledger "foo | probe-foo | 2026-09-03 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"seal mode 'none'"* ]]
    [[ "$output" == *"foo"* ]]
}

@test "a capture stage with no seal.json is recorded unsealed and does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "" absent
    write_ledger "foo | probe-foo | 2026-09-03 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"seal mode 'none'"* ]]
}

@test "a legacy-unsealed v2 capture contract stays replayable but does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "" legacy
    write_ledger "foo | probe-foo | 2026-08-26 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"legacy-unsealed"* ]]
    [[ "$output" == *"foo"* ]]
}

@test "a sealed capture whose denied roots omit the checkout does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "" omit-checkout
    write_ledger "foo | probe-foo | 2026-09-03 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"
    local repo_real
    repo_real="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$FIX")"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"denied_read_roots omit: $repo_real"* ]]
    [[ "$output" == *"foo"* ]]
}

@test "a sealed capture whose denied roots omit a skill root does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "" omit-skill-root
    write_ledger "foo | probe-foo | 2026-09-03 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"denied_read_roots"* ]]
    [[ "$output" == *"$REAL_HOME/.codex/skills"* ]]
}

@test "a skill root symlinked into the sealed checkout counts through its realpath" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL canonical-skill "" symlinked-skill-root
    write_ledger "foo | probe-foo | 2026-09-03 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 0 ]
    [[ "$output" != *"foo"* ]]
}

@test "a scorecard seal copy is cross-checked against the bound contract seal" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    write_ledger "foo | probe-foo | 2026-09-03 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"
    local seal_record="$PROBES/probe-foo/fixtures-test/seal.json"

    # The verbatim live-harness copy agrees with the contract and counts.
    set_scorecard_seal "$FIX/$BOUND_SCORECARD_REL" "$seal_record"
    run bash "$GATE" --strict
    [ "$status" -eq 0 ]
    [[ "$output" != *"foo"* ]]

    # A copy that claims more denied roots than the bound seal is refused.
    set_scorecard_seal "$FIX/$BOUND_SCORECARD_REL" "$seal_record" \
        'record["denied_read_roots"] = record["denied_read_roots"][1:]'
    run bash "$GATE" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"scorecard seal copy disagrees with the seal bound in the capture contract"* ]]
}

@test "bound injected-prelude evidence is replayable but does not count as skill coverage" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL injected-prelude
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"tier coverage requires treatment_source 'canonical-skill'"* ]]
    [[ "$output" == *"foo"* ]]
}

@test "LEGACY-UNVERIFIED and UNMEASURED rows remain excluded" {
    make_skill foo product
    write_ledger \
        "foo | probe-old | 2026-08-15 | LEGACY-UNVERIFIED | historical" \
        "foo | probe-null | 2026-08-16 | UNMEASURED | no usable arms"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"foo"* ]]
}

@test "a fabricated scorecard classification is rejected by discriminator replay" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    python3 - "$FIX/$BOUND_SCORECARD_REL" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["verdict"] = "INERT"
value["treatment"] = {"present": 0, "usable": 2, "rate": 0.0}
for entry in value["per_rep"]:
    entry["treatment"] = "ABSENT"
path.write_text(json.dumps(value))
PY
    write_ledger "foo | probe-foo | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"do not match discriminator replay"* ]]
}

@test "tampered transcript or manifest evidence does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    printf 'tamper\n' >> "$PROBES/probe-foo/fixtures-test/control-1.txt"
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"transcript digest mismatch"* ]]
}

@test "canonical skill drift invalidates otherwise unchanged bound evidence" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    printf '\nchanged after capture\n' >> "$SKILL_PROBE_SKILLS_DIR/foo/SKILL.md"
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"current canonical skill differs from the self-contained capture"* ]]
}

@test "a scorecard with no fixture manifest does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    rm -f "$PROBES/probe-foo/fixtures-test/fixture-set.json"
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"verified replay requires immutable capture metadata"* ]]
}

@test "scorecard and manifest binding mismatch does not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    python3 - "$FIX/$BOUND_SCORECARD_REL" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["fixture_set"]["binding_sha256"] = "sha256:" + "0" * 64
path.write_text(json.dumps(value))
PY
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"scorecard/manifest fixture binding mismatch"* ]]
}

@test "scorecard producer mismatch does not count" {
    make_skill foo product
    make_bound_result foo probe-foo INERT
    python3 - "$FIX/$BOUND_SCORECARD_REL" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["producer"]["model"] = "relabeled-model"
path.write_text(json.dumps(value))
PY
    write_ledger "foo | probe-foo | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"scorecard/manifest producer mismatch"* ]]
}

@test "scorecard reps mismatch does not count" {
    make_skill foo product
    make_bound_result foo probe-foo INERT
    python3 - "$FIX/$BOUND_SCORECARD_REL" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["reps"] = 1
value["schedule"] = [
    {"position": 1, "rep": 1, "arm": "control"},
    {"position": 2, "rep": 1, "arm": "treatment"},
]
path.write_text(json.dumps(value))
PY
    write_ledger "foo | probe-foo | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"scorecard/manifest reps mismatch"* ]]
}

@test "scorecard and ledger skill/probe/verdict mismatches do not count" {
    make_skill foo product
    make_bound_result foo probe-foo BEHAVIORAL
    write_ledger "foo | other-probe | 2026-08-16 | BEHAVIORAL | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"scorecard/ledger probe mismatch"* ]]

    write_ledger "foo | probe-foo | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"
    run bash "$GATE" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"scorecard/ledger verdict mismatch"* ]]
}

@test "unsafe relative and absolute scorecard paths are rejected" {
    make_skill foo product
    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`../outside.json\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"unsafe repository-relative path"* ]]

    write_ledger "foo | probe-foo | 2026-08-16 | BEHAVIORAL | scorecard: \`/tmp/outside.json\`"
    run bash "$GATE" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"unsafe repository-relative path"* ]]
}

@test "a scorecard path that traverses a symlink is rejected" {
    make_skill foo product
    make_bound_result foo probe-foo INERT
    mv "$FIX/docs/evals/scorecards" "$FIX/docs/evals/real-scorecards"
    ln -s real-scorecards "$FIX/docs/evals/scorecards"
    write_ledger "foo | probe-foo | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"must not traverse a symlink"* ]]
}

@test "a meta-tier v1 row is ignored without noise or denominator impact" {
    make_skill foo product
    make_skill operationalize meta
    make_bound_result operationalize anti-ceremony INERT
    python3 - "$PROBES/anti-ceremony/fixtures-test/fixture-set.json" <<'PY'
import hashlib, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["schema"] = "agentops-skill-probe-fixture-set.v1"
value.pop("canonical_skill")
value.pop("treatment_source")
value["capture_evaluator"].pop("preamble")
value["capture_evaluator"].pop("dispatch_helper")
payload = {key: item for key, item in value.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
value["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
path.write_text(json.dumps(value))
PY
    write_ledger "operationalize | anti-ceremony | 2026-08-16 | INERT | scorecard: \`$BOUND_SCORECARD_REL\`"

    run --separate-stderr bash "$GATE" --json

    [ "$status" -eq 0 ]
    [ -z "$stderr" ]
    [ "$(json_field "$output" gated_total)" = "1" ]
    [ "$(json_field "$output" measured)" = "0" ]
    [ "$(json_field "$output" unmeasured_count)" = "1" ]
}

@test "execution-tier and redirect-only skills remain exempt" {
    make_skill bar execution
    make_redirect legacy-foo
    write_ledger

    run bash "$GATE" --strict

    [ "$status" -eq 0 ]
}

@test "a missing ledger remains advisory and reports gated skills" {
    make_skill foo product
    rm -f "$SKILL_PROBE_LEDGER_FILE"

    run bash "$GATE"

    [ "$status" -eq 0 ]
    [[ "$output" == *"foo"* ]]
}

@test "the compatibility SKILL_PROBE_TIERS_FILE seam still works" {
    make_skill foo product
    unset SKILL_PROBE_LEDGER_FILE
    export SKILL_PROBE_TIERS_FILE="$FIX/compat-ledger.md"
    write_ledger

    run bash "$GATE" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"foo"* ]]
}

@test "the real repository remains advisory with exactly 0/12 current results" {
    unset SKILL_PROBE_SKILLS_DIR SKILL_PROBE_LEDGER_FILE SKILL_PROBE_TIERS_FILE
    unset SKILL_PROBE_EVIDENCE_ROOT SKILL_PROBE_METADATA_TOOL SKILL_PROBE_EXCLUSIONS_FILE

    run --separate-stderr bash "$GATE" --json

    [ "$status" -eq 0 ]
    # 12 product/judgment-tier badges and no declared-denominator exclusion:
    # the `goals` alias was retired on 2026-09-03 (Train 2), so nothing is
    # excluded and the denominator is the badge count. The headline stays
    # 0/12 until a v3 capture-manifest-backed run records a current verdict.
    [ "$(json_field "$output" tier_total)" = "12" ]
    [ "$(json_field "$output" excluded_count)" = "0" ]
    [ "$(json_field "$output" gated_total)" = "12" ]
    [ "$(json_field "$output" measured)" = "0" ]
    [ "$(json_field "$output" unmeasured_count)" = "12" ]
}

@test "the real exclusion list resolves and argues every entry on stdout" {
    unset SKILL_PROBE_SKILLS_DIR SKILL_PROBE_LEDGER_FILE SKILL_PROBE_TIERS_FILE
    unset SKILL_PROBE_EVIDENCE_ROOT SKILL_PROBE_METADATA_TOOL SKILL_PROBE_EXCLUSIONS_FILE

    run --separate-stderr bash "$GATE"

    [ "$status" -eq 0 ]
    # The declared exclusion list is empty since the goals alias was retired
    # (2026-09-03); the gate must run clean with nothing to argue and must not
    # print a stale exclusion line.
    ! grep -v '^#' "$REPO_ROOT/scripts/.skill-probe-denominator-exclusions" | grep -q '[^[:space:]]'
    [[ "$output" != *"denominator excludes"* ]]
}

# --- declared denominator ------------------------------------------------------

@test "an excluded skill leaves the denominator and stops being a finding" {
    make_skill foo product
    make_skill bar product
    write_ledger

    run --separate-stderr bash "$GATE" --json
    [ "$(json_field "$output" gated_total)" = "2" ]

    write_exclusions 'bar  # fixture: an alias that delegates verbatim, so a probe of it measures the other skill'

    run --separate-stderr bash "$GATE" --json
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" tier_total)" = "2" ]
    [ "$(json_field "$output" excluded_count)" = "1" ]
    [ "$(json_field "$output" gated_total)" = "1" ]
    [ "$(json_field "$output" unmeasured_count)" = "1" ]
    # The excluded skill leaves `unmeasured` entirely; it is still NAMED under
    # `excluded`, so the denominator stays legible instead of just smaller.
    [ "$(json_field "$output" unmeasured)" = "['foo']" ]
    [ "$(json_field "$output" excluded)" = "['bar']" ]
}

@test "excluding the only unmeasured skill flips strict mode to pass" {
    make_skill foo product
    write_ledger

    run bash "$GATE" --strict
    [ "$status" -eq 1 ]

    write_exclusions 'foo  # fixture: category error, not missing work'

    run bash "$GATE" --strict
    [ "$status" -eq 0 ]
    [[ "$output" == *"denominator excludes 'foo'"* ]]
    [[ "$output" == *"category error, not missing work"* ]]
}

@test "an exclusion with no argument is rejected (misuse, not a silent shrink)" {
    make_skill foo product
    write_ledger
    write_exclusions 'foo'

    run bash "$GATE"

    [ "$status" -eq 2 ]
    [[ "$output" == *"carries no '# <argument>'"* ]]
}

@test "an exclusion with an empty argument is rejected" {
    make_skill foo product
    write_ledger
    write_exclusions 'foo  #   '

    run bash "$GATE"

    [ "$status" -eq 2 ]
    [[ "$output" == *"EMPTY argument"* ]]
}

@test "a stale exclusion naming a non-gated skill is rejected" {
    make_skill foo product
    write_ledger
    write_exclusions 'nonexistent  # fixture: this skill is not product/judgment tier'

    run bash "$GATE"

    [ "$status" -eq 2 ]
    [[ "$output" == *"stale exclusion"* ]]
}

@test "a duplicate exclusion is rejected" {
    make_skill foo product
    write_ledger
    write_exclusions \
        'foo  # fixture: first argument' \
        'foo  # fixture: second argument'

    run bash "$GATE"

    [ "$status" -eq 2 ]
    [[ "$output" == *"duplicate exclusion"* ]]
}

@test "comment and blank lines in the exclusion list are ignored" {
    make_skill foo product
    write_ledger
    write_exclusions \
        '# a header comment' \
        '' \
        '   ' \
        'foo  # fixture: category error, not missing work'

    run --separate-stderr bash "$GATE" --json

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" excluded_count)" = "1" ]
    [ "$(json_field "$output" gated_total)" = "0" ]
}

@test "a missing exclusion file leaves the full tier set as the denominator" {
    make_skill foo product
    write_ledger
    rm -f "$EXCLUSIONS"

    run --separate-stderr bash "$GATE" --json

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" excluded_count)" = "0" ]
    [ "$(json_field "$output" gated_total)" = "1" ]
}
