#!/usr/bin/env bash
# Test: OpenCode structural smoke: skill shape and explicit-destination links.
# These checks do not establish that a real OpenCode session loads the files.
# Standalone: does NOT require a live OpenCode runtime.
# Promoted from: tests/_quarantine/opencode/ (structural checks only)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
SKIP=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  SKIP: $1"; SKIP=$((SKIP + 1)); }

echo "=== OpenCode Runtime Smoke Tests ==="
echo "Proof tier: Tier S structural/install smoke"
echo ""

# ── 1. OpenCode install docs (npx skills path) ───────────────────────────────
echo "Stage 1: OpenCode install surface"

OPENCODE_DOCS="$REPO_ROOT/.opencode/INSTALL.md"
if [[ ! -e "$REPO_ROOT/scripts/install-opencode.sh" ]]; then
    pass "install-opencode.sh deleted"
else
    fail "install-opencode.sh still exists"
fi

if [[ -f "$OPENCODE_DOCS" ]] && grep -q 'npx skills@latest add boshu2/agentops -a opencode' "$OPENCODE_DOCS"; then
    pass ".opencode/INSTALL.md documents the npx skills path"
else
    fail ".opencode/INSTALL.md missing npx skills install guidance"
fi

echo ""

# ── 2. Skill SKILL.md files have no OpenCode-breaking characters ──────────────
echo "Stage 2: Skill frontmatter OpenCode compatibility"

skill_count=0
broken=0
for skill_md in "$REPO_ROOT/skills"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    # Skip leading-underscore scaffolding (e.g. skills/_fixtures/) — planted
    # test fixtures, not real skills.
    case "$(basename "$(dirname "$skill_md")")" in _*) continue ;; esac
    skill_count=$((skill_count + 1))
    # SKILL.md must start with --- (YAML frontmatter) — required by all runtimes
    if ! head -1 "$skill_md" | grep -q '^---'; then
        fail "$(basename "$(dirname "$skill_md")")/SKILL.md missing frontmatter start"
        broken=$((broken + 1))
    fi
done

if [[ $broken -eq 0 && $skill_count -gt 0 ]]; then
    pass "$skill_count skills have valid frontmatter start"
elif [[ $skill_count -eq 0 ]]; then
    fail "No SKILL.md files found under skills/"
fi

echo ""

# ── 3. Skills directory structure ─────────────────────────────────────────────
echo "Stage 3: Runtime-agnostic skill structure"

# Every skill must have SKILL.md (the cross-runtime entry point)
missing_skillmd=0
for skill_dir in "$REPO_ROOT/skills"/*/; do
    [[ -d "$skill_dir" ]] || continue
    case "$(basename "$skill_dir")" in _*) continue ;; esac
    if [[ ! -f "$skill_dir/SKILL.md" ]]; then
        fail "$(basename "$skill_dir") missing SKILL.md"
        missing_skillmd=$((missing_skillmd + 1))
    fi
done
if [[ $missing_skillmd -eq 0 ]]; then
    pass "All skill directories have SKILL.md"
fi

# No README.md in skill dirs (CI rule: SKILL.md is the entry point)
readme_found=0
for skill_dir in "$REPO_ROOT/skills"/*/; do
    [[ -d "$skill_dir" ]] || continue
    case "$(basename "$skill_dir")" in _*) continue ;; esac
    if [[ -f "$skill_dir/README.md" ]]; then
        fail "$(basename "$skill_dir") has README.md (should be SKILL.md only)"
        readme_found=$((readme_found + 1))
    fi
done
if [[ $readme_found -eq 0 ]]; then
    pass "No skill directories have README.md (SKILL.md is entry point)"
fi

echo ""

# ── 4. OpenCode install docs ──────────────────────────────────────────────────
echo "Stage 4: OpenCode config path compatibility"

if [[ -f "$OPENCODE_DOCS" ]]; then
    if grep -qE 'opencode|\.opencode|ao skills link' "$OPENCODE_DOCS"; then
        pass ".opencode/INSTALL.md references opencode / skills link path"
    else
        skip ".opencode/INSTALL.md missing opencode path reference"
    fi
fi

echo ""

echo "Stage 5: Isolated OpenCode source-link installation"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
AO_BIN="$TMP_ROOT/ao"
DEST="$TMP_ROOT/home/.config/opencode/skills"
mkdir -p "$DEST/test" "$TMP_ROOT/foreign"
printf 'user-owned\n' > "$DEST/test/keep.txt"
ln -s "$TMP_ROOT/foreign" "$DEST/foreign"

if (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao); then
    pass "source-matched ao built for isolated install"
else
    fail "could not build source-matched ao"
    exit 1
fi

if (cd "$REPO_ROOT" && env HOME="$TMP_ROOT/home" "$AO_BIN" skills link \
    --dest "$DEST" --skill test --skill refactor --dry-run --json > "$TMP_ROOT/preview.json") &&
    [[ ! -e "$DEST/refactor" ]] &&
    jq -e 'length == 1 and .[0].dry_run and .[0].linked == ["refactor"] and .[0].conflicts == ["test"]' "$TMP_ROOT/preview.json" >/dev/null; then
    pass "explicit-destination preview preserves selection and reports user-owned conflict"
else
    fail "explicit-destination preview changed files or missed selection/conflict"
fi

if (cd "$REPO_ROOT" && env HOME="$TMP_ROOT/home" "$AO_BIN" skills link \
    --dest "$DEST" --skill test --skill refactor --json > "$TMP_ROOT/install.json") &&
    [[ -L "$DEST/refactor" ]] &&
    [[ "$(readlink "$DEST/refactor")" == "$REPO_ROOT/skills/refactor" ]] &&
    [[ -f "$DEST/refactor/SKILL.md" ]] &&
    [[ ! -e "$DEST/security" && ! -e "$TMP_ROOT/home/.agents" ]] &&
    [[ ! -L "$DEST/test" ]] &&
    [[ "$(cat "$DEST/test/keep.txt")" == user-owned ]] &&
    [[ "$(readlink "$DEST/foreign")" == "$TMP_ROOT/foreign" ]] &&
    jq -e 'length == 1 and .[0].linked == ["refactor"] and .[0].conflicts == ["test"]' "$TMP_ROOT/install.json" >/dev/null; then
    pass "selected install reaches only explicit OpenCode destination and preserves user entries"
else
    fail "selected install destination, source identity or ownership contract failed"
fi

if (cd "$REPO_ROOT" && env HOME="$TMP_ROOT/home" "$AO_BIN" skills link \
    --dest "$DEST" --skill test --skill refactor --json > "$TMP_ROOT/repeat.json") &&
    jq -e '.[0].present == ["refactor"] and .[0].conflicts == ["test"] and ((.[0].linked // []) | length) == 0' "$TMP_ROOT/repeat.json" >/dev/null &&
    [[ "$(cat "$DEST/test/keep.txt")" == user-owned ]]; then
    pass "repeating the selected install is idempotent and keeps conflicts intact"
else
    fail "repeating the selected install changed ownership or selection"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
