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
# ORACLE DISCIPLINE. Test B deliberately does NOT re-implement
# `first_sentence`. Copying the implementation's regex would make the test
# agree with the generator by construction, including on its bugs. Instead it
# applies a WEAKER, independently stated rule that no fragment can satisfy
# (twin prose is a prefix of source prose, ending at a terminator followed by a
# space, plus the full Triggers clause), and Test B2 pins five named skills to
# LITERAL expected strings. The abbreviation and closing-quote edge cases are
# pinned end-to-end, against the real generator, in
# tests/scripts/test-codex-sync-generator.sh.
#
# Witness inventory (each maps to one observed defect):
#   A   description cut mid-clause immediately before `Triggers:`   (51 files)
#   B   prose cut inside a sentence rather than at a boundary       (all 56)
#   B2  the five core router entries, pinned literally
#   C   cross-runtime body corrupted by runtime substitution        (flywheel)
#   D   H1 title slash-rewritten to `# $skill`                      (3 files)
#   E   dormant emitter for the non-existent `ao codex ensure-start` (deleted)
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

# B ── every twin, checked against an INDEPENDENT rule (see ORACLE DISCIPLINE):
#   1. the twin ends with the source's full Triggers clause, verbatim;
#   2. the twin's prose is a prefix of the source's prose;
#   3. that prefix is either the whole prose, or ends at a sentence terminator
#      (optionally followed by one closing quote) that the source follows with a
#      space.
# A fragment fails (2)+(3); a dropped or reworded clause fails (1).
@test "every twin description ends at a sentence boundary and keeps the full Triggers clause" {
    run python3 - <<'PYCHECK'
import re
import pathlib

REPO = pathlib.Path(__import__("os").environ["REPO_ROOT"])
TERMINATORS = ".!?"
CLOSING_QUOTES = "\"'”"


def description(path):
    """The frontmatter description, unfolded and unquoted."""
    lines = path.read_text(encoding="utf-8").splitlines()
    if not lines or lines[0].strip() != "---":
        raise SystemExit(f"{path}: no frontmatter")
    out = None
    for line in lines[1:]:
        if line.strip() == "---":
            break
        if out is None:
            if line.startswith("description:"):
                out = line[len("description:"):].strip()
            continue
        if re.match(r"^[A-Za-z0-9_-]+:", line):
            break
        out = f"{out} {line.strip()}"
    if out is None:
        raise SystemExit(f"{path}: no description")
    out = re.sub(r"\s+", " ", out).strip()
    # Unwrap the YAML scalar exactly once, then unescape a doubled quote.
    if len(out) >= 2 and out[0] == out[-1] and out[0] in "'\"":
        quote, out = out[0], out[1:-1]
        out = out.replace(quote * 2, quote)
    return out.strip()


def split(desc):
    m = re.search(r"\s+[Tt]riggers?:", desc)
    if not m:
        return desc.strip(), ""
    return desc[: m.start()].strip(), desc[m.start():].strip()


twins = sorted(p for p in (REPO / "skills-codex").iterdir() if (p / "SKILL.md").is_file())
if not twins:
    raise SystemExit("no skills-codex twins found")

failures = []
for twin_dir in twins:
    name = twin_dir.name
    source = REPO / "skills" / name / "SKILL.md"
    if not source.is_file():
        failures.append(f"{name}: twin has no source skill")
        continue
    src_prose, src_trig = split(description(source))
    twin_prose, twin_trig = split(description(twin_dir / "SKILL.md"))

    if twin_trig != src_trig:
        failures.append(
            f"{name}: Triggers clause not preserved verbatim\n"
            f"  source: {src_trig!r}\n  twin:   {twin_trig!r}"
        )

    if twin_prose == src_prose:
        continue
    if not twin_prose or not src_prose.startswith(twin_prose):
        failures.append(
            f"{name}: twin prose is not a prefix of the source prose\n"
            f"  source: {src_prose!r}\n  twin:   {twin_prose!r}"
        )
        continue
    end = twin_prose
    if end[-1] in CLOSING_QUOTES:
        end = end[:-1]
    tail = src_prose[len(twin_prose):]
    if not end or end[-1] not in TERMINATORS or not tail.startswith(" "):
        failures.append(
            f"{name}: twin prose does not end at a sentence boundary\n"
            f"  source: {src_prose!r}\n  twin:   {twin_prose!r}"
        )

if failures:
    print("\n".join(failures))
    raise SystemExit(1)
print(f"checked {len(twins)} twins against the independent boundary rule")
PYCHECK
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

# B2 ── the five core router entries a stranger hits first, pinned to literal
# expected text. No rule, no derivation: if the projection changes, this fails.
@test "the five core twin descriptions are exactly the expected literal strings" {
    expect() { # expect <skill> <literal description value>
        local skill="$1" want="$2" got
        got="$(sed -n '2,/^---$/p' "$REPO_ROOT/skills-codex/$skill/SKILL.md" \
               | sed -n "s/^description: '\(.*\)'$/\1/p" | sed "s/''/'/g")"
        if [ "$got" != "$want" ]; then
            echo "$skill:" >&2
            echo "  want: $want" >&2
            echo "  got:  $got" >&2
            return 1
        fi
    }

    expect validate 'Freshly judge whether a finished change is actually proven against bead or caller acceptance — the independent verdict before merge; optionally persist verdict.v2 for a declared consumer, and stop. Triggers: "validate", "independently validate", "is this proven", "vibe".'
    expect rpi 'Coordinate one RPI traversal: one bounded Plan, Implement, and fresh Validate experiment, then report and stop. Triggers: "run rpi", "run one traversal", "execute this plan", orchestration or worker delegation that implements changes.'
    expect plan 'Shape or refine the existing bead or caller intent without a second planning artifact. Triggers: "plan", "discover and plan", "shape this goal".'
    expect council 'Collect independent perspectives for an explicitly high-stakes or contested judgment. Triggers: "council", "multi-judge review", "independent perspectives".'
    expect domain 'Load the AgentOps language and bounded-context contracts when a term needs precise meaning. Triggers: "define this domain term", "check the bounded context".'
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
