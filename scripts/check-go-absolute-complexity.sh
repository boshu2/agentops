#!/usr/bin/env bash
set -euo pipefail

# Check that no Go function in the given directory exceeds a complexity threshold.
# Unlike check-go-complexity.sh (delta-only), this checks ALL functions — not just changed ones.

usage() {
  cat <<USAGE
Usage: $0 [--dir <path>] [--threshold <n>] [--per-file]

Checks that no non-test Go function exceeds the given cyclomatic complexity threshold.

Options:
  --dir        Directory to scan (default: cli/)
  --threshold  Max allowed complexity (default: 10)
  --per-file   Output per-file violation counts instead of raw violations
USAGE
}

DIR="cli/"
THRESHOLD=10
PER_FILE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir)       DIR="$2"; shift 2 ;;
    --threshold) THRESHOLD="$2"; shift 2 ;;
    --per-file)  PER_FILE=true; shift ;;
    -h|--help)   usage; exit 0 ;;
    *)           echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if ! command -v gocyclo >/dev/null 2>&1; then
  # Self-heal when 'go' is available: install gocyclo into GOBIN / $GOPATH/bin.
  # Set GOCYCLO_NO_AUTOINSTALL=1 to opt out (CI already pre-installs gocyclo).
  if command -v go >/dev/null 2>&1 && [[ -z "${GOCYCLO_NO_AUTOINSTALL:-}" ]]; then
    GOBIN_DIR="${GOBIN:-}"
    if [[ -z "$GOBIN_DIR" ]]; then
      GOPATH_DIR="$(go env GOPATH 2>/dev/null || echo "$HOME/go")"
      GOBIN_DIR="$GOPATH_DIR/bin"
    fi
    mkdir -p "$GOBIN_DIR"
    echo "gocyclo not found; auto-installing into $GOBIN_DIR" >&2
    if GOBIN="$GOBIN_DIR" go install github.com/fzipp/gocyclo/cmd/gocyclo@latest >&2; then
      export PATH="$GOBIN_DIR:$PATH"
    fi
  fi
  if ! command -v gocyclo >/dev/null 2>&1; then
    echo "gocyclo not found; install with: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest" >&2
    exit 2
  fi
fi

if [[ ! -d "$DIR" ]]; then
  echo "Directory $DIR does not exist; skipping." >&2
  exit 0
fi

# Find all non-test Go files
mapfile -t GO_FILES < <(find "$DIR" -name '*.go' ! -name '*_test.go' -type f 2>/dev/null || true)

if [[ ${#GO_FILES[@]} -eq 0 ]]; then
  echo "No non-test Go files found in $DIR."
  exit 0
fi

# Run gocyclo and capture functions over threshold
VIOLATIONS=$(gocyclo -over "$THRESHOLD" "${GO_FILES[@]}" 2>/dev/null || true)

if [[ -z "$VIOLATIONS" ]]; then
  echo "All functions in $DIR are below complexity $THRESHOLD."
  exit 0
fi

if [[ "$PER_FILE" == "true" ]]; then
  # gocyclo output: <complexity> <package> <function> <file:line:col>
  # Count violations per file and report
  echo "Per-file violations (complexity > $THRESHOLD in $DIR):"
  echo "$VIOLATIONS" | awk '{
    path = $4
    sub(/:[0-9]+:[0-9]+$/, "", path)
    count[path]++
  }
  END {
    for (f in count) {
      # Strip leading directory to show just filename
      n = split(f, parts, "/")
      printf "%s: %d violation(s)\n", parts[n], count[f]
    }
  }' | sort -t: -k2 -rn
  TOTAL=$(echo "$VIOLATIONS" | wc -l | tr -d ' ')
  echo "Total: $TOTAL violation(s)"
  exit 1
fi

echo "ERROR: Functions exceeding complexity $THRESHOLD in $DIR:"
echo "$VIOLATIONS"
exit 1
