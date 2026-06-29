// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"crypto/sha1" // #nosec G505 nosemgrep -- short stable id digest for filenames, not a security primitive.
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	ingestClaudeSource string
	ingestClaudeDest   string
	ingestClaudeDryRun bool
)

// memoryIngestClaudeCmd ingests the Claude per-project memory silos
// (~/.claude/projects/*/memory/*.md) into ao's machine-tier curated corpus
// (~/.agents/learnings/), so `ao recall` covers facts that were stranded in the
// Claude-only silos. Content is PRESERVED verbatim (not LLM-extracted), each hit
// is tagged source=claude-memory, and the operation is idempotent (stable dest
// filename per source → re-run overwrites, never duplicates). Implements
// age-unified-agent-memory-nyfq.5.
var memoryIngestClaudeCmd = &cobra.Command{
	Use:   "ingest-claude",
	Short: "Ingest the Claude per-project memory silos into ao's recall corpus (source=claude-memory)",
	Long: `Pull the curated facts in ~/.claude/projects/*/memory/*.md into ao's
machine-tier corpus (~/.agents/learnings/) so 'ao recall' surfaces them.

Each silo file is wrapped in an ao-learning with provenance (tier=machine,
source=claude-memory, origin_path, the source mtime as the decay date) and its
ORIGINAL content preserved verbatim in the body — no LLM extraction. Idempotent:
the destination filename is derived from the source path, so re-running
refreshes in place and never creates duplicates.`,
	RunE: runMemoryIngestClaude,
}

func init() {
	memoryIngestClaudeCmd.Flags().StringVar(&ingestClaudeSource, "source", "", "Source dir (default: ~/.claude/projects)")
	memoryIngestClaudeCmd.Flags().StringVar(&ingestClaudeDest, "dest", "", "Destination learnings dir (default: ~/.agents/learnings)")
	memoryIngestClaudeCmd.Flags().BoolVar(&ingestClaudeDryRun, "dry-run", false, "Report what would be ingested; write nothing")
	memoryCmd.AddCommand(memoryIngestClaudeCmd)
}

func runMemoryIngestClaude(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	source := ingestClaudeSource
	if source == "" {
		source = filepath.Join(home, ".claude", "projects")
	}
	dest := ingestClaudeDest
	if dest == "" {
		dest = filepath.Join(home, ".agents", "learnings")
	}

	files, err := claudeMemoryFiles(source)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Printf("ao memory ingest-claude: no silo files found under %s\n", source)
		return nil
	}

	if !ingestClaudeDryRun {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("create dest %s: %w", dest, err)
		}
	}

	ingested := 0
	for _, src := range files {
		destName := claudeMemoryDestName(source, src)
		if ingestClaudeDryRun {
			fmt.Printf("[dry-run] %s -> %s\n", src, filepath.Join(dest, destName))
			ingested++
			continue
		}
		wrapped, werr := wrapClaudeMemory(source, src)
		if werr != nil {
			VerbosePrintf("ingest-claude: skip %s: %v\n", src, werr)
			continue
		}
		if err := os.WriteFile(filepath.Join(dest, destName), []byte(wrapped), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", destName, err)
		}
		ingested++
	}

	verb := "ingested"
	if ingestClaudeDryRun {
		verb = "would ingest"
	}
	fmt.Printf("ao memory ingest-claude: %s %d Claude memory file(s) into %s (source=claude-memory, tier=machine)\n", verb, ingested, dest)
	return nil
}

// claudeMemoryFiles returns the curated memory markdown under
// <source>/*/memory/**.md, sorted for deterministic output.
func claudeMemoryFiles(source string) ([]string, error) {
	projects, err := os.ReadDir(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read source %s: %w", source, err)
	}
	var out []string
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		memDir := filepath.Join(source, p.Name(), "memory")
		_ = filepath.WalkDir(memDir, func(path string, d os.DirEntry, werr error) error {
			if werr != nil {
				return nil // missing memory dir for this project is fine
			}
			if !d.IsDir() && strings.HasSuffix(path, ".md") {
				out = append(out, path)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out, nil
}

// claudeMemoryDestName derives a STABLE destination filename from the source
// path (project segment + relative file), so re-ingest overwrites in place and
// never duplicates. The claude-memory-- prefix makes the provenance visible in
// recall's cited path and keeps the ingested set identifiable/removable.
func claudeMemoryDestName(source, src string) string {
	rel, err := filepath.Rel(source, src)
	if err != nil {
		rel = filepath.Base(src)
	}
	slug := strings.TrimSuffix(rel, ".md")
	slug = strings.NewReplacer(string(os.PathSeparator), "-", " ", "-", "/", "-").Replace(slug)
	// Keep filenames bounded; disambiguate with a short digest of the full rel path.
	sum := sha1.Sum([]byte(rel)) // #nosec G401 nosemgrep -- filename disambiguation digest, not security.
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return fmt.Sprintf("claude-memory--%s--%x.md", slug, sum[:4])
}

// wrapClaudeMemory wraps a Claude silo file in an ao-learning with provenance,
// preserving the ORIGINAL content verbatim in the body (no LLM extraction). The
// decay date is the source file's mtime — the real age of the memory.
func wrapClaudeMemory(source, src string) (string, error) {
	body, err := os.ReadFile(src) // #nosec G304 -- src is enumerated from the user's own ~/.claude tree.
	if err != nil {
		return "", err
	}
	rel, relErr := filepath.Rel(source, src)
	if relErr != nil {
		rel = filepath.Base(src)
	}
	id := claudeMemoryDestName(source, src)
	id = strings.TrimSuffix(id, ".md")

	date := "2026-01-01"
	if fi, statErr := os.Stat(src); statErr == nil {
		date = fi.ModTime().UTC().Format("2006-01-02")
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	b.WriteString("type: learning\n")
	fmt.Fprintf(&b, "date: %s\n", date)
	b.WriteString("tier: machine\n")
	b.WriteString("source: claude-memory\n")
	b.WriteString("maturity: provisional\n")
	b.WriteString("utility: 0.6000\n")
	b.WriteString("confidence: 0.8000\n")
	b.WriteString("sensitivity: unknown\n")
	b.WriteString("publishable: false\n")
	fmt.Fprintf(&b, "origin_path: %s\n", src)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# Claude memory: %s\n\n", rel)
	b.WriteString("> Ingested verbatim from a Claude per-project memory silo (source=claude-memory).\n\n")
	b.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.String(), nil
}
