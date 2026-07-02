//go:build flywheel

// practices: [fail-closed-safety, wiki-knowledge-surface]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/corpusscan"
)

var corpusScanJSON bool

// corpusScanExitError (the typed exit-code error) lives in the untagged
// corpus_scan_error.go so root.go's spine Execute() switch can type-assert it
// after this command archives behind //go:build flywheel (age-nzwo).

const (
	corpusScanClean    = 0
	corpusScanLeak     = 1
	corpusScanInternal = 2
)

// corpusScanCmd is the layer-3 deny/PII leak detector for the corpus
// public/publish pipeline. It scans RENDERED text for the canonical marker
// registry (cli/internal/corpusscan) and FAILS CLOSED on any hit. It never
// modifies a file — detect only, never redact.
var corpusScanCmd = &cobra.Command{
	Use:   "scan <path>",
	Short: "Fail-closed deny/PII leak scan of rendered corpus output (detect only, never redact)",
	Long: `Scan a file or directory of RENDERED public text (markdown/json/txt/html)
for fleet, client, peer-agent, private-namespace, mythology, brand, and
landmine leak markers before publication.

FAIL CLOSED: any single marker hit — OR any file that cannot be read — exits
nonzero. A fully clean tree exits 0. The scanner only DETECTS; it never
modifies, redacts, or rewrites a file.

The marker registry is the single canonical set in cli/internal/corpusscan,
shared with the CI publish gate so detection can never drift between them.

Exit codes:
  0  clean (publishable)
  1  leak detected or a file could not be read (FAIL CLOSED)
  2  internal error

  ao corpus scan public/wiki/
  ao corpus scan public/wiki/index.md --json`,
	Args: cobra.ExactArgs(1),
	RunE: runCorpusScan,
}

func init() {
	corpusCmd.AddCommand(corpusScanCmd)
	corpusScanCmd.Flags().BoolVar(&corpusScanJSON, "json", false, "Emit the scan report as JSON (per-file hits)")
}

// runCorpusScan is the RunE entry point for `ao corpus scan`.
func runCorpusScan(cmd *cobra.Command, args []string) error {
	cmd.SilenceErrors = true
	path := args[0]

	rep, err := corpusscan.Scan(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ao corpus scan: "+err.Error())
		return &corpusScanExitError{code: corpusScanInternal, msg: err.Error()}
	}

	if corpusScanJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Root       string                  `json:"root"`
			Clean      bool                    `json:"clean"`
			HitCount   int                     `json:"hit_count"`
			ErrorCount int                     `json:"error_count"`
			MarkerSet  int                     `json:"marker_set_size"`
			Files      []corpusscan.FileResult `json:"files"`
		}{
			Root:       rep.Root,
			Clean:      rep.Clean(),
			HitCount:   rep.HitCount(),
			ErrorCount: rep.ErrorCount(),
			MarkerSet:  corpusscan.MarkerCount(),
			Files:      rep.Files,
		})
	} else {
		printCorpusScanHuman(rep)
	}

	if !rep.Clean() {
		return &corpusScanExitError{
			code: corpusScanLeak,
			msg:  fmt.Sprintf("corpus scan FAILED CLOSED: %d hit(s), %d unreadable file(s)", rep.HitCount(), rep.ErrorCount()),
		}
	}
	return nil
}

// printCorpusScanHuman renders each hit by name, class, file, and line.
func printCorpusScanHuman(rep corpusscan.Report) {
	if rep.Clean() {
		fmt.Printf("CLEAN: no leak markers in %s (registry: %d markers)\n", rep.Root, corpusscan.MarkerCount())
		return
	}
	fmt.Printf("FAIL CLOSED: leak markers detected in %s\n", rep.Root)
	for _, f := range rep.Files {
		if f.Clean() {
			continue
		}
		if f.Err != "" {
			fmt.Printf("  %s: UNREADABLE (treated as unsafe): %s\n", f.Path, f.Err)
			continue
		}
		for _, h := range f.Hits {
			fmt.Printf("  %s:%d  [%s/%s]  %q\n", f.Path, h.Line, h.Class, h.Marker, h.Match)
		}
	}
	fmt.Printf("\n%d hit(s), %d unreadable file(s). NOT publishable.\n", rep.HitCount(), rep.ErrorCount())
}
