// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/corpus_fs"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// corpusCaptureCmd exposes the corpus_fs.Writer real adapter via the
// CLI. Companion to cycle 146's `ao corpus inject` (which exposes
// CorpusReader). Together they close the BC1 reader+writer pair on
// the CLI side.
//
// Built using the cycle-147 cli-wiring template.
var corpusCaptureCmd = &cobra.Command{
	Use:   "capture --path <relpath> [--body <text>] [--body-file <file>] [--body-stdin] [--root <dir>] [--meta k=v ...]",
	Short: "Write a corpus artifact via BC1 CorpusWriterPort",
	Long: `Write an artifact to a corpus root via the typed BC1
CorpusWriterPort (corpus_fs.Writer real adapter). Default root
is .agents/learnings/ under the project root.

The --path argument is the relative path WITHIN the root; absolute
paths and parent-traversal ('..') are rejected (port contract).

Body source options (mutually exclusive):
  --body <text>          inline text
  --body-file <file>     read from file
  --body-stdin           read from stdin

Metadata frontmatter is rendered if --meta key=value flags are
passed. If the body already starts with '---\n', the existing
frontmatter is preserved and --meta is ignored (port contract).

Examples:
  ao corpus capture --path notes/x.md --body "hello world"
  ao corpus capture --path findings/y.md --body-file ./input.md
  echo "body text" | ao corpus capture --path z.md --body-stdin
  ao corpus capture --path n.md --body "..." --meta tag=evolve --meta date=2026-05-13`,
	RunE: runCorpusCapture,
}

type corpusCaptureOptions struct {
	path      string
	root      string
	body      string
	bodyFile  string
	bodyStdin bool
	meta      []string
	stdin     io.Reader
	writer    io.Writer
	captureFn func(ctx context.Context, opts corpusCaptureOptions, body []byte, meta map[string]string) (ports.CorpusWriteResult, error)
}

func init() {
	corpusCaptureCmd.Flags().String("path", "", "relative path within root (required)")
	corpusCaptureCmd.Flags().String("root", "", "corpus root (default: .agents/learnings/)")
	corpusCaptureCmd.Flags().String("body", "", "body text (mutually exclusive with --body-file and --body-stdin)")
	corpusCaptureCmd.Flags().String("body-file", "", "read body from file")
	corpusCaptureCmd.Flags().Bool("body-stdin", false, "read body from stdin")
	corpusCaptureCmd.Flags().StringArray("meta", nil, "metadata key=value (repeatable)")
	_ = corpusCaptureCmd.MarkFlagRequired("path")
	corpusCmd.AddCommand(corpusCaptureCmd)
}

func runCorpusCapture(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("path")
	root, _ := cmd.Flags().GetString("root")
	body, _ := cmd.Flags().GetString("body")
	bodyFile, _ := cmd.Flags().GetString("body-file")
	bodyStdin, _ := cmd.Flags().GetBool("body-stdin")
	meta, _ := cmd.Flags().GetStringArray("meta")
	return corpusCaptureRun(cmd.Context(), corpusCaptureOptions{
		path:      path,
		root:      root,
		body:      body,
		bodyFile:  bodyFile,
		bodyStdin: bodyStdin,
		meta:      meta,
		stdin:     cmd.InOrStdin(),
		writer:    cmd.OutOrStdout(),
	})
}

func corpusCaptureRun(ctx context.Context, opts corpusCaptureOptions) error {
	if opts.path == "" {
		return errors.New("corpus capture: --path required")
	}
	body, err := corpusCaptureResolveBody(opts)
	if err != nil {
		return fmt.Errorf("corpus capture: %w", err)
	}
	meta, err := corpusCaptureParseMeta(opts.meta)
	if err != nil {
		return fmt.Errorf("corpus capture: %w", err)
	}
	fn := opts.captureFn
	if fn == nil {
		fn = corpusCaptureViaPort
	}
	res, err := fn(ctx, opts, body, meta)
	if err != nil {
		return fmt.Errorf("corpus capture: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	createdLabel := "updated"
	if res.Created {
		createdLabel = "created"
	}
	fmt.Fprintf(opts.writer, "%s %s\n", createdLabel, res.ResolvedPath)
	return nil
}

// corpusCaptureResolveBody picks one of --body, --body-file,
// --body-stdin. Exactly one source must be provided.
func corpusCaptureResolveBody(opts corpusCaptureOptions) ([]byte, error) {
	sources := 0
	if opts.body != "" {
		sources++
	}
	if opts.bodyFile != "" {
		sources++
	}
	if opts.bodyStdin {
		sources++
	}
	if sources == 0 {
		return nil, errors.New("body source required (--body, --body-file, or --body-stdin)")
	}
	if sources > 1 {
		return nil, errors.New("only one body source allowed")
	}
	if opts.body != "" {
		return []byte(opts.body), nil
	}
	if opts.bodyFile != "" {
		data, err := os.ReadFile(opts.bodyFile)
		if err != nil {
			return nil, fmt.Errorf("read --body-file: %w", err)
		}
		return data, nil
	}
	// stdin
	if opts.stdin == nil {
		opts.stdin = os.Stdin
	}
	data, err := io.ReadAll(opts.stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return data, nil
}

// corpusCaptureParseMeta turns []string{"k=v","a=b"} into a map.
func corpusCaptureParseMeta(meta []string) (map[string]string, error) {
	if len(meta) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(meta))
	for _, kv := range meta {
		idx := strings.IndexByte(kv, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("--meta %q: expected key=value", kv)
		}
		out[kv[:idx]] = kv[idx+1:]
	}
	return out, nil
}

func corpusCaptureViaPort(ctx context.Context, opts corpusCaptureOptions, body []byte, meta map[string]string) (ports.CorpusWriteResult, error) {
	root := opts.root
	if root == "" {
		cwd, err := resolveProjectDir()
		if err != nil {
			return ports.CorpusWriteResult{}, err
		}
		root = filepath.Join(cwd, ".agents", "learnings")
	}
	w := corpus_fs.NewWriter(root)
	return w.Capture(ctx, ports.CorpusWriteRequest{
		Path:     opts.path,
		Body:     body,
		Metadata: meta,
	})
}
