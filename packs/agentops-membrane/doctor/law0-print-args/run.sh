#!/usr/bin/env bash
# law0-print-args — pack doctor check enforcing LAW 0: no `claude -p`, ever, and
# no headless one-shot `--print` sink on any sub-backed provider.
#
# Asserts that EVERY provider that resolves to a print-capable coding CLI —
# claude (Base=="builtin:claude", Command=="claude", or the "claude" key with no
# command override) AND the antigravity/gemini one-shot sink (Base=="builtin:
# antigravity", Command in {agy,gemini,antigravity}, or the "antigravity"/
# "gemini" keys) — declares print_args = [] (an EMPTY, NON-NULL array). A null
# PrintArgs means the builtin default (claude ["-p"], antigravity ["--print"])
# is LIVE — the async title-gen and `gc prompt` sinks that bill the API / burn
# the weekly quota. Non-empty means someone re-armed print mode. Both violate
# LAW 0. Absent claude provider table = builtin default applies unguarded = FAIL.
#
# Doctor protocol: exit 0 = OK, 1 = warning, 2 = error (blocking). First stdout
# line = message. Deps: bash + jq only.
set -u

city="${GC_CITY_PATH:-$PWD}"
gc_bin="${GC_BIN:-}"
if [ -z "$gc_bin" ] || [ ! -x "$gc_bin" ]; then gc_bin="$(command -v gc || true)"; fi
if [ -z "$gc_bin" ]; then echo "law0: cannot locate gc binary (set GC_BIN or put gc on PATH)"; exit 2; fi
if ! command -v jq >/dev/null 2>&1; then echo "law0: jq not found (this check needs jq)"; exit 2; fi
if ! json="$(cd "$city" && "$gc_bin" config show --json 2>/dev/null)"; then
  echo "law0: '$gc_bin config show --json' failed in $city"; exit 2
fi

verdict="$(jq -r '
  (.config.Providers // {}) as $p
  # claude family (the LAW 0 core: claude -p)
  | [ $p | to_entries[]
      | select((.value.Command == "claude") or (.value.Base == "builtin:claude")
               or ((.key == "claude") and ((.value.Command // "") == ""))) ] as $claude
  # antigravity/gemini one-shot --print sink (AGY path). NB: compare Command with
  # explicit ==, never `index(.value...)` — inside index(), `.` is the array
  # literal, so `.value` would index the array (jq: "cannot index array").
  | [ $p | to_entries[]
      | select((.value.Base == "builtin:antigravity")
               or (.value.Command == "agy") or (.value.Command == "gemini")
               or (.value.Command == "antigravity")
               or (((.key == "antigravity") or (.key == "gemini")) and ((.value.Command // "") == ""))) ] as $agy
  | ($claude + $agy | unique_by(.key)) as $printers
  | if ($claude | length) == 0 then
      "FAIL no claude provider table declared — builtin default print_args [\"-p\"] is live; add [providers.claude] with print_args = []"
    else
      [ $printers[]
        | select((.value.PrintArgs == null) or ((.value.PrintArgs | length) > 0))
        | .key + (if .value.PrintArgs == null
                  then " (print_args missing — builtin default live)"
                  else " (print_args = " + (.value.PrintArgs | tojson) + ")" end) ] as $bad
      | if ($bad | length) > 0 then
          "FAIL provider(s) able to run a headless print sink: " + ($bad | join(", "))
        else
          "PASS " + ([$printers[].key] | join(", ")) + ": print_args = [] (headless print sinks structurally dead)"
        end
    end
' <<<"$json")" || { echo "law0: could not parse gc config show --json output"; exit 2; }

case "$verdict" in
  PASS*) echo "LAW 0 ok — ${verdict#PASS }"; exit 0 ;;
  FAIL*) echo "LAW 0 VIOLATION — ${verdict#FAIL }"; exit 2 ;;
  *)     echo "law0: internal error — unexpected verdict: $verdict"; exit 2 ;;
esac
