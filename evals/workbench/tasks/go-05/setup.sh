#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../go-cli" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Add a RunCommand function with command injection vulnerability to main.go
# Insert import for os/exec and the vulnerable function

# First, add os/exec to imports
sed -i 's|"os"|"os"\n\t"os/exec"|' "$WORKDIR/cmd/wb/main.go"

# Add the vulnerable RunCommand function and a "run" case to main
cat >> "$WORKDIR/cmd/wb/main.go" << 'GOEOF'

// RunCommand executes a user-provided system command and prints output.
func RunCommand(userInput string) (string, error) {
	cmd := exec.Command("sh", "-c", userInput)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
GOEOF

# Add "run" case to the main switch
sed -i '/case "store":/i\\tcase "run":\n\t\tif len(os.Args) < 3 {\n\t\t\tfmt.Fprintln(os.Stderr, "Usage: wb run <command>")\n\t\t\tos.Exit(1)\n\t\t}\n\t\tout, err := RunCommand(os.Args[2])\n\t\tif err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, "error: %v\\n", err)\n\t\t\tos.Exit(1)\n\t\t}\n\t\tfmt.Print(out)' "$WORKDIR/cmd/wb/main.go"

# Update usage string to include run command
sed -i 's|store list                 List all key-value pairs`|store list                 List all key-value pairs\n  run <command>              Run a system command`|' "$WORKDIR/cmd/wb/main.go"
