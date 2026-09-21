#!/usr/bin/env bash
# Current replacement for the removed membrane/prevention-ratchet integration.
# Real CLI provenance preserves observations; it does not promote policy or work.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOG_ROOT="${AGENTOPS_TEST_LOG_DIR:-${TMPDIR:-/tmp}}"
mkdir -p "$LOG_ROOT"
ARTIFACTS="$(mktemp -d "$LOG_ROOT/observation-provenance.XXXXXX")"
echo "Diagnostics: $ARTIFACTS"
AO_BIN="$ARTIFACTS/ao"
(cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao) > "$ARTIFACTS/build.log" 2>&1 || {
    cat "$ARTIFACTS/build.log" >&2
    exit 1
}
mkdir "$ARTIFACTS/project"
cd "$ARTIFACTS/project"
git init -q
mkdir -p docs evidence
printf 'Caller-owned policy stays unchanged.\n' > AGENTS.md
printf 'Source content.\n' > docs/guide.md
printf '{"status":"open"}\n' > tracker.json
for observation in objective-a-attempt-1 objective-a-attempt-2 objective-b-attempt-1; do
    printf '{"observation":"%s","finding":"same-defect"}\n' "$observation" > "evidence/$observation.json"
done
git add AGENTS.md docs/guide.md tracker.json evidence

# Retired calls refuse before creating either legacy state or a ledger.
for verb in catch digest status; do
    status=0
    "$AO_BIN" membrane "$verb" > "$ARTIFACTS/membrane-$verb.log" 2>&1 || status=$?
    [[ "$status" -eq 1 ]]
    grep -Fq 'record observations as verdict findings or generic provenance' "$ARTIFACTS/membrane-$verb.log"
    grep -Fq 'docs/MIGRATION.md' "$ARTIFACTS/membrane-$verb.log"
    [[ ! -e .agents && ! -e docs/provenance ]]
done
echo "PASS: removed membrane commands refuse with guidance and create no state"

# 'verdict' is a real node type. Observation identifiers/evidence remain caller data.
add_observation() {
    "$AO_BIN" provenance add "verdict:$1" docs/guide.md \
        --from-type verdict --to-type artifact --relation wasAttributedTo \
        --trust-tier authored --evidence "evidence/$1.json" --ts "$2" --json
}
add_observation objective-a-attempt-1 2026-09-21T00:00:00Z > "$ARTIFACTS/first.json"
cp docs/provenance/ledger.jsonl "$ARTIFACTS/before-retry.jsonl"
add_observation objective-a-attempt-1 2026-09-21T01:00:00Z > "$ARTIFACTS/retry.json"
cmp "$ARTIFACTS/first.json" "$ARTIFACTS/retry.json"
cmp "$ARTIFACTS/before-retry.jsonl" docs/provenance/ledger.jsonl
echo "PASS: exact source/evidence retry returns the existing edge without appending"

add_observation objective-a-attempt-2 2026-09-21T02:00:00Z > "$ARTIFACTS/second.json"
add_observation objective-b-attempt-1 2026-09-21T03:00:00Z > "$ARTIFACTS/third.json"
"$AO_BIN" provenance list --json > "$ARTIFACTS/list.json"
jq -e '
    length == 3
    and [.[].from_id] == ["verdict:objective-a-attempt-1", "verdict:objective-a-attempt-2", "verdict:objective-b-attempt-1"]
    and [.[].evidence_ref] == ["evidence/objective-a-attempt-1.json", "evidence/objective-a-attempt-2.json", "evidence/objective-b-attempt-1.json"]
    and all(.[]; .from_type == "verdict" and .to_type == "artifact"
        and .to_id == "docs/guide.md" and .relation == "wasAttributedTo" and .trust_tier == "authored")
    and .[1].prev_hash == .[0].hash and .[2].prev_hash == .[1].hash
' "$ARTIFACTS/list.json" > /dev/null
"$AO_BIN" provenance list --from-id verdict:objective-a-attempt-2 --json |
    jq -e 'length == 1 and .[0].evidence_ref == "evidence/objective-a-attempt-2.json"' > /dev/null
"$AO_BIN" provenance verify --json > "$ARTIFACTS/verified.json"
jq -e '.Pass == true and .RecordCount == 3 and .FirstBrokenLine == 0' "$ARTIFACTS/verified.json" > /dev/null
echo "PASS: distinct observations retain source/evidence identities in a valid chain"

# No recurrence reducer ships. Repeated findings only add ledger/lock files;
# they cannot acquire policy, producer-candidate or lifecycle authority here.
git diff --exit-code
[[ ! -e .agents && ! -e .beads ]]
find . -path './.git' -prune -o -type f -print | LC_ALL=C sort > "$ARTIFACTS/actual-files.txt"
cat > "$ARTIFACTS/expected-files.txt" <<'FILES'
./AGENTS.md
./docs/guide.md
./docs/provenance/ledger.jsonl
./docs/provenance/ledger.jsonl.lock
./evidence/objective-a-attempt-1.json
./evidence/objective-a-attempt-2.json
./evidence/objective-b-attempt-1.json
./tracker.json
FILES
cmp "$ARTIFACTS/expected-files.txt" "$ARTIFACTS/actual-files.txt"
echo "PASS: repeated findings do not mutate caller policy/work or emit producer candidates"

# Evidence pointers are opaque identities, not verification of referenced bytes.
# Tamper with a sealed identity and require the real verifier to reject the chain.
jq -c 'if input_line_number == 2 then .evidence_ref = "evidence/substituted.json" else . end' \
    docs/provenance/ledger.jsonl > "$ARTIFACTS/tampered.jsonl"
cp "$ARTIFACTS/tampered.jsonl" docs/provenance/ledger.jsonl
status=0
"$AO_BIN" provenance verify --json > "$ARTIFACTS/corrupt.json" 2> "$ARTIFACTS/corrupt.err" || status=$?
[[ "$status" -ne 0 ]]
jq -e '.Pass == false and .FirstBrokenLine == 2 and (.Message | length > 0)' "$ARTIFACTS/corrupt.json" > /dev/null
echo "PASS: corrupt evidence identity fails chain verification"
