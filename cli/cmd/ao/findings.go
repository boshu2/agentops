// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/search"
)

type findingStats = search.FindingStats

var (
	findingsListLimit   int
	findingsListAll     bool
	findingsExportTo    string
	findingsExportAll   bool
	findingsExportForce bool
	findingsPullFrom    string
	findingsPullAll     bool
	findingsPullForce   bool
	findingsRetireBy    string
)

var findingsCmd = &cobra.Command{
	Use:   "findings",
	Short: "Manage promoted findings",
	Long: `Manage promoted finding artifacts under .agents/findings/.

Use this command family to inspect, export, import, retire, and summarize
promoted findings. The lookup/inject runtime consumes these artifacts for
cross-session prevention, and active citations update their lifecycle fields.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var findingsListCmd = &cobra.Command{
	Use:   "list [query]",
	Short: "List active findings",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runFindingsList,
}

var findingsExportCmd = &cobra.Command{
	Use:   "export <id...>",
	Short: "Export finding artifacts to another repo or findings directory",
	Args:  cobra.ArbitraryArgs,
	RunE:  runFindingsExport,
}

var findingsPullCmd = &cobra.Command{
	Use:   "pull <id...>",
	Short: "Pull finding artifacts from another repo or findings directory",
	Args:  cobra.ArbitraryArgs,
	RunE:  runFindingsPull,
}

var findingsRetireCmd = &cobra.Command{
	Use:   "retire <id>",
	Short: "Retire a finding artifact",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindingsRetire,
}

var findingsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Summarize local finding artifact inventory",
	RunE:  runFindingsStats,
}

func init() {
	findingsCmd.GroupID = "knowledge"
	rootCmd.AddCommand(findingsCmd)
	findingsCmd.AddCommand(findingsListCmd)
	findingsCmd.AddCommand(findingsExportCmd)
	findingsCmd.AddCommand(findingsPullCmd)
	findingsCmd.AddCommand(findingsRetireCmd)
	findingsCmd.AddCommand(findingsStatsCmd)

	findingsListCmd.Flags().IntVar(&findingsListLimit, "limit", 20, "Maximum findings to return")
	findingsListCmd.Flags().BoolVar(&findingsListAll, "all", false, "Include retired and superseded findings")

	findingsExportCmd.Flags().StringVar(&findingsExportTo, "to", "", "Destination repo root or .agents/findings directory")
	findingsExportCmd.Flags().BoolVar(&findingsExportAll, "all", false, "Export every local finding")
	findingsExportCmd.Flags().BoolVar(&findingsExportForce, "force", false, "Overwrite destination files if they already exist")

	findingsPullCmd.Flags().StringVar(&findingsPullFrom, "from", "", "Source repo root or .agents/findings directory")
	findingsPullCmd.Flags().BoolVar(&findingsPullAll, "all", false, "Pull every source finding")
	findingsPullCmd.Flags().BoolVar(&findingsPullForce, "force", false, "Overwrite local files if they already exist")

	findingsRetireCmd.Flags().StringVar(&findingsRetireBy, "by", "", "Retired-by marker (defaults to current user or ao findings retire)")
}

func runFindingsList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	cfg, _ := config.Load(nil)
	globalFindingsDir := globalFindingsDirFromConfig(cfg)
	globalWeight := 0.8
	if cfg != nil {
		globalWeight = cfg.Paths.GlobalWeight
	}

	findings, err := collectFindingsWithOptions(cwd, query, findingsListLimit, globalFindingsDir, globalWeight, findingsListAll)
	if err != nil {
		return err
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	}

	if len(findings) == 0 {
		fmt.Println("No findings found.")
		return nil
	}

	for _, f := range findings {
		scope := "local"
		if f.Global {
			scope = "global"
		}
		fmt.Printf("%s\t%s\t%d\t%s\t%s\n", f.ID, emptyIfMissing(f.Status), f.HitCount, scope, f.Title)
	}
	return nil
}

func runFindingsExport(cmd *cobra.Command, args []string) error {
	if findingsExportTo == "" {
		return fmt.Errorf("--to is required")
	}
	if !findingsExportAll && len(args) == 0 {
		return fmt.Errorf("provide one or more finding IDs or use --all")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	sourceDir := repoFindingsDir(cwd)
	targetDir := resolveManagedFindingsDir(findingsExportTo)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}

	selected, err := selectFindingFiles(sourceDir, args, findingsExportAll)
	if err != nil {
		return err
	}

	var copied []string
	for _, src := range selected {
		dst := filepath.Join(targetDir, filepath.Base(src))
		if err := copyFindingFile(src, dst, findingsExportForce); err != nil {
			return err
		}
		copied = append(copied, dst)
	}

	return printFindingTransferResult("exported", copied)
}

func runFindingsPull(cmd *cobra.Command, args []string) error {
	if findingsPullFrom == "" {
		return fmt.Errorf("--from is required")
	}
	if !findingsPullAll && len(args) == 0 {
		return fmt.Errorf("provide one or more finding IDs or use --all")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	sourceDir, err := resolveExistingFindingsDir(findingsPullFrom)
	if err != nil {
		return err
	}
	targetDir := repoFindingsDir(cwd)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create local findings dir: %w", err)
	}

	selected, err := selectFindingFiles(sourceDir, args, findingsPullAll)
	if err != nil {
		return err
	}

	var copied []string
	for _, src := range selected {
		dst := filepath.Join(targetDir, filepath.Base(src))
		if err := copyFindingFile(src, dst, findingsPullForce); err != nil {
			return err
		}
		copied = append(copied, dst)
	}
	bestEffortRefreshFindingCompiler(cwd)
	return printFindingTransferResult("pulled", copied)
}

func runFindingsRetire(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	finding, err := findLocalFindingByID(cwd, args[0])
	if err != nil {
		return err
	}

	retiredBy := findingsRetireBy
	if retiredBy == "" {
		retiredBy = strings.TrimSpace(os.Getenv("USER"))
	}
	if retiredBy == "" {
		retiredBy = "ao findings retire"
	}

	if err := updateFindingFrontMatter(finding.Source, map[string]string{
		"status":     "retired",
		"retired_by": retiredBy,
	}); err != nil {
		return err
	}
	bestEffortRefreshFindingCompiler(cwd)

	if GetOutput() == "json" {
		result := map[string]string{
			"id":         finding.ID,
			"status":     "retired",
			"retired_by": retiredBy,
			"source":     finding.Source,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Retired finding %s (%s)\n", finding.ID, retiredBy)
	return nil
}

func runFindingsStats(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	findings, err := collectFindingsFromDir(repoFindingsDir(cwd), "", time.Now(), false, true)
	if err != nil {
		return err
	}
	stats := buildFindingStats(findings)

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	fmt.Printf("Total findings: %d\n", stats.Total)
	fmt.Printf("Total hits: %d\n", stats.TotalHits)
	fmt.Println("By status:")
	printStringCountMap(stats.ByStatus)
	fmt.Println("By severity:")
	printStringCountMap(stats.BySeverity)
	fmt.Println("By detectability:")
	printStringCountMap(stats.ByDetectability)
	if len(stats.MostCited) > 0 {
		fmt.Println("Most cited:")
		for _, f := range stats.MostCited {
			fmt.Printf("  %s (%d hits)\n", f.ID, f.HitCount)
		}
	}
	return nil
}

func globalFindingsDirFromConfig(cfg *config.Config) string {
	if cfg == nil || cfg.Paths.GlobalLearningsDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfg.Paths.GlobalLearningsDir), SectionFindings)
}

func repoFindingsDir(cwd string) string {
	return search.RepoFindingsDir(cwd)
}

func resolveManagedFindingsDir(path string) string {
	return search.ResolveManagedFindingsDir(path)
}

func resolveExistingFindingsDir(path string) (string, error) {
	return search.ResolveExistingFindingsDir(path)
}

func selectFindingFiles(dir string, ids []string, all bool) ([]string, error) {
	return search.SelectFindingFiles(dir, ids, all, matchesID)
}

func findLocalFindingByID(cwd, id string) (knowledgeFinding, error) {
	return search.FindLocalFindingByID(cwd, id, matchesID)
}

func copyFindingFile(src, dst string, force bool) error {
	return search.CopyFindingFile(src, dst, force)
}

func printFindingTransferResult(action string, paths []string) error {
	if GetOutput() == "json" {
		result := map[string]any{
			"action": action,
			"files":  paths,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if len(paths) == 0 {
		fmt.Printf("No findings %s.\n", action)
		return nil
	}
	fmt.Printf("%s %d finding(s):\n", titleCase(action), len(paths))
	for _, path := range paths {
		fmt.Printf("  %s\n", path)
	}
	return nil
}

func buildFindingStats(findings []knowledgeFinding) findingStats {
	return search.BuildFindingStats(findings)
}

func normalizeStatKey(value, fallback string) string {
	return search.NormalizeStatKey(value, fallback)
}

func printStringCountMap(values map[string]int) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("  %s: %d\n", key, values[key])
	}
}

func updateFindingFrontMatter(path string, updates map[string]string) error {
	return search.UpdateFindingFrontMatter(path, updates)
}

func writeFindingFileAtomic(path string, data []byte, mode os.FileMode) error {
	return search.WriteFindingFileAtomic(path, data, mode)
}

func bestEffortRefreshFindingCompiler(cwd string) {
	script := filepath.Join(cwd, "hooks", "finding-compiler.sh")
	if _, err := os.Stat(script); err != nil {
		return
	}
	cmd := exec.Command("bash", script, "--quiet")
	cmd.Dir = cwd
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
