// skill-trial-report is a development-only reader for native Harbor outputs.
// Run from cli: go run ./cmd/skill-trial-report --job control=/path/to/job
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boshu2/agentops/cli/internal/skilltrial"
)

type values []string

func (v *values) String() string         { return strings.Join(*v, ", ") }
func (v *values) Set(value string) error { *v = append(*v, value); return nil }

func run(args []string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("skill-trial-report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var jobs, sessions values
	var reward string
	flags.Var(&jobs, "job", "explicit [arm=]Harbor job directory (repeatable; arm is a caller label)")
	flags.Var(&sessions, "session", "additional explicit native Codex JSONL file (repeatable; never scans operator home)")
	flags.StringVar(&reward, "reward-key", "reward", "native verifier reward key; only 1 and 0 classify success and failure")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments; use --job or --session")
	}
	inputs := make([]skilltrial.JobInput, 0, len(jobs))
	for _, value := range jobs {
		arm, dir, ok := strings.Cut(value, "=")
		if !ok {
			dir, arm = arm, ""
		}
		if dir == "" {
			return fmt.Errorf("job directory must not be empty")
		}
		inputs = append(inputs, skilltrial.JobInput{Arm: arm, Directory: dir})
	}
	report, err := skilltrial.Build(inputs, sessions, reward)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
