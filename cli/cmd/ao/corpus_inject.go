// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/corpus_fs"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// corpusInjectCmd is slice 3 of soc-y5vh.5 (cycle 146). Reads from the
// .agents/learnings/ tree (and optionally additional roots) via the
// typed BC1 CorpusReaderPort (corpus_fs.Reader real adapter),
// emitting line-delimited JSON CorpusItem records ranked by query
// match. Completes the soc-y5vh.5 prerequisite — 3rd production
// adapter now reachable from the operator-facing CLI.
//
// Companion to:
//   - ao loop history (cycle 144 — productionLoopReader)
//   - ao ci latest    (cycle 145 — productionCIStatus)
var corpusInjectCmd = &cobra.Command{
	Use:   "inject [--query <text>] [--root <path>] [--limit N]",
	Short: "Inject corpus matches via BC1 CorpusReaderPort (typed lookup)",
	Long: `Read knowledge from a corpus root via the typed BC1
CorpusReaderPort (corpus_fs.Reader real adapter). Default root is
.agents/learnings/ under the project root. Emits one JSON CorpusItem
per line, ranked by query match (title hit weighs 2, body hit weighs 1).

Useful for /evolve Step 0 prior-failure injection (soc-y5vh.1
consumer) and any caller that wants typed corpus retrieval without
re-implementing the file walk + ranker.

Examples:
  ao corpus inject --query "hexagonal"            # default root
  ao corpus inject --query "wire-up" --limit 3    # top 3 matches
  ao corpus inject --root docs/learnings           # specific root
  ao corpus inject                                  # all .md (empty query)`,
	RunE: runCorpusInject,
}

type corpusInjectOptions struct {
	query  string
	root   string
	limit  int
	writer io.Writer
	// injectFn lets tests substitute the port without writing temp files
	injectFn func(ctx context.Context, opts corpusInjectOptions) ([]ports.CorpusItem, error)
}

func init() {
	corpusInjectCmd.Flags().String("query", "", "ranking query (empty = all items, score 0)")
	corpusInjectCmd.Flags().String("root", "", "corpus root (default: .agents/learnings/)")
	corpusInjectCmd.Flags().Int("limit", 10, "max items to emit (0 = all)")
	corpusCmd.AddCommand(corpusInjectCmd)
}

func runCorpusInject(cmd *cobra.Command, _ []string) error {
	query, _ := cmd.Flags().GetString("query")
	root, _ := cmd.Flags().GetString("root")
	limit, _ := cmd.Flags().GetInt("limit")
	return corpusInjectRun(cmd.Context(), corpusInjectOptions{
		query:  query,
		root:   root,
		limit:  limit,
		writer: cmd.OutOrStdout(),
	})
}

func corpusInjectRun(ctx context.Context, opts corpusInjectOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	injectFn := opts.injectFn
	if injectFn == nil {
		injectFn = corpusInjectViaPort
	}
	items, err := injectFn(ctx, opts)
	if err != nil {
		return fmt.Errorf("corpus inject: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return fmt.Errorf("corpus inject encode: %w", err)
		}
	}
	return nil
}

// corpusInjectViaPort wires the corpus_fs.Reader real adapter to the
// caller's root. Default root resolves to <project>/.agents/learnings.
func corpusInjectViaPort(ctx context.Context, opts corpusInjectOptions) ([]ports.CorpusItem, error) {
	root := opts.root
	if root == "" {
		cwd, err := resolveProjectDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(cwd, ".agents", "learnings")
	}
	reader := corpus_fs.NewReader(root)
	return reader.Lookup(ctx, ports.LookupOptions{
		Query: opts.query,
		Limit: opts.limit,
	})
}
