#!/usr/bin/env bash
# skill-eval.sh — gate a single skill's SKILL.md through Jeff Emanuel's `ms`
# (meta_skill) linter + validator.
#
# Thin wrapper around `ms lint` + `ms validate`. The contract:
#
#   * BLOCKING ms rules  -> the script exits non-zero (a real gate failure).
#   * WARNING/info rules -> annotated only (printed, never fail the gate).
#   * `ms` not on PATH   -> LOUD hard-fail (`::error::` + non-zero exit).
#                           NEVER skip-and-pass: a silent skip recreates the
#                           exact "no skill evaluation" gap this gate closes.
#
# BLOCKING rule IDs (ms `rule_id` values from `ms lint --list-rules`):
#
#   no-secrets         hardcoded secrets / API keys / credentials
#   no-injection       prompt-injection payloads
#   safe-paths         unsafe / dangerous file paths (doc-link "../" refs are
#                      filtered out as false-positives — see ag-eatf filter)
#   required-metadata  missing id / name / description
#   no-cycle           circular skill dependencies
#   valid-version      version must be valid semver
#
# (ms's *own* severity/exit code is intentionally NOT trusted as the gate: ms
# reports several of these as `warning` and exits 0. This wrapper decides
# gate-fail purely on whether a blocking rule_id appears in the findings.)
#
# Usage:
#   scripts/skill-eval.sh <skill-id>            # skills/<skill-id>/SKILL.md
#   scripts/skill-eval.sh _fixtures/bad-skill   # nested id under skills/
#   scripts/skill-eval.sh path/to/SKILL.md      # explicit path
#   scripts/skill-eval.sh --help
#
# Env:
#   MS_BIN          override the `ms` binary (default: ms on PATH)
#   SKILL_EVAL_SKILLS_ROOT   skills/ root (default: <repo>/skills)
#
# Exit codes:
#   0  gate passed (no blocking findings)
#   1  gate failed (one or more blocking findings)
#   2  usage error (bad args / skill not found)
#   3  tooling unavailable (ms missing, or ms produced unparseable output)

set -euo pipefail

# --- blocking rule set ------------------------------------------------------
BLOCKING_RULES=(
	no-secrets
	no-injection
	safe-paths
	required-metadata
	no-cycle
	valid-version
)

PROG="$(basename "$0")"
MS_BIN="${MS_BIN:-ms}"

err() { printf '%s\n' "$*" >&2; }
# GitHub-Actions workflow-command annotations; harmless plain text locally.
annotate_error() { printf '::error::%s\n' "$*" >&2; }
annotate_warn() { printf '::warning::%s\n' "$*" >&2; }

# safe-paths false-positive filter (ag-eatf) --------------------------------
# ms's `safe-paths` rule is a blunt regex that flags EVERY "../" — including
# the relative markdown doc-links that are the repo-wide SKILL.md convention
# (47+ skills use `[text](../other-skill/SKILL.md)` and `../../docs/...md`).
# SKILL.md is documentation, not executed code, so a "../" inside a markdown
# link target or a relative path to a doc (*.md/.markdown/.mdx/.txt/.rst) is
# not a real path-traversal threat — flagging it is pure noise that reds the
# gate on every skill-touching PR. This filter preserves real protection:
# it returns 0 (a REAL violation exists -> KEEP safe-paths blocking) only when
# a "../" survives stripping (1) markdown inline-link targets `](...)`, (2)
# relative doc-path tokens (*.md/.markdown/.mdx/.txt/.rst), and (3) backtick-
# wrapped relative paths that are REPO-INTERNAL (<=2 "../" from a depth-2
# skills/<id>/SKILL.md, no intermediate "..") AND end in a known repo-file
# extension. A backtick path that ESCAPES the repo (>=3 "../") or has no
# extension still counts as a real violation -> blocks — extension alone is not
# enough (a backtick `../../../../etc/cron.d/x.sh` must NOT be exempt). Both
# constraints came from cross-model quorum (Codex: repo-internal; agy: escape-
# via-allowed-extension). Otherwise returns 1 (every "../" is a doc reference
# -> false positive, downgrade to advisory).
safe_paths_has_real_violation() {
	local file="$1"
	local stripped
	stripped="$(sed -E \
		-e 's/\]\([^)]*\)//g' \
		-e 's#(\.\./)+[A-Za-z0-9_.@/-]+\.(md|markdown|mdx|txt|rst)([#][A-Za-z0-9_-]+)?##g' \
		-e 's#`(\.\./){1,2}([A-Za-z0-9_@-]+/)*[A-Za-z0-9_.@-]+\.(md|markdown|mdx|txt|rst|json|ya?ml|toml|sh|go|py|ts|rs)`##g' \
		"$file")"
	printf '%s' "$stripped" | grep -qF '../'
}

usage() {
	cat <<EOF
$PROG — gate one skill's SKILL.md through \`ms lint\` + \`ms validate\`

Usage:
  $PROG <skill-id>            evaluate skills/<skill-id>/SKILL.md
  $PROG <nested/id>           nested id (e.g. _fixtures/bad-skill)
  $PROG <path/to/SKILL.md>    evaluate an explicit SKILL.md path
  $PROG --help

Blocking rules (gate-fail -> non-zero exit): ${BLOCKING_RULES[*]}
Everything else is annotated only.

If \`ms\` is not on PATH the script HARD-FAILS (::error:: + non-zero) — it
never skips-and-passes.
EOF
}

# Resolve the SKILL.md path from a skill-id or explicit path argument.
resolve_skill_md() {
	local arg="$1"
	# Explicit path to a SKILL.md (or any file): use as-is.
	if [ -f "$arg" ]; then
		printf '%s\n' "$arg"
		return 0
	fi
	# Treat as a skill id under the skills root.
	local repo_root skills_root candidate
	repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
	skills_root="${SKILL_EVAL_SKILLS_ROOT:-$repo_root/skills}"
	candidate="$skills_root/$arg/SKILL.md"
	if [ -f "$candidate" ]; then
		printf '%s\n' "$candidate"
		return 0
	fi
	return 1
}

main() {
	case "${1:-}" in
	-h | --help)
		usage
		exit 0
		;;
	"")
		err "$PROG: missing <skill-id> argument"
		usage >&2
		exit 2
		;;
	-*)
		err "$PROG: unknown flag: $1"
		usage >&2
		exit 2
		;;
	esac

	local skill_arg="$1"
	local skill_md
	if ! skill_md="$(resolve_skill_md "$skill_arg")"; then
		annotate_error "skill-eval: cannot find SKILL.md for '$skill_arg' (looked for an existing file and skills/<id>/SKILL.md)"
		exit 2
	fi

	# --- HARD-FAIL if ms is not available. Never skip-and-pass. -------------
	if ! command -v "$MS_BIN" >/dev/null 2>&1; then
		annotate_error "skill-eval: \`$MS_BIN\` (meta_skill / Jeff Emanuel's \`ms\`) is NOT on PATH — cannot evaluate '$skill_md'. Install it (\`cargo install --path /path/to/meta_skill --bin ms\`) and re-run. Refusing to skip-and-pass: a silent skip would recreate the no-evaluation gap this gate exists to close."
		exit 3
	fi
	if ! command -v jq >/dev/null 2>&1; then
		annotate_error "skill-eval: \`jq\` is required to parse ms JSON output but is not on PATH."
		exit 3
	fi

	err "skill-eval: evaluating $skill_md"
	err "skill-eval: using $("$MS_BIN" --version 2>/dev/null || echo "$MS_BIN")"

	# --- ms lint (JSON) -----------------------------------------------------
	# `ms lint` may exit non-zero on its own; capture output regardless and
	# decide the gate ourselves from the diagnostics' rule_ids.
	local lint_json
	if ! lint_json="$("$MS_BIN" lint "$skill_md" -f json 2>/dev/null)"; then
		: # non-zero exit from ms lint is fine; we parse the JSON below.
	fi
	if [ -z "$lint_json" ] || ! printf '%s' "$lint_json" | jq -e . >/dev/null 2>&1; then
		annotate_error "skill-eval: \`ms lint\` produced no parseable JSON for '$skill_md'."
		exit 3
	fi

	# --- ms validate (JSON) — advisory layer on top of lint ----------------
	# `ms validate` hard-errors (no JSON) on some malformed specs, e.g. a
	# missing skill id ("skill ID is required"). That same defect is already
	# caught by the `required-metadata` lint rule, which IS the gate source of
	# truth. So validate is best-effort: parse it when it yields JSON, and when
	# it cannot, annotate (do NOT exit 3 — the lint gate below still decides).
	local validate_json="" validate_ok=0
	if validate_json="$("$MS_BIN" validate "$skill_md" -O json 2>/dev/null)" &&
		[ -n "$validate_json" ] && printf '%s' "$validate_json" | jq -e . >/dev/null 2>&1; then
		validate_ok=1
	else
		annotate_warn "skill-eval [validate]: \`ms validate\` could not produce JSON for '$skill_md' (likely a metadata defect already flagged by lint); relying on lint diagnostics for the gate."
	fi

	# --- partition lint diagnostics into blocking vs. non-blocking ----------
	# jq filter selecting blocking-rule diagnostics.
	local blocking_filter
	blocking_filter="$(printf '%s\n' "${BLOCKING_RULES[@]}" | jq -R . | jq -s '. as $b | [.[]]' | jq -c '{rules: .}')"

	# safe-paths is partitioned separately (it gets the ag-eatf doc-link
	# false-positive filter below); the primary blocking bucket excludes it and
	# real safe-paths violations are folded back in afterward.
	local blocking_lines safepaths_lines nonblocking_lines
	blocking_lines="$(
		printf '%s' "$lint_json" | jq -r --argjson cfg "$blocking_filter" '
			[.files[].diagnostics[]] as $d
			| $d[]
			| select(.rule_id != "safe-paths")
			| select(.rule_id as $r | ($cfg.rules | index($r)) != null)
			| "\(.rule_id)\t\(.severity)\t\(.message)"
		'
	)"
	safepaths_lines="$(
		printf '%s' "$lint_json" | jq -r '
			[.files[].diagnostics[]] as $d
			| $d[]
			| select(.rule_id == "safe-paths")
			| "\(.rule_id)\t\(.severity)\t\(.message)"
		'
	)"
	nonblocking_lines="$(
		printf '%s' "$lint_json" | jq -r --argjson cfg "$blocking_filter" '
			[.files[].diagnostics[]] as $d
			| $d[]
			| select(.rule_id != "safe-paths")
			| select(.rule_id as $r | ($cfg.rules | index($r)) == null)
			| "\(.rule_id)\t\(.severity)\t\(.message)"
		'
	)"

	# --- safe-paths doc-link false-positive filter (ag-eatf) ----------------
	# Fold safe-paths findings into BLOCKING only if a real (non-doc-link) "../"
	# survives in the source; otherwise downgrade them to advisory annotations.
	if [ -n "$safepaths_lines" ]; then
		if safe_paths_has_real_violation "$skill_md"; then
			blocking_lines="$(printf '%s\n%s' "$blocking_lines" "$safepaths_lines" | sed '/^[[:space:]]*$/d')"
		else
			local sp_count
			sp_count="$(printf '%s' "$safepaths_lines" | grep -c .)"
			annotate_warn "skill-eval [safe-paths]: $sp_count '../' finding(s) are relative markdown doc-links / doc paths (repo SKILL.md convention) — non-blocking false-positives (ag-eatf). SKILL.md is documentation, not executed; no real path traversal."
			nonblocking_lines="$(printf '%s\n%s' "$nonblocking_lines" "$safepaths_lines" | sed '/^[[:space:]]*$/d')"
		fi
	fi

	# --- annotate non-blocking findings -------------------------------------
	if [ -n "$nonblocking_lines" ]; then
		while IFS=$'\t' read -r rule sev msg; do
			[ -z "$rule" ] && continue
			annotate_warn "skill-eval [$rule/$sev]: $msg"
		done <<<"$nonblocking_lines"
	fi

	# validate warnings are advisory (structural hints like missing tags).
	if [ "$validate_ok" -eq 1 ]; then
		local validate_warn_count
		validate_warn_count="$(printf '%s' "$validate_json" | jq -r '(.warnings // []) | length')"
		if [ "$validate_warn_count" -gt 0 ]; then
			printf '%s' "$validate_json" | jq -r '(.warnings // [])[] | "\(.field)\t\(.message)"' |
				while IFS=$'\t' read -r field msg; do
					[ -z "$field" ] && continue
					annotate_warn "skill-eval [validate/$field]: $msg"
				done
		fi
	fi

	# --- decide the gate ----------------------------------------------------
	if [ -n "$blocking_lines" ]; then
		annotate_error "skill-eval: BLOCKING findings for '$skill_md' — gate failed:"
		while IFS=$'\t' read -r rule sev msg; do
			[ -z "$rule" ] && continue
			annotate_error "  [$rule/$sev] $msg"
		done <<<"$blocking_lines"
		exit 1
	fi

	err "skill-eval: PASS — no blocking findings for $skill_md"
	exit 0
}

main "$@"
