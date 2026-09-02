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
# Witness inventory (each maps to one observed defect):
#   A  description cut mid-clause immediately before `Triggers:`   (51 files)
#   B  prose cut inside a sentence rather than at a boundary       (all 56)
#   C  blind "Claude Code" -> "Codex" producing "Codex, Codex CLI" (flywheel)
#   D  H1 title slash-rewritten to `# $skill`                      (3 files)
#   E  dormant emitter for the non-existent `ao codex ensure-start` (deleted)
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

# B ── the frozen projection rule, checked on EVERY twin, not a sample:
#
#   twin description == first_sentence(source prose) + " " + source Triggers
#
# where a sentence terminator is '.', '!' or '?' followed by whitespace or the
# end of the prose. ';' and dashes are NOT terminators — cutting at one still
# yields a fragment. Requiring whitespace after the mark is what keeps
# "verdict.v2" and "reality-check." from being read as sentence ends. This test
# re-derives the expected string from source independently of the generator, so
# a generator that starts inventing catalog text fails here.
@test "every twin description is the source's first sentence plus the full Triggers clause" {
    run python3 - <<'PYCHECK'
import re
import pathlib

REPO = pathlib.Path(__import__("os").environ["REPO_ROOT"])


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
    if len(out) >= 2 and out[0] == out[-1] and out[0] in "'\"":
        out = out[1:-1]
    return out.strip()


def split(desc):
    m = re.search(r"\s+[Tt]riggers?:", desc)
    if not m:
        return desc.strip(), ""
    return desc[: m.start()].strip(), desc[m.start():].strip()


def first_sentence(prose):
    m = re.search(r"[.!?](?=\s|$)", prose)
    return prose[: m.end()] if m else prose


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
    expected = " ".join(p for p in (first_sentence(src_prose), src_trig) if p)
    actual = description(twin_dir / "SKILL.md")
    if actual != expected:
        failures.append(
            f"{name}: description is not first-sentence + full Triggers\n"
            f"  expected: {expected!r}\n  actual:   {actual!r}"
        )

if failures:
    print("\n".join(failures))
    raise SystemExit(1)
print(f"checked {len(twins)} twins: first sentence + full Triggers clause, verbatim")
PYCHECK
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

# C ── the runtime-name substitution is phrase-aware. "Claude Code, Codex CLI"
# must collapse to "Codex CLI", never expand to "Codex, Codex CLI".
@test "using-flywheel twin says 'Codex CLI' exactly twice and 'Codex, Codex' never" {
    twin="$REPO_ROOT/skills-codex/using-flywheel/SKILL.md"
    [ -f "$twin" ]

    # skills/using-flywheel/SKILL.md names the runtime trio "Claude Code, Codex
    # CLI, and Antigravity CLI" at lines 47 and 72. The phrase-aware table
    # collapses each to a single "Codex CLI"; the blind replacement produced
    # "Codex, Codex CLI" at both sites.
    twin_cli=$(grep -o 'Codex CLI' "$twin" | wc -l | tr -d ' ')
    dupes=$(grep -c 'Codex, Codex' "$twin" || true)
    echo "twin 'Codex CLI'=$twin_cli  'Codex, Codex'=$dupes" >&2
    [ "$twin_cli" = "2" ]
    [ "$dupes" = "0" ]
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
