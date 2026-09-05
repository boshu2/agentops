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

run_linked_reference_identity_check() {
  python3 - \
    "$REPO_ROOT/scripts/check-cathedral-cut-conformance.py" \
    "$1" <<'PY'
import importlib.util
from pathlib import Path
import sys

script = Path(sys.argv[1])
root = Path(sys.argv[2])
spec = importlib.util.spec_from_file_location("cathedral_cut", script)
module = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(module)

try:
    module.check_linked_skill_reference_identity(root)
except AssertionError as exc:
    print(exc)
    raise SystemExit(1)
PY
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
    "RPI -> Plan -> Implement -> fresh Validate -> repair to convergence -> report"
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

@test "linked skill references reject retired operations-layer terminology" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p \
    "$fixture/skills/example/references" \
    "$fixture/skills/unlinked/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Template](references/template.md)' \
    '- [Testing](references/testing.md)' \
    '- [Identity](references/identity.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Template' \
    '' \
    'Searchability was formerly described as a knowledge flywheel.' \
    >"$fixture/skills/example/references/template.md"
  printf '%s\n' \
    '# Testing' \
    '' \
    '## Operating-loop use' \
    >"$fixture/skills/example/references/testing.md"
  printf '%s\n' \
    '# Identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/identity.md"
  printf '%s\n' \
    '# Historical note' \
    '' \
    'The knowledge flywheel label appeared in this unlinked archive.' \
    >"$fixture/skills/unlinked/references/history.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"template.md:3"* ]]
  [[ "$output" == *"knowledge flywheel"* ]]
  [[ "$output" == *"testing.md:3"* ]]
  [[ "$output" == *"Operating-loop use"* ]]
  [[ "$output" == *"identity.md:3"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
  [[ "$output" != *"unlinked/references/history.md"* ]]
}

@test "linked skill reference scan permits external factory terminology" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Factory](references/factory.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# External factory' \
    '' \
    'The Agentic Coding Flywheel is a supported external factory.' \
    >"$fixture/skills/example/references/factory.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "reference-style skill links scan full collapsed and shortcut forms" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Policy][p]' \
    '- [Testing][]' \
    '- [Shortcut]' \
    '- [Identity][identity]' \
    '' \
    '[P]: references/policy.md "policy"' \
    '[testing]: <references/testing.md>' \
    '[shortcut]: references/shortcut.md' \
    '[identity]: references/identity.md' \
    '[Archive]: references/archive.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Policy' \
    '' \
    'Searchability was formerly described as a knowledge flywheel.' \
    >"$fixture/skills/example/references/policy.md"
  printf '%s\n' \
    '# Testing' \
    '' \
    '## Operating-loop use' \
    >"$fixture/skills/example/references/testing.md"
  printf '%s\n' \
    '# Shortcut' \
    '' \
    'The knowledge-flywheel label is retired.' \
    >"$fixture/skills/example/references/shortcut.md"
  printf '%s\n' \
    '# Identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/identity.md"
  printf '%s\n' \
    '# Unlinked archive' \
    '' \
    '## Operating-loop use' \
    >"$fixture/skills/example/references/archive.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"policy.md:3"* ]]
  [[ "$output" == *"testing.md:3"* ]]
  [[ "$output" == *"shortcut.md:3"* ]]
  [[ "$output" == *"identity.md:3"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
  [[ "$output" != *"archive.md"* ]]
}

@test "reference-style scan permits safe target and ignores unused archive definition" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Factory][factory]' \
    '' \
    '[factory]: references/factory.md' \
    '[archive]: references/archive.md "[archive]"' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# External factory' \
    '' \
    'The Agentic Coding Flywheel is a supported external factory.' \
    >"$fixture/skills/example/references/factory.md"
  printf '%s\n' \
    '# Historical archive' \
    '' \
    'The knowledge flywheel label appeared here.' \
    >"$fixture/skills/example/references/archive.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "duplicate reference definitions use the first destination" {
  bad_first="$BATS_TEST_TMPDIR/bad-first"
  safe_first="$BATS_TEST_TMPDIR/safe-first"
  mkdir -p \
    "$bad_first/skills/example/references" \
    "$safe_first/skills/example/references"

  printf '%s\n' \
    '# Bad first' \
    '' \
    '[Policy][p]' \
    '' \
    '[p]: references/bad.md' \
    '[p]: references/safe.md' \
    >"$bad_first/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe first' \
    '' \
    '[Policy][p]' \
    '' \
    '[p]: references/safe.md' \
    '[p]: references/bad.md' \
    >"$safe_first/skills/example/SKILL.md"

  for fixture in "$bad_first" "$safe_first"; do
    printf '%s\n' \
      '# Bad identity' \
      '' \
      'AgentOps is the operating loop every coding agent follows.' \
      >"$fixture/skills/example/references/bad.md"
    printf '%s\n' \
      '# External factory' \
      '' \
      'The Agentic Coding Flywheel is a supported external factory.' \
      >"$fixture/skills/example/references/safe.md"
  done

  run run_linked_reference_identity_check "$bad_first"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad.md:3"* ]]
  [[ "$output" != *"safe.md"* ]]

  run run_linked_reference_identity_check "$safe_first"
  [ "$status" -eq 0 ]
}

@test "reference-like syntax in code or escaped prose does not create links" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Factory][safe]' \
    '' \
    '`[Inline][bad]`' \
    '\[Escaped][bad]' \
    '' \
    '```markdown' \
    '[Fenced][bad]' \
    '[inside]: references/bad.md' \
    '```' \
    '' \
    '[safe]: references/safe.md' \
    '[bad]: references/bad.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# External factory' \
    '' \
    'The Agentic Coding Flywheel is a supported external factory.' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "unordered-list fenced code is excluded from links and prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Safe](references/safe.md)' \
    '' \
    '- ```markdown' \
    '  [Inactive](references/bad.md)' \
    '  ```' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe reference' \
    '' \
    '- ```text' \
    '  AgentOps is the operating loop every coding agent follows.' \
    '  ```' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "ordered-list fenced code is excluded from links and prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Safe](references/safe.md)' \
    '' \
    '1. ~~~markdown' \
    '   [Inactive](references/bad.md)' \
    '   ~~~' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe reference' \
    '' \
    '1. ~~~text' \
    '   The knowledge flywheel label is retired.' \
    '   ~~~' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "nested-list fenced code is excluded from links and prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Safe](references/safe.md)' \
    '' \
    '- Parent item' \
    '  1. ~~~markdown' \
    '     [Inactive](references/bad.md)' \
    '     ~~~' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe reference' \
    '' \
    '- Parent item' \
    '  1. ~~~text' \
    '     AgentOps is the operating loop every coding agent follows.' \
    '     ~~~' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "ordinary list prose remains active" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[List prose](references/list-prose.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# List prose' \
    '' \
    '- AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/list-prose.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"list-prose.md:3"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
}

@test "multiline HTML comments are excluded from links and prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Safe](references/safe.md)' \
    '' \
    '<!--' \
    '[Inactive](references/bad.md)' \
    '-->' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe reference' \
    '' \
    '<!--' \
    'AgentOps is the operating loop every coding agent follows.' \
    'The knowledge flywheel label is retired.' \
    '-->' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "raw HTML code containers are excluded from links and prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Safe](references/safe.md)' \
    '' \
    '<pre>' \
    '[Inactive](references/bad.md)' \
    '</pre>' \
    '<script>' \
    '[Inactive](references/bad.md)' \
    '</script>' \
    '<style>' \
    '[Inactive](references/bad.md)' \
    '</style>' \
    '<textarea>' \
    '[Inactive](references/bad.md)' \
    '</textarea>' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe reference' \
    '' \
    '<pre>' \
    'AgentOps is the operating loop every coding agent follows.' \
    '</pre>' \
    '<script>' \
    'The knowledge flywheel label is retired.' \
    '</script>' \
    '<style>' \
    'AgentOps is the operating loop every coding agent follows.' \
    '</style>' \
    '<textarea>' \
    'The knowledge flywheel label is retired.' \
    '</textarea>' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "backtick fence info rejects backticks while tilde info permits them" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Invalid backtick fence](references/invalid.md)' \
    '[Valid tilde fence](references/tilde.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Invalid backtick fence' \
    '' \
    '```bad`info' \
    'AgentOps is the operating loop every coding agent follows.' \
    '```' \
    >"$fixture/skills/example/references/invalid.md"
  printf '%s\n' \
    '# Valid tilde fence' \
    '' \
    '~~~bad`info' \
    'The knowledge flywheel label is retired.' \
    '~~~' \
    >"$fixture/skills/example/references/tilde.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"invalid.md:3"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
  [[ "$output" != *"tilde.md"* ]]
}

@test "escaped brackets in reference labels still resolve direct links" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Policy][foo\]]' \
    '' \
    '[foo\]]: references/bad.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad.md:3"* ]]
}

@test "reference definitions accept a destination on the next indented line" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Policy][foo]' \
    '' \
    '[foo]:' \
    '  references/bad.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad.md:3"* ]]
}

@test "multiline reference labels normalize whitespace and resolve" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Policy][foo' \
    'bar]' \
    '' \
    '[foo' \
    'bar]: references/bad.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad.md:3"* ]]
}

@test "nested brackets in reference link text do not hide the target" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[[Policy]][foo]' \
    '[Policy [nested]][foo]' \
    '' \
    '[foo]: references/bad.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad.md:3"* ]]
}

@test "angle destinations reject an unescaped opening angle" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Safe](<references/safe.md>)' \
    '[Invalid](<references/bad<identity.md>)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Safe reference' \
    '' \
    'The Agentic Coding Flywheel is an external factory.' \
    >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad<identity.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "backslash hard breaks do not split retired rendered prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Hard breaks](references/hard-breaks.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Hard breaks' \
    '' \
    'AgentOps is the\' \
    'operating loop every coding agent follows.' \
    '' \
    'The knowledge\' \
    'flywheel label is retired.' \
    >"$fixture/skills/example/references/hard-breaks.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
  [[ "$output" == *"knowledge flywheel"* ]]
}

@test "emphasis delimiters do not split retired rendered prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Emphasis](references/emphasis.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Emphasis' \
    '' \
    'AgentOps is the **operating loop** every coding agent follows.' \
    '' \
    'AgentOps is the operating _loop_ every coding agent follows.' \
    '' \
    'The knowledge *flywheel* label is retired.' \
    >"$fixture/skills/example/references/emphasis.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
  [[ "$output" == *"knowledge flywheel"* ]]
}

@test "rendered prose normalization decodes inline Markdown and HTML" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '[Rendered prose](references/rendered.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Rendered prose' \
    '' \
    'AgentOps is the operating\-loop every coding agent follows.' \
    '' \
    'The knowledge&#32;flywheel label is retired.' \
    '' \
    'AgentOps is the [operating loop](https://example.com) every coding agent follows.' \
    '' \
    'AgentOps is the <em>operating loop</em> every coding agent follows.' \
    >"$fixture/skills/example/references/rendered.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"operating-loop"* ]]
  [[ "$output" == *"knowledge flywheel"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
}

@test "inline skill links accept balanced and escaped destination parentheses" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Balanced](references/bad_(balanced).md)' \
    '- [Escaped](references/bad_\(escaped\).md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Balanced destination' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad_(balanced).md"
  printf '%s\n' \
    '# Escaped destination' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/bad_(escaped).md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad_(balanced).md:3"* ]]
  [[ "$output" == *"bad_(escaped).md:3"* ]]
}

@test "local skill links decode percent-encoded filename characters once" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Dot](references/bad%2emd)' \
    '- [Space](references/bad%20identity.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Encoded dot' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"
  printf '%s\n' \
    '# Encoded space' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/bad identity.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bad.md:3"* ]]
  [[ "$output" == *"bad identity.md:3"* ]]
}

@test "percent decoding cannot introduce separators NUL or traversal" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Encoded slash](references%2fbad.md)' \
    '- [Encoded backslash](references%5cbad.md)' \
    '- [Encoded NUL](references/bad.md%00)' \
    '- [Encoded traversal](references/%2e%2e/outside.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"
  printf '%s\n' \
    '# Outside references' \
    '' \
    'The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/outside.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "indented CommonMark code does not link or trigger prose identity" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  {
    printf '%s\n' '# Example' '' '[Safe](references/safe.md)' ''
    printf '    [Indented](references/bad.md)\n'
    printf '\t[Tabbed](references/bad.md)\n'
  } >"$fixture/skills/example/SKILL.md"
  {
    printf '%s\n' '# Safe reference' ''
    printf '    AgentOps is the operating loop every coding agent follows.\n'
    printf '\tThe knowledge flywheel label is retired.\n'
  } >"$fixture/skills/example/references/safe.md"
  printf '%s\n' \
    '# Bad identity' \
    '' \
    'AgentOps is the operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/bad.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "linked prose scan catches forbidden Setext headings" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Setext](references/setext.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Reference' \
    'Operating-loop use' \
    '==================' \
    >"$fixture/skills/example/references/setext.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"setext.md:2"* ]]
  [[ "$output" == *"Operating-loop use"* ]]
}

@test "indented continuation remains prose when it cannot interrupt a paragraph" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Continuation](references/continuation.md)' \
    >"$fixture/skills/example/SKILL.md"
  {
    printf '%s\n' '# Continuation' '' 'AgentOps is the'
    printf '    operating loop every coding agent follows.\n'
  } >"$fixture/skills/example/references/continuation.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"continuation.md:3"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
}

@test "blockquoted indented CommonMark code is excluded from prose" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Quoted code](references/quoted-code.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Quoted code' \
    '' \
    '>     AgentOps is the operating loop every coding agent follows.' \
    '>     The knowledge flywheel label is retired.' \
    >"$fixture/skills/example/references/quoted-code.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 0 ]
}

@test "linked prose scan catches forbidden blockquoted Setext headings" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Quoted Setext](references/quoted-setext.md)' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Reference' \
    '> Operating-loop use' \
    '> ==================' \
    >"$fixture/skills/example/references/quoted-setext.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"quoted-setext.md:2"* ]]
  [[ "$output" == *"Operating-loop use"* ]]
}

@test "linked prose scan catches obsolete identities across wrapped lines" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/skills/example/references"
  printf '%s\n' \
    '# Example' \
    '' \
    '- [Identity](references/identity.md)' \
    '- [Flywheel][flywheel]' \
    '' \
    '[flywheel]: references/flywheel.md' \
    >"$fixture/skills/example/SKILL.md"
  printf '%s\n' \
    '# Identity' \
    '' \
    'AgentOps is the' \
    'operating loop every coding agent follows.' \
    >"$fixture/skills/example/references/identity.md"
  printf '%s\n' \
    '# Flywheel' \
    '' \
    'The retired knowledge' \
    'flywheel framing should not return.' \
    >"$fixture/skills/example/references/flywheel.md"

  run run_linked_reference_identity_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"identity.md:3"* ]]
  [[ "$output" == *"AgentOps is the operating loop"* ]]
  [[ "$output" == *"flywheel.md:3"* ]]
  [[ "$output" == *"knowledge flywheel"* ]]
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

# --- workflows/rpi.js: executable in-memory probes -------------------------
#
# rpi.js is a Workflow script, not a module: its only capabilities are the
# harness globals `args`, `agent`, `phase`, and `log`, and it ends in a
# top-level `return`. The probe below rebuilds it as an AsyncFunction over
# exactly those four names and drives it with a SCRIPTED agent queue, so the
# traversal's control flow (the convergence law, the tie-break, the premortem
# gate) is executed rather than grepped. Nothing here spawns a real agent.

write_rpi_probe() {
  cat >"$BATS_TEST_TMPDIR/rpi-probe.mjs" <<'JS'
import { readFileSync } from 'node:fs';

const source = readFileSync(process.env.RPI_SRC, 'utf8').replace('export const meta', 'const meta');
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
const traversal = new AsyncFunction('args', 'agent', 'phase', 'log', source);

const queue = JSON.parse(process.env.RPI_AGENTS);
const calls = [];
const prompts = {};
const agent = async (prompt, options) => {
  const label = options && options.label;
  const next = queue.shift();
  if (!next) throw new Error('unexpected agent call: ' + label);
  if (next.label !== label) {
    throw new Error('agent call order: got ' + label + ', expected ' + next.label);
  }
  calls.push(label);
  prompts[label] = prompt;
  return Object.prototype.hasOwnProperty.call(next, 'result') ? next.result : null;
};

const result = await traversal(JSON.parse(process.env.RPI_INPUT), agent, () => {}, () => {});
process.stdout.write(JSON.stringify({ result, calls, unusedResponses: queue.length, prompts }));
JS
}

rpi_probe() {
  write_rpi_probe
  RPI_SRC="$REPO_ROOT/workflows/rpi.js" RPI_INPUT="$1" RPI_AGENTS="$2" \
    node "$BATS_TEST_TMPDIR/rpi-probe.mjs" >"$BATS_TEST_TMPDIR/probe.json"
}

PLAN_RESULT='{"label":"plan","result":{"acceptance":"the behavior holds","writeScope":["tests/**"],"intentDigest":"aaaa","intentPath":"/tmp/i.intent"}}'
PLAN_RESULT_SAFE='{"label":"plan","result":{"acceptance":"the behavior holds","writeScope":["docs/**"],"intentDigest":"aaaa","intentPath":"/tmp/i.intent"}}'
IMPL_RESULT='{"label":"implement","result":{"contextId":"author-1","changedPaths":["docs/a.md"],"checkReceipts":[{"name":"repro","command":"true","outcome":"green"}],"filesSummary":"note"}}'
IMPL_RESULT_RISKY='{"label":"implement","result":{"contextId":"author-1","changedPaths":["tests/a.bats"],"checkReceipts":[{"name":"repro","command":"true","outcome":"green"}],"filesSummary":"note"}}'
ORPHANS_ABSENT='{"label":"orphans","result":{"scriptPresent":false}}'

@test "rpi.js premortem blocks a risky write scope before any Implement" {
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds"}' \
    "[$PLAN_RESULT,{\"label\":\"premortem\",\"result\":{\"blocking\":[{\"id\":\"seal:unpinned\",\"class\":\"seal.pinning\",\"summary\":\"the plan never pins the seal\"}],\"notes\":[\"nit\"]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert probe["calls"] == ["plan", "premortem"], probe["calls"]
result = probe["result"]
assert result["verdict"] == "NOT_PLANNED", result["verdict"]
assert result["premortem"]["status"] == "blocked", result["premortem"]
assert [f["id"] for f in result["findings"]] == ["seal:unpinned"], result["findings"]
assert result["verdictPath"] is None
# The judge must see the frozen plan, not a summary of it.
assert "the behavior holds" in probe["prompts"]["premortem"]
assert "tests/**" in probe["prompts"]["premortem"]
PY
}

@test "rpi.js records a caller-declared premortem skip and implements anyway" {
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","repairRounds":0}' \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"tests/a.bats\"]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "premortem" not in probe["calls"], probe["calls"]
result = probe["result"]
assert result["premortem"]["status"] == "skipped", result["premortem"]
# A skipped premortem never launders the risky-surface diversity rule.
assert result["verdict"] == "NOT_PROVEN", result["verdict"]
assert result["orphanedEvidence"] is None
assert result["orphanedEvidenceReason"] == "script-absent"
PY
}

@test "rpi.js attaches the orphaned-evidence receipt when the script is present" {
  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","repairRounds":0}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,{\"label\":\"orphans\",\"result\":{\"scriptPresent\":true,\"json\":\"{\\\"orphans\\\":[\\\"receipt-1\\\"]}\"}},{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"docs/a.md\"]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
result = probe["result"]
assert result["orphanedEvidence"] == {"orphans": ["receipt-1"]}, result["orphanedEvidence"]
assert result["orphanedEvidenceReason"] is None, result["orphanedEvidenceReason"]
assert result["verdict"] == "PASS", result["verdict"]
assert "scripts/evidence-orphans.sh" in probe["prompts"]["orphans"]
PY
}

@test "rpi.js stops repair when a new finding reopens a closed class" {
  validate_round() {
    printf '{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"%s","criteria":[],"validatorContextId":"v","findings":[%s],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md"]}}' "$1" "$2"
  }
  local r0 r1 r2
  r0="$(validate_round aa '{"id":"f1","class":"seal.pinning","summary":"seal not pinned"}')"
  r1="$(validate_round bb '{"id":"f2","summary":"other defect"}')"
  r2="$(validate_round cc '{"id":"f3","class":"seal.pinning","summary":"seal pinned at the wrong layer"}')"
  local repair='{"label":"repair:1","result":{"contextId":"author-2","changedPaths":["docs/a.md"],"checkReceipts":[],"filesSummary":"n"}}'
  local repair2='{"label":"repair:2","result":{"contextId":"author-3","changedPaths":["docs/a.md"],"checkReceipts":[],"filesSummary":"n"}}'

  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","repairRounds":3}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,$ORPHANS_ABSENT,$r0,$repair,$r1,$repair2,$r2]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
result = probe["result"]
assert result["verdict"] == "NOT_PROVEN", result["verdict"]
assert "class" in result["stoppedBy"], result["stoppedBy"]
assert "seal.pinning" in result["stoppedBy"], result["stoppedBy"]
assert "f3" in result["stoppedBy"], result["stoppedBy"]
assert result["repairRoundsUsed"] == 2, result["repairRoundsUsed"]
assert result["converged"] is False
PY
}

@test "rpi.js honors a declared binding judge and carries the dissent" {
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","bindingJudge":"primary","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"tests/a.bats\"]}},{\"label\":\"validate\",\"result\":{\"verdict\":\"FAIL\",\"verdictPath\":null,\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v2\",\"findings\":[{\"id\":\"x:y\",\"summary\":\"cross-family objection\"}],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"tests/a.bats\"]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "council" not in probe["calls"], probe["calls"]
result = probe["result"]
assert result["verdict"] == "PASS", result["verdict"]
assert result["tieBreak"] == "bindingJudge:primary", result["tieBreak"]
assert result["findings"] == [], result["findings"]
dissent = result["dissent"]
assert len(dissent) == 1 and dissent[0]["verdict"] == "FAIL", dissent
assert [f["id"] for f in dissent[0]["findings"]] == ["x:y"], dissent
# A split never exposes one leg's persisted verdict file as the aggregate.
assert result["verdictPath"] is None
PY
}

@test "rpi.js convenes one council on an undeclared split over a risky surface" {
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"tests/a.bats\"]}},{\"label\":\"validate\",\"result\":{\"verdict\":\"FAIL\",\"verdictPath\":null,\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v2\",\"findings\":[{\"id\":\"x:y\",\"summary\":\"cross-family objection\"}],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"tests/a.bats\"]}},{\"label\":\"council\",\"result\":{\"verdict\":\"FAIL\",\"reason\":\"the objection reproduces\"}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert probe["calls"].count("council") == 1, probe["calls"]
result = probe["result"]
assert result["verdict"] == "FAIL", result["verdict"]
assert result["council"]["verdict"] == "FAIL", result["council"]
assert result["council"]["reason"] == "the objection reproduces", result["council"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
# The council rules between the two legs; it never mints a third verdict.
prompt = probe["prompts"]["council"]
assert "PASS" in prompt and "FAIL" in prompt
assert "cross-family objection" in prompt
PY
}

@test "rpi.js keeps worst-of on a non-risky split and never convenes a council" {
  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,$ORPHANS_ABSENT,{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"docs/a.md\"]}},{\"label\":\"validate\",\"result\":{\"verdict\":\"FAIL\",\"verdictPath\":null,\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v2\",\"findings\":[{\"id\":\"x:y\",\"summary\":\"single-family objection\"}],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"docs/a.md\"]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "council" not in probe["calls"], probe["calls"]
result = probe["result"]
assert result["verdict"] == "FAIL", result["verdict"]
assert result["dissent"] is None, result["dissent"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
PY
}
