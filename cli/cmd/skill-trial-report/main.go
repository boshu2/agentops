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

	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/boshu2/agentops/cli/internal/skilltrial"
)

type values []string

func (v *values) String() string         { return strings.Join(*v, ", ") }
func (v *values) Set(value string) error { *v = append(*v, value); return nil }

func run(args []string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("skill-trial-report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var jobs, sessions values
	var providers, verdicts values
	var judgment evidence.JudgmentOptions
	var profiles string
	var reward string
	flags.Var(&jobs, "job", "explicit [arm=]Harbor job directory (repeatable; arm is a caller label)")
	flags.Var(&sessions, "session", "additional explicit native Codex JSONL file (repeatable; never scans operator home)")
	flags.StringVar(&reward, "reward-key", "reward", "native verifier reward key; only 1 and 0 classify success and failure")
	flags.StringVar(&judgment.Root, "root", "", "explicit subject directory for one independent judgment join")
	flags.StringVar(&judgment.Manifest, "manifest", "", "expected subject-manifest.v1 file")
	flags.StringVar(&judgment.BaseManifest, "base-manifest", "", "base manifest for deletions")
	flags.StringVar(&judgment.Intent, "intent", "", "independent immutable acceptance file")
	flags.StringVar(&judgment.EvidenceRoot, "evidence-root", "", "private non-Git root for original verdicts, receipts and reviewer transcripts")
	flags.StringVar(&judgment.AuthorContextID, "author-context-id", "", "independent author identity matching an explicitly supplied native session")
	flags.StringVar(&profiles, "required-profiles", "", "independent required-profiles JSON used by provenance verify-judgments")
	flags.Var(&providers, "allowed-provider", "independently authorized provider (repeatable)")
	flags.Var(&verdicts, "verdict", "original content-addressed verdict.v2 file (repeatable; missing required legs stay unproven)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments; use --job or --session")
	}
	judgmentSelected := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name != "job" && f.Name != "session" && f.Name != "reward-key" {
			judgmentSelected = true
		}
	})
	if judgmentSelected {
		for _, required := range []struct{ name, value string }{{"root", judgment.Root}, {"manifest", judgment.Manifest}, {"intent", judgment.Intent}, {"evidence-root", judgment.EvidenceRoot}, {"author-context-id", judgment.AuthorContextID}, {"required-profiles", profiles}} {
			if required.value == "" {
				return fmt.Errorf("judgment join requires --%s", required.name)
			}
		}
		if len(providers) == 0 {
			return fmt.Errorf("judgment join requires --allowed-provider")
		}
		var err error
		judgment.Required, err = evidence.LoadJudgeProfiles(profiles)
		if err != nil {
			return err
		}
		judgment.AllowedProviders, judgment.Verdicts = providers, verdicts
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
	if judgmentSelected {
		report.Work = skilltrial.InspectWork(report, judgment)
		report.Limits = append(report.Limits, "Work joins one explicitly selected subject to its observed native author. Accepted requires completed execution and the existing verifier's satisfied independent PASS coverage; endpoint reward remains separate. A verified FAIL is retained as failed, while missing or invalid proof is not_proven. Association cannot choose among different session copies or multiple trials.", "Judgment verification binds original verdicts, subject/acceptance, receipts and native reviewer identities. Arbitrary criterion evidence references are not mechanically resolved; the fresh reviewer owns their semantic assessment. The reader issues no semantic verdict and establishes no complete billing or general skill benefit.")
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
