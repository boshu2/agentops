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
# THROUGH the tool's own renderer, so the fixture profile and the fixture block
# are related exactly the way a real capture relates them. The run-directory
# paths are the fictional shape the harness builds: one run root under the
# (denied) real temp root holding home/, ws/, tmp/ and dispatch/.
write_seal() {
    local path="$1" mode="$2"
    shift 2
    # The payload lives OUTSIDE the stage: a snapshot refuses a stage that holds
    # anything but its seal record.
    local payload
    payload="$(mktemp "$BATS_TEST_TMPDIR/seal-payload.XXXXXX")"
    # The launcher exception is checked against the filesystem now: the chain
    # has to be a real file whose digest matches launcher_sha256, and it has to
    # sit under a denied root for the allowance to be re-derivable.
    mkdir -p "$REAL_HOME/bin"
    printf '#!/bin/sh\nexit 0\n' > "$REAL_HOME/bin/codex"
    local launcher_sha
    launcher_sha="sha256:$(shasum -a 256 "$REAL_HOME/bin/codex" | cut -d" " -f1)"
    # The config text is pinned to the generator's output for the bound effort,
    # so the fixture takes it from the generator rather than restating it.
    local config_file
    config_file="$(mktemp "$BATS_TEST_TMPDIR/probe-config.XXXXXX")"
    python3 "$META_TOOL" probe-config --effort low --target "$config_file" >/dev/null
    python3 - "$payload" "$mode" "$REAL_HOME" "$FIX_REAL" "$launcher_sha" "$config_file" "$@" <<'SEALPY'
import json, pathlib, sys

payload_path = pathlib.Path(sys.argv[1])
mode = sys.argv[2]
real_home = sys.argv[3]
git_common_root = sys.argv[4]
launcher_sha = sys.argv[5]
config_text = pathlib.Path(sys.argv[6]).read_text()
denied = sys.argv[7:]
sealed = mode == "seatbelt"
run_root = "/private/tmp/probe-run.aaaaaa"
dispatch = run_root + "/dispatch"
launcher = real_home + "/bin/codex"
payload = {
    "seal_mode": mode,
    "sandbox_exec": "/usr/bin/sandbox-exec" if sealed else "",
    "platform": "Darwin",
    "profile_file": run_root + "/home/seal.sb" if sealed else "",
    "denied_read_roots": denied,
    "denied_read_data_roots": [dispatch] if sealed else [],
    "denied_link_roots": (list(denied) + [dispatch]) if sealed else [],
    "writable_roots": (
        [run_root + "/home", run_root + "/ws", run_root + "/tmp"] if sealed else []
    ),
    "dev_write_paths": (
        ["/dev/null", "/dev/zero", "/dev/dtracehelper", "/dev/tty"] if sealed else []
    ),
    # The launcher lives under the denied temp root, so the seal must re-allow
    # exactly it and coverage must be able to re-derive that from the chain.
    "allowed_read_paths": [launcher] if sealed else [],
    # The chain is recorded as structure: this fixture is a one-entry chain, the
    # binary itself, so its head is also its only file entry.
    "launcher_chain": (
        [{"path": launcher, "kind": "file", "sha256": launcher_sha}] if sealed else []
    ),
    "launcher_invoked": launcher if sealed else "",
    "launcher_sha256": launcher_sha if sealed else "",
    "timeout_bin": "/opt/homebrew/bin/timeout" if sealed else "",
    "timeout_seconds": 240 if sealed else 0,
    "env_allowlist": ["PATH", "HOME", "CODEX_HOME", "TMPDIR"],
    "rep_env": {
        "HOME": run_root + "/home",
        "CODEX_HOME": run_root + "/home/.codex",
        "TMPDIR": run_root + "/tmp",
    },
    "real_home": real_home,
    "real_codex_home": real_home + "/.codex",
    "real_tmpdir": "/private/tmp" if sealed else "",
    "cache_root": "/private/tmp/fixture-cache" if sealed else "",
    "git_common_root": git_common_root if sealed else "",
    "run_root": run_root if sealed else "",
    "workspace_root": run_root + "/ws" if sealed else "",
    "dispatch_root": dispatch if sealed else "",
    "network": {
        "mode": "proxy-allowlist" if sealed else "open",
        "hosts": ["chatgpt.com"] if sealed else [],
        "ports": [443] if sealed else [],
        "proxy": "127.0.0.1:54321" if sealed else None,
        "unix_sockets": [],
    },
    "config_sanitized": ["web_search"] if sealed else None,
    "config_sha256": (
        "sha256:d1a0e0b3b9a4bd0ec2f5f6f9b0f2f8e4b6b5f9e1b1c0a9d8e7f6a5b4c3d2e1f0"
        if sealed
        else ""
    ),
    "config_text": config_text if sealed else "",
    "auth_copied": bool(sealed),
}
if sealed:
    import hashlib

    payload["config_sha256"] = (
        "sha256:" + hashlib.sha256(payload["config_text"].encode()).hexdigest()
    )
payload_path.write_text(json.dumps(payload, sort_keys=True))
SEALPY
    rm -f "$path"
    python3 "$META_TOOL" seal-record --payload "$payload" --output "$path" >/dev/null
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

# write_network_log DIR REPS — the proxy log a sealed fixture set publishes.
# One accepted CONNECT per rep, as an attempt paired with its decision, which is
# the shape the verifier now requires.
write_network_log() {
    python3 - "$1" "${2:-2}" <<'NETLOGPY'
import json, pathlib, sys

directory = pathlib.Path(sys.argv[1])
reps = int(sys.argv[2])
contract = directory / "capture-contract.json"
# Only a seatbelt capture publishes a log: the transcripts of any other mode
# bind no egress, and a log with nothing to check against is refused.
if not contract.is_file():
    raise SystemExit(0)
seal = json.loads(contract.read_text()).get("seal") or {}
if seal.get("mode") != "seatbelt" or "network" not in seal:
    raise SystemExit(0)
lines = []
for rep in range(1, reps + 1):
    for arm in ("control", "treatment"):
        key = f"{arm}-{rep}"
        for decision in ("attempt", "allowed"):
            lines.append(
                json.dumps(
                    {
                        "decision": decision,
                        "host": "chatgpt.com",
                        "port": 443,
                        "rep": key,
                        "ts": "2026-09-03T00:00:00Z",
                    },
                    sort_keys=True,
                )
            )
(directory / "network.log").write_text("\n".join(lines) + "\n")
NETLOGPY
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
    if "network" in seal:
        import hashlib

        log_path = directory / "network.log"
        own = [
            line
            for line in log_path.read_text().splitlines()
            if line.strip() and json.loads(line).get("rep") == name
        ]
        blob = ("\n".join(own) + "\n").encode() if own else b""
        event["network_egress"] = {
            "allowed": sum(
                1 for line in own if json.loads(line)["decision"] == "allowed"
            ),
            "refused": 0,
            "log_sha256": "sha256:" + hashlib.sha256(blob).hexdigest(),
        }
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

# rebind_contract_profile PATH PYTHON_SNIPPET — mutate a contract, then
# RE-RENDER its profile and re-stamp the digest and the wrap before rebinding.
# Mutating `allowed_read_paths` changes the profile the block renders to, so
# without this the profile-digest check fires first and the rule under test is
# never reached.
rebind_contract_profile() {
    python3 - "$META_TOOL" "$1" "$2" <<'REBINDPY'
import hashlib, importlib.util, json, sys

spec = importlib.util.spec_from_file_location("probe_fixture_metadata", sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
path = sys.argv[2]
contract = json.load(open(path, encoding="utf-8"))
exec(sys.argv[3], {"contract": contract})
seal = contract["seal"]
profile = module.render_seal_profile(module.validate_seal(seal))
digest = "sha256:" + hashlib.sha256(profile.encode()).hexdigest()
seal["profile_sha256"] = digest
seal["wrap"] = [seal["wrap"][0], "-p", digest] + list(seal["wrap"][3:])
payload = {key: value for key, value in contract.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
contract["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
open(path, "w").write(json.dumps(contract, indent=2, sort_keys=True) + "\n")
REBINDPY
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    if "network" in seal:
        import hashlib

        log_path = directory / "network.log"
        own = [
            line
            for line in log_path.read_text().splitlines()
            if line.strip() and json.loads(line).get("rep") == name
        ]
        blob = ("\n".join(own) + "\n").encode() if own else b""
        event["network_egress"] = {
            "allowed": sum(
                1 for line in own if json.loads(line)["decision"] == "allowed"
            ),
            "refused": 0,
            "log_sha256": "sha256:" + hashlib.sha256(blob).hexdigest(),
        }
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    write_network_log "$directory"

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
    write_network_log "$directory"

    rebind_contract "$contract" '
contract["seal"]["denied_read_roots"] = [
    root for root in contract["seal"]["denied_read_roots"] if root != "/private/tmp"
]
'
    run coverage_reason "$contract"
    [[ "$output" == *"denied for BOTH reads and links"* ]]
    [[ "$output" == *"read-denied: /private/tmp"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    rm "$contract"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract "$contract" 'contract["seal"]["config_sanitized"] = None'
    run coverage_reason "$contract"
    [[ "$output" == *"producer config to be generated"* ]]

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
    [[ "$output" == *"denied for BOTH reads and links"* ]]
    [[ "$output" == *"link-denied:"* ]]

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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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
    write_network_log "$directory"
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

@test "the rep config is generated from an allowlist, not filtered from the operator's" {
    local target="$BATS_TEST_TMPDIR/generated.toml"

    run python3 "$META_TOOL" probe-config --effort low --target "$target"

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" keys)" = "model_reasoning_effort" ] || true
    [[ "$(json_field "$output" sha256)" == sha256:* ]]
    grep -q '^model_reasoning_effort = "low"$' "$target"
    # Web search is a second egress path the network seal does not cover, so it
    # is off in the file every rep runs under.
    grep -q '^web_search = "disabled"$' "$target"
    # Nothing else: no table, no operator key, no notify hook.
    ! grep -q '^\[' "$target"
    ! grep -q 'notify\|mcp_servers\|projects\|approval_policy' "$target"

    # Without an effort the file is the web-search line alone.
    run python3 "$META_TOOL" probe-config --target "$BATS_TEST_TMPDIR/bare.toml"
    [ "$status" -eq 0 ]
    ! grep -q 'model_reasoning_effort' "$BATS_TEST_TMPDIR/bare.toml"
}

@test "the only permitted config growth is codex's own trust table for the workspace" {
    local base="$BATS_TEST_TMPDIR/base.toml"
    local live="$BATS_TEST_TMPDIR/live.toml"
    python3 "$META_TOOL" probe-config --effort low --target "$base" >/dev/null
    cp "$base" "$live"

    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" findings)" = "[]" ] || [ "$output" = '{"findings":[]}' ]

    # codex writes a trust entry for the directory it ran in: expected growth.
    printf '\n[projects."/run/ws"]\ntrust_level = "trusted"\n' >> "$live"
    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [ "$status" -eq 0 ]
    [ "$output" = '{"findings":[]}' ]

    # A trust entry for ANOTHER directory, or a changed key, is a rep editing
    # the file it was measured under.
    printf '\n[projects."/somewhere/else"]\ntrust_level = "trusted"\n' >> "$live"
    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [ "$status" -eq 0 ]
    [[ "$output" == *"outside the workspace"* ]]

    cp "$base" "$live"
    printf 'web_search = "live"\n' > "$live"
    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [ "$status" -eq 0 ]
    [[ "$output" == *"changed"* ]]
}

@test "the bound seal block rebuilds its own profile to the recorded digest" {
    local directory="$PROBE_DIR/fixtures-render"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # Positive: the record the harness renderer produced rebuilds exactly.
    run coverage_reason "$directory/capture-contract.json"
    [ "$status" -eq 0 ]
    [ "$output" = "ELIGIBLE" ]

    # M2V-03: before this, the block's roots were assertions ALONGSIDE an opaque
    # profile, so a record could claim anything and stay eligible. Each of these
    # tampered blocks keeps the recorded digest and is now refused.
    local contract="$directory/capture-contract.json"
    rebind_contract "$contract" '
contract["seal"]["writable_roots"] = ["/"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"writable roots reach the"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    rm "$contract"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract "$contract" '
seal = contract["seal"]
seal["allowed_read_paths"] = seal["allowed_read_paths"] + [seal["git_common_root"] + "/skills/x/SKILL.md"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"launcher chain"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["allowed_read_paths"] = [seal["launcher_chain"][0]["path"]]
seal["real_tmpdir"] = None
'
    run coverage_reason "$contract"
    [[ "$output" == *"requires real_tmpdir"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["real_tmpdir"] = "/private/tmp"
seal["git_common_root"] = None
'
    run coverage_reason "$contract"
    [[ "$output" == *"requires git_common_root"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["git_common_root"] = seal["repository_root"]
seal["denied_read_roots"] = seal["denied_read_roots"] + ["/extra-allowance"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"rebuild its own profile"* ]]
}

@test "a seatbelt seal with no network mode cannot be coverage" {
    local directory="$PROBE_DIR/fixtures-network"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    rebind_contract "$contract" 'contract["seal"]["network"]["mode"] = "open"'
    run coverage_reason "$contract"
    [[ "$output" == *"network mode 'proxy-allowlist'"* ]]
    [[ "$output" == *"fetch the canonical SKILL.md"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["network"]["mode"] = "proxy-allowlist"
seal["network"]["hosts"] = []
'
    run coverage_reason "$contract"
    [[ "$output" == *"non-empty network host allowlist"* ]]
}

@test "a seatbelt seal that names a shadowed sandbox-exec cannot be coverage" {
    local directory="$PROBE_DIR/fixtures-wrap"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    rebind_contract "$contract" '
seal = contract["seal"]
seal["sandbox_exec"] = "/tmp/evil/sandbox-exec"
seal["wrap"] = ["/tmp/evil/sandbox-exec", "-p", seal["profile_sha256"]]
'
    run coverage_reason "$contract"
    [[ "$output" == *"system seatbelt binary"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["sandbox_exec"] = "/usr/bin/sandbox-exec"
seal["wrap"] = ["sandbox-exec", "-p", seal["profile_sha256"]]
'
    run coverage_reason "$contract"
    [[ "$output" == *"dispatch wrap to be the bound profile"* ]]
}

# --- the harness CONNECT proxy ------------------------------------------------
# The network seal has two halves: the seatbelt profile denies every destination
# but the proxy, and the proxy allows only the bound hosts. These exercise the
# second half directly, since a hole in it is a hole in the seal.

@test "the harness proxy allows only the bound hosts and logs every attempt" {
    local proxy="$REPO_ROOT/scripts/lib/probe-connect-proxy.py"
    local log="$BATS_TEST_TMPDIR/proxy.log"
    local port_file="$BATS_TEST_TMPDIR/proxy.port"
    local rep_file="$BATS_TEST_TMPDIR/proxy.rep"
    printf 'control-1\n' > "$rep_file"

    # A local listener stands in for an allowed destination, so the test needs
    # no outbound network of its own.
    python3 - "$BATS_TEST_TMPDIR/upstream.port" <<'PY' &
import socket, sys, threading
listener = socket.socket()
listener.bind(("127.0.0.1", 0))
listener.listen(4)
open(sys.argv[1], "w").write(str(listener.getsockname()[1]))
while True:
    client, _ = listener.accept()
    threading.Thread(target=lambda c=client: (c.recv(65536), c.sendall(b"pong"), c.close()), daemon=True).start()
PY
    local upstream_pid=$!
    while [[ ! -s "$BATS_TEST_TMPDIR/upstream.port" ]]; do sleep 0.05; done
    local upstream_port
    upstream_port="$(cat "$BATS_TEST_TMPDIR/upstream.port")"

    # --allow-private-upstream exists only for this test: a real capture refuses
    # a destination that resolves into loopback or private space, so a local
    # stand-in upstream is unreachable without it. --allow-port is what makes
    # the ephemeral upstream port admissible; a capture pins 443.
    python3 "$proxy" --allow-host 127.0.0.1 --allow-host chatgpt.com \
        --allow-host .oaiusercontent.com \
        --allow-port "$upstream_port" --allow-port 443 \
        --allow-private-upstream \
        --log "$log" --port-file "$port_file" --rep-file "$rep_file" \
        >/dev/null 2>&1 &
    local proxy_pid=$!
    while [[ ! -s "$port_file" ]]; do sleep 0.05; done
    local port
    port="$(cat "$port_file")"

    # An allowed destination tunnels; a denied one is refused with 403.
    run python3 - "$port" "$upstream_port" <<'PY'
import socket, sys

def connect(port, authority):
    sock = socket.create_connection(("127.0.0.1", int(port)), timeout=5)
    sock.sendall(f"CONNECT {authority} HTTP/1.1\r\nHost: {authority}\r\n\r\n".encode())
    head = sock.recv(4096).decode("latin-1", "replace")
    sock.close()
    return head.splitlines()[0]

print("allowed:", connect(sys.argv[1], f"127.0.0.1:{sys.argv[2]}"))
print("denied:", connect(sys.argv[1], "raw.githubusercontent.com:443"))
print("suffix:", connect(sys.argv[1], "sdmntprsouthcentralus.oaiusercontent.com:443"))
print("lookalike:", connect(sys.argv[1], "evil-oaiusercontent.com:443"))
PY
    kill "$proxy_pid" "$upstream_pid" 2>/dev/null || true

    [ "$status" -eq 0 ]
    [[ "$output" == *"allowed: HTTP/1.1 200"* ]]
    [[ "$output" == *"denied: HTTP/1.1 403"* ]]
    # A rotating-region content host is allowed by the named domain suffix; a
    # lookalike that merely ends with the same characters is not.
    [[ "$output" == *"lookalike: HTTP/1.1 403"* ]]

    # Every attempt is on the record, attributed to the rep that made it.
    run python3 "$META_TOOL" proxy-egress --log "$log" --rep control-1
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" allowed)" -ge 1 ]
    [ "$(json_field "$output" refused)" -ge 2 ]
    [[ "$(json_field "$output" log_sha256)" == sha256:* ]]
    grep -q 'raw.githubusercontent.com' "$log"

    # A different rep's summary sees none of it.
    run python3 "$META_TOOL" proxy-egress --log "$log" --rep treatment-9
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" allowed)" -eq 0 ]
    [ "$(json_field "$output" refused)" -eq 0 ]
}

@test "the proxy refuses a plain HTTP proxy request, not only a denied host" {
    local proxy="$REPO_ROOT/scripts/lib/probe-connect-proxy.py"
    local log="$BATS_TEST_TMPDIR/plain.log"
    local port_file="$BATS_TEST_TMPDIR/plain.port"
    python3 "$proxy" --allow-host chatgpt.com --log "$log" \
        --port-file "$port_file" >/dev/null 2>&1 &
    local proxy_pid=$!
    while [[ ! -s "$port_file" ]]; do sleep 0.05; done

    run python3 - "$(cat "$port_file")" <<'PY'
import socket, sys
sock = socket.create_connection(("127.0.0.1", int(sys.argv[1])), timeout=5)
sock.sendall(b"GET http://chatgpt.com/ HTTP/1.1\r\nHost: chatgpt.com\r\n\r\n")
print(sock.recv(4096).decode("latin-1", "replace").splitlines()[0])
sock.close()
PY
    kill "$proxy_pid" 2>/dev/null || true

    [ "$status" -eq 0 ]
    [[ "$output" == *"405"* ]]
}

@test "the proxy admits one port and refuses a name that resolves into local space" {
    local proxy="$REPO_ROOT/scripts/lib/probe-connect-proxy.py"
    local log="$BATS_TEST_TMPDIR/pin.log"
    local port_file="$BATS_TEST_TMPDIR/pin.port"
    # No --allow-private-upstream and no extra --allow-port: this is the shape a
    # capture actually runs.
    python3 "$proxy" --allow-host chatgpt.com --allow-host 127.0.0.1 \
        --log "$log" --port-file "$port_file" >/dev/null 2>&1 &
    local proxy_pid=$!
    while [[ ! -s "$port_file" ]]; do sleep 0.05; done

    run python3 - "$(cat "$port_file")" <<'PY'
import socket, sys

def connect(port, authority):
    sock = socket.create_connection(("127.0.0.1", int(port)), timeout=5)
    sock.sendall(f"CONNECT {authority} HTTP/1.1\r\nHost: {authority}\r\n\r\n".encode())
    head = sock.recv(4096).decode("latin-1", "replace")
    sock.close()
    return head.splitlines()[0]

# An allowed host on a port the capture does not permit.
print("otherport:", connect(sys.argv[1], "chatgpt.com:80"))
# An allowed NAME that resolves to loopback: a rebinding answer must not become
# a tunnel into a local service.
print("loopback:", connect(sys.argv[1], "127.0.0.1:443"))
PY
    kill "$proxy_pid" 2>/dev/null || true

    [ "$status" -eq 0 ]
    [[ "$output" == *"otherport: HTTP/1.1 403"* ]]
    [[ "$output" == *"loopback: HTTP/1.1 403"* ]]
    grep -q 'port is not on the capture allowlist' "$log"
    grep -q 'resolves into local address space' "$log"
    # Every CONNECT leaves an `attempt` record before any decision.
    [ "$(grep -c '"decision": "attempt"' "$log")" -eq 2 ]
}

@test "the seal's egress policy is pinned, not merely recorded" {
    local directory="$PROBE_DIR/fixtures-egresspin"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    run coverage_reason "$contract"
    [ "$output" = "ELIGIBLE" ]

    # Recording a host list is not constraining one: before this a capture could
    # allowlist the forge, record that it did, and still count.
    rebind_contract "$contract" '
contract["seal"]["network"]["hosts"] = ["chatgpt.com", "raw.githubusercontent.com"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"pinned host set"* ]]
    [[ "$output" == *"raw.githubusercontent.com"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["network"]["hosts"] = ["chatgpt.com"]
seal["network"]["ports"] = [443, 8080]
'
    run coverage_reason "$contract"
    [[ "$output" == *"only on ports [443]"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["network"]["ports"] = [443]
seal["network"]["unix_sockets"] = ["/var/run/mDNSResponder"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"no unix-socket egress"* ]]

    rebind_contract "$contract" '
seal = contract["seal"]
seal["network"]["unix_sockets"] = []
seal["network"]["mode"] = "proxy-custom"
'
    run coverage_reason "$contract"
    [[ "$output" == *"network mode 'proxy-allowlist'"* ]]
}

@test "the seal must deny the real CODEX_HOME and the Darwin cache root" {
    local directory="$PROBE_DIR/fixtures-dataroots"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # A CODEX_HOME configured outside the operator home was recorded and never
    # required, so a seal could name it and deny nothing.
    rebind_contract "$contract" '
contract["seal"]["real_codex_home"] = "/private/tmp/elsewhere/.codex"
contract["seal"]["denied_read_roots"] = [
    root for root in contract["seal"]["denied_read_roots"] if root != "/private/tmp"
]
'
    run coverage_reason "$contract"
    [[ "$output" == *"denied for BOTH reads and links"* ]]
    [[ "$output" == *"/private/tmp/elsewhere/.codex"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    rm "$contract"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract "$contract" '
contract["seal"]["cache_root"] = "/private/var/folders/elsewhere/C"
'
    run coverage_reason "$contract"
    [[ "$output" == *"denied for BOTH reads and links"* ]]
    [[ "$output" == *"/private/var/folders/elsewhere/C"* ]]
}

@test "writable roots, devices, environment and launcher chain are all pinned" {
    local directory="$PROBE_DIR/fixtures-pins"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    rebind_contract "$contract" '
contract["seal"]["writable_roots"] = contract["seal"]["writable_roots"] + ["/private/tmp/elsewhere"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"writable root under the run"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    rebind_contract "$contract" 'contract["seal"]["dev_write_paths"] = ["/dev/null"]'
    run coverage_reason "$contract"
    [[ "$output" == *"exactly the devices"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    rebind_contract "$contract" '
contract["seal"]["env_allowlist"] = contract["seal"]["env_allowlist"] + ["AWS_SECRET_ACCESS_KEY"]
'
    run coverage_reason "$contract"
    [[ "$output" == *"AWS_SECRET_ACCESS_KEY"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    write_network_log "$directory"
    # A chain whose paths are absent on THIS host is still fine: the record is
    # the evidence, and a pin that only held where the capture happened is what
    # broke CI. What is refused is a chain the record itself contradicts.
    rebind_contract_profile "$contract" '
seal = contract["seal"]
entry = dict(seal["launcher_chain"][0])
entry["path"] = "/nonexistent-root/bin/codex"
seal["launcher_chain"] = [entry]
seal["launcher_invoked"] = entry["path"]
seal["allowed_read_paths"] = []
'
    run coverage_reason "$contract"
    [ "$output" = "ELIGIBLE" ]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract "$contract" 'contract["seal"]["launcher_sha256"] = "sha256:" + "0" * 64'
    run coverage_reason "$contract"
    [[ "$output" == *"digest the chain"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract "$contract" '
contract["seal"]["launcher_invoked"] = "/nonexistent-root/bin/other"
'
    run coverage_reason "$contract"
    [[ "$output" == *"start at the invoked launcher"* ]]

    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    rebind_contract "$contract" 'contract["seal"]["timeout_bin"] = None'
    run coverage_reason "$contract"
    [[ "$output" == *"resolved timeout binary"* ]]
}

@test "a seal record carrying an unknown field is refused before any dispatch" {
    local directory="$PROBE_DIR/fixtures-unknown"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    edit_seal "$directory/seal.json" 'record["extra_capability"] = "anything"'

    run snapshot "$directory"

    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown fields"* ]]
    [[ "$output" == *"extra_capability"* ]]
    [ ! -e "$directory/capture-contract.json" ]
}

@test "a required root that is read-denied but not link-denied is not coverage" {
    local directory="$PROBE_DIR/fixtures-linkgap"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # A hard link or a clone turns a denied file into a readable one, so a root
    # that is read-denied and not link-denied is not actually denied. Coverage
    # only checked the checkout and the home for link denies.
    rebind_contract "$contract" '
seal = contract["seal"]
# The cache root is only covered by the temp-root entry, so dropping that entry
# leaves it read-denied and link-open, which is the gap under test.
seal["denied_link_roots"] = [
    root for root in seal["denied_link_roots"] if root != "/private/tmp"
]
'
    run coverage_reason "$contract"
    [[ "$output" == *"denied for BOTH reads and links"* ]]
    [[ "$output" == *"link-denied:"* ]]
}

@test "a null cache root is not coverage even when nothing else changed" {
    local directory="$PROBE_DIR/fixtures-nullcache"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # A falsy value used to make the root VANISH from the required list, so
    # nulling the field meant nothing had to be denied.
    rebind_contract "$contract" 'contract["seal"]["cache_root"] = None'
    run coverage_reason "$contract"
    [[ "$output" == *"cache_root is null"* ]]

    rebind_contract "$contract" '
contract["seal"]["cache_root"] = "/private/tmp/fixture-cache"
contract["seal"]["real_codex_home"] = None
'
    run coverage_reason "$contract"
    [ "$status" -ne 0 ] || [[ "$output" == *"real_codex_home"* ]]
}

@test "a timeout budget of zero is not coverage even with a timeout binary" {
    local directory="$PROBE_DIR/fixtures-nobudget"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # `--timeout 0` omitted the wrapper entirely while the record still named a
    # binary, so an unbounded run looked exactly like a bounded one.
    rebind_contract "$contract" '
seal = contract["seal"]
seal["timeout_seconds"] = 0
seal["wrap"] = [seal["sandbox_exec"], "-p", seal["profile_sha256"]]
'
    run coverage_reason "$contract"
    [[ "$output" == *"positive timeout budget"* ]]
}

@test "a launcher chain with an unrelated file prepended is not coverage" {
    local directory="$PROBE_DIR/fixtures-chain"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # Existence alone let any file be called a launcher. A chain is a symlink
    # walk: entry N is a symlink to entry N+1 and only the last is a real file,
    # and the record says which is which so any host can check it.
    printf 'CANONICAL_ACTION\n' > "$REAL_HOME/decoy"
    rebind_contract_profile "$contract" '
seal = contract["seal"]
decoy = seal["original_home"] + "/decoy"
seal["launcher_chain"] = [
    {"path": decoy, "kind": "file", "sha256": seal["launcher_sha256"]}
] + seal["launcher_chain"]
seal["launcher_invoked"] = decoy
seal["allowed_read_paths"] = [entry["path"] for entry in seal["launcher_chain"]]
'
    run coverage_reason "$contract"
    [[ "$output" == *"symlink to the next"* ]]

    # The same prepend declared as a symlink still fails: it points nowhere the
    # chain continues to.
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract_profile "$contract" '
seal = contract["seal"]
decoy = seal["original_home"] + "/decoy"
seal["launcher_chain"] = [
    {"path": decoy, "kind": "symlink", "target": "/somewhere/else"}
] + seal["launcher_chain"]
seal["launcher_invoked"] = decoy
seal["allowed_read_paths"] = [entry["path"] for entry in seal["launcher_chain"]]
'
    run coverage_reason "$contract"
    [[ "$output" == *"adjacent"* ]]

    # And a chain that repeats a path is refused outright.
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract_profile "$contract" '
seal = contract["seal"]
head = seal["launcher_chain"][0]["path"]
seal["launcher_chain"] = [
    {"path": head, "kind": "symlink", "target": head}
] + seal["launcher_chain"]
# Deduplicated, because allowed_read_paths refuses duplicates on its own: the
# rule under test here belongs to the chain, not to the allowance.
seal["allowed_read_paths"] = [head]
'
    run coverage_reason "$contract"
    [[ "$output" == *"repeats a path"* ]]
}

@test "a launcher chain the host has is cross-checked against the filesystem" {
    local directory="$PROBE_DIR/fixtures-hostcheck"
    local contract="$directory/capture-contract.json"
    mkdir -p "$directory"
    local -a roots
    mapfile -t roots < <(full_denied_roots)
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"
    snapshot "$directory" >/dev/null
    write_network_log "$directory"

    # A REAL two-link chain under the fixture home: head is a symlink to the
    # binary. The record is the evidence everywhere; where the host has the
    # chain, the record must also agree with it.
    ln -sf "$REAL_HOME/bin/codex" "$REAL_HOME/bin/codex-link"
    rebind_contract_profile "$contract" '
seal = contract["seal"]
home = seal["original_home"]
head = home + "/bin/codex-link"
tail = seal["launcher_chain"][-1]
seal["launcher_chain"] = [
    {"path": head, "kind": "symlink", "target": tail["path"]},
    tail,
]
seal["launcher_invoked"] = head
seal["allowed_read_paths"] = [head, tail["path"]]
'
    run coverage_reason "$contract"
    [ "$output" = "ELIGIBLE" ]

    # Now the record says the head points somewhere the filesystem does not.
    rebind_contract "$contract" '
seal = contract["seal"]
seal["launcher_chain"][0]["target"] = seal["original_home"] + "/bin/elsewhere"
seal["launcher_chain"] = [seal["launcher_chain"][0]]
seal["launcher_chain"][0]["kind"] = "symlink"
seal["launcher_sha256"] = seal["launcher_sha256"]
'
    run coverage_reason "$contract"
    # The record is self-inconsistent (a symlink as the last entry): the record
    # rule fires, by name, before any host walk.
    [[ "$output" == *"binary, not a link"* ]]

    # A record that is structurally sound but disagrees with the host it names.
    write_seal "$directory/seal.json" seatbelt "${roots[@]}"; rm "$contract"; snapshot "$directory" >/dev/null
    write_network_log "$directory"
    rebind_contract_profile "$contract" '
seal = contract["seal"]
home = seal["original_home"]
head = home + "/bin/codex-link"
tail = dict(seal["launcher_chain"][-1])
seal["launcher_chain"] = [
    {"path": head, "kind": "symlink", "target": home + "/bin/not-the-target"},
    {"path": home + "/bin/not-the-target", "kind": "file",
     "sha256": tail["sha256"]},
]
seal["launcher_invoked"] = head
seal["launcher_sha256"] = tail["sha256"]
seal["allowed_read_paths"] = [entry["path"] for entry in seal["launcher_chain"]]
'
    run coverage_reason "$contract"
    [[ "$output" == *"this host has"* ]]
}

@test "a published proxy log with a null rep or an unpaired attempt is refused" {
    local log="$BATS_TEST_TMPDIR/strict.log"
    local directory="$PROBE_DIR/fixtures-strictlog"
    mkdir -p "$directory"

    # A refusal logged with no rep belonged to no rep, so a per-rep filter made
    # it invisible: exactly the shape an unattributed egress would take.
    printf '%s\n' \
        '{"decision": "attempt", "host": "chatgpt.com", "port": 443, "rep": null, "ts": "t"}' \
        > "$log"
    run python3 - "$META_TOOL" "$log" <<'PY'
import importlib.util, sys
spec = importlib.util.spec_from_file_location("m", sys.argv[1])
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
try:
    m.parse_proxy_log(open(sys.argv[2], "rb").read())
except m.MetadataError as exc:
    print("REFUSED:", exc)
else:
    print("ACCEPTED")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"REFUSED"* ]]
    [[ "$output" == *"belongs to no rep"* ]]

    # An attempt with no decision is a connection whose fate the log does not
    # record; a decision with no attempt is a decision from nowhere.
    printf '%s\n' \
        '{"decision": "attempt", "host": "chatgpt.com", "port": 443, "rep": "control-1", "ts": "t"}' \
        > "$directory/network.log"
    run python3 - "$META_TOOL" "$directory" <<'PY'
import importlib.util, pathlib, sys
spec = importlib.util.spec_from_file_location("m", sys.argv[1])
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
try:
    m.verify_network_log(pathlib.Path(sys.argv[2]), [])
except m.MetadataError as exc:
    print("REFUSED:", exc)
else:
    print("ACCEPTED")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"REFUSED"* ]]
    [[ "$output" == *"decision(s)"* ]]

    # An unknown field is a shape the reader cannot check.
    printf '%s\n' \
        '{"decision": "allowed", "host": "h", "port": 443, "rep": "control-1", "ts": "t", "extra": 1}' \
        > "$log"
    run python3 - "$META_TOOL" "$log" <<'PY'
import importlib.util, sys
spec = importlib.util.spec_from_file_location("m", sys.argv[1])
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
try:
    m.parse_proxy_log(open(sys.argv[2], "rb").read())
except m.MetadataError as exc:
    print("REFUSED:", exc)
else:
    print("ACCEPTED")
PY
    [[ "$output" == *"unknown fields"* ]]
}

@test "the permitted config growth must be trust_level with a value codex writes" {
    local base="$BATS_TEST_TMPDIR/cfgbase.toml"
    local live="$BATS_TEST_TMPDIR/cfglive.toml"
    python3 "$META_TOOL" probe-config --effort low --target "$base" >/dev/null

    # An empty table is not codex writing its trust entry.
    cp "$base" "$live"
    printf '\n[projects."/run/ws"]\n' >> "$live"
    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [[ "$output" == *"exactly trust_level"* ]]

    # Neither is a differently typed or unexpected value.
    cp "$base" "$live"
    printf '\n[projects."/run/ws"]\ntrust_level = 7\n' >> "$live"
    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [[ "$output" == *"not one codex writes"* ]]

    cp "$base" "$live"
    printf '\n[projects."/run/ws"]\ntrust_level = "trusted"\n' >> "$live"
    run python3 "$META_TOOL" config-drift --path "$live" \
        --expected-file "$base" --workspace /run/ws
    [ "$output" = '{"findings":[]}' ]
}
