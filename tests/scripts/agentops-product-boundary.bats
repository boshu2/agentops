#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  ACTIVE_AUTHORITY=(
    AGENTS.md
    PRODUCT.md
    README.md
    docs/CI-CD.md
    docs/architecture/rpi-traversal.md
    cli/cmd/ao/root.go
    cli/internal/commands/demo/module.go
  )
}

scan_obsolete_authority() {
  local file="$1"
  grep -Eni \
    'ao[[:space:]]+(land|pawl|plan-pawl|done|close|governor|yield|claim|next-work|worktree|validate|converge|reconcile|membrane|crank)|/pawl-review|semantic[[:space:]]+pre-push|push-as-CI|Validate[[:space:]]+verdict[^[:cntrl:]]*(push|merge|release|delivery)|automatic[^[:cntrl:]]*(repair|retry|replan)' \
    "$file"
}

require_text() {
  grep -Fq -- "$2" "$REPO_ROOT/$1" || {
    echo "$1 is missing: $2" >&2
    return 1
  }
}

@test "active authority teaches one bounded experiment and stop" {
  local file
  for file in "${ACTIVE_AUTHORITY[@]}"; do
    run scan_obsolete_authority "$REPO_ROOT/$file"
    [ "$status" -eq 1 ] || {
      echo "$output" >&2
      return 1
    }
  done

  require_text AGENTS.md \
    "RPI -> Plan -> Implement -> fresh Validate -> report and stop"
  require_text AGENTS.md \
    "Persist \`verdict.v2\` only when"
  require_text PRODUCT.md \
    "AgentOps is not a new GitLab, CI service, tracker, merge queue, delivery system"
  require_text docs/architecture/rpi-traversal.md \
    "RPI invokes the anti-ceremony guard exactly once."
  require_text docs/CI-CD.md \
    "Repositories own delivery policy for local and cloud agents."
}

@test "RPI dependencies are exactly the anti-ceremony guard and core phases" {
  run python3 - "$REPO_ROOT" <<'PY'
from pathlib import Path
import sys
import yaml

root = Path(sys.argv[1])
actual = {}
for name in ("rpi", "plan", "implement", "validate"):
    data = yaml.safe_load((root / "skills" / name / "SKILL.md").read_text().split("---", 2)[1])
    actual[name] = set(data["metadata"]["dependencies"])
expected = {"rpi": {"anti-ceremony", "plan", "implement", "validate"}, "plan": set(), "implement": set(), "validate": set()}
if actual != expected:
    raise SystemExit(actual)
PY
  [ "$status" -eq 0 ]

  require_text skills/standards/references/skill-structure.md \
    "rpi -> anti-ceremony"
  require_text docs/contracts/skill-ports-and-adapters.md \
    "Only RPI depends on the anti-ceremony guard and all three core phases."
  require_text docs/reference/skill-system-evolution.md \
    "Anti-Ceremony, Plan, Implement, and Validate"
  require_text docs/architecture/rpi-traversal.md \
    "\`STOP\` dispatches none of Plan, Implement, or Validate"
  require_text docs/agent-workflow-reference.md \
    "\`CONTINUE\` creates no process artifact"
}

@test "validation worksheet requires fresh judgment and makes persistence optional" {
  require_text docs/templates/slice-validation.md \
    '- Validation result: `<PASS | FAIL | NOT_PROVEN>`'
  require_text docs/templates/slice-validation.md \
    '- Verdict artifact (optional): `<content-addressed path when requested>`'
  require_text docs/templates/slice-validation.md \
    "Validate returns one fresh result and stops"
  require_text docs/templates/slice-validation.md \
    "Persist \`verdict.v2\` only when"

  run grep -F "Validate persists one verdict" \
    "$REPO_ROOT/docs/templates/slice-validation.md"
  [ "$status" -eq 1 ]
}

@test "negative fixtures reject semantic Git authority and automatic continuation" {
  fixture="$BATS_TEST_TMPDIR/obsolete.md"
  printf '%s\n' \
    'Run ao land after the review.' \
    'Require semantic pre-push approval.' \
    'A FAIL triggers automatic repair.' >"$fixture"

  run scan_obsolete_authority "$fixture"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ao land"* ]]
  [[ "$output" == *"semantic pre-push"* ]]
  [[ "$output" == *"automatic repair"* ]]
}

# --- Operations-layer identity conformance (2026-08-07 alignment) -----------

# Current-authority surfaces that must carry the canonical category and must
# not re-frame AgentOps as a loop, control plane, orchestrator, or the retired
# knowledge-flywheel product. Historical records (CHANGELOG, MIGRATION, dated
# plans/audits/releases) are deliberately NOT scanned.
IDENTITY_AUTHORITY=(
  README.md
  PRODUCT.md
  GOALS.md
  PROGRAM.md
  AGENTS.md
  docs/index.md
  docs/agent-workflow-reference.md
  cli/README.md
  mkdocs.yml
  .claude-plugin/plugin.json
  .codex-plugin/plugin.json
  .goreleaser.yml
  cli/Formula/agentops.rb
)

scan_obsolete_identity() {
  local file="$1"
  grep -Eni \
    'AgentOps is (the|an?|your) (seven-move )?(operating loop|operating system|global control plane|execution orchestrator|software factory)|Knowledge Flywheel|knowledge-flywheel|ao[[:space:]]+flywheel|installs the corpus' \
    "$file"
}

@test "current authority carries the operations-layer category" {
  require_text README.md \
    "AgentOps is the operations layer for agentic engineering."
  require_text README.md \
    "federated integration graph"
  require_text PRODUCT.md \
    "operations layer for agentic engineering"
  require_text GOALS.md \
    "operations layer for agentic engineering"
  require_text AGENTS.md \
    "operations layer for agentic engineering"
  require_text AGENTS.md \
    "Standard RPI traversal"
  require_text AGENTS.md \
    "Federated source authority"
  require_text docs/contracts/ubiquitous-language.md \
    "Operations layer"
  require_text docs/contracts/ubiquitous-language.md \
    "Federated integration graph"
  require_text docs/contracts/ubiquitous-language.md \
    "Semantic work-and-proof protocol"
  require_text docs/contracts/ubiquitous-language.md \
    "RPI traversal"
  require_text docs/contracts/ubiquitous-language.md \
    "Forbidden conflations"
}

@test "current authority does not re-frame AgentOps as loop, flywheel, or orchestrator" {
  local file
  for file in "${IDENTITY_AUTHORITY[@]}"; do
    run scan_obsolete_identity "$REPO_ROOT/$file"
    [ "$status" -eq 1 ] || {
      echo "obsolete identity claim in $file:" >&2
      echo "$output" >&2
      return 1
    }
  done
}

@test "identity scan flags planted obsolete claims" {
  fixture="$BATS_TEST_TMPDIR/identity.md"
  printf '%s\n' \
    'AgentOps is the operating loop a coding agent follows.' \
    'Run ao flywheel status to see compounding.' \
    'One command installs the corpus into every agent.' \
    'The Knowledge Flywheel is the product.' >"$fixture"

  run scan_obsolete_identity "$fixture"
  [ "$status" -eq 0 ]
  [[ "$output" == *"operating loop"* ]]
  [[ "$output" == *"ao flywheel"* ]]
  [[ "$output" == *"installs the corpus"* ]]
  [[ "$output" == *"Knowledge Flywheel"* ]]
}

@test "identity scan stays quiet on legitimate third-party flywheel mentions" {
  fixture="$BATS_TEST_TMPDIR/legit.md"
  printf '%s\n' \
    'The Agentic Coding Flywheel is a supported external factory.' \
    'Use the using-flywheel skill to operate it.' >"$fixture"

  run scan_obsolete_identity "$fixture"
  [ "$status" -eq 1 ]
}

@test "ao init scaffolds only declared evidence destinations" {
  initapp="$REPO_ROOT/cli/internal/initapp/initapp.go"
  grep -Fq '"intents", "sha256"' "$initapp"
  grep -Fq '"verdicts", "sha256"' "$initapp"

  # The retired stores must not come back as scaffolding.
  run grep -En 'SessionsDir|IndexDir|ProvenanceDir|"handoff"' "$initapp"
  [ "$status" -eq 1 ]
}

@test "legacy operating-loop workflow is a routing tombstone" {
  wf="$REPO_ROOT/workflows/operating-loop.js"
  grep -Fq "throw new Error" "$wf"
  grep -Fq "workflows/rpi.js" "$wf"
  grep -Fq "ship-beads.js" "$wf"
  # No live seven-move dispatch remains.
  run grep -F "agent(" "$wf"
  [ "$status" -eq 1 ]
}

@test "documentation index projection rebuilds byte-identically in a temp fixture" {
  fixture="$BATS_TEST_TMPDIR/proj"
  mkdir -p "$fixture/scripts" "$fixture/docs"
  cp "$REPO_ROOT/scripts/generate-documentation-index.py" "$fixture/scripts/"
  cp -R "$REPO_ROOT/docs/contracts" "$fixture/docs/contracts"
  cp "$REPO_ROOT/docs/documentation-index.md" "$fixture/expected.md"

  # Delete the projection in the fixture and rebuild it from sources.
  rm -f "$fixture/docs/documentation-index.md"
  run python3 "$fixture/scripts/generate-documentation-index.py"
  [ "$status" -eq 0 ]
  run diff -u "$fixture/expected.md" "$fixture/docs/documentation-index.md"
  [ "$status" -eq 0 ]
}
