#!/usr/bin/env bats
# Presence + content test for the GC posture/boundary doc (soc-7blko).
# Asserts the canonical doc states GC is an adapter behind AgentOps ports,
# the managed-city guardrail with its flap rationale, and the sovereignty
# promise — and that the doc is registered in the docs index.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    DOC="$REPO_ROOT/docs/architecture/gc-posture.md"
    INDEX="$REPO_ROOT/docs/documentation-index.md"
}

# Case-insensitive fixed-string presence in the posture doc.
doc_has() {
    grep -qiF -- "$1" "$DOC"
}

@test "gc-posture doc exists" {
    [ -f "$DOC" ]
}

@test "states GC is an adapter behind AgentOps ports" {
    doc_has "adapter"
    doc_has "ports"
}

@test "states the adapter is opt-in and swappable" {
    doc_has "opt-in"
    doc_has "swappable"
}

@test "states the managed-city guardrail with the flap rationale" {
    doc_has "never"
    doc_has "managed-city"
    doc_has "1,478"
}

@test "asserts sovereignty: no cloud required and self-hostable/local" {
    doc_has "no cloud required"
    grep -qiE -- "self-hostable|local" "$DOC"
}

@test "doc is linked from the documentation index" {
    grep -qF -- "architecture/gc-posture.md" "$INDEX"
}
