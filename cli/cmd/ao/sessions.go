package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type sessionsPruneOrphansResult struct {
	SessionsDir     string   `json:"sessions_dir"`
	DryRun          bool     `json:"dry_run"`
	Scanned         int      `json:"scanned"`
	Kept            int      `json:"kept"`
	Deleted         int      `json:"deleted"`
	WouldDelete     int      `json:"would_delete"`
	SkippedNoSource int      `json:"skipped_no_source"`
	Orphans         []string `json:"orphans,omitempty"`
}

type sessionPageSource struct {
	path   string
	source string
}

var sessionsCmd = &cobra.Command{
	Use:     "sessions",
	Short:   "Manage indexed session pages",
	Long:    "Manage derived session pages under .agents/ao/sessions without touching source transcripts.",
	GroupID: "knowledge",
}

var sessionsIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Maintain indexed session pages",
	Long: `Maintain derived session pages under .agents/ao/sessions.

This narrow v2 surface currently supports retention realignment only:
  ao sessions index --prune-orphans

It scans derived session markdown pages, reads their source_jsonl frontmatter,
and removes pages whose source transcript no longer exists. It never deletes
files under ~/.claude/projects or any other source transcript directory.`,
	RunE: runSessionsIndex,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsIndexCmd)
	sessionsIndexCmd.Flags().Bool("prune-orphans", false, "Delete derived session pages whose source_jsonl no longer exists")
	sessionsIndexCmd.Flags().String("sessions-dir", "", "Directory containing derived session pages (default: .agents/ao/sessions)")
}

func runSessionsIndex(cmd *cobra.Command, args []string) error {
	prune, err := cmd.Flags().GetBool("prune-orphans")
	if err != nil {
		return err
	}
	if !prune {
		return fmt.Errorf("ao sessions index currently only supports --prune-orphans")
	}
	repoRoot, err := resolveProjectDir()
	if err != nil {
		return err
	}
	sessionsDir, err := resolveSessionsDir(cmd, repoRoot)
	if err != nil {
		return err
	}
	result, err := pruneOrphanSessionPages(repoRoot, sessionsDir, GetDryRun())
	if err != nil {
		return err
	}
	return writeSessionsPruneResult(cmd.OutOrStdout(), result)
}

func resolveSessionsDir(cmd *cobra.Command, repoRoot string) (string, error) {
	sessionsDir, err := cmd.Flags().GetString("sessions-dir")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(sessionsDir) == "" {
		sessionsDir = filepath.Join(repoRoot, ".agents", "ao", "sessions")
	}
	if filepath.IsAbs(sessionsDir) {
		return filepath.Clean(sessionsDir), nil
	}
	return filepath.Clean(filepath.Join(repoRoot, sessionsDir)), nil
}

func pruneOrphanSessionPages(repoRoot, sessionsDir string, dryRun bool) (sessionsPruneOrphansResult, error) {
	result := sessionsPruneOrphansResult{
		SessionsDir: sessionsDir,
		DryRun:      dryRun,
	}
	entries, err := os.ReadDir(sessionsDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("read sessions dir %s: %w", sessionsDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		pagePath := filepath.Join(sessionsDir, entry.Name())
		page, err := readSessionPageSource(pagePath)
		if err != nil {
			return result, err
		}
		result.Scanned++
		if strings.TrimSpace(page.source) == "" {
			result.SkippedNoSource++
			continue
		}
		sourcePath := resolveSessionSourcePath(repoRoot, page.source)
		if _, err := os.Stat(sourcePath); err == nil {
			result.Kept++
			continue
		} else if !os.IsNotExist(err) {
			return result, fmt.Errorf("stat source %s: %w", sourcePath, err)
		}
		result.Orphans = append(result.Orphans, page.path)
		if dryRun {
			result.WouldDelete++
			continue
		}
		if err := os.Remove(page.path); err != nil {
			return result, fmt.Errorf("delete orphan session page %s: %w", page.path, err)
		}
		result.Deleted++
	}
	return result, nil
}

func readSessionPageSource(path string) (sessionPageSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionPageSource{}, fmt.Errorf("read session page %s: %w", path, err)
	}
	return sessionPageSource{path: path, source: parseSessionSourceJSONL(string(data))}, nil
}

func parseSessionSourceJSONL(content string) string {
	inFrontmatter := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			return ""
		}
		if !inFrontmatter {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "source_jsonl" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}

func resolveSessionSourcePath(repoRoot, source string) string {
	source = strings.TrimSpace(source)
	if filepath.IsAbs(source) {
		return filepath.Clean(source)
	}
	return filepath.Clean(filepath.Join(repoRoot, source))
}

func writeSessionsPruneResult(w io.Writer, result sessionsPruneOrphansResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	action := "Deleted"
	count := result.Deleted
	if result.DryRun {
		action = "Would delete"
		count = result.WouldDelete
	}
	fmt.Fprintf(w, "%s %d orphan session page(s); scanned=%d kept=%d skipped_no_source=%d\n",
		action, count, result.Scanned, result.Kept, result.SkippedNoSource)
	return nil
}
