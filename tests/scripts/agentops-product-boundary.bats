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
# traversal's control flow (the convergence law, the tie-break disposition, the
# premortem gate) is executed rather than grepped. Nothing here spawns a real
# agent.

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

# The traversal REFUSES a malformed validate leg by throwing, exactly as the
# Python reference behavior raises. The probe process dies with it.
rpi_probe_refused() {
  write_rpi_probe
  run env RPI_SRC="$REPO_ROOT/workflows/rpi.js" RPI_INPUT="$1" RPI_AGENTS="$2" \
    node "$BATS_TEST_TMPDIR/rpi-probe.mjs"
  [ "$status" -ne 0 ]
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
# NOT_PLANNED is a STATUS, not a verdict: nothing was built, so nothing was judged.
assert result["status"] == "NOT_PLANNED", result["status"]
assert result["verdict"] is None, result["verdict"]
assert result["stopReason"] == "premortem_blocking", result["stopReason"]
assert result["premortem"]["status"] == "blocking", result["premortem"]
assert [f["id"] for f in result["findings"]] == ["seal:unpinned"], result["findings"]
assert result["verdictPath"] is None
# The judge must see the frozen plan, not a summary of it.
assert "the behavior holds" in probe["prompts"]["premortem"]
assert "tests/**" in probe["prompts"]["premortem"]
PY
}

@test "rpi.js classifies a declared write scope by glob intersection, not path matching" {
  # `cli/**` matches none of the risky-path regexes as literal text, yet it
  # authorizes every gate in cli/internal/gates/. A bare `**` authorizes the
  # repository. Both must buy a premortem.
  # Witness intersection alone is blind to any pattern NARROWER than the
  # witness: a literal risky file, a scope under one risky skill, a risky root
  # file. Those are the commonest real write scopes there are.
  for scope in 'cli/**' '**' '*' './tests/**' 'scripts/*.sh' \
    'scripts/check-doc-claims-tracked.sh' 'skills/rpi/scripts/**' 'lib/preamble.sh' \
    '.github/workflows/validate.yml' 'cli/internal/gates/new.go'; do
    rpi_probe \
      "{\"intent\":\"harden the seal\",\"writeScope\":[\"$scope\"],\"acceptance\":\"the behavior holds\"}" \
      "[{\"label\":\"plan\",\"result\":{\"acceptance\":\"the behavior holds\",\"writeScope\":[\"$scope\"],\"intentDigest\":\"aaaa\",\"intentPath\":\"/tmp/i.intent\"}},{\"label\":\"premortem\",\"result\":{\"blocking\":[{\"id\":\"x:y\",\"class\":\"k\",\"summary\":\"blocks\"}],\"notes\":[]}}]"
    python3 - "$BATS_TEST_TMPDIR/probe.json" "$scope" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "premortem" in probe["calls"], (sys.argv[2], probe["calls"])
assert probe["result"]["premortem"]["status"] == "blocking", (sys.argv[2], probe["result"]["premortem"])
PY
  done
}

@test "rpi.js leaves a genuinely non-risky declared scope without a premortem" {
  for scope in 'docs/**' 'workflows/rpi.js' 'scripts/lib/preamble.sh' 'cli/internal/statusapp/**'; do
    rpi_probe \
      "{\"intent\":\"do the thing\",\"writeScope\":[\"$scope\"],\"acceptance\":\"the behavior holds\",\"repairRounds\":0}" \
      "[{\"label\":\"plan\",\"result\":{\"acceptance\":\"the behavior holds\",\"writeScope\":[\"$scope\"],\"intentDigest\":\"aaaa\",\"intentPath\":\"/tmp/i.intent\"}},$IMPL_RESULT,$ORPHANS_ABSENT,{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"docs/a.md\"]}}]"
    python3 - "$BATS_TEST_TMPDIR/probe.json" "$scope" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "premortem" not in probe["calls"], (sys.argv[2], probe["calls"])
assert probe["result"]["premortem"]["status"] == "not-required", (sys.argv[2], probe["result"]["premortem"])
assert probe["result"]["stopReason"] == "converged", probe["result"]["stopReason"]
PY
  done
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

@test "rpi.js carries a recorded premortem skip through an Implement failure" {
  # The 2026-09-03 shape: a hand-built early return dropped the recorded skip,
  # and the run read as if no premortem had ever been waived.
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip"}' \
    "[$PLAN_RESULT,{\"label\":\"implement\"}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert result["status"] == "NOT_PROVEN", result["status"]
assert result["stopReason"] == "implement_failed", result["stopReason"]
assert result["premortem"]["status"] == "skipped", result["premortem"]
PY
}

@test "rpi.js runs the orphan receipt on the validator-derived path set and feeds it to the validator" {
  # The author reported one path; the validator derived another. The receipt
  # must re-run over the UNION, and the next validator leg must see it as a
  # check receipt rather than only the caller reading it beside the verdict.
  local v0 v1
  v0='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v1","findings":[{"id":"f1","summary":"defect"}],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md","scripts/harness.sh"]}}'
  v1='{"label":"validate","result":{"verdict":"PASS","verdictPath":"/tmp/v.json","subjectDigest":"bb","criteria":[],"validatorContextId":"v2","findings":[],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md","scripts/harness.sh"]}}'
  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","repairRounds":1}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,{\"label\":\"orphans\",\"result\":{\"scriptPresent\":true,\"json\":\"{\\\"count\\\":0}\"}},$v0,{\"label\":\"repair:1\",\"result\":{\"contextId\":\"author-2\",\"changedPaths\":[\"docs/a.md\"],\"checkReceipts\":[],\"filesSummary\":\"n\"}},{\"label\":\"orphans\",\"result\":{\"scriptPresent\":true,\"json\":\"{\\\"count\\\":2}\"}},$v1]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
# Two receipts: one over the author paths, one over the widened union.
assert probe["calls"].count("orphans") == 2, probe["calls"]
assert "scripts/harness.sh" in probe["prompts"]["orphans"], probe["prompts"]["orphans"]
result = probe["result"]
assert result["orphanedEvidence"] == {"count": 2}, result["orphanedEvidence"]
# The receipt reached the second validator leg as a check receipt.
assert "evidence-orphans" in probe["prompts"]["validate"], probe["prompts"]["validate"]
assert result["verdict"] == "PASS", result["verdict"]
assert result["stopReason"] == "converged", result["stopReason"]
PY
}

@test "rpi.js reruns the orphan receipt after a repair that changed no paths" {
  # The receipt was keyed on the PATH SET, so a repair that edited the same
  # files never reran it and the post-repair validator judged the repaired
  # subject against the pre-repair exposure. The key is the round.
  local v0 v1 repair
  v0='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v1","findings":[{"id":"f1","summary":"defect"}],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md"]}}'
  v1='{"label":"validate","result":{"verdict":"PASS","verdictPath":"/tmp/v.json","subjectDigest":"bb","criteria":[],"validatorContextId":"v2","findings":[],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md"]}}'
  repair='{"label":"repair:1","result":{"contextId":"author-2","changedPaths":["docs/a.md"],"checkReceipts":[],"filesSummary":"n"}}'

  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","repairRounds":1}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,{\"label\":\"orphans\",\"result\":{\"scriptPresent\":true,\"json\":\"{\\\"binding_count\\\":0}\"}},$v0,$repair,{\"label\":\"orphans\",\"result\":{\"scriptPresent\":true,\"json\":\"{\\\"binding_count\\\":7}\"}},$v1]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
# An identical path set across the two rounds must still produce two receipts.
assert probe["calls"].count("orphans") == 2, probe["calls"]
result = probe["result"]
assert result["orphanedEvidence"] == {"binding_count": 7}, result["orphanedEvidence"]
# The round-2 validator must judge against the FRESH receipt, not round 1's.
# The receipt outcome is embedded as an escaped JSON string inside the
# pretty-printed checkReceipts block; compare with the noise removed.
prompt = probe["prompts"]["validate"].replace(" ", "").replace("\\", "")
assert '"binding_count":7' in prompt, prompt[-1200:]
assert '"binding_count":0' not in prompt, prompt[-1200:]
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

# --- the SHARED convergence-law corpus, driven through the workflow ---------
#
# tests/fixtures/rpi-convergence-law/cases.json is the same file
# skills/rpi/tests/test_run_once.py reads. A case honored by only one
# implementation is a parity break, and a parity break is exactly how the
# continuous-rename hole survived.

rpi_law_case() {
  local name="$1"
  python3 - "$REPO_ROOT" "$name" "$BATS_TEST_TMPDIR" <<'PY'
import json, pathlib, sys
root, name, tmp = pathlib.Path(sys.argv[1]), sys.argv[2], pathlib.Path(sys.argv[3])
doc = json.loads((root / "tests" / "fixtures" / "rpi-convergence-law" / "cases.json").read_text())
case = next(c for c in doc["cases"] if c["name"] == name)
queue = [
    {"label": "plan", "result": {"acceptance": "the behavior holds", "writeScope": ["docs/**"],
                                 "intentDigest": "aaaa", "intentPath": "/tmp/i.intent"}},
    {"label": "implement", "result": {"contextId": "author-1", "changedPaths": ["docs/a.md"],
                                      "checkReceipts": [], "filesSummary": "note"}},
    {"label": "orphans", "result": {"scriptPresent": False}},
]
for index, spec in enumerate(case["rounds"]):
    if index > 0:
        queue.append({"label": "repair:%d" % index,
                      "result": {"contextId": "author-%d" % (index + 1), "changedPaths": ["docs/a.md"],
                                 "checkReceipts": [], "filesSummary": "n"}})
        # One receipt per round: the rerun is keyed on the round, not the paths.
        queue.append({"label": "orphans", "result": {"scriptPresent": False}})
    queue.append({"label": "validate", "result": {
        "verdict": spec["verdict"], "verdictPath": None, "subjectDigest": spec["digest"] * 2,
        "criteria": [], "validatorContextId": "v%d" % index,
        "findings": spec["findings"], "evidenceRefs": [], "derivedChangedPaths": ["docs/a.md"]}})
(tmp / "law-input.json").write_text(json.dumps({
    "intent": "do the thing", "writeScope": ["docs/**"],
    "acceptance": "the behavior holds", "repairRounds": case["repair_rounds"]}))
(tmp / "law-queue.json").write_text(json.dumps(queue))
(tmp / "law-expect.json").write_text(json.dumps(case["expect"]))
PY
}

# Case names are DERIVED from the corpus, never listed here: a hardcoded list
# is how a case gets added on one side and silently never read on the other,
# which is the parity break the shared corpus exists to prevent.
rpi_law_names() {
  python3 -c '
import json, pathlib, sys
doc = json.loads(pathlib.Path(sys.argv[1]).read_text())
print(" ".join(c["name"] for c in doc["cases"] if c["expect"]["kind"] == sys.argv[2]))
' "$REPO_ROOT/tests/fixtures/rpi-convergence-law/cases.json" "$1"
}

@test "rpi.js honors every shared convergence-law case that stops" {
  local names
  names="$(rpi_law_names stop)"
  [ -n "$names" ]
  for name in $names; do
    rpi_law_case "$name"
    rpi_probe "$(cat "$BATS_TEST_TMPDIR/law-input.json")" "$(cat "$BATS_TEST_TMPDIR/law-queue.json")"
    python3 - "$BATS_TEST_TMPDIR/probe.json" "$BATS_TEST_TMPDIR/law-expect.json" "$name" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
expect = json.load(open(sys.argv[2]))
name = sys.argv[3]
assert expect["kind"] == "stop", name
assert result["stopReason"] == expect["stop_reason"], (name, result["stopReason"])
assert result["stopReasons"] == expect["stop_reasons"], (name, result["stopReasons"])
assert result["status"] == expect["status"], (name, result["status"])
assert result["verdict"] == expect["status"], (name, result["verdict"])
assert result["repairRoundsUsed"] == expect["rounds_used"], (name, result["repairRoundsUsed"])
PY
  done
}

@test "rpi.js refuses every shared convergence-law case that is invalid" {
  local names
  names="$(rpi_law_names invalid)"
  [ -n "$names" ]
  for name in $names; do
    rpi_law_case "$name"
    rpi_probe_refused "$(cat "$BATS_TEST_TMPDIR/law-input.json")" "$(cat "$BATS_TEST_TMPDIR/law-queue.json")"
    python3 - "$BATS_TEST_TMPDIR/law-expect.json" "$name" <<PY
import json, sys
expect = json.load(open("$BATS_TEST_TMPDIR/law-expect.json"))
assert expect["kind"] == "invalid", "$name"
assert expect["message"] in """$output""", ("$name", """$output""")
PY
  done
}

@test "rpi.js reports both dispositions when one round reopens an id and its class" {
  rpi_law_case simultaneous-id-and-class-reopen
  rpi_probe "$(cat "$BATS_TEST_TMPDIR/law-input.json")" "$(cat "$BATS_TEST_TMPDIR/law-queue.json")"
  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert result["stopReason"] == "reopened_finding", result["stopReason"]
assert "class_reopened" in result["stopReasons"], result["stopReasons"]
# A class reopen degrades the status even when a reopened id outranks it.
assert result["status"] == "NOT_PROVEN", result["status"]
assert "class" in result["stoppedBy"] and "reopened" in result["stoppedBy"], result["stoppedBy"]
PY
}

# --- the split law: a tie-break is a disposition, never a verdict override ---

RISKY_SPLIT_ARGS='{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}'
LEG_PASS='{"label":"validate","result":{"verdict":"PASS","verdictPath":"/tmp/v.json","subjectDigest":"aa","criteria":[],"validatorContextId":"v1","findings":[],"evidenceRefs":[],"derivedChangedPaths":["tests/a.bats"]}}'
LEG_FAIL='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v2","findings":[{"id":"x:y","class":"seal.pinning","summary":"cross-family objection"}],"evidenceRefs":[],"derivedChangedPaths":["tests/a.bats"]}}'

@test "rpi.js records a declared binding judge without letting it certify a split" {
  rpi_probe \
    "$(python3 -c 'import json,sys; a=json.loads(sys.argv[1]); a["bindingJudge"]="primary"; print(json.dumps(a))' "$RISKY_SPLIT_ARGS")" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"real\",\"evidence_refs\":[]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
result = probe["result"]
# The law stands: a split never certifies PASS, whoever was declared binding.
assert result["verdict"] == "FAIL", result["verdict"]
# And no leg's findings leave the open set.
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
disposition = result["declaredDisposition"]
assert disposition["judge"] == "primary", disposition
assert disposition["source"] == "caller", disposition
assert disposition["applies"] is True, disposition
assert result["verdictPath"] is None, result["verdictPath"]
dissent = result["dissent"]
assert len(dissent) == 1 and dissent[0]["verdict"] == "PASS", dissent
PY
}

@test "rpi.js records and ignores a declared binding judge off a risky scope" {
  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","bindingJudge":"cross","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,$ORPHANS_ABSENT,{\"label\":\"validate\",\"result\":{\"verdict\":\"PASS\",\"verdictPath\":\"/tmp/v.json\",\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v1\",\"findings\":[],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"docs/a.md\"]}},{\"label\":\"validate\",\"result\":{\"verdict\":\"FAIL\",\"verdictPath\":null,\"subjectDigest\":\"aa\",\"criteria\":[],\"validatorContextId\":\"v2\",\"findings\":[{\"id\":\"x:y\",\"summary\":\"single-family objection\"}],\"evidenceRefs\":[],\"derivedChangedPaths\":[\"docs/a.md\"]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "council" not in probe["calls"], probe["calls"]
result = probe["result"]
assert result["verdict"] == "FAIL", result["verdict"]
assert result["tieBreak"] == "worst-of", result["tieBreak"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
disposition = result["declaredDisposition"]
assert disposition["applies"] is False, disposition
assert "not risky" in disposition["note"], disposition
PY
}

@test "rpi.js refuses a caller and plan that declare different binding judges" {
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","bindingJudge":"primary"}' \
    '[{"label":"plan","result":{"acceptance":"the behavior holds","writeScope":["tests/**"],"intentDigest":"aaaa","intentPath":"/tmp/i.intent","binding_judge":"cross"}}]'

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert probe["calls"] == ["plan"], probe["calls"]
result = probe["result"]
assert result["status"] == "NOT_PROVEN", result["status"]
assert result["stopReason"] == "binding_judge_conflict", result["stopReason"]
assert result["declaredDisposition"]["caller"] == "primary", result["declaredDisposition"]
assert result["declaredDisposition"]["plan"] == "cross", result["declaredDisposition"]
# The risky scope's premortem was required and never ran; say so.
assert result["premortem"]["status"] == "required", result["premortem"]
PY
}

@test "rpi.js accepts a binding judge declared only by the frozen plan" {
  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[{\"label\":\"plan\",\"result\":{\"acceptance\":\"the behavior holds\",\"writeScope\":[\"tests/**\"],\"intentDigest\":\"aaaa\",\"intentPath\":\"/tmp/i.intent\",\"binding_judge\":\"cross\"}},$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"real\",\"evidence_refs\":[]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert result["declaredDisposition"]["judge"] == "cross", result["declaredDisposition"]
assert result["declaredDisposition"]["source"] == "plan", result["declaredDisposition"]
assert result["verdict"] == "FAIL", result["verdict"]
PY
}

@test "rpi.js does not convene a council on a split that repair still has rounds to fix" {
  # Five contract sentences say the council convenes on a risky split that
  # SURVIVES repair. Convening it inside every validateOnce spent a third judge
  # on a disagreement the very next repair round was about to settle.
  local repair v1
  repair='{"label":"repair:1","result":{"contextId":"author-2","changedPaths":["tests/a.bats"],"checkReceipts":[],"filesSummary":"n"}}'
  v1='{"label":"validate","result":{"verdict":"PASS","verdictPath":"/tmp/v.json","subjectDigest":"bb","criteria":[],"validatorContextId":"v3","findings":[],"evidenceRefs":[],"derivedChangedPaths":["tests/a.bats"]}}'

  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":1}' \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,$repair,$ORPHANS_ABSENT,$v1,$v1]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "council" not in probe["calls"], probe["calls"]
result = probe["result"]
assert result["verdict"] == "PASS", result["verdict"]
assert result["council"] is None, result["council"]
assert result["repairRoundsUsed"] == 1, result["repairRoundsUsed"]
PY
}

@test "rpi.js convenes the council only once the risky split survives repair" {
  # repairRounds 0: the first split is already terminal, so it survives repair
  # vacuously and the council convenes exactly once.
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"real\",\"evidence_refs\":[]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert probe["calls"].count("council") == 1, probe["calls"]
# The council runs AFTER the repair phase, so it is the last judge called.
assert probe["calls"][-1] == "council", probe["calls"]
result = probe["result"]
assert result["verdict"] == "FAIL", result["verdict"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
PY
}

@test "rpi.js revalidates once when the council closes a finding and a round remains" {
  # Round 1 repairs nothing (same digest, same finding), so the law stops the
  # phase with a round still unspent and the risky split live. The council then
  # closes the finding the law tripped over, and its disproof is new BOUND
  # evidence over an unchanged subject, which is exactly the case condition 4
  # admits: spend the remaining round and let a fresh validator judge it.
  local repair v_after
  repair='{"label":"repair:1","result":{"contextId":"author-2","changedPaths":["tests/a.bats"],"checkReceipts":[],"filesSummary":"n"}}'
  # The revalidator cites its OWN re-derivation, not the council's file: merely
  # re-citing the council's artifact confirms nothing independently.
  v_after='{"label":"validate","result":{"verdict":"PASS","verdictPath":"/tmp/v2.json","subjectDigest":"aa","criteria":[],"validatorContextId":"v9","findings":[],"evidenceRefs":[{"ref":".agents/ao/revalidation.txt","subjectDigest":"aa","resolves":["x:y"]}],"derivedChangedPaths":["tests/a.bats"]}}'

  rpi_probe \
    '{"intent":"harden the seal","writeScope":["tests/**"],"acceptance":"the behavior holds","premortem":"skip","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":2}' \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,$repair,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\"tests/a.bats\"]}]}},{\"label\":\"council-evidence\",\"result\":{\"resolved\":[{\"ref\":\"tests/a.bats\",\"exists\":true}]}},$ORPHANS_ABSENT,$v_after,$v_after]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert probe["calls"].count("council") == 1, probe["calls"]
# Two legs per validation: two repair-phase rounds plus one council round.
assert probe["calls"].count("validate") == 6, probe["calls"]
result = probe["result"]
assert result["council"]["closed"] == ["x:y"], result["council"]
assert result["repairRoundsUsed"] == 2, result["repairRoundsUsed"]
assert result["verdict"] == "PASS", result["verdict"]
assert any("council round 2" in line for line in result["repairLog"]), result["repairLog"]
PY
}

@test "rpi.js says so when the council closes a finding with no round left to revalidate" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\"tests/a.bats\"]}]}},{\"label\":\"council-evidence\",\"result\":{\"resolved\":[{\"ref\":\"tests/a.bats\",\"exists\":true}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert result["council"]["closed"] == ["x:y"], result["council"]
assert result["stopReason"] == "repair_budget_exhausted", result["stopReason"]
assert result["verdict"] == "NOT_PROVEN", result["verdict"]
assert "not re-validated" in result["stoppedBy"], result["stoppedBy"]
PY
}

@test "rpi.js closes nothing on a council ref that does not resolve in the tree" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\"tests/never-written.txt\"]}]}},{\"label\":\"council-evidence\",\"result\":{\"resolved\":[{\"ref\":\"tests/never-written.txt\",\"exists\":false}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
assert result["council"]["closed"] == [], result["council"]
assert result["council"]["rulings"][0]["applied"] == "kept-open", result["council"]
assert result["verdict"] == "FAIL", result["verdict"]
PY
}

@test "rpi.js closes nothing on a blank council evidence ref" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\"\",\"   \"]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
# Every ref is blank, so there is nothing to resolve and no receipt is spent.
assert "council-evidence" not in probe["calls"], probe["calls"]
result = probe["result"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
assert result["council"]["closed"] == [], result["council"]
assert result["verdict"] == "FAIL", result["verdict"]
PY
}

@test "rpi.js accepts a council sha256 ref already present in the evidence set" {
  # A digest ref needs no filesystem lookup: it either names bytes the legs
  # already bound or it names nothing.
  local leg_fail_ev
  leg_fail_ev='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v2","findings":[{"id":"x:y","class":"seal.pinning","summary":"cross-family objection"}],"evidenceRefs":[{"ref":"sha256:beef","subjectDigest":"aa","resolves":[]}],"derivedChangedPaths":["tests/a.bats"]}}'

  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$leg_fail_ev,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\"sha256:beef\"]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert "council-evidence" not in probe["calls"], probe["calls"]
result = probe["result"]
assert result["council"]["closed"] == ["x:y"], result["council"]
PY
}

@test "rpi.js closes nothing on a sha256 ref no leg ever bound" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\"sha256:invented\"]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert result["council"]["closed"] == [], result["council"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
PY
}

@test "rpi.js convenes a council that adjudicates findings and never a verdict" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"real\",\"evidence_refs\":[\".agents/ao/repro.txt\"]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
probe = json.load(open(sys.argv[1]))
assert probe["calls"].count("council") == 1, probe["calls"]
result = probe["result"]
assert result["verdict"] == "FAIL", result["verdict"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
assert result["council"]["status"] == "ruled", result["council"]
assert result["council"]["closed"] == [], result["council"]
assert result["council"]["rulings"][0]["applied"] == "kept-open", result["council"]
# The packet is bounded, structured, and marked untrusted.
prompt = probe["prompts"]["council"]
assert "UNTRUSTED DATA" in prompt, prompt
assert "cross-family objection" in prompt
assert "the behavior holds" in prompt
assert "tests/a.bats" in prompt
# The council never returns a verdict, so it is never asked for one.
assert "Return rulings" in prompt
PY
}

@test "rpi.js closes a finding only on a council disproof that carries evidence" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
# An unsupported dismissal closes nothing.
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
assert result["verdict"] == "FAIL", result["verdict"]
assert result["council"]["rulings"][0]["applied"] == "kept-open", result["council"]
PY
}

@test "rpi.js treats a FAIL leg whose findings the council all closed as NOT_PROVEN" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"x:y\",\"ruling\":\"not_real\",\"evidence_refs\":[\".agents/ao/disproof.txt\"]}]}},{\"label\":\"council-evidence\",\"result\":{\"resolved\":[{\"ref\":\".agents/ao/disproof.txt\",\"exists\":true}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
# A verdict with no surviving findings proves nothing either way; it is never
# promoted to the other leg's PASS, and with no round left to re-judge the
# closure it stays NOT_PROVEN and says why.
assert result["verdict"] == "NOT_PROVEN", result["verdict"]
assert result["findings"] == [], result["findings"]
assert result["council"]["closed"] == ["x:y"], result["council"]
assert "not re-validated" in result["stoppedBy"], result["stoppedBy"]
assert result["verdictPath"] is None
PY
}

@test "rpi.js keeps every finding open when the council does not rule" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\"}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert result["council"]["status"] == "unavailable", result["council"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
assert result["verdict"] == "FAIL", result["verdict"]
PY
}

@test "rpi.js ignores a council ruling on a finding that is not on the table" {
  rpi_probe "$RISKY_SPLIT_ARGS" \
    "[$PLAN_RESULT,$IMPL_RESULT_RISKY,$ORPHANS_ABSENT,$LEG_PASS,$LEG_FAIL,{\"label\":\"council\",\"result\":{\"rulings\":[{\"id\":\"invented:id\",\"ruling\":\"not_real\",\"evidence_refs\":[\"x\"]}]}}]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
assert result["council"]["rulings"][0]["applied"] == "unknown-finding", result["council"]
assert result["council"]["closed"] == [], result["council"]
assert result["verdict"] == "FAIL", result["verdict"]
PY
}

@test "rpi.js fails closed when two legs reuse one finding id with different classes" {
  # Both legs FAIL on the same id, so there is no split and no council: the ids
  # simply fold. Folding two different KINDS into one id is a silent identity
  # collision, and the class law then reasons over a key that means two things.
  local a b
  a='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v1","findings":[{"id":"x:y","class":"seal.pinning","summary":"leg one"}],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md"]}}'
  b='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v2","findings":[{"id":"x:y","class":"scope.coverage","summary":"leg two"}],"evidenceRefs":[],"derivedChangedPaths":["docs/a.md"]}}'

  rpi_probe_refused \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,$ORPHANS_ABSENT,$a,$b]"
  [[ "$output" == *"x:y"* ]]
  [[ "$output" == *"class"* ]]
}

@test "rpi.js unions two legs that name one finding id compatibly" {
  # Same id, one leg classes it and the other does not: a compatible identity.
  # The class survives, both families are recorded, and the evidence unions.
  local a b
  a='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v1","findings":[{"id":"x:y","class":"seal.pinning","summary":"leg one"}],"evidenceRefs":[{"ref":".agents/ao/one.txt","subjectDigest":"aa","resolves":[]}],"derivedChangedPaths":["docs/a.md"]}}'
  b='{"label":"validate","result":{"verdict":"FAIL","verdictPath":null,"subjectDigest":"aa","criteria":[],"validatorContextId":"v2","findings":[{"id":"x:y","summary":"leg two"}],"evidenceRefs":[{"ref":".agents/ao/two.txt","subjectDigest":"aa","resolves":[]}],"derivedChangedPaths":["docs/a.md"]}}'

  rpi_probe \
    '{"intent":"do the thing","writeScope":["docs/**"],"acceptance":"the behavior holds","crossFamily":{"command":"codex exec --read-only judge"},"repairRounds":0}' \
    "[$PLAN_RESULT_SAFE,$IMPL_RESULT,$ORPHANS_ABSENT,$a,$b]"

  python3 - "$BATS_TEST_TMPDIR/probe.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))["result"]
findings = result["findings"]
assert [f["id"] for f in findings] == ["x:y"], findings
assert findings[0]["class"] == "seal.pinning", findings
# Both families are on the record; neither leg's naming is dropped.
assert "spawned" in findings[0]["family"] and "cross-family" in findings[0]["family"], findings
refs = sorted(e["ref"] for e in result["evidenceRefs"])
assert refs == [".agents/ao/one.txt", ".agents/ao/two.txt"], refs
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
assert result["council"] is None, result["council"]
assert [f["id"] for f in result["findings"]] == ["x:y"], result["findings"]
PY
}

@test "rpi.js stop reasons come from the fixed enum, never from prose" {
  python3 - "$REPO_ROOT/workflows/rpi.js" "$REPO_ROOT/skills/rpi/scripts/run_once.py" <<'PY'
import re, sys
js = open(sys.argv[1]).read()
py = open(sys.argv[2]).read()
block = re.search(r"const STOP_REASONS = Object\.freeze\(\{(.*?)\}\);", js, re.S).group(1)
js_values = set(re.findall(r"'([a-z_]+)'", block))
py_values = set(re.findall(r'"([a-z_]+)"', re.search(r"STOP_REASONS = \((.*?)\)", py, re.S).group(1)))
assert len(py_values) == 8, py_values
missing = py_values - js_values
assert not missing, "the workflow enum does not mirror the reference behavior: %s" % sorted(missing)
# The workflow adds only stops a dispatcher can reach.
assert js_values - py_values, "the workflow declares no workflow-only stop reasons"
# No stopReason is ever assigned a prose literal; the enum is the only source.
literal = re.search(r"stopReason(?:s)?\s*[:=]\s*['\"]", js)
assert literal is None, "stopReason assigned a string literal: %s" % literal.group(0)
# And the report always carries one.
assert "stopReason: fields.stopReason" in js
PY
}
