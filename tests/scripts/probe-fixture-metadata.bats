#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Contract tests for the capture-contract `seal` block written by
# scripts/lib/probe-fixture-metadata.py. The 2026-08-28 contamination was
# control-arm reps reading SKILL.md off the checkout; the seal is the
# prevention, and a coverage row has to PROVE the run was sealed. The harness
# writes an `agentops-skill-probe-seal.v1` record (`seal.json`) into the capture
# stage before `snapshot`; these tests place that record by hand and exercise
# the metadata tool directly.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HARNESS="$REPO_ROOT/scripts/probe-skill.sh"
    META_TOOL="$REPO_ROOT/scripts/lib/probe-fixture-metadata.py"
    PREAMBLE="$REPO_ROOT/scripts/lib/preamble.sh"
    DISPATCH_HELPER="$REPO_ROOT/scripts/lib/codex-exec.sh"
    FIX="$BATS_TEST_TMPDIR/repo"
    FIX_REAL="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$BATS_TEST_TMPDIR")/repo"
    # The fixture home lives OUTSIDE the fixture checkout so a checkout deny
    # cannot cover a skill root by ancestry.
    REAL_HOME="${FIX_REAL%/repo}/home"
    PROBES="$FIX/evals/skill-probes"
    SKILLS="$FIX/skills"
    PROBE_DIR="$PROBES/demo"
    mkdir -p "$PROBE_DIR" "$SKILLS/demo-skill" "$FIX/test-bin" "$BATS_TEST_TMPDIR/home"

    cat > "$PROBE_DIR/probe.json" <<'JSON'
{"id":"demo","skill":"demo-skill","reps":2,"discriminator":"discriminator.sh","treatment_source":"canonical-skill"}
JSON
    cat > "$SKILLS/demo-skill/SKILL.md" <<'MD'
---
name: demo-skill
description: Fixture skill.
---
# Demo skill

CANONICAL_ACTION
MD
    printf 'QUESTION\n' > "$PROBE_DIR/question.md"
    cat > "$PROBE_DIR/discriminator.sh" <<'SH'
#!/usr/bin/env bash
if grep -q '^INFRA$' "$1"; then exit 2; fi
grep -q '^ACTION$' "$1"
SH
    chmod +x "$PROBE_DIR/discriminator.sh"

    # Native-path runtime identity stub: coverage eligibility needs a
    # PATH-resolved `codex`, never an explicit override.
    cat > "$FIX/test-bin/codex" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
    printf 'codex-cli seal-test\n'
    exit 0
fi
printf 'runtime identity stub is not a live producer\n' >&2
exit 70
SH
    chmod +x "$FIX/test-bin/codex"
    export PATH="$FIX/test-bin:$PATH"
}

# write_seal PATH MODE [DENIED_ROOT...] — write the harness-shaped seal record
# (agentops-skill-probe-seal.v1) with REAL_HOME as the recorded real home.
write_seal() {
    local path="$1" mode="$2"
    shift 2
    python3 - "$path" "$mode" "$REAL_HOME" "$@" <<'PY'
import hashlib, json, pathlib, sys

path = pathlib.Path(sys.argv[1])
mode = sys.argv[2]
real_home = sys.argv[3]
denied = sys.argv[4:]
sealed = mode == "seatbelt"
profile = None
if sealed:
    profile = "(version 1)\n(allow default)\n(deny file-read*\n" + "".join(
        f'  (subpath "{root}")\n' for root in denied
    ) + ")"
record = {
    "schema": "agentops-skill-probe-seal.v1",
    "seal_mode": mode,
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
path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
PY
}

full_denied_roots() {
    printf '%s\n' "$FIX_REAL" "$REAL_HOME/.agents" "$REAL_HOME/.claude/skills" \
        "$REAL_HOME/.gemini/skills" "$REAL_HOME/.codex/skills"
}

snapshot() {
    local directory="$1"
    shift
    python3 "$META_TOOL" snapshot \
        --fixture-dir "$directory" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo \
        --requested-model fixture-model \
        --requested-effort low "$@"
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
    {"type": "agentops.probe-input.v1", "arm": arm, "rep": rep, "position": position, "prompt": prompt},
    {"type": "thread.started", "thread_id": f"seal-{directory.name}-{name}"},
    {"type": "turn.started"},
    {"type": "item.completed", "item": {"id": f"item-{name}", "type": "agent_message", "text": body}},
    {"type": "turn.completed", "usage": {"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
]
(directory / f"{name}.txt").write_text(
    "".join(json.dumps(event, sort_keys=True, separators=(",", ":")) + "\n" for event in events)
)
PY
}

create() {
    local directory="$1"
    python3 "$META_TOOL" create \
        --fixture-dir "$directory" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --harness "$HARNESS" \
        --preamble "$PREAMBLE" \
        --dispatch-helper "$DISPATCH_HELPER" \
        --probe demo \
        --reps 2 \
        --requested-model fixture-model \
        --requested-effort low
}

verify() {
    local directory="$1"
    python3 "$META_TOOL" verify \
        --fixture-dir "$directory" --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" --probe demo
}

contract_field() {
    python3 - "$1" "$2" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
    value = value[int(part)] if isinstance(value, list) else value[part]
if value is None:
    print("null")
elif isinstance(value, bool):
    print("true" if value else "false")
elif isinstance(value, list):
    print(json.dumps(value))
else:
    print(value)
PY
}

json_field() {
    python3 -c '
import json, sys
value = json.loads(sys.argv[1])
for part in sys.argv[2].split("."):
    value = value[int(part)] if isinstance(value, list) else value[part]
if value is None:
    print("null")
elif isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
' "$1" "$2"
}

# rebind_contract PATH PYTHON_SNIPPET — mutate a contract in place and recompute
# its binding so only the mutated semantics, not the binding, are under test.
rebind_contract() {
    python3 - "$1" "$2" <<'PY'
import hashlib, json, sys
path = sys.argv[1]
contract = json.load(open(path))
exec(sys.argv[2], {"contract": contract})
payload = {key: value for key, value in contract.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
contract["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
open(path, "w").write(json.dumps(contract, indent=2, sort_keys=True) + "\n")
PY
}

# edit_seal PATH PYTHON_SNIPPET — mutate a seal record in place (no rebind).
edit_seal() {
    python3 - "$1" "$2" <<'PY'
import json, sys
path = sys.argv[1]
record = json.load(open(path))
exec(sys.argv[2], {"record": record})
open(path, "w").write(json.dumps(record, indent=2, sort_keys=True) + "\n")
PY
}

@test "snapshot without seal.json records seal mode none and an ineligible producer" {
    local directory="$PROBE_DIR/fixtures-none"
    mkdir -p "$directory"

    run snapshot "$directory"

    [ "$status" -eq 0 ]
    local contract="$directory/capture-contract.json"
    [ "$(contract_field "$contract" schema)" = "agentops-skill-probe-capture.v3" ]
    [ "$(contract_field "$contract" seal.mode)" = "none" ]
    [ "$(contract_field "$contract" seal.profile_sha256)" = "null" ]
    [ "$(contract_field "$contract" seal.denied_read_roots)" = "[]" ]
    [ "$(contract_field "$contract" producer_request.identity.coverage_eligible)" = "false" ]
    [ "$(contract_field "$contract" producer_request.identity.source)" = "native-codex-path" ]
}

@test "snapshot with a seatbelt seal.json binds the seal and keeps the producer eligible" {
    local directory="$PROBE_DIR/fixtures-sealed"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"

    run snapshot "$directory"

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" seal.mode)" = "seatbelt" ]
    local contract="$directory/capture-contract.json"
    [ "$(contract_field "$contract" seal.mode)" = "seatbelt" ]
    [ "$(contract_field "$contract" seal.original_home)" = "$REAL_HOME" ]
    [ "$(contract_field "$contract" seal.repository_root)" = "$FIX_REAL" ]
    [ "$(contract_field "$contract" seal.denied_read_roots)" = "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "${roots[@]}")" ]
    [ "$(contract_field "$contract" seal.profile_sha256)" = "$(contract_field "$directory/seal.json" profile_sha256)" ]
    [ "$(contract_field "$contract" producer_request.identity.coverage_eligible)" = "true" ]
}

@test "snapshot reads an explicit --seal-file when the record lives outside the stage" {
    local directory="$PROBE_DIR/fixtures-sealfile" workspace="$BATS_TEST_TMPDIR/ws"
    mkdir -p "$directory" "$workspace"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$workspace/seal.json" seatbelt "${roots[@]}"

    run snapshot "$directory" --seal-file "$workspace/seal.json"

    [ "$status" -eq 0 ]
    [ "$(contract_field "$directory/capture-contract.json" seal.mode)" = "seatbelt" ]
    [ "$(contract_field "$directory/capture-contract.json" producer_request.identity.coverage_eligible)" = "true" ]

    run snapshot "$PROBE_DIR/fixtures-missing" --seal-file "$workspace/absent.json"
    [ "$status" -eq 2 ]
}

@test "a seal record that contradicts itself is refused before any dispatch" {
    local directory="$PROBE_DIR/fixtures-selfcheck"
    mkdir -p "$directory"
    write_seal "$directory/seal.json" seatbelt "$FIX_REAL"

    edit_seal "$directory/seal.json" 'record["coverage_eligible"] = False'
    run snapshot "$directory"
    [ "$status" -eq 2 ]
    [[ "$output" == *"coverage_eligible disagrees with its seal_mode"* ]]
    [ ! -e "$directory/capture-contract.json" ]

    write_seal "$directory/seal.json" seatbelt "$FIX_REAL"
    edit_seal "$directory/seal.json" 'record["profile_sha256"] = "sha256:" + "0" * 64'
    run snapshot "$directory"
    [ "$status" -eq 2 ]
    [[ "$output" == *"profile_sha256 does not match"* ]]

    write_seal "$directory/seal.json" seatbelt "$FIX_REAL"
    edit_seal "$directory/seal.json" 'record["profile"] = None'
    run snapshot "$directory"
    [ "$status" -eq 2 ]
    [[ "$output" == *"seal.json profile"* ]]
    [ ! -e "$directory/capture-contract.json" ]
}

@test "a v3 capture contract without a seal block is rejected" {
    local directory="$PROBE_DIR/fixtures-noseal"
    mkdir -p "$directory"
    snapshot "$directory" >/dev/null
    rebind_contract "$directory/capture-contract.json" 'contract.pop("seal")'

    run python3 "$META_TOOL" capture-file --fixture-dir "$directory" --probe demo --name question.md

    [ "$status" -eq 2 ]
    [[ "$output" == *"capture contract must contain exactly"* ]]
    [[ "$output" == *"seal"* ]]
}

@test "a v3 contract cannot claim eligibility without a seatbelt seal" {
    local directory="$PROBE_DIR/fixtures-claim"
    mkdir -p "$directory"
    snapshot "$directory" >/dev/null
    rebind_contract "$directory/capture-contract.json" \
        'contract["producer_request"]["identity"]["coverage_eligible"] = True'

    run python3 "$META_TOOL" capture-file --fixture-dir "$directory" --probe demo --name question.md

    [ "$status" -eq 2 ]
    [[ "$output" == *"producer coverage_eligible is inconsistent"* ]]
}

@test "a legacy v2 capture contract is accepted as legacy-unsealed and stays replayable" {
    local directory="$PROBE_DIR/fixtures-legacy"
    mkdir -p "$directory"
    snapshot "$directory" >/dev/null
    rebind_contract "$directory/capture-contract.json" '
contract.pop("seal")
contract["schema"] = "agentops-skill-probe-capture.v2"
contract["producer_request"]["identity"]["coverage_eligible"] = True
'
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" ABSENT
        write_transcript "$directory" "treatment-$rep" ACTION
    done

    run create "$directory"
    [ "$status" -eq 0 ]

    run verify "$directory"
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" seal.mode)" = "legacy-unsealed" ]
}

@test "create binds the seal.json sidecar and refuses one swapped after snapshot" {
    local directory="$PROBE_DIR/fixtures-swap"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" ABSENT
        write_transcript "$directory" "treatment-$rep" ACTION
    done
    # Swap the seal after the contract was bound: drop the checkout root.
    write_seal "$directory/seal.json" seatbelt "$REAL_HOME/.agents"

    run create "$directory"

    [ "$status" -eq 2 ]
    [[ "$output" == *"seal.json"* ]]
    [ ! -e "$directory/fixture-set.json" ]
}

@test "a sealed fixture set verifies with or without its seal.json sidecar" {
    local directory="$PROBE_DIR/fixtures-ok"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" ABSENT
        write_transcript "$directory" "treatment-$rep" ACTION
    done
    create "$directory" >/dev/null

    run verify "$directory"
    [ "$status" -eq 0 ]
    [ -e "$directory/seal.json" ]
    [ "$(json_field "$output" seal.mode)" = "seatbelt" ]
    [ "$(json_field "$output" producer.identity.coverage_eligible)" = "true" ]

    # The relocated-sidecar shape (record kept in the workspace) still verifies:
    # the contract, not the sidecar, is the bound seal.
    mv "$directory/seal.json" "$BATS_TEST_TMPDIR/relocated-seal.json"
    run verify "$directory"
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" seal.mode)" = "seatbelt" ]
}
