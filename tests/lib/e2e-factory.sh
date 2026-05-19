# shellcheck shell=bash
# tests/lib/e2e-factory.sh — test-data factories for e2e tests.
#
# Implements the "Test Data Factories" pattern from the
# testing-real-service-e2e-no-mocks skill. Replaces the ad-hoc inline
# "mktemp + git init + commit" boilerplate currently duplicated across
# tests/e2e/*.sh with a shared, well-tested builder.
#
# Each factory:
#   - Returns the created path on stdout (capture with $(...))
#   - Writes only inside the supplied sandbox root (never $HOME, never $PWD)
#   - Is idempotent on partial-state inputs (safe to call into an existing dir)
#
# Usage:
#   source "${SCRIPT_DIR}/../lib/e2e-factory.sh"
#   SANDBOX="$(e2e_factory_sandbox flywheel-proof)"   # mktemp -d /tmp/...-XXX
#   REPO="$(e2e_factory_repo "$SANDBOX/repo")"        # git init + initial commit
#   AO_BIN="$(e2e_factory_ao_bin "$SANDBOX/bin" "$REPO_ROOT")"  # reuse or build
#   e2e_factory_agents_dir "$REPO"                    # seed .agents/ skeleton

[[ -n "${E2E_FACTORY_SH_LOADED:-}" ]] && return 0
E2E_FACTORY_SH_LOADED=1

_e2e_factory_die() {
  printf '[e2e-factory] FATAL: %s\n' "$*" >&2
  exit 70  # EX_SOFTWARE
}

# e2e_factory_sandbox <slug>
# Creates a top-level temp directory for a single test run. Slug is used in
# the dirname for grep-ability when triaging leftover dirs.
e2e_factory_sandbox() {
  local slug="${1:-e2e}"
  local prefix="${TMPDIR:-/tmp}"
  prefix="${prefix%/}"
  local dir
  dir="$(mktemp -d "${prefix}/agentops-${slug}-XXXXXX")" \
    || _e2e_factory_die "mktemp failed under $prefix"
  printf '%s\n' "$dir"
}

# e2e_factory_repo <repo-path>
# Creates an empty git repo at <repo-path> with a deterministic identity and
# one initial commit. Idempotent: if .git already exists, leaves it alone.
e2e_factory_repo() {
  local repo="$1"
  [[ -n "$repo" ]] || _e2e_factory_die "e2e_factory_repo: path is required"
  mkdir -p "$repo"
  if [[ -d "$repo/.git" ]]; then
    printf '%s\n' "$repo"
    return 0
  fi
  (
    cd "$repo"
    git init -q
    git config user.email "e2e@agentops.test"
    git config user.name "AgentOps E2E"
    git config commit.gpgsign false
    git config init.defaultBranch main
    if [[ ! -f README.md ]]; then
      printf '# E2E Sandbox Repo\n' >README.md
      git add README.md
      git commit -q -m "init"
    fi
  ) || _e2e_factory_die "git init failed at $repo"
  printf '%s\n' "$repo"
}

# e2e_factory_ao_bin <build-dir> <repo-root>
# Resolves an ao binary into <build-dir>/ao. Honors:
#   PROOF_AO_BIN          — explicit override (must point at an executable)
#   PROOF_FORCE_BUILD=1   — disable reuse and always go build from source
#   <repo-root>/cli/bin/ao — auto-detected reuse when present
# Falls back to a fresh `go build` under <repo-root>/cli.
e2e_factory_ao_bin() {
  local build_dir="$1" repo_root="$2"
  [[ -n "$build_dir" ]] || _e2e_factory_die "e2e_factory_ao_bin: build_dir is required"
  [[ -n "$repo_root" ]] || _e2e_factory_die "e2e_factory_ao_bin: repo_root is required"
  mkdir -p "$build_dir"
  local out="$build_dir/ao"
  local src="${PROOF_AO_BIN:-}"
  if [[ -z "$src" && "${PROOF_FORCE_BUILD:-0}" != "1" && -x "$repo_root/cli/bin/ao" ]]; then
    src="$repo_root/cli/bin/ao"
  fi
  if [[ -n "$src" && -x "$src" ]]; then
    cp "$src" "$out"
  else
    command -v go >/dev/null 2>&1 \
      || _e2e_factory_die "no prebuilt ao binary and 'go' is not on PATH"
    (
      cd "$repo_root/cli"
      go build -o "$out" ./cmd/ao
    ) >/dev/null || _e2e_factory_die "go build ./cmd/ao failed"
  fi
  chmod +x "$out"
  printf '%s\n' "$out"
}

# e2e_factory_agents_dir <repo-path>
# Seeds an empty .agents/ skeleton with the directories tests typically need.
# Idempotent — only creates missing directories.
e2e_factory_agents_dir() {
  local repo="$1"
  [[ -d "$repo" ]] || _e2e_factory_die "e2e_factory_agents_dir: repo missing: $repo"
  local d
  for d in knowledge/pending pool/pending ao briefings; do
    mkdir -p "$repo/.agents/$d"
  done
}

# e2e_factory_fixture <fixture-src> <repo-path> [dest-relpath]
# Copies a fixture file from the repo's tests/fixtures/ tree into the sandbox
# repo. Refuses if the source path escapes the repo's fixtures dir (defence
# against `../../../etc/passwd`-style relpaths in tests).
e2e_factory_fixture() {
  local src="$1" repo="$2" dest_rel="${3:-}"
  [[ -f "$src" ]] || _e2e_factory_die "fixture missing: $src"
  [[ -d "$repo" ]] || _e2e_factory_die "repo missing: $repo"
  case "$src" in
    *"/tests/fixtures/"*) ;;
    *) _e2e_factory_die "fixture $src is not under tests/fixtures/ — refusing to copy" ;;
  esac
  local dest
  if [[ -n "$dest_rel" ]]; then
    dest="$repo/$dest_rel"
    mkdir -p "$(dirname "$dest")"
  else
    dest="$repo/$(basename "$src")"
  fi
  cp "$src" "$dest"
  printf '%s\n' "$dest"
}
