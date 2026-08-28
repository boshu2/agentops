#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HARNESS="$REPO_ROOT/scripts/probe-skill.sh"
    META_TOOL="$REPO_ROOT/scripts/lib/probe-fixture-metadata.py"
    PREAMBLE="$REPO_ROOT/scripts/lib/preamble.sh"
    DISPATCH_HELPER="$REPO_ROOT/scripts/lib/codex-exec.sh"
    PROBES="$BATS_TEST_TMPDIR/probes"
    SKILLS="$BATS_TEST_TMPDIR/skills"
    PROBE_DIR="$PROBES/demo"
    mkdir -p "$PROBE_DIR" "$SKILLS/demo-skill"
    export SKILL_PROBES_DIR="$PROBES"
    export SKILL_PROBE_SKILLS_DIR="$SKILLS"

    cat > "$PROBE_DIR/probe.json" <<'JSON'
{"id":"demo","skill":"demo-skill","reps":2,"discriminator":"discriminator.sh","treatment_source":"injected-prelude"}
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
    printf 'PRELUDE_ACTION\n' > "$PROBE_DIR/treatment-prelude.md"
    cat > "$PROBE_DIR/discriminator.sh" <<'SH'
#!/usr/bin/env bash
if grep -q '^INFRA$' "$1"; then exit 2; fi
grep -q '^ACTION$' "$1"
SH
    chmod +x "$PROBE_DIR/discriminator.sh"

    RUNTIME_STUB="$BATS_TEST_TMPDIR/codex-runtime-identity"
    cat > "$RUNTIME_STUB" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
    printf 'codex-cli probe-test\n'
    exit 0
fi
printf 'runtime identity stub is not a live producer\n' >&2
exit 70
SH
    chmod +x "$RUNTIME_STUB"
}

write_transcript() {
    local directory="$1" name="$2" model="$3" effort="$4" body="$5"
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
    {"type": "thread.started", "thread_id": f"thread-{directory.name}-{name}"},
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

write_legacy_transcript() {
    local directory="$1" name="$2" model="$3" effort="$4" body="$5"
    {
        printf 'OpenAI Codex fixture\n'
        printf '%s\n' '--------'
        printf 'workdir: /fixture\n'
        printf 'model: %s\n' "$model"
        printf 'reasoning effort: %s\n' "$effort"
        printf '%s\n' '--------'
        printf 'user\nQUESTION\n'
        printf 'codex\n'
        printf '%s\n' "$body"
        printf 'tokens used: 1\n'
    } > "$directory/$name.txt"
}

make_fixture_set() {
    local name="$1" model="$2" effort="$3" control_body="$4" treatment_body="$5"
    local directory="$PROBE_DIR/$name" rep
    mkdir -p "$directory"
    python3 "$META_TOOL" snapshot \
        --fixture-dir "$directory" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo \
        --requested-model "$model" \
        --requested-effort "$effort" \
        --producer-override-bin "$RUNTIME_STUB" >/dev/null
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" "$model" "$effort" "$control_body"
        write_transcript "$directory" "treatment-$rep" "$model" "$effort" "$treatment_body"
    done
    python3 "$META_TOOL" create \
        --fixture-dir "$directory" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --harness "$HARNESS" \
        --preamble "$PREAMBLE" \
        --dispatch-helper "$DISPATCH_HELPER" \
        --probe demo \
        --reps 2 \
        --requested-model "$model" \
        --requested-effort "$effort" >/dev/null
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

@test "git attributes preserve transcript bytes and normalize all hashed text inputs" {
    run git -C "$REPO_ROOT" check-attr text whitespace -- \
        evals/skill-probes/example/fixtures/control-1.txt \
        evals/skill-probes/example/fixtures-xhigh-2026-08-04/treatment-2.txt

    [ "$status" -eq 0 ]
    [ "$(printf '%s\n' "$output" | grep -c ': text: unset$')" -eq 2 ]
    [ "$(printf '%s\n' "$output" | grep -c ': whitespace: unset$')" -eq 2 ]

    run git -C "$REPO_ROOT" check-attr text eol -- \
        evals/skill-probes/example/probe.json \
        evals/skill-probes/example/question.md \
        evals/skill-probes/example/treatment-prelude.md \
        evals/skill-probes/example/fixtures-v3/capture-contract.json \
        evals/skill-probes/example/discriminator.sh \
        skills/example/SKILL.md \
        scripts/probe-skill.sh \
        scripts/lib/preamble.sh \
        scripts/lib/codex-exec.sh \
        scripts/lib/probe-fixture-metadata.py

    [ "$status" -eq 0 ]
    [ "$(printf '%s\n' "$output" | grep -c ': text: set$')" -eq 10 ]
    [ "$(printf '%s\n' "$output" | grep -c ': eol: lf$')" -eq 10 ]
}

@test "replay takes producer provenance and binding from verified fixture metadata" {
    make_fixture_set fixtures observed-model low ABSENT ACTION

    run bash "$HARNESS" --probe demo --replay
    [ "$status" -eq 0 ]
    [ "$(json_field "$output" schema)" = "agentops-skill-probe.v3" ]
    [ "$(json_field "$output" producer.model)" = "observed-model" ]
    [ "$(json_field "$output" producer.effort)" = "low" ]
    [ "$(json_field "$output" producer.identity.source)" = "test-override" ]
    [ "$(json_field "$output" producer.identity.coverage_eligible)" = "false" ]
    [ "$(json_field "$output" producer.threads.0.path)" = "control-1.txt" ]
    [ "$(json_field "$output" fixture_set.name)" = "fixtures" ]
    [[ "$(json_field "$output" fixture_set.binding_sha256)" == sha256:* ]]
    [ "$(json_field "$output" evaluator_matches_capture)" = "true" ]
    [ "$(json_field "$output" treatment_source)" = "injected-prelude" ]
    [[ "$(json_field "$output" honesty)" == *"NOT full-skill activation"* ]]
    [ "$(json_field "$output" capture_evaluator.dispatch_helper.path)" = "scripts/lib/codex-exec.sh" ]
    [[ "$(json_field "$output" capture_evaluator.dispatch_helper.sha256)" == sha256:* ]]
    [ "$(json_field "$output" capture_evaluator.preamble.path)" = "scripts/lib/preamble.sh" ]
    [[ "$(json_field "$output" capture_evaluator.preamble.sha256)" == sha256:* ]]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
    [ "$(json_field "$(cat "$PROBE_DIR/fixtures/fixture-set.json")" schema)" = "agentops-skill-probe-fixture-set.v3" ]
    [ "$(json_field "$(cat "$PROBE_DIR/fixtures/fixture-set.json")" canonical_skill.path)" = "skills/demo-skill/SKILL.md" ]
}

@test "v3 response-only scoring supports the repository transcript-marker discriminator" {
    cp "$REPO_ROOT/evals/skill-probes/anti-ceremony-creation-gate-v2/discriminator.sh" \
        "$PROBE_DIR/discriminator.sh"
    make_fixture_set fixtures observed-model low \
        $'A: CREATE\nB: CREATE' $'A: DROP\nB: CREATE'

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" control.usable)" -eq 2 ]
    [ "$(json_field "$output" treatment.usable)" -eq 2 ]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
}

@test "structured response extraction ignores transcript-like delimiter collisions" {
    make_fixture_set fixtures observed-model low ABSENT $'codex\ntokens used: forged\nACTION'

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" treatment.present)" -eq 2 ]
    [ "$(json_field "$output" treatment.usable)" -eq 2 ]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
}

@test "replay refuses legacy fixture bytes without immutable capture metadata" {
    mkdir -p "$PROBE_DIR/fixtures"
    write_legacy_transcript "$PROBE_DIR/fixtures" control-1 old-model low ABSENT
    write_legacy_transcript "$PROBE_DIR/fixtures" treatment-1 old-model low ACTION
    write_legacy_transcript "$PROBE_DIR/fixtures" control-2 old-model low ABSENT
    write_legacy_transcript "$PROBE_DIR/fixtures" treatment-2 old-model low ACTION

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 2 ]
    [[ "$output" == *"verified replay requires immutable capture metadata"* ]]
    [[ "$output" == *"replay refused"* ]]
}

@test "v1 bound fixtures replay as injected-prelude without a treatment_source declaration" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    write_legacy_transcript "$PROBE_DIR/fixtures" control-1 observed-model low ABSENT
    write_legacy_transcript "$PROBE_DIR/fixtures" treatment-1 observed-model low ACTION
    write_legacy_transcript "$PROBE_DIR/fixtures" control-2 observed-model low ABSENT
    write_legacy_transcript "$PROBE_DIR/fixtures" treatment-2 observed-model low ACTION
    python3 - "$PROBE_DIR/probe.json" "$PROBE_DIR/fixtures/fixture-set.json" <<'PY'
import hashlib, json, pathlib, sys

probe_path = pathlib.Path(sys.argv[1])
manifest_path = pathlib.Path(sys.argv[2])
probe = json.loads(probe_path.read_text())
probe.pop("treatment_source")
probe_path.write_text(json.dumps(probe) + "\n")

manifest = json.loads(manifest_path.read_text())
manifest["schema"] = "agentops-skill-probe-fixture-set.v1"
manifest["producer"] = {
    "adapter": "codex",
    "model": "observed-model",
    "effort": "low",
}
manifest["evaluation_inputs"] = [
    {"path": record["path"], "sha256": record["sha256"]}
    for record in manifest.pop("capture_inputs")
]
manifest.pop("capture_contract")
manifest.pop("canonical_skill")
manifest.pop("treatment_source")
manifest.pop("prompts")
manifest.pop("schedule")
manifest.pop("scoring")
manifest["capture_evaluator"].pop("preamble")
manifest["capture_evaluator"].pop("dispatch_helper")
for record in manifest["transcripts"]:
    transcript_path = manifest_path.parent / record["path"]
    record["sha256"] = "sha256:" + hashlib.sha256(transcript_path.read_bytes()).hexdigest()
for record in manifest["evaluation_inputs"]:
    if record["path"] == "probe.json":
        record["sha256"] = "sha256:" + hashlib.sha256(probe_path.read_bytes()).hexdigest()
payload = {key: value for key, value in manifest.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
manifest["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
manifest_path.write_text(json.dumps(manifest) + "\n")
manifest_path.with_name("capture-contract.json").unlink()
PY

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" treatment_source)" = "injected-prelude" ]
    [[ "$(json_field "$output" honesty)" == *"NOT full-skill activation"* ]]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
}

@test "replay rejects transcript tampering before discrimination" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    printf 'ACTION\n' >> "$PROBE_DIR/fixtures/control-1.txt"

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 2 ]
    [[ "$output" == *"transcript digest mismatch for control-1.txt"* ]]
}

@test "replay rejects producer-config tampering through the fixture-set binding" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    python3 - "$PROBE_DIR/fixtures/fixture-set.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["requested_producer"]["model"] = "relabeled-model"
path.write_text(json.dumps(value))
PY

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 2 ]
    [[ "$output" == *"requested_producer disagrees with bound producer request"* ]]
}

@test "fixture creation rejects prompt-event tampering before manifest binding" {
    local directory="$PROBE_DIR/fixtures"
    local rep
    mkdir -p "$directory"
    python3 "$META_TOOL" snapshot \
        --fixture-dir "$directory" --probe-dir "$PROBE_DIR" --skills-dir "$SKILLS" \
        --probe demo --requested-model observed-model --requested-effort low \
        --producer-override-bin "$RUNTIME_STUB" >/dev/null
    for rep in 1 2; do
        write_transcript "$directory" "control-$rep" observed-model low ABSENT
        write_transcript "$directory" "treatment-$rep" observed-model low ACTION
    done
    python3 - "$directory/control-1.txt" <<'PY'
import base64, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
events = [json.loads(line) for line in path.read_text().splitlines()]
events[0]["prompt"]["content_base64"] = base64.b64encode(b"TAMPERED\n").decode()
path.write_text("".join(json.dumps(event) + "\n" for event in events))
PY

    run python3 "$META_TOOL" create \
        --fixture-dir "$directory" --probe-dir "$PROBE_DIR" --skills-dir "$SKILLS" \
        --harness "$HARNESS" --preamble "$PREAMBLE" \
        --dispatch-helper "$DISPATCH_HELPER" --probe demo --reps 2 \
        --requested-model observed-model --requested-effort low

    [ "$status" -eq 2 ]
    [[ "$output" == *"probe input event does not match bound control-1 prompt"* ]]
    [ ! -e "$directory/fixture-set.json" ]
}

@test "fixture creation refuses retroactive capture-contract creation" {
    local directory="$PROBE_DIR/fixtures"
    mkdir -p "$directory"
    write_legacy_transcript "$directory" control-1 observed-model low ABSENT
    write_legacy_transcript "$directory" treatment-1 observed-model low ACTION
    write_legacy_transcript "$directory" control-2 observed-model low ABSENT
    write_legacy_transcript "$directory" treatment-2 observed-model low ACTION

    run python3 "$META_TOOL" create \
        --fixture-dir "$directory" --probe-dir "$PROBE_DIR" --skills-dir "$SKILLS" \
        --harness "$HARNESS" --preamble "$PREAMBLE" \
        --dispatch-helper "$DISPATCH_HELPER" --probe demo --reps 2 \
        --requested-model observed-model --requested-effort low

    [ "$status" -eq 2 ]
    [[ "$output" == *"requires a pre-existing capture contract written before execution"* ]]
    [ ! -e "$directory/capture-contract.json" ]
    [ ! -e "$directory/fixture-set.json" ]
}

@test "v3 replay scores captured discriminator bytes after current probe drift" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    printf '# changed scoring input\n' >> "$PROBE_DIR/discriminator.sh"

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
}

@test "v3 replay remains self-contained after current canonical skill drift" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    printf '\nchanged canonical guidance\n' >> "$SKILLS/demo-skill/SKILL.md"

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
}

@test "capture rejects a canonical skill whose declared identity mismatches probe.json" {
    sed -i.bak 's/name: demo-skill/name: unrelated-skill/' "$SKILLS/demo-skill/SKILL.md"
    rm -f "$SKILLS/demo-skill/SKILL.md.bak"
    mkdir -p "$PROBE_DIR/fixtures"

    run python3 "$META_TOOL" snapshot \
        --fixture-dir "$PROBE_DIR/fixtures" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo \
        --requested-model observed-model \
        --requested-effort low \
        --producer-override-bin "$RUNTIME_STUB"

    [ "$status" -eq 2 ]
    [[ "$output" == *"canonical skill identity mismatch"* ]]
    [ ! -e "$PROBE_DIR/fixtures/capture-contract.json" ]
}

@test "replay flags are constraints and mismatches fail rather than relabel fixtures" {
    make_fixture_set fixtures observed-model low ABSENT ACTION

    run bash "$HARNESS" --probe demo --replay --model another-model
    [ "$status" -eq 2 ]
    [[ "$output" == *"does not match bound fixture producer request"* ]]

    run bash "$HARNESS" --probe demo --replay --effort xhigh
    [ "$status" -eq 2 ]
    [[ "$output" == *"does not match bound fixture producer request"* ]]

    run env PROBE_MODEL=ambient-relabel SKILL_PROBES_DIR="$PROBES" \
        bash "$HARNESS" --probe demo --replay
    [ "$status" -eq 2 ]
    [[ "$output" == *"does not match bound fixture producer request"* ]]
}

@test "a zero-usable control arm cannot produce a false BEHAVIORAL verdict" {
    make_fixture_set fixtures observed-model low INFRA ACTION

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" control.usable)" = "0" ]
    [ "$(json_field "$output" treatment.usable)" = "2" ]
    [ "$(json_field "$output" verdict)" = "UNMEASURED" ]
}

@test "both zero-usable arms yield UNMEASURED with null rates" {
    make_fixture_set fixtures observed-model low INFRA INFRA

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" control.rate)" = "null" ]
    [ "$(json_field "$output" treatment.rate)" = "null" ]
    [ "$(json_field "$output" verdict)" = "UNMEASURED" ]
}

@test "named alternate fixture sets are independently selectable and verified" {
    make_fixture_set fixtures default-model low ABSENT ACTION
    make_fixture_set fixtures-xhigh-2026-08-04 alternate-model xhigh ABSENT ABSENT

    run bash "$HARNESS" --probe demo --replay \
        --fixtures fixtures-xhigh-2026-08-04 \
        --model alternate-model \
        --effort xhigh

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" fixture_set.name)" = "fixtures-xhigh-2026-08-04" ]
    [ "$(json_field "$output" producer.model)" = "alternate-model" ]
    [ "$(json_field "$output" producer.effort)" = "xhigh" ]
    [ "$(json_field "$output" verdict)" = "INERT" ]
}

@test "producer strings are JSON serialized without shell interpolation" {
    make_fixture_set fixtures 'gpt-"quoted"' low ABSENT ACTION

    run bash "$HARNESS" --probe demo --replay --model 'gpt-"quoted"'

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" producer.model)" = 'gpt-"quoted"' ]
}

@test "replay discloses when the current evaluator differs from capture" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    python3 - "$PROBE_DIR/fixtures/fixture-set.json" <<'PY'
import hashlib, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["capture_evaluator"]["harness"]["sha256"] = "sha256:" + "0" * 64
payload = {key: item for key, item in value.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
value["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
path.write_text(json.dumps(value))
PY

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" evaluator_matches_capture)" = "false" ]
    [ "$(json_field "$output" capture_evaluator.harness.sha256)" = "sha256:0000000000000000000000000000000000000000000000000000000000000000" ]
    [[ "$(json_field "$output" evaluator.harness.sha256)" == sha256:* ]]
}

@test "v3 replay requires the dispatch helper in capture evaluator identity" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    python3 - "$PROBE_DIR/fixtures/fixture-set.json" <<'PY'
import hashlib, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["capture_evaluator"].pop("dispatch_helper")
payload = {key: item for key, item in value.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
value["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
path.write_text(json.dumps(value))
PY

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 2 ]
    [[ "$output" == *"capture_evaluator must contain exactly"* ]]
    [[ "$output" == *"dispatch_helper"* ]]
}

@test "v3 replay requires the sourced preamble in capture evaluator identity" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    python3 - "$PROBE_DIR/fixtures/fixture-set.json" <<'PY'
import hashlib, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["capture_evaluator"].pop("preamble")
payload = {key: item for key, item in value.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
value["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
path.write_text(json.dumps(value))
PY

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 2 ]
    [[ "$output" == *"capture_evaluator must contain exactly"* ]]
    [[ "$output" == *"preamble"* ]]
}

@test "replay discloses dispatch helper drift from a bound v3 capture" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    python3 - "$PROBE_DIR/fixtures/fixture-set.json" <<'PY'
import hashlib, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["capture_evaluator"]["dispatch_helper"]["sha256"] = "sha256:" + "0" * 64
payload = {key: item for key, item in value.items() if key != "binding_sha256"}
canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
value["binding_sha256"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
path.write_text(json.dumps(value))
PY

    run bash "$HARNESS" --probe demo --replay

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" evaluator_matches_capture)" = "false" ]
    [ "$(json_field "$output" capture_evaluator.dispatch_helper.sha256)" = "sha256:0000000000000000000000000000000000000000000000000000000000000000" ]
    [[ "$(json_field "$output" evaluator.dispatch_helper.sha256)" == sha256:* ]]
}

@test "live capture refuses to overwrite an existing immutable fixture set" {
    make_fixture_set fixtures stale-model low ABSENT ACTION
    cp "$PROBE_DIR/fixtures/fixture-set.json" "$BATS_TEST_TMPDIR/manifest.before"
    cp "$PROBE_DIR/fixtures/treatment-1.txt" "$BATS_TEST_TMPDIR/treatment.before"

    run --separate-stderr env CODEX_EXEC_BIN=definitely-missing-probe-producer \
        SKILL_PROBES_DIR="$PROBES" \
        bash "$HARNESS" --probe demo --live --reps 2

    [ "$status" -eq 2 ]
    [ -z "$output" ]
    [[ "$stderr" == *"refusing to overwrite immutable fixture set"* ]]
    [[ "$stderr" == *"choose a new --fixtures name"* ]]
    cmp "$BATS_TEST_TMPDIR/manifest.before" "$PROBE_DIR/fixtures/fixture-set.json"
    cmp "$BATS_TEST_TMPDIR/treatment.before" "$PROBE_DIR/fixtures/treatment-1.txt"
}

@test "publish refuses a fixture destination that appears during dispatch" {
    local producer="$BATS_TEST_TMPDIR/codex-publish-race"
    local count="$BATS_TEST_TMPDIR/publish-race-count"
    local target="$PROBE_DIR/fixtures-raced"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli race-stub\n'; exit 0; fi
n=$(cat "$PROBE_STUB_COUNT" 2>/dev/null || printf '0')
n=$((n + 1))
printf '%s\n' "$n" > "$PROBE_STUB_COUNT"
if [[ "$n" -eq 4 ]]; then
    mkdir "$PROBE_RACE_TARGET"
    printf 'external-writer\n' > "$PROBE_RACE_TARGET/sentinel"
fi
prompt="$(cat)"
if [[ "$prompt" == *PRELUDE_ACTION* ]]; then response=ACTION; else response=ABSENT; fi
printf '{"type":"thread.started","thread_id":"race-%s"}\n' "$n"
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"item-%s","type":"agent_message","text":"%s"}}\n' "$n" "$response"
printf '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}\n'
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" \
        PROBE_STUB_COUNT="$count" \
        PROBE_RACE_TARGET="$target" \
        bash "$HARNESS" --probe demo --live --reps 2 --fixtures fixtures-raced

    [ "$status" -eq 1 ]
    [ "$(cat "$target/sentinel")" = "external-writer" ]
    [ ! -e "$target/fixture-set.json" ]
    [ "$(find "$target" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')" -eq 1 ]
    [[ "$stderr" == *"refusing to replace existing immutable fixture set"* ]]
    [[ "$stderr" == *"failed to publish fixture set"* ]]
}

@test "publisher rejects an externally mutated hidden-stage transcript without exposing target" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    local stage="$PROBE_DIR/.fixtures-publish-stage"
    local target="$PROBE_DIR/fixtures-publish-mutated"
    mv "$PROBE_DIR/fixtures" "$stage"

    printf 'MUTATED_BEFORE_PUBLISH\n' >> "$stage/treatment-1.txt"

    run python3 "$META_TOOL" publish \
        --stage-dir "$stage" \
        --target-dir "$target" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo
    local publish_status="$status"
    [ "$publish_status" -eq 2 ]
    [[ "$output" == *"transcript digest mismatch"* ]]
    [ ! -e "$target" ]
    [ -f "$stage/fixture-set.json" ]
}

@test "publisher never exposes or deletes an externally replaced hidden-stage transcript" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    local stage="$PROBE_DIR/.fixtures-replaced-transcript-stage"
    local target="$PROBE_DIR/fixtures-replaced-transcript"
    local replacement="$BATS_TEST_TMPDIR/replacement-transcript"
    mv "$PROBE_DIR/fixtures" "$stage"
    printf 'replacement-owner\n' > "$replacement"

    mv "$replacement" "$stage/control-1.txt"

    run python3 "$META_TOOL" publish \
        --stage-dir "$stage" \
        --target-dir "$target" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo
    local publish_status="$status"
    [ "$publish_status" -eq 2 ]
    [[ "$output" == *"transcript digest mismatch"* ]]
    [ ! -e "$target" ]
    [ "$(cat "$stage/control-1.txt")" = "replacement-owner" ]
    [ -f "$stage/fixture-set.json" ]
}

@test "atomic publisher preserves an externally owned destination" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    local stage="$PROBE_DIR/.fixtures-raced-stage"
    local target="$PROBE_DIR/fixtures-raced-at-rename"
    mv "$PROBE_DIR/fixtures" "$stage"

    mkdir "$target"
    printf 'external-writer\n' > "$target/sentinel"

    run python3 "$META_TOOL" publish \
        --stage-dir "$stage" \
        --target-dir "$target" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo
    local publish_status="$status"
    [ "$publish_status" -eq 2 ]
    [[ "$output" == *"refusing to replace existing immutable fixture set"* ]]
    [ "$(cat "$target/sentinel")" = "external-writer" ]
    [ ! -e "$target/fixture-set.json" ]
    [ "$(find "$target" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')" -eq 1 ]
    [ -f "$stage/fixture-set.json" ]
}

@test "publisher rejects a bound evaluator hash that does not match the repository file" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    local stage="$PROBE_DIR/.fixtures-bad-evaluator-stage"
    local target="$PROBE_DIR/fixtures-bad-evaluator"
    mv "$PROBE_DIR/fixtures" "$stage"
    python3 - "$stage/fixture-set.json" <<'PY'
import hashlib
import json
import sys

path = sys.argv[1]
manifest = json.load(open(path, encoding="utf-8"))
digest = manifest["capture_evaluator"]["harness"]["sha256"]
manifest["capture_evaluator"]["harness"]["sha256"] = (
    digest[:-1] + ("0" if digest[-1] != "0" else "1")
)
payload = dict(manifest)
payload.pop("binding_sha256")
manifest["binding_sha256"] = "sha256:" + hashlib.sha256(
    json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()
).hexdigest()
with open(path, "w", encoding="utf-8") as handle:
    json.dump(manifest, handle, sort_keys=True, separators=(",", ":"))
    handle.write("\n")
PY

    run python3 "$META_TOOL" publish \
        --stage-dir "$stage" \
        --target-dir "$target" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo
    [ "$status" -eq 2 ]
    [[ "$output" == *"capture evaluator hashes do not match the exact repo-local evaluator files"* ]]
    [ ! -e "$target" ]
    [ -f "$stage/fixture-set.json" ]
}

@test "publisher rejects current probe inputs that differ from the capture" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    local stage="$PROBE_DIR/.fixtures-input-race-stage"
    local target="$PROBE_DIR/fixtures-publish-input-mutated"
    mv "$PROBE_DIR/fixtures" "$stage"

    printf 'MUTATED_BEFORE_PUBLISH\n' >> "$PROBE_DIR/question.md"

    run python3 "$META_TOOL" publish \
        --stage-dir "$stage" \
        --target-dir "$target" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo
    local publish_status="$status"
    [ "$publish_status" -eq 2 ]
    [[ "$output" == *"current probe inputs differ from the self-contained capture"* ]]
    [ ! -e "$target" ]
    [ -f "$stage/fixture-set.json" ]
}

@test "failed live dispatch does not publish or score any prior transcript" {
    local producer="$BATS_TEST_TMPDIR/codex-failing"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli failing-stub\n'; exit 0; fi
cat >/dev/null
printf 'producer failure\n' >&2
exit 77
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" \
        SKILL_PROBES_DIR="$PROBES" \
        bash "$HARNESS" --probe demo --live --reps 2 --fixtures fixtures-failed

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" verdict)" = "UNMEASURED" ]
    [ "$(json_field "$output" control.usable)" = "0" ]
    [ "$(json_field "$output" treatment.usable)" = "0" ]
    [ "$(json_field "$output" fixture_set.binding_sha256)" = "null" ]
    [ ! -e "$PROBE_DIR/fixtures-failed" ]
    [[ "$stderr" == *"producer failure"* ]]
    [[ "$stderr" == *"incomplete live run; fixture set not published"* ]]
}

@test "a partial live run stays UNMEASURED even when each arm has one usable rep" {
    local producer="$BATS_TEST_TMPDIR/codex-partial"
    local count="$BATS_TEST_TMPDIR/partial-count"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli partial-stub\n'; exit 0; fi
n=$(cat "$PROBE_STUB_COUNT" 2>/dev/null || printf '0')
n=$((n + 1))
printf '%s\n' "$n" > "$PROBE_STUB_COUNT"
if [[ "$n" -gt 2 ]]; then printf 'producer failure\n' >&2; exit 77; fi
prompt="$(cat)"
if [[ "$prompt" == *PRELUDE_ACTION* ]]; then response=ACTION; else response=ABSENT; fi
printf '{"type":"thread.started","thread_id":"partial-%s"}\n' "$n"
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"item-%s","type":"agent_message","text":"%s"}}\n' "$n" "$response"
printf '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}\n'
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" PROBE_STUB_COUNT="$count" \
        SKILL_PROBES_DIR="$PROBES" \
        bash "$HARNESS" --probe demo --live --reps 2 --fixtures fixtures-partial

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" control.usable)" = "1" ]
    [ "$(json_field "$output" treatment.usable)" = "1" ]
    [ "$(json_field "$output" verdict)" = "UNMEASURED" ]
    [ ! -e "$PROBE_DIR/fixtures-partial" ]
}

@test "multi-rep live capture fails closed when canonical input mutates during dispatch" {
    python3 - "$PROBE_DIR/probe.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["treatment_source"] = "canonical-skill"
path.write_text(json.dumps(value))
PY
    local producer="$BATS_TEST_TMPDIR/codex-mutating"
    local count="$BATS_TEST_TMPDIR/mutating-count"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli mutating-stub\n'; exit 0; fi
n=$(cat "$PROBE_STUB_COUNT" 2>/dev/null || printf '0')
n=$((n + 1))
printf '%s\n' "$n" > "$PROBE_STUB_COUNT"
prompt="$(cat)"
if [[ "$n" -eq 1 ]]; then printf '\nMUTATED_DURING_CAPTURE\n' >> "$PROBE_MUTATE_FILE"; fi
if [[ "$prompt" == *CANONICAL_ACTION* ]]; then response=ACTION; else response=ABSENT; fi
printf '{"type":"thread.started","thread_id":"mutating-%s"}\n' "$n"
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"item-%s","type":"agent_message","text":"%s"}}\n' "$n" "$response"
printf '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}\n'
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" \
        PROBE_STUB_COUNT="$count" \
        PROBE_MUTATE_FILE="$SKILLS/demo-skill/SKILL.md" \
        bash "$HARNESS" --probe demo --live --reps 2 --fixtures fixtures-mutated

    [ "$status" -eq 2 ]
    [[ "$stderr" == *"live capture inputs changed during dispatch"* ]]
    [ ! -e "$PROBE_DIR/fixtures-mutated" ]
}

@test "scorecard output refuses to overwrite existing evidence" {
    make_fixture_set fixtures observed-model low ABSENT ACTION
    local scorecard="$BATS_TEST_TMPDIR/scorecard.json"
    printf 'sentinel\n' > "$scorecard"

    run --separate-stderr bash "$HARNESS" --probe demo --replay --output "$scorecard"

    [ "$status" -eq 2 ]
    [ "$(cat "$scorecard")" = "sentinel" ]
    [[ "$stderr" == *"refusing to overwrite immutable scorecard output"* ]]
}

@test "successful live capture publishes transcripts and manifest as one verified set" {
    local producer="$BATS_TEST_TMPDIR/codex-success"
    local count="$BATS_TEST_TMPDIR/success-count"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli success-stub\n'; exit 0; fi
n=$(cat "$PROBE_STUB_COUNT" 2>/dev/null || printf '0')
n=$((n + 1))
printf '%s\n' "$n" > "$PROBE_STUB_COUNT"
prompt="$(cat)"
if [[ "$prompt" == *PRELUDE_ACTION* ]]; then response=ACTION; else response=ABSENT; fi
printf '{"type":"thread.started","thread_id":"success-%s"}\n' "$n"
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"item-%s","type":"agent_message","text":"%s"}}\n' "$n" "$response"
printf '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}\n'
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" PROBE_STUB_COUNT="$count" \
        SKILL_PROBES_DIR="$PROBES" \
        bash "$HARNESS" --probe demo --live --capture --reps 2 \
        --model requested-live --effort high

    [ "$status" -eq 0 ]
    [ -f "$PROBE_DIR/fixtures/fixture-set.json" ]
    [ ! -e "$PROBE_DIR/fixtures/.capture-inputs" ]
    [ "$(json_field "$output" producer.model)" = "requested-live" ]
    [ "$(json_field "$output" producer.identity.source)" = "test-override" ]
    [ "$(json_field "$output" producer.identity.coverage_eligible)" = "false" ]
    [ "$(json_field "$output" requested_producer.model)" = "requested-live" ]
    [[ "$(json_field "$output" fixture_set.binding_sha256)" == sha256:* ]]
    [ "$(json_field "$output" evaluator_matches_capture)" = "true" ]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
    run python3 "$META_TOOL" verify \
        --fixture-dir "$PROBE_DIR/fixtures" \
        --probe-dir "$PROBE_DIR" \
        --skills-dir "$SKILLS" \
        --probe demo
    [ "$status" -eq 0 ]
}

@test "canonical-skill treatment mode injects the bound SKILL.md rather than the prelude" {
    python3 - "$PROBE_DIR/probe.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text())
value["treatment_source"] = "canonical-skill"
path.write_text(json.dumps(value))
PY
    rm "$PROBE_DIR/treatment-prelude.md"
    local producer="$BATS_TEST_TMPDIR/codex-canonical"
    local count="$BATS_TEST_TMPDIR/canonical-count"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli canonical-stub\n'; exit 0; fi
n=$(cat "$PROBE_STUB_COUNT" 2>/dev/null || printf '0')
n=$((n + 1))
printf '%s\n' "$n" > "$PROBE_STUB_COUNT"
prompt="$(cat)"
if [[ "$prompt" == *CANONICAL_ACTION* ]]; then response=ACTION; else response=ABSENT; fi
printf '{"type":"thread.started","thread_id":"canonical-%s"}\n' "$n"
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"item-%s","type":"agent_message","text":"%s"}}\n' "$n" "$response"
printf '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}\n'
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" \
        PROBE_STUB_COUNT="$count" \
        bash "$HARNESS" --probe demo --live --reps 2 --fixtures fixtures-canonical

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" treatment_source)" = "canonical-skill" ]
    [[ "$(json_field "$output" honesty)" == *"exact bound canonical SKILL.md treatment"* ]]
    [ "$(json_field "$output" verdict)" = "BEHAVIORAL" ]
    run python3 - "$PROBE_DIR/fixtures-canonical/capture-contract.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
raise SystemExit(0 if [item["path"] for item in value["capture_inputs"]] == [
    "probe.json", "question.md", "discriminator.sh"
] else 1)
PY
    [ "$status" -eq 0 ]
}

@test "transcript self-report cannot override bound runtime producer identity" {
    local producer="$BATS_TEST_TMPDIR/codex-identity-spoof"
    local count="$BATS_TEST_TMPDIR/count"
    cat > "$producer" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then printf 'codex-cli immutable-stub-version\n'; exit 0; fi
n=$(cat "$PROBE_STUB_COUNT" 2>/dev/null || printf '0')
n=$((n + 1))
printf '%s\n' "$n" > "$PROBE_STUB_COUNT"
cat >/dev/null
printf '{"type":"thread.started","thread_id":"spoof-%s","model":"self-reported-%s"}\n' "$n" "$n"
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"item-%s","type":"agent_message","text":"model: forged-model\\nreasoning effort: forged-effort\\nABSENT"}}\n' "$n"
printf '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}\n'
SH
    chmod +x "$producer"

    run --separate-stderr env CODEX_EXEC_BIN="$producer" PROBE_STUB_COUNT="$count" \
        SKILL_PROBES_DIR="$PROBES" \
        bash "$HARNESS" --probe demo --live --reps 2 --fixtures fixtures-identity-spoof \
        --model bound-model --effort low

    [ "$status" -eq 0 ]
    [ "$(json_field "$output" producer.model)" = "bound-model" ]
    [ "$(json_field "$output" producer.effort)" = "low" ]
    [ "$(json_field "$output" producer.identity.version)" = "codex-cli immutable-stub-version" ]
    [ "$(json_field "$output" producer.identity.source)" = "test-override" ]
    [ "$(json_field "$output" producer.identity.coverage_eligible)" = "false" ]
}

@test "replay DEGRADES a rep whose transcript reads a SKILL.md (skill-read-contamination)" {
    # The committed premortem tier-2 fixture is the real contaminated capture:
    # every rep fetched skills/premortem/SKILL.md off disk with exit 0. Under
    # the contamination rule the whole set collapses to UNMEASURED 0/0 usable.
    # setup() points the harness at a per-test sandbox; this case reads the
    # real committed fixture, so resolve the real repo trees.
    unset SKILL_PROBES_DIR SKILL_PROBE_SKILLS_DIR
    run --separate-stderr bash "$REPO_ROOT/scripts/probe-skill.sh" \
        --probe premortem-plan-shape-t2 --replay --fixtures fixtures-low-2026-08-26
    [ "$status" -eq 0 ]
    [[ "$stderr" == *"skill-read-contamination"* ]]
    [[ "$output" == *'"verdict": "UNMEASURED"'* ]]
}
