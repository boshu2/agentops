#!/usr/bin/env bash
# membrane-health — pack doctor check: "is the AgentOps verification membrane
# actually deployable-and-sound in THIS city?".
#
# Stock gc already ships the housekeeping substrate (20 orders, gc doctor's 73
# checks, events, dashboard). This check adds ONLY the membrane-specific
# self-verification the substrate cannot know about: it proves the fail-closed
# CLOSE DOOR itself is installed, executable, and wired, and that the importing
# city carries the cross-family posture the door structurally requires.
#
# It asserts, in the city that imports agentops-membrane:
#   1. the close door is present + executable — membrane/close-gate.sh,
#      membrane/finalize.sh, and the deterministic verdict membrane/finalize.jq;
#   2. the membrane-quest formula resolves (file present; and, best-effort, that
#      `gc formula list` reports it);
#   3. the trinity agents are present — planner, builder, verifier (+ the
#      agy-verifier third family);
#   4. >=2 DISTINCT provider families are configured — the cross-family
#      precondition the close gate requires. A 1-family city can NEVER emit a
#      CONFIRMED verdict (finalize.jq rejects fewer_than_two_families), so this
#      is flagged EARLY here rather than discovered on the first close attempt.
#
# Doctor protocol: exit 0 = OK, 1 = warning, 2 = error (blocking). First stdout
# line = message; remaining lines = detail (verbose mode). Deps: bash + jq only
# (toolchain-free, same posture as doctor/law0-print-args).
#
# Env contract (internal/doctor/pack_checks.go): cwd = pack dir; GC_PACK_DIR =
# absolute pack dir; GC_CITY_PATH = city root; GC_PACK_STATE_DIR set.
set -u

PACK="${GC_PACK_DIR:-$PWD}"
city="${GC_CITY_PATH:-$PWD}"

gc_bin="${GC_BIN:-}"
if [ -z "$gc_bin" ] || [ ! -x "$gc_bin" ]; then gc_bin="$(command -v gc || true)"; fi
if ! command -v jq >/dev/null 2>&1; then
  echo "membrane-health: jq not found (this check needs jq)"; exit 2
fi

problems=()   # each entry => a blocking failure (exit 2)
detail=()     # verbose-mode context lines

# --- 1. the close door is installed + executable ----------------------------
# .sh must be executable (the control-dispatcher execs them directly); .jq is
# read by jq so it only needs to exist.
for f in membrane/close-gate.sh membrane/finalize.sh; do
  if [ ! -f "$PACK/$f" ]; then
    problems+=("close door missing: $f")
  elif [ ! -x "$PACK/$f" ]; then
    problems+=("close door not executable: $f (chmod +x)")
  else
    detail+=("ok: $f present + executable")
  fi
done
if [ ! -f "$PACK/membrane/finalize.jq" ]; then
  problems+=("deterministic verdict missing: membrane/finalize.jq")
else
  detail+=("ok: membrane/finalize.jq present")
fi

# --- 2. the membrane-quest formula resolves ---------------------------------
if [ ! -f "$PACK/formulas/membrane-quest.toml" ]; then
  problems+=("membrane-quest formula missing: formulas/membrane-quest.toml")
else
  detail+=("ok: formulas/membrane-quest.toml present")
fi
# best-effort: confirm the city actually resolves the formula (non-fatal — the
# --json shape can drift across gc versions; the file check above is the gate).
if [ -n "$gc_bin" ]; then
  if flist="$( (cd "$city" && "$gc_bin" formula list --json) 2>/dev/null)"; then
    if printf '%s' "$flist" | jq -e '..|strings|select(test("(^|[.:/])membrane-quest$"))' >/dev/null 2>&1 \
       || printf '%s' "$flist" | grep -q 'membrane-quest'; then
      detail+=("ok: gc formula list resolves membrane-quest")
    else
      detail+=("note: gc formula list did not report membrane-quest (run 'gc import install'?)")
    fi
  fi
fi

# --- 3. trinity agents present ----------------------------------------------
for a in planner builder verifier agy-verifier; do
  if [ ! -f "$PACK/agents/$a/agent.toml" ]; then
    problems+=("trinity agent missing: agents/$a/agent.toml")
  else
    detail+=("ok: agent $a present")
  fi
done

# --- 4. cross-family precondition: >=2 DISTINCT provider families -----------
# Derive a family token per configured provider from its Base/Command, exactly
# as the close gate's lanes are cross-family (builder=claude vs {codex=gpt,
# agy=gemini}). finalize.jq REFUTES/DEGRADES any quorum with <2 families, so a
# 1-family city can never CONFIRM — flag it here, not at first close.
fam_json=""
if [ -n "$gc_bin" ]; then
  fam_json="$( (cd "$city" && "$gc_bin" config show --json) 2>/dev/null )"
fi
if [ -z "$fam_json" ]; then
  problems+=("cannot read providers: '$gc_bin config show --json' failed in $city")
else
  families="$(printf '%s' "$fam_json" | jq -r '
    def fam:
      (.Base // "") as $b | (.Command // "") as $c
      | if   ($b == "builtin:claude")      or ($c == "claude")      then "claude"
        elif ($b == "builtin:codex")       or ($c == "codex")       then "gpt"
        elif ($b == "builtin:antigravity") or ($c == "agy")
             or ($c == "gemini")           or ($c == "antigravity") then "gemini"
        elif ($b == "builtin:pi")          or ($c == "pi")          then "pi"
        elif ($b | startswith("builtin:")) then ($b | sub("builtin:"; ""))
        elif ($b != "") then $b
        elif ($c != "") then $c
        else empty end;
    [ (.config.Providers // {}) | to_entries[]
      | (.key) as $k | (.value | fam) // $k ]
    | map(select(. != "" and . != null)) | unique | join(",")
  ' 2>/dev/null)"
  nfam=0
  [ -n "$families" ] && nfam="$(printf '%s' "$families" | tr ',' '\n' | grep -c .)"
  if [ "$nfam" -lt 2 ]; then
    problems+=("cross-family precondition UNMET: only $nfam distinct provider family(ies) configured [${families:-none}] — the close gate requires >=2; a 1-family city can NEVER CONFIRM (finalize.jq: fewer_than_two_families). Add a second family, e.g. [providers.codex] base=\"builtin:codex\" and/or [providers.antigravity] base=\"builtin:antigravity\".")
  else
    detail+=("ok: $nfam distinct provider families configured [$families]")
  fi
fi

# --- verdict ----------------------------------------------------------------
if [ "${#problems[@]}" -gt 0 ]; then
  echo "membrane NOT sound (${#problems[@]} issue(s)) — the close door is not deployable here"
  for p in "${problems[@]}"; do echo "  FAIL: $p"; done
  for d in "${detail[@]}"; do echo "  $d"; done
  exit 2
fi

echo "membrane sound — close door installed + executable, trinity present, cross-family posture met"
for d in "${detail[@]}"; do echo "  $d"; done
exit 0
