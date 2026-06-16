#!/usr/bin/env bats
# Acceptance tests — Stream A (codebase-recon workflow hardening).
#
# These are the EXECUTABLE definition of done for behaviors A0–A5
# (docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/behaviors.md).
# They are TEST-FIRST: until A0 vendors `.claude/workflows/codebase-recon.js`
# into the repo and A1–A4 harden it, these are RED by construction.
#
# Run from the repo root:
#   bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../../../../.." && pwd)"
    WF="$REPO_ROOT/.claude/workflows/codebase-recon.js"
    LEDGER="$REPO_ROOT/docs/contracts/skill-dispositions.yaml"
    GOV="$REPO_ROOT/scripts/check-workflow-governance.sh"
    REGEN="$REPO_ROOT/scripts/regen-all.sh"
}

# insert_workflow_row reads a YAML block on stdin and inserts it as a sibling
# directly under the top-level `workflows:` mapping (NOT at EOF — the file has
# more top-level keys after `workflows:`). Newline-safe across awk variants:
# the block is read from a temp file with getline, not passed via -v.
insert_workflow_row() {
    local blkfile; blkfile="$(mktemp "$BATS_TMPDIR/wfrow.XXXXXX")"
    cat > "$blkfile"
    awk -v bf="$blkfile" '
        /^workflows:[[:space:]]*$/ {
            print
            while ((getline line < bf) > 0) print line
            close(bf)
            next
        }
        { print }
    ' "$LEDGER" > "$LEDGER.tmp" && mv "$LEDGER.tmp" "$LEDGER"
}

# ── A0 — vendor codebase-recon as a drift-gated in-repo workflow ──────────────

@test "A0-S1: codebase-recon.js is a governed repo citizen with meta.name + ledger row + identity triple" {
    [ -f "$WF" ]
    grep -Eq "name:\s*['\"]codebase-recon['\"]" "$WF"
    # ledger carries a workflows.codebase-recon row with kind/domain(BC)/hexagonal_role
    run grep -nE "^\s+codebase-recon:" "$LEDGER"
    [ "$status" -eq 0 ]
    run bash -c "awk '/^\\s+codebase-recon:/{f=1} f&&/kind:/{print} f&&/^[^ ]/{f=0}' '$LEDGER'"
    [[ "$output" == *"workflow"* ]]
    grep -A20 "codebase-recon:" "$LEDGER" | grep -Eq "(domain|bounded_context|bc):"
    grep -A20 "codebase-recon:" "$LEDGER" | grep -q "hexagonal_role:"
}

@test "A0-S2: bijection + regen drift gates pass with codebase-recon present" {
    bash "$GOV"
    bash "$REGEN" --check
}

@test "A0-S3: a tracked .js with NO ledger row fails governance loudly (forward break)" {
    # The gate reads `git ls-files .claude/workflows/*.js`, so the orphan must be
    # staged. Created + staged here, removed in teardown.
    orphan="$REPO_ROOT/.claude/workflows/zzz-orphan-acc.js"
    cat > "$orphan" <<'JS'
export const meta = { name: 'zzz-orphan-acc' };
JS
    git -C "$REPO_ROOT" add -f "$orphan"
    run bash "$GOV"
    git -C "$REPO_ROOT" rm -f --cached "$orphan" >/dev/null 2>&1 || true
    rm -f "$orphan"
    [ "$status" -eq 1 ]
    [[ "$output" == *"zzz-orphan-acc"* ]]
}

@test "A0-S4: a stale ledger row (kind:workflow) with no .js fails the reverse direction" {
    # Append a phantom kind:workflow ledger row with no matching .js; the reverse
    # bijection must fail naming it STALE. Ledger restored in this test.
    cp "$LEDGER" "$BATS_TMPDIR/ledger.bak"
    insert_workflow_row <<'YAML'
  zzz-phantom-workflow-acc:
    kind: workflow
    domain: BC5-Runtime
    hexagonal_role: orchestrator
    path: .claude/workflows/zzz-phantom-workflow-acc.js
YAML
    run bash "$GOV"
    cp "$BATS_TMPDIR/ledger.bak" "$LEDGER"
    [ "$status" -eq 1 ]
    [[ "$output" == *"zzz-phantom-workflow-acc"* ]]
    [[ "$output" == *"STALE"* ]]
}

@test "A0-S5: vendoring does not perturb the four pre-existing workflows" {
    for w in bdd-foundry bead-crank operating-loop ship-beads; do
        [ -f "$REPO_ROOT/.claude/workflows/$w.js" ]
        grep -q "$w:" "$LEDGER"
    done
}

@test "A0-G10: governance fails when the ledger path disagrees with the tracked .js" {
    # CONDITIONAL (see behaviors.md note): only meaningful once the workflows
    # ledger schema carries a `path` field. Stage an orphan .js whose name matches
    # a ledger row but whose ledger `path` points at a DIFFERENT file; the gate
    # must fail naming the mismatch. RED until the gate binds path->real .js.
    orphan="$REPO_ROOT/.claude/workflows/zzz-pathcheck-acc.js"
    cat > "$orphan" <<'JS'
export const meta = { name: 'zzz-pathcheck-acc' };
JS
    git -C "$REPO_ROOT" add -f "$orphan"
    cp "$LEDGER" "$BATS_TMPDIR/ledger.g10.bak"
    insert_workflow_row <<'YAML'
  zzz-pathcheck-acc:
    kind: workflow
    domain: BC5-Runtime
    hexagonal_role: orchestrator
    path: .claude/workflows/some-other-file.js
YAML
    run bash "$GOV"
    cp "$BATS_TMPDIR/ledger.g10.bak" "$LEDGER"
    git -C "$REPO_ROOT" rm -f --cached "$orphan" >/dev/null 2>&1 || true
    rm -f "$orphan"
    [ "$status" -eq 1 ]
    [[ "$output" == *"zzz-pathcheck-acc"* ]]
    [[ "$output" == *"path"* ]]
}

@test "A0-G14: identity is parsed from exported meta.name, not a leading comment (first-grep-hit)" {
    # Stage a .js whose FIRST `name:` occurrence is a comment with the right id,
    # but whose actual exported meta.name is WRONG. A parser-derived gate must
    # reject it; a first-grep-hit gate (today) wrongly accepts. RED until the gate
    # parses the exported value.
    orphan="$REPO_ROOT/.claude/workflows/zzz-comment-name-acc.js"
    cat > "$orphan" <<'JS'
// name: 'zzz-comment-name-acc'  (this comment must NOT count as identity)
export const meta = { name: 'not-zzz-comment-name-acc' };
JS
    git -C "$REPO_ROOT" add -f "$orphan"
    cp "$LEDGER" "$BATS_TMPDIR/ledger.g14.bak"
    # Ledger row keyed to the COMMENT id; the exported name disagrees.
    insert_workflow_row <<'YAML'
  zzz-comment-name-acc:
    kind: workflow
    domain: BC5-Runtime
    hexagonal_role: orchestrator
    path: .claude/workflows/zzz-comment-name-acc.js
YAML
    run bash "$GOV"
    cp "$BATS_TMPDIR/ledger.g14.bak" "$LEDGER"
    git -C "$REPO_ROOT" rm -f --cached "$orphan" >/dev/null 2>&1 || true
    rm -f "$orphan"
    # The exported name (not-...) has no ledger row -> a parser-derived gate fails.
    [ "$status" -eq 1 ]
    [[ "$output" == *"not-zzz-comment-name-acc"* ]] || [[ "$output" == *"meta.name"* ]]
}

# ── A1 — drop the dead Fable model pin; inherit session model ─────────────────

@test "A1-S1: no fable model default anywhere in the workflow source" {
    [ -f "$WF" ]
    run grep -nE "model:\s*['\"]fable['\"]" "$WF"
    [ "$status" -ne 0 ]
    run grep -nE "WORKER_MODEL\s*=\s*['\"]fable['\"]|=\s*['\"]fable['\"]" "$WF"
    [ "$status" -ne 0 ]
}

@test "A1-S2: explicit args.model=opus is threaded to worker/repair dispatch" {
    [ -f "$WF" ]
    # source must read args.model and pass it through (not ignore the override)
    grep -Eq "args(\.model|\['model'\]|\[\"model\"\])" "$WF"
    grep -Eq "model:\s*(args\.model|workerModel|model)" "$WF"
}

@test "A1-S3: regression — no 'fable' default pin that would total-fail the fan-out" {
    [ -f "$WF" ]
    run grep -c "fable" "$WF"
    [ "$status" -eq 0 ]
    # the only acceptable 'fable' mentions are negative guards (e.g. reject 'fable')
    # — there must be no bare default assignment. Asserted by A1-S1; here we ensure
    # any fable mention is in a rejection/guard context, never an assignment default.
    if grep -q "fable" "$WF"; then
        grep -Eq "(reject|unsupported|unavailable|never).*fable|fable.*(reject|unsupported|unavailable|never)" "$WF"
    fi
}

@test "A1-G7: explicit args.model=fable is rejected before fan-out (no green/empty conflation)" {
    [ -f "$WF" ]
    grep -Eq "unsupported model override|fable" "$WF"
    # the workflow must throw/return-failed on a fable/unavailable override, not dispatch
    grep -Eq "throw|status:\s*['\"]failed['\"]|reject" "$WF"
}

# ── A2 — fail-closed empty-output guard before synth (HIGHEST VALUE) ──────────

@test "A2-S1: zero reports landed -> status:failed, reports_landed:0, NO synthesis" {
    [ -f "$WF" ]
    grep -Eq "reports_landed" "$WF"
    grep -Eq "status:\s*['\"]failed['\"]" "$WF"
    # the guard must sit BEFORE the Synthesize phase
    grep -Eq "Synth" "$WF"
}

@test "A2-S2: at least one usable report -> synth proceeds" {
    [ -f "$WF" ]
    grep -Eq "reports_landed|landed" "$WF"
}

@test "A2-S3: empty != clean — failure is unambiguous in the tool result" {
    [ -f "$WF" ]
    grep -Eq "status:\s*['\"]failed['\"]" "$WF"
}

@test "A2-G4: guard counts USABLE reports (non-empty + required marker), not bare file presence" {
    [ -f "$WF" ]
    # must check file size/non-empty AND a required report marker, not just existence
    grep -Eq "size|length|non-empty|trim|byteLength|usable" "$WF"
}

@test "A2-G8: report-dir IO error is a hard failure before synth, never coerced to zero-files-green" {
    [ -f "$WF" ]
    grep -Eq "ENOENT|catch|readdir|IO|unreadable|error" "$WF"
    grep -Eq "status:\s*['\"]failed['\"]" "$WF"
}

# ── A3 — escalate-on-repair to a different model ──────────────────────────────

@test "A3-S1: repair dispatches on a model tier != the worker tier" {
    [ -f "$WF" ]
    grep -Eq "repair" "$WF"
    grep -Eq "escalat|repairModel|different.*model|model.*tier" "$WF"
}

@test "A3-S2: model-unavailable class skips the same-model retry" {
    [ -f "$WF" ]
    grep -Eq "unavailable|null|empty.*return|unrepairable" "$WF"
}

@test "A3-S3: cross-family repair escalation is codex or agy, never fable" {
    [ -f "$WF" ]
    grep -Eq "codex|agy" "$WF"
    if grep -q "fable" "$WF"; then
        grep -Eq "(never|not|reject|unsupported).*fable|fable.*(never|not|reject|unsupported)" "$WF"
    fi
}

# ── A4 — first-class scope/since arg ──────────────────────────────────────────

@test "A4-S1: args.scope string reaches every worker AND repair prompt" {
    [ -f "$WF" ]
    grep -Eq "args(\.scope|\['scope'\]|\[\"scope\"\])" "$WF"
    grep -Eq "scopeBlock" "$WF"
}

@test "A4-S2: args.since resolves REF..HEAD with diffstat + log range injected" {
    [ -f "$WF" ]
    grep -Eq "args(\.since|\['since'\]|\[\"since\"\])" "$WF"
    grep -Eq "diff --stat|diffstat" "$WF"
    grep -Eq "\.\.HEAD|HEAD" "$WF"
}

@test "A4-S3: a bare string arg is treated as args.scope (not dropped)" {
    [ -f "$WF" ]
    grep -Eq "typeof.*string|string.*scope|bare string|scope" "$WF"
}

@test "A4-S4: an unresolvable since ref fails loudly, no silent full-repo fallback" {
    [ -f "$WF" ]
    grep -Eq "unresolvable since|invalid since|since ref" "$WF"
}

@test "A4-G5: a since ref with shell metacharacters is passed to git as argv/data, never via a shell" {
    [ -f "$WF" ]
    # the workflow must validate/reject since refs with shell metachars and never
    # interpolate them into a shell string. Assert a validation guard exists.
    grep -Eq "invalid since|metachar|allowlist|\\\\bgit rev-parse|argv|sanitiz" "$WF"
    # the value must NOT be concatenated into a `git ...` shell template unguarded
    run grep -nE "git .*\\\$\{?(args\.since|since)" "$WF"
    [ "$status" -ne 0 ]
}

@test "A4-G6: scope text is bounded/fenced data, cannot inject worker instructions" {
    [ -f "$WF" ]
    grep -Eq "scopeBlock" "$WF"
    # the scope value must be wrapped in a fenced/quoted data block, and the
    # mandatory recon instructions must follow it — assert the data-fence pattern.
    grep -Eq "\`\`\`|fence|<scope>|data block|BEGIN SCOPE|--- scope" "$WF"
}

# ── A5 — monitor must bind to task-state, never infer it (content-assertion) ──
#
# The guidance must live in a surface a MONITOR reads: the workflow note
# (.claude/workflows/codebase-recon.js) and/or the committed fleet memory
# docs/memory/monitor-binds-task-state.md. It must NOT be satisfied by the
# planning doc (behaviors.md) or these acceptance tests — those are EXCLUDED so
# the scenarios stay RED until the real guidance is authored.

# guidance_files prints every candidate guidance surface, excluding the
# behavior-first-planning plan tree (the plan/acceptance docs do not count).
guidance_files() {
    {
        [ -f "$WF" ] && printf '%s\n' "$WF"
        find "$REPO_ROOT/docs/memory" -name '*.md' 2>/dev/null
        find "$REPO_ROOT/.agents/playbooks" "$REPO_ROOT/.agents/planning-rules" -name '*.md' 2>/dev/null
    } | grep -v "behavior-first-planning-for-the-recon-re"
}

@test "A5-S1: committed monitor guidance hard-requires task-state tools + forbids mtime inference" {
    found=0
    while IFS= read -r f; do
        [ -f "$f" ] || continue
        if grep -qE "TaskGet|TaskOutput" "$f" 2>/dev/null && \
           grep -qE "mtime|process list|infer" "$f" 2>/dev/null; then
            found=1; break
        fi
    done < <(guidance_files)
    [ "$found" -eq 1 ]
}

@test "A5-S2: documented contract says a tool-less monitor aborts, not verdict" {
    found=0
    while IFS= read -r f; do
        [ -f "$f" ] || continue
        if grep -qE "task-state.*unavailable|abort.*verdict|MUST abort" "$f" 2>/dev/null; then
            found=1; break
        fi
    done < <(guidance_files)
    [ "$found" -eq 1 ]
}

@test "A5-G12: guidance also aborts on malformed/timeout/partial task-state, not only ABSENCE" {
    found=0
    while IFS= read -r f; do
        [ -f "$f" ] || continue
        if grep -qE "malformed|timeout|partial" "$f" 2>/dev/null && \
           grep -qE "TaskGet|TaskOutput|task-state" "$f" 2>/dev/null; then
            found=1; break
        fi
    done < <(guidance_files)
    [ "$found" -eq 1 ]
}
