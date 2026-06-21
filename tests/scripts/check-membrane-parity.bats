#!/usr/bin/env bats
# age-membrane-memory-arch-tz2s.2.5 — check-membrane-parity.sh is the demo
# preflight that stops the e2e membrane demo (.2.6) from silently running
# against a STALE `ao` that lacks the current source's membrane surface and
# emitting a false proof. These cases exercise it deterministically via
# injectable stub binaries (AO_SRC_BIN = source reference, AO_BIN = demo-path
# binary under test) so they need no real `go build` and no installed ao.
#
# Parity is checked at TWO levels — subcommand NAMES and the per-subcommand FLAG
# surface — because a binary built from older source can carry a subcommand yet
# lack a flag the demo invokes (a name-only check would false-pass; cross-family
# refuter, 2026-06-21).

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-membrane-parity.sh"
  FIX="$(mktemp -d)"
}

teardown() { rm -rf "$FIX"; }

# make_stub <path> <spec>...  — a fake `ao` modelling cobra. Each spec is either
# "sub" or "sub=--flagA,--flagB". `membrane --help` lists the sub names;
# `membrane <sub> --help` prints a Flags: block with -h/--help plus the spec's
# flags (exit 0); any unknown subcommand exits non-zero.
make_stub() {
  local path="$1"; shift
  {
    echo '#!/usr/bin/env bash'
    echo 'if [[ "$1" == "membrane" && "$2" == "--help" ]]; then'
    echo '  echo "Available Commands:"'
    for spec in "$@"; do echo "  echo \"  ${spec%%=*}   stub ${spec%%=*}\""; done
    echo '  echo ""; echo "Flags:"; echo "  -h, --help   help"; exit 0'
    echo 'fi'
    echo 'if [[ "$1" == "membrane" ]]; then case "$2" in'
    for spec in "$@"; do
      local sub="${spec%%=*}" flags=""
      [[ "$spec" == *=* ]] && flags="${spec#*=}"
      echo "    $sub)"
      echo '      echo "Flags:"'
      echo "      echo \"  -h, --help   help for $sub\""
      if [[ -n "$flags" ]]; then
        local farr; IFS=',' read -ra farr <<< "$flags"
        for fl in "${farr[@]}"; do echo "      echo \"      $fl string   stub flag\""; done
      fi
      echo '      echo ""; echo "Global Flags:"; echo "  -o, --output string   fmt"'
      echo '      exit 0 ;;'
    done
    echo '    *) echo "unknown command \"$2\"" >&2; exit 1 ;; esac; fi'
    echo 'exit 1'
  } > "$path"
  chmod +x "$path"
}

@test "full parity: demo binary matches every source subcommand AND flag -> exit 0" {
  make_stub "$FIX/ao-src" "recall=--domain,--json" "derive-checks=--run"
  make_stub "$FIX/ao-bin" "recall=--domain,--json" "derive-checks=--run"
  run env AO_SRC_BIN="$FIX/ao-src" AO_BIN="$FIX/ao-bin" bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"full membrane parity"* ]]
}

@test "stale demo binary: missing a whole subcommand -> exit 1 names the subcommand" {
  make_stub "$FIX/ao-src" "recall=--domain" "derive-checks=--run"
  make_stub "$FIX/ao-bin" "recall=--domain"            # derive-checks absent
  run env AO_SRC_BIN="$FIX/ao-src" AO_BIN="$FIX/ao-bin" bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"STALE"* ]]
  [[ "$output" == *"missing subcommand: ao membrane derive-checks"* ]]
  [[ "$output" == *"make install"* ]]
}

@test "stale demo binary: subcommand present but missing a FLAG -> exit 1 (false-pass regression)" {
  make_stub "$FIX/ao-src" "recall=--domain" "derive-checks=--run"
  make_stub "$FIX/ao-bin" "recall=--domain" "derive-checks"   # derive-checks lacks --run
  run env AO_SRC_BIN="$FIX/ao-src" AO_BIN="$FIX/ao-bin" bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing flag: ao membrane derive-checks --run"* ]]
}

@test "robust parse: a flag named only in another flag's DESCRIPTION is not counted present" {
  # Source requires --run. Demo's derive-checks LACKS --run but mentions the
  # string "--run" inside the --help line's description. A token-grep parser
  # would false-pass; the structural column parser must still flag it stale.
  make_stub "$FIX/ao-src" "recall=--domain" "derive-checks=--run"
  cat > "$FIX/ao-bin" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == "membrane" && "$2" == "--help" ]]; then echo "Available Commands:"; echo "  derive-checks d"; echo "  recall r"; echo ""; echo "Flags:"; echo "  -h, --help"; exit 0; fi
if [[ "$1" == "membrane" && "$2" == "recall" ]]; then echo "Flags:"; echo "  -h, --help   help"; echo "      --domain string   x"; echo ""; exit 0; fi
if [[ "$1" == "membrane" && "$2" == "derive-checks" ]]; then echo "Flags:"; echo "  -h, --help   help; pass --run on newer builds"; echo ""; exit 0; fi
exit 1
STUB
  chmod +x "$FIX/ao-bin"
  run env AO_SRC_BIN="$FIX/ao-src" AO_BIN="$FIX/ao-bin" bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing flag: ao membrane derive-checks --run"* ]]
}

@test "missing demo binary: AO_BIN path is not executable -> exit 1 with remediation" {
  make_stub "$FIX/ao-src" "recall=--domain" "derive-checks=--run"
  run env AO_SRC_BIN="$FIX/ao-src" AO_BIN="$FIX/does-not-exist/ao" bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not executable"* ]]
}

@test "harness error: source reference exposes no membrane subcommands -> exit 2" {
  make_stub "$FIX/ao-src"                                     # no subcommands
  make_stub "$FIX/ao-bin" "recall=--domain" "derive-checks=--run"
  run env AO_SRC_BIN="$FIX/ao-src" AO_BIN="$FIX/ao-bin" bash "$SCRIPT"
  [ "$status" -eq 2 ]
  [[ "$output" == *"no 'membrane' subcommands"* ]]
}
