#!/usr/bin/env bats
# Regression fence for the Codex description projection produced by
# scripts/codex-sync.sh (`codex_catalog_description` and `transform_body`).
#
# WHY THIS EXISTS. The generator cut skill prose at a 44-character word
# boundary before re-appending the `Triggers:` clause, so the always-loaded
# Codex activation catalog shipped 51/56 descriptions that read as fragments
# ("Freshly judge whether a finished change is Triggers: ..."). A catalog whose
# whole job is routing cannot route on half a clause, so the budget that
# produced the truncation defeated the budget's own purpose. These assertions
# pin the repaired projection against the exact defects the 2026-09-02 field
# audit found, so the fragment cannot come back silently.
#
# Later first-sentence truncation also lost use cases and preconditions from
# reality-check and ms. Complete source-description parity is now the oracle;
# fixture regression tests exercise independent required-input and exclusion
# sentences without pinning mutable catalog copy to historical prose.
#
# These run against the generated tree in the checkout, so they are only
# meaningful after `bash scripts/regen-all.sh` has projected the current
# generator. That is the point: they fence the artifact a stranger installs.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
}

# A ── no twin description is a fragment ending immediately before Triggers:.
# `[a-z] Triggers:` is the exact signature of the old word-boundary cut: a
# lowercase word character butted straight against the trigger clause with no
# sentence terminator between them.
@test "no skills-codex description is cut mid-clause before Triggers:" {
    run bash -c "grep -lE '^description:.*[a-z] Triggers:' \"$REPO_ROOT\"/skills-codex/*/SKILL.md || true"
    [ "$status" -eq 0 ]
    if [ -n "$output" ]; then
        echo "truncated Codex twin descriptions ($(echo "$output" | wc -l | tr -d ' ') files):" >&2
        echo "$output" >&2
    fi
    [ -z "$output" ]
}

# B ── every generated twin retains the source's complete routing signal.
@test "every twin description preserves all source routing content" {
    run python3 - <<'PYCHECK'
import os
import pathlib
import yaml

repo = pathlib.Path(os.environ["REPO_ROOT"])


def description(path):
    frontmatter = path.read_text(encoding="utf-8").split("---", 2)[1]
    return " ".join(yaml.safe_load(frontmatter)["description"].split())


twins = sorted((repo / "skills-codex").glob("*/SKILL.md"))
if not twins:
    raise SystemExit("no skills-codex twins found")

failures = []
for twin in twins:
    source = repo / "skills" / twin.parent.name / "SKILL.md"
    if not source.is_file():
        failures.append(f"{twin.parent.name}: twin has no source skill")
    elif description(twin) != description(source):
        failures.append(f"{twin.parent.name}: source description was changed or truncated")

if failures:
    raise SystemExit("\n".join(failures))
print(f"checked complete descriptions for {len(twins)} twins")
PYCHECK
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

# C ── using-flywheel is a CROSS-RUNTIME skill: it names three worker runtimes
# side by side and tells the operator to check two distinct install paths. The
# blanket "Claude Code" -> "Codex" / ~/.claude -> ~/.codex rewrite collapsed the
# trio to two names and printed the same path twice, silently deleting the check
# for the runtime the step exists to verify. The fix is the exemption list at
# scripts/lint/codex-cross-runtime-skills.txt, so the twin body must now be the
# source body verbatim at both sites.
@test "using-flywheel twin preserves the cross-runtime trio and both distinct install paths" {
    twin="$REPO_ROOT/skills-codex/using-flywheel/SKILL.md"
    src="$REPO_ROOT/skills/using-flywheel/SKILL.md"
    [ -f "$twin" ]
    [ -f "$src" ]

    # The runtime-trio line (source line 47) survives verbatim.
    trio_src="$(grep -n 'multi-agent factory:' "$src" | cut -d: -f2-)"
    trio_twin="$(grep -n 'multi-agent factory:' "$twin" | cut -d: -f2-)"
    echo "source: $trio_src" >&2
    echo "twin:   $trio_twin" >&2
    [ -n "$trio_src" ]
    [ "$trio_twin" = "$trio_src" ]
    case "$trio_src" in
        *"Claude Code, Codex CLI, and Antigravity CLI"*) ;;
        *) echo "source no longer states the trio — update this test" >&2; return 1 ;;
    esac

    # The verification line (source line 76) lists TWO DISTINCT paths, each once.
    verify="$(grep 'skills/validate' "$twin")"
    echo "verify: $verify" >&2
    [ "$(printf '%s' "$verify" | grep -o '~/\.claude/skills/validate' | wc -l | tr -d ' ')" = "1" ]
    [ "$(printf '%s' "$verify" | grep -o '~/\.codex/skills/validate' | wc -l | tr -d ' ')" = "1" ]

    # And nothing else in the body drifted from source.
    run python3 -c "
import pathlib, sys
def body(p): return pathlib.Path(p).read_text(encoding='utf-8').split('---', 2)[2].lstrip('\n')
s, w = body('$src'), body('$twin')
sys.exit(0 if s == w else 1)
"
    [ "$status" -eq 0 ]
}

# D ── the slash-to-\$ rewrite is for slash-COMMAND invocations in prose, never
# for the document title. `# \$route` is not a heading anyone reads.
@test "no Codex twin title starts with '# \$'" {
    run bash -c "grep -l '^# \\\$' \"$REPO_ROOT\"/skills-codex/*/SKILL.md || true"
    [ "$status" -eq 0 ]
    if [ -n "$output" ]; then
        echo "twins with a \$-rewritten title:" >&2
        echo "$output" >&2
    fi
    [ -z "$output" ]
}

# E ── `ao codex ensure-start` is not a subcommand of `ao`. The generator
# carried a dormant emitter for it (no catalog entry declared the marker that
# would fire it, so 0 twins ever carried the block). Dormant is not harmless:
# it is a live path to a command that would fail. Nothing may reference it.
@test "no script or twin references 'ao codex ensure-start'" {
    run bash -c "cd \"$REPO_ROOT\" && grep -rn 'ensure-start' scripts skills-codex || true"
    [ "$status" -eq 0 ]
    if [ -n "$output" ]; then
        echo "$output" >&2
    fi
    [ -z "$output" ]
}
