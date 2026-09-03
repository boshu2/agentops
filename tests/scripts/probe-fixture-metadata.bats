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
# (agentops-skill-probe-seal.v1) with REAL_HOME as the recorded real home. The
# run-directory paths are the fictional shape the harness builds: one run root
# under the (denied) real temp root holding home/, ws/, tmp/ and dispatch/.
write_seal() {
    local path="$1" mode="$2"
    shift 2
    python3 - "$path" "$mode" "$REAL_HOME" "$FIX_REAL" "$@" <<'SEALPY'
import hashlib, json, pathlib, sys

path = pathlib.Path(sys.argv[1])
mode = sys.argv[2]
real_home = sys.argv[3]
git_common_root = sys.argv[4]
denied = sys.argv[5:]
sealed = mode == "seatbelt"
run_root = "/private/tmp/probe-run.aaaaaa"
dispatch = run_root + "/dispatch"
profile = None
if sealed:
    profile = "(version 1)\n(allow default)\n(deny file-read*\n" + "".join(
        f'  (subpath "{root}")\n' for root in denied
    ) + ")"
profile_sha = ("sha256:" + hashlib.sha256(profile.encode()).hexdigest()) if sealed else None
record = {
    "schema": "agentops-skill-probe-seal.v1",
    "seal_mode": mode,
    "coverage_eligible": sealed,
    "mechanism": "sandbox-exec" if sealed else None,
    "sandbox_exec": "/usr/bin/sandbox-exec" if sealed else None,
    "platform": "Darwin",
    "wrap": ["sandbox-exec", "-p", profile_sha] if sealed else [],
    "profile": profile,
    "profile_file": run_root + "/home/seal.sb" if sealed else None,
    "profile_sha256": profile_sha,
    "denied_read_roots": denied,
    "denied_read_data_roots": [dispatch] if sealed else [],
    "denied_link_roots": (list(denied) + [dispatch]) if sealed else [],
    "writable_roots": (
        [run_root + "/home", run_root + "/ws", run_root + "/tmp", "/dev"] if sealed else []
    ),
    "allowed_read_paths": [],
    "rep_env": {
        "HOME": run_root + "/home",
        "CODEX_HOME": run_root + "/home/.codex",
        "TMPDIR": run_root + "/tmp",
    },
    "real_home": real_home,
    "real_codex_home": real_home + "/.codex",
    "real_tmpdir": "/private/tmp" if sealed else None,
    "git_common_root": git_common_root if sealed else None,
    "run_root": run_root if sealed else None,
    "workspace_root": run_root + "/ws" if sealed else None,
    "dispatch_root": dispatch if sealed else None,
    "config_sanitized": ["model", "model_reasoning_effort"] if sealed else None,
    "auth_copied": bool(sealed),
}
path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
SEALPY
}

# The roots a hardened seal must deny: the checkout (also the shared git root in
# these fixtures), the operator home, the four skill roots under it, and the
# real temp root that holds every other run's debris.
full_denied_roots() {
    printf '%s\n' "$FIX_REAL" "$REAL_HOME" "$REAL_HOME/.agents" \
        "$REAL_HOME/.claude/skills" "$REAL_HOME/.gemini/skills" \
        "$REAL_HOME/.codex/skills" /private/tmp
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
event = {
    "type": "agentops.probe-input.v1", "arm": arm, "rep": rep,
    "position": position, "prompt": prompt,
}
seal = contract.get("seal") or {}
if seal.get("mode") == "seatbelt" and seal.get("workspace_root"):
    event["workspace"] = seal["workspace_root"]
    event["workspace_reset"] = True
events = [
    event,
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

# write_command_transcript DIR NAME BODY COMMAND OUTPUT — like write_transcript,
# but the rep first runs one successful (exit 0) command_execution whose
# command string is COMMAND and whose aggregated_output is OUTPUT.
write_command_transcript() {
    local directory="$1" name="$2" body="$3" command="$4" output="$5"
    python3 - "$directory" "$name" "$body" "$command" "$output" <<'PY'
import json, pathlib, sys

directory = pathlib.Path(sys.argv[1])
name = sys.argv[2]
body, command, output = sys.argv[3:6]
arm, rep_text = name.rsplit("-", 1)
rep = int(rep_text)
contract = json.loads((directory / "capture-contract.json").read_text())
prompt = contract["prompts"][0 if arm == "control" else 1]
position = next(
    item["position"]
    for item in contract["schedule"]
    if item["arm"] == arm and item["rep"] == rep
)
started = {
    "id": f"cmd-{name}", "type": "command_execution", "command": command,
    "aggregated_output": "", "exit_code": None, "status": "in_progress",
}
completed = dict(started, aggregated_output=output, exit_code=0, status="completed")
event = {
    "type": "agentops.probe-input.v1", "arm": arm, "rep": rep,
    "position": position, "prompt": prompt,
}
seal = contract.get("seal") or {}
if seal.get("mode") == "seatbelt" and seal.get("workspace_root"):
    event["workspace"] = seal["workspace_root"]
    event["workspace_reset"] = True
events = [
    event,
    {"type": "thread.started", "thread_id": f"sibling-{directory.name}-{name}"},
    {"type": "turn.started"},
    {"type": "item.started", "item": started},
    {"type": "item.completed", "item": completed},
    {"type": "item.completed", "item": {"id": f"item-{name}", "type": "agent_message", "text": body}},
    {"type": "turn.completed", "usage": {"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
]
(directory / f"{name}.txt").write_text(
    "".join(json.dumps(event, sort_keys=True, separators=(",", ":")) + "\n" for event in events)
)
PY
}

score() {
    local directory="$1"
    python3 "$META_TOOL" score \
        --fixture-dir "$directory" --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" --probe demo
}

@test "a rep that reads or lists a sibling prompt or harness artifact is DEGRADED (sibling-prompt-read)" {
    # 2026-09-03 xhigh control-2: `rg --files | head -200` listed
    # treatment-1.prompt in the shared workspace, then `sed -n '1,220p'
    # treatment-1.prompt` read the canonical SKILL.md bytes out of it. Neither
    # command names SKILL.md, so the skill-read trap let it score.
    local directory="$PROBE_DIR/fixtures-sibling"
    mkdir -p "$directory"
    snapshot "$directory" >/dev/null
    write_command_transcript "$directory" control-1 ACTION \
        "/bin/zsh -lc \"sed -n '1,220p' treatment-1.prompt; sed -n '1,220p' control-1.prompt\"" \
        "---\nname: demo-skill\nCANONICAL_ACTION\n"
    write_command_transcript "$directory" control-2 ACTION \
        "/bin/zsh -lc \"rg --files | head -200\"" \
        "control-2.codex.stderr\ntreatment-1.prompt\ncontrol-1.codex.jsonl\n"
    write_command_transcript "$directory" treatment-1 ACTION \
        "/bin/zsh -lc 'cat capture-contract.json seal.json'" \
        "{}"
    write_command_transcript "$directory" treatment-2 ACTION \
        "/bin/zsh -lc 'ls -la'" \
        "total 0\nnotes.md\n"
    create "$directory" >/dev/null

    run --separate-stderr score "$directory"
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" per_rep.0.control)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.1.control)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.0.treatment)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.1.treatment)" = "PRESENT" ]
    [ "$(json_field "$output" verdict)" = "UNMEASURED" ]
    [[ "$stderr" == *"sibling-prompt-read"* ]]
    [[ "$stderr" == *"treatment-1.prompt"* ]]
    [[ "$stderr" == *"capture-contract.json"* ]]
    [[ "$stderr" != *"skill-read-contamination"* ]]
}

@test "a successful command that only names SKILL.md still trips the skill-read trap, not the sibling trap" {
    local directory="$PROBE_DIR/fixtures-skillread"
    mkdir -p "$directory"
    snapshot "$directory" >/dev/null
    write_transcript "$directory" control-1 ABSENT
    write_command_transcript "$directory" control-2 ACTION \
        "/bin/zsh -lc 'cat /somewhere/skills/demo-skill/SKILL.md'" "CANONICAL_ACTION"
    write_transcript "$directory" treatment-1 ACTION
    write_transcript "$directory" treatment-2 ACTION
    create "$directory" >/dev/null

    run --separate-stderr score "$directory"
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" per_rep.1.control)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.0.control)" = "ABSENT" ]
    [[ "$stderr" == *"skill-read-contamination"* ]]
    [[ "$stderr" != *"sibling-prompt-read"* ]]
}

# coverage_reason CONTRACT -> the reason the bound seal cannot be tier coverage,
# or ELIGIBLE. Drives the exact function verify-scorecard calls.
coverage_reason() {
    python3 - "$META_TOOL" "$1" <<'REASONPY'
import importlib.util, json, sys

spec = importlib.util.spec_from_file_location("probe_fixture_metadata", sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
contract = json.load(open(sys.argv[2], encoding="utf-8"))
seal = module.validate_seal(contract["seal"])
print(module.seal_coverage_failure(seal) or "ELIGIBLE")
REASONPY
}

@test "the seal block is coverage only when every bound field proves the sandbox" {
    local directory="$PROBE_DIR/fixtures-eligibility"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null

    run coverage_reason "$contract"
    [ "$status" -eq 0 ]
    [ "$output" = "ELIGIBLE" ]

    # The Codex CONTRACT-SEAL-004 record: seatbelt claimed on Linux with no
    # mechanism and a two-line profile. The pre-repair block bound only mode,
    # roots, digest and home, so it counted.
    rebind_contract "$contract" '
contract["seal"]["platform"] = "Linux"
contract["seal"]["mechanism"] = None
'
    run coverage_reason "$contract"
    [ "$status" -eq 0 ]
    [[ "$output" == *"platform 'Linux'"* ]]

    rebind_contract "$contract" 'contract["seal"]["platform"] = "Darwin"'
    run coverage_reason "$contract"
    [[ "$output" == *"requires mechanism 'sandbox-exec'"* ]]

    rebind_contract "$contract" 'contract["seal"]["mechanism"] = "sandbox-exec"'
    rebind_contract "$contract" 'contract["seal"]["wrap"] = ["sandbox-exec", "-p", "(version 1)"]'
    run coverage_reason "$contract"
    [[ "$output" == *"dispatch wrap to be the bound profile"* ]]
}

@test "a seatbelt seal that omits a required root or a hardening step is not coverage" {
    local directory="$PROBE_DIR/fixtures-partial"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null

    rebind_contract "$contract" '
contract["seal"]["denied_read_roots"] = [
    root for root in contract["seal"]["denied_read_roots"] if root != "/private/tmp"
]
'
    run coverage_reason "$contract"
    [[ "$output" == *"denied_read_roots omit"* ]]
    [[ "$output" == *"/private/tmp"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    rm "$contract"
    snapshot "$directory" >/dev/null
    rebind_contract "$contract" 'contract["seal"]["config_sanitized"] = None'
    run coverage_reason "$contract"
    [[ "$output" == *"producer config to be sanitized"* ]]

    rebind_contract "$contract" '
contract["seal"]["config_sanitized"] = ["model"]
contract["seal"]["auth_copied"] = False
'
    run coverage_reason "$contract"
    [[ "$output" == *"auth.json to be COPIED"* ]]

    rebind_contract "$contract" '
contract["seal"]["auth_copied"] = True
contract["seal"]["denied_link_roots"] = []
'
    run coverage_reason "$contract"
    [[ "$output" == *"link and clone to be denied"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["denied_link_roots"] = seal["denied_read_roots"] + [seal["dispatch_root"]]
seal["rep_env"]["HOME"] = seal["original_home"] + "/scratch"
'
    run coverage_reason "$contract"
    [[ "$output" == *"HOME"* ]]
}

@test "the 2026-09-03 first-shape seal block still replays but is never coverage" {
    local directory="$PROBE_DIR/fixtures-firstshape"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    rm "$directory/seal.json"
    rebind_contract "$contract" '
keep = {
    "mode", "denied_read_roots", "writable_roots", "profile_sha256",
    "original_home", "repository_root",
}
contract["seal"] = {k: v for k, v in contract["seal"].items() if k in keep}
'

    # It still loads: an older capture stays readable rather than failing shut.
    run python3 "$META_TOOL" capture-file --fixture-dir "$directory" --probe demo --name question.md
    [ "$status" -eq 0 ]

    run coverage_reason "$contract"
    [ "$status" -eq 0 ]
    [[ "$output" == *"hardened seal block"* ]]
}

@test "a sealed rep must bind the workspace it ran in and its reset" {
    local directory="$PROBE_DIR/fixtures-noworkspace"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" ABSENT
        write_transcript "$directory" "treatment-$rep" ACTION
    done
    # Strip the workspace binding from one rep BEFORE the manifest binds the
    # bytes, so the digest check cannot mask the semantic one: a sealed capture
    # cannot accept a transcript that will not say where it ran.
    python3 - "$directory/control-1.txt" <<'STRIPPY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
lines = path.read_text(encoding="utf-8").splitlines()
event = json.loads(lines[0])
event.pop("workspace", None)
event.pop("workspace_reset", None)
lines[0] = json.dumps(event, sort_keys=True, separators=(",", ":"))
path.write_text("\n".join(lines) + "\n", encoding="utf-8")
STRIPPY
    run verify "$directory"
    [ "$status" -eq 2 ]
    [[ "$output" == *"workspace"* ]]
}

@test "a sealed rep whose workspace is outside the seal's writable roots is refused" {
    local directory="$PROBE_DIR/fixtures-badworkspace"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" ABSENT
        write_transcript "$directory" "treatment-$rep" ACTION
    done
    python3 - "$directory/treatment-2.txt" <<'MOVEPY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
lines = path.read_text(encoding="utf-8").splitlines()
event = json.loads(lines[0])
event["workspace"] = "/private/tmp/somewhere-else"
lines[0] = json.dumps(event, sort_keys=True, separators=(",", ":"))
path.write_text("\n".join(lines) + "\n", encoding="utf-8")
MOVEPY
    run create "$directory"
    [ "$status" -eq 2 ]
    [[ "$output" == *"writable roots"* ]]
    [ ! -e "$directory/fixture-set.json" ]
}

@test "the sibling trap names the run directory's harness-private children" {
    local directory="$PROBE_DIR/fixtures-runnames"
    mkdir -p "$directory"
    snapshot "$directory" >/dev/null
    write_command_transcript "$directory" control-1 ACTION \
        "/bin/zsh -lc 'cat /private/tmp/probe-run.Ab12Cd/dispatch/treatment-1.prompt'" ""
    write_command_transcript "$directory" control-2 ACTION \
        "/bin/zsh -lc 'ls /private/tmp/probe-ws.Ab12Cd'" ""
    write_command_transcript "$directory" treatment-1 ACTION \
        "/bin/zsh -lc 'ls /private/tmp/probe-run.Ab12Cd/home/.codex'" ""
    # Its OWN workspace by absolute path is ordinary work, not a sibling read.
    write_command_transcript "$directory" treatment-2 ACTION \
        "/bin/zsh -lc 'ls /private/tmp/probe-run.Ab12Cd/ws'" "notes.md"
    create "$directory" >/dev/null

    run --separate-stderr score "$directory"
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" per_rep.0.control)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.1.control)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.0.treatment)" = "DEGRADED" ]
    [ "$(json_field "$output" per_rep.1.treatment)" = "PRESENT" ]
    [[ "$stderr" == *"sibling-prompt-read"* ]]
}

@test "the sanitized producer config keeps top-level scalars and drops every table" {
    local source="$BATS_TEST_TMPDIR/config.toml"
    local target="$BATS_TEST_TMPDIR/sanitized.toml"
    cat > "$source" <<'TOML'
model = "gpt-5.6-luna"
model_reasoning_effort = "low"
web_search = true
service_tier = 3
notify = ["/Users/operator/.codex/hook.app/Contents/MacOS/hook", "turn-ended"]

[mcp_servers.smart-connections]
command = "npx"
args = ["-y", "smart-connections-mcp"]

[mcp_servers.smart-connections.env]
OBSIDIAN_VAULT = "/Users/operator/vault"

[projects."/Users/operator/dev/agentops"]
trust_level = "trusted"
TOML

    run python3 "$META_TOOL" sanitize-codex-config --source "$source" --target "$target"

    [ "$status" -eq 0 ]
    [ "$output" = '["model","model_reasoning_effort","service_tier","web_search"]' ]
    grep -q '^model = "gpt-5.6-luna"$' "$target"
    grep -q '^model_reasoning_effort = "low"$' "$target"
    grep -q '^web_search = true$' "$target"
    grep -q '^service_tier = 3$' "$target"
    ! grep -q 'mcp_servers' "$target"
    ! grep -q 'OBSIDIAN_VAULT' "$target"
    ! grep -q 'projects' "$target"
    ! grep -q 'npx' "$target"
    # `notify` is a scalar, but it names an operator program codex runs at the
    # end of every turn; a sealed rep must not inherit it.
    ! grep -q 'notify' "$target"
}
