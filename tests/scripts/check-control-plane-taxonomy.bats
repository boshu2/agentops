#!/usr/bin/env bats
# Tests for scripts/check-control-plane-taxonomy.sh (age-fey — the membrane on
# the control-plane docs). L2: the negatives copy the real docs to a tmp arch
# dir and mutate one invariant each, proving the gate bites.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-control-plane-taxonomy.sh"
    TMP_ARCH="$BATS_TEST_TMPDIR/architecture"
    mkdir -p "$TMP_ARCH"
    cp "$REPO_ROOT"/docs/architecture/*.md "$TMP_ARCH/"
}

@test "checker exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "green: real repo architecture docs pass" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "green: copied docs pass under AGENTOPS_ARCH_DIR override" {
    run env AGENTOPS_ARCH_DIR="$TMP_ARCH" bash "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "red: reintroducing 'bd/Dolt = etcd' store binding fails, naming the file" {
    printf '\nState store = bd/Dolt as the etcd-analog (data gravity).\n' >> "$TMP_ARCH/canonical-loop-model.md"
    run env AGENTOPS_ARCH_DIR="$TMP_ARCH" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"retired bd/Dolt store binding"* ]]
    [[ "$output" == *"canonical-loop-model.md"* ]]
}

@test "red: a retired-store line that DESCRIBES the retirement is not an offender" {
    printf '\nThe etcd-analog was bd/Dolt; that store binding is now retired.\n' >> "$TMP_ARCH/canonical-loop-model.md"
    run env AGENTOPS_ARCH_DIR="$TMP_ARCH" bash "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "red: dropping the two-altitude note while the agent is classified fails" {
    # Strip the reconciliation note from ports-and-adapters only.
    sed -i.bak 's/two altitudes/two scales/g; s/two-altitude/two-scale/g' "$TMP_ARCH/ports-and-adapters.md"
    run env AGENTOPS_ARCH_DIR="$TMP_ARCH" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"two-altitude reconciliation note"* ]]
}

@test "red: breaking a bidirectional cross-link fails" {
    # Remove the back-link from loop-map to the-agent-factory.
    sed -i.bak 's#the-agent-factory\.md#the-agent-FACTORY-MOVED.md#g' "$TMP_ARCH/loop-map.md"
    run env AGENTOPS_ARCH_DIR="$TMP_ARCH" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"loop-map.md does not link back"* ]]
}
