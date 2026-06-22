#!/usr/bin/env bash
# check-doc-hooks-drift.sh — hooks-runtime regression guard for docs/ (ag-rryf).
#
# AgentOps 3.0 is hookless: the default install ships zero hooks, there is no
# `hooks/hooks.json` runtime, and `ao hooks` is not a registered command. The
# ag-t1ca epic de-hooked ~16 live-facing docs but did NOT gate the class, so the
# rot can silently re-enter. This is the companion to scripts/sync-skill-counts.sh
# (skill-count drift) and scripts/check-hookless-cold-start.sh (a FIXED-list cold-
# start gate); this gate broadens to the live-facing narrative/onboarding/operator
# docs tree.
#
# It FAILS when a scoped doc presents a LIVE hooks runtime:
#   - `hooks/hooks.json`            — the retired manifest, presented as live
#   - a `hooks/<name>.sh` path      — a hook runtime path presented as live
#   - `ao hooks`                    — a command that does not exist
#   - bare `session-{start,end,autostart}*.sh` — hook-file refs presented as live
#   - `SessionStart hook` / `session-start hook` / `startup hooks` promises that
#     describe hidden session-boundary behavior without an opt-in/historical hedge
#   - `auto-inject ... hook` phrasing that reintroduces the removed context-push
#     model without naming a hook file path
# unless the SAME line hedges the reference as hookless / opt-in / historical /
# removed / a git-hook (see HEDGE). Per-line hedging mirrors check-hookless-cold-
# start.sh: an artifact that merely names a hook in a "this is gone / author your
# own" frame is legitimate; an unhedged present-tense promise is the regression.
#
# SCOPE — docs/ MINUS surfaces that legitimately/archivally reference hooks:
#   archival history : CHANGELOG.md, releases/, learnings/, plans/, adr/,
#                      convergence/, sovereignty-proof/  (document the past / the
#                      removal itself — must keep naming hooks)
#   opt-in subsystem : contracts/ (hooks-authoring opt-in subsystem specs),
#                      runbooks/ (diagnostic hook enumeration),
#                      code-map/   (point-in-time code audits),
#                      audits/     (point-in-time doc-sweep audits — a findings
#                                   record QUOTES the stale-as-current hook
#                                   pattern it is flagging, in its own claim field)
# These are deliberately out of the live-facing risk class. The gate keeps the
# onboarding / operator / architecture narrative hookless.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS="$ROOT/docs"

# Live-hooks patterns. A doc line matching any of these is a candidate violation
# unless it also matches HEDGE.
PAT='hooks/hooks\.json|hooks/[A-Za-z0-9_-]+\.sh|\bao hooks\b|\b[a-z0-9-]*session-(start|end|autostart)[a-z0-9-]*\.sh\b|Session(Start|End) hook|session-(start|end|autostart) hook|[Ss]tartup hooks?|precompact-snapshot hook|auto-inject[^[:cntrl:]]*hook|hook[^[:cntrl:]]*auto-inject'

# A reference is "hedged" (allowed) when the SAME line carries one of these.
# They frame the hook as NOT a live default surface: hookless / opt-in / author-
# it-yourself / historical / removed / replaced / a git-hook. "SCHEMAS legacy
# hooks-manifest label" is covered by `legacy`/`opt-in`; the hooks-authoring
# skill by `hooks-authoring`/`author`; ".githooks" git hooks by `git-hook`.
HEDGE='hookless|hooks-authoring|ships (zero|no) hooks|no hooks|no `?ao hooks|removed|are gone|is gone|gone by design|retired|no longer|deleted|teardown|historical|legacy|formerly|used to|previously|was wired|pre-existed|opt-in|optional|if you author|author one|author via|author your own|example|when installed|hook-capable|can be triggered|injected|curated|replaced|instead|→|not wired|inactive|zero matches|grep yields|\.githooks|git-hook|git hook'

# Archival + opt-in-subsystem surfaces (basenames; unique under docs/).
EXCLUDE_DIRS=(releases learnings plans adr convergence sovereignty-proof contracts runbooks code-map audits)

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -d "$DOCS" ]] || fail "docs/ not found under $ROOT"

exclude_args=(--exclude=CHANGELOG.md)
for d in "${EXCLUDE_DIRS[@]}"; do
  exclude_args+=(--exclude-dir="$d")
done

violations=0
while IFS= read -r match; do
  [[ -n "$match" ]] || continue
  # match form: <relpath>:<lineno>:<content>
  file="${match%%:*}"
  rest="${match#*:}"
  lineno="${rest%%:*}"
  content="${rest#*:}"
  if ! grep -qiE "$HEDGE" <<<"$content"; then
    rel="${file#"$ROOT"/}"
    printf 'FAIL: %s:%s presents a hooks runtime as live without a hookless/opt-in/historical hedge\n' \
      "$rel" "$lineno" >&2
    printf '      %s\n' "$content" >&2
    violations=$((violations + 1))
  fi
done < <(grep -rnIE "$PAT" "$DOCS" "${exclude_args[@]}" || true)

if [[ "$violations" -gt 0 ]]; then
  fail "$violations live-hooks reference(s) re-entered live-facing docs. Reframe as explicit \`ao\` commands, or hedge as opt-in (author via the hooks-authoring skill) / historical."
fi

printf 'PASS: no live-hooks references in live-facing docs/\n'
