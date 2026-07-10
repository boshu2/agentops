#!/bin/bash
#
# verify-buildtags.sh — prove the ADR-0012 build-tag archive mechanism.
#
# The default `go build` must OMIT the archived command sets; the `flywheel` and
# `legacy` tags must restore them. We can't grep for archived commands until
# Tagged builds retain the hidden `ao buildtags` introspection surface. The
# default ADR-0012 spine deliberately removes that archived command entirely:
#   default build        -> `ao buildtags` absent
#   -tags flywheel       -> "flywheel"
#   -tags legacy         -> "legacy"
#   -tags flywheel legacy-> "flywheel" + "legacy"
#
# Exit codes: 0 = mechanism works; 1 = a variant did not report as expected;
#             2 = script/build error.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLI="$ROOT/cli"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

build() { # build <out> [tags...]
	local out="$1"; shift
	local tagflag=()
	if [ "$#" -gt 0 ]; then tagflag=(-tags "$*"); fi
	( cd "$CLI" && go build "${tagflag[@]}" -o "$out" ./cmd/ao ) || {
		echo "FAIL: go build (tags='$*') errored — archived code must stay buildable" >&2
		exit 2
	}
}

expect() { # expect <binary> <substr> <present|absent>
	local bin="$1" needle="$2" mode="$3" out
	out="$("$bin" buildtags 2>/dev/null || true)"
	# Whole-line match: a tag is emitted on its own line, so the spine's
	# explanatory text ("…corpus/flywheel…") never false-matches a tag check.
	case "$mode" in
		present)
			if ! printf '%s\n' "$out" | grep -qx "$needle"; then
				echo "FAIL: '$bin buildtags' = '$out' — expected a line '$needle'" >&2
				exit 1
			fi ;;
		absent)
			if printf '%s\n' "$out" | grep -qx "$needle"; then
				echo "FAIL: '$bin buildtags' = '$out' — expected NO line '$needle'" >&2
				exit 1
			fi ;;
	esac
}

echo "verify-buildtags: default (spine) build…"
build "$TMP/ao-spine"
if spine_out="$("$TMP/ao-spine" buildtags 2>&1)"; then
	echo "FAIL: default build must omit archived 'ao buildtags'" >&2; exit 1
fi
if ! printf '%s\n' "$spine_out" | grep -q 'unknown command "buildtags"'; then
	echo "FAIL: default build did not reject archived 'ao buildtags': $spine_out" >&2; exit 1
fi

echo "verify-buildtags: -tags flywheel…"
build "$TMP/ao-flywheel" flywheel
expect "$TMP/ao-flywheel" "flywheel" present
expect "$TMP/ao-flywheel" "legacy"   absent

echo "verify-buildtags: -tags legacy (AGENTOPS_LEGACY path)…"
build "$TMP/ao-legacy" legacy
expect "$TMP/ao-legacy" "legacy"   present
expect "$TMP/ao-legacy" "flywheel" absent

echo "verify-buildtags: -tags 'flywheel legacy' (make build-flywheel)…"
build "$TMP/ao-all" flywheel legacy
expect "$TMP/ao-all" "flywheel" present
expect "$TMP/ao-all" "legacy"   present

echo "OK: build-tag mechanism verified — default build omits archived sets; flywheel/legacy restore them."
