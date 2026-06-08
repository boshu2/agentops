package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	skillsEditSealSkill          string
	skillsEditSealMessage        string
	skillsEditSealActor          string
	skillsEditSealCriticalPolicy string
	skillsEditSealAllowCritical  bool
	skillsEditSealDryRun         bool

	skillsEditDigestSince          string
	skillsEditDigestCriticalPolicy string
	skillsEditDigestJSON           bool
)

type skillEditSealOptions struct {
	RepoRoot           string
	Skill              string
	Message            string
	Actor              string
	CriticalPolicyPath string
	AllowCritical      bool
	DryRun             bool
}

type skillEditDigestOptions struct {
	RepoRoot           string
	Since              string
	CriticalPolicyPath string
}

type skillEditDigestEntry struct {
	Hash     string   `json:"hash"`
	When     string   `json:"when"`
	Author   string   `json:"author"`
	Subject  string   `json:"subject"`
	Skills   []string `json:"skills"`
	Files    []string `json:"files"`
	Critical bool     `json:"critical"`
}

var skillsEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Seal and summarize live skill edits",
	Long: `Immune-system commands for the live skill tier.

Use "ao skills edit seal" after an agent edits skills/<name>/SKILL.md. It
commits the live skill edit as a rollback point and refuses critical skills
unless the caller explicitly allows a human-supervised critical edit.

Use "ao skills edit digest" for the daily operator digest of recent skill
changes.`,
}

var skillsEditSealCmd = &cobra.Command{
	Use:   "seal",
	Short: "Commit one live skill edit with critical-skill protection",
	RunE:  runSkillsEditSeal,
}

var skillsEditDigestCmd = &cobra.Command{
	Use:   "digest",
	Short: "Summarize recent committed skill edits",
	RunE:  runSkillsEditDigest,
}

func init() {
	skillsCmd.AddCommand(skillsEditCmd)

	skillsEditCmd.AddCommand(skillsEditSealCmd)
	skillsEditSealCmd.Flags().StringVar(&skillsEditSealSkill, "skill", "", "Skill slug under skills/<slug> to seal")
	skillsEditSealCmd.Flags().StringVar(&skillsEditSealMessage, "message", "", "Commit subject (default: chore(skills): update <skill> via live edit)")
	skillsEditSealCmd.Flags().StringVar(&skillsEditSealActor, "actor", "", "Agent/operator name recorded in the commit body")
	skillsEditSealCmd.Flags().StringVar(&skillsEditSealCriticalPolicy, "critical-policy", "", "Critical skills policy file (default: docs/contracts/critical-skills.txt)")
	skillsEditSealCmd.Flags().BoolVar(&skillsEditSealAllowCritical, "allow-critical", false, "Allow a critical skill edit; use only for human-supervised edits")
	skillsEditSealCmd.Flags().BoolVar(&skillsEditSealDryRun, "dry-run", false, "Check policy and print the commit action without staging or committing")

	skillsEditCmd.AddCommand(skillsEditDigestCmd)
	skillsEditDigestCmd.Flags().StringVar(&skillsEditDigestSince, "since", "24 hours ago", "git log --since value")
	skillsEditDigestCmd.Flags().StringVar(&skillsEditDigestCriticalPolicy, "critical-policy", "", "Critical skills policy file (default: docs/contracts/critical-skills.txt)")
	skillsEditDigestCmd.Flags().BoolVar(&skillsEditDigestJSON, "json", false, "Emit JSON")
}

func runSkillsEditSeal(cmd *cobra.Command, args []string) error {
	repoRoot, err := resolveRepoRootForSkills()
	if err != nil {
		return err
	}
	opts := skillEditSealOptions{
		RepoRoot:           repoRoot,
		Skill:              skillsEditSealSkill,
		Message:            skillsEditSealMessage,
		Actor:              skillsEditSealActor,
		CriticalPolicyPath: skillsEditSealCriticalPolicy,
		AllowCritical:      skillsEditSealAllowCritical,
		DryRun:             skillsEditSealDryRun,
	}
	result, err := sealSkillEdit(opts)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), result)
	return nil
}

func runSkillsEditDigest(cmd *cobra.Command, args []string) error {
	repoRoot, err := resolveRepoRootForSkills()
	if err != nil {
		return err
	}
	entries, err := digestSkillEdits(skillEditDigestOptions{
		RepoRoot:           repoRoot,
		Since:              skillsEditDigestSince,
		CriticalPolicyPath: skillsEditDigestCriticalPolicy,
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if skillsEditDigestJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	fmt.Fprintf(out, "# Skill Edit Digest\n\nSince: %s\n\n", skillsEditDigestSince)
	if len(entries) == 0 {
		fmt.Fprintln(out, "No committed skill edits found.")
		return nil
	}
	for _, entry := range entries {
		marker := ""
		if entry.Critical {
			marker = " [CRITICAL]"
		}
		fmt.Fprintf(out, "- %s %s %s%s\n", shortSkillEditHash(entry.Hash), entry.When, entry.Subject, marker)
		fmt.Fprintf(out, "  author: %s\n", entry.Author)
		fmt.Fprintf(out, "  skills: %s\n", strings.Join(entry.Skills, ", "))
		for _, file := range entry.Files {
			fmt.Fprintf(out, "  - %s\n", file)
		}
	}
	return nil
}

func sealSkillEdit(opts skillEditSealOptions) (string, error) {
	if strings.TrimSpace(opts.Skill) == "" {
		return "", fmt.Errorf("--skill is required")
	}
	skill := strings.TrimSpace(opts.Skill)
	if strings.Contains(skill, "/") || strings.Contains(skill, string(filepath.Separator)) || skill == "." || skill == ".." {
		return "", fmt.Errorf("invalid skill slug %q", skill)
	}
	repoRoot := strings.TrimSpace(opts.RepoRoot)
	if repoRoot == "" {
		return "", fmt.Errorf("repo root is required")
	}
	skillDir := filepath.Join(repoRoot, "skills", skill)
	if !isDir(skillDir) {
		return "", fmt.Errorf("skill not found: skills/%s", skill)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		return "", fmt.Errorf("skill missing SKILL.md: skills/%s", skill)
	}

	critical, err := loadCriticalSkills(repoRoot, opts.CriticalPolicyPath)
	if err != nil {
		return "", err
	}
	if critical[skill] && !opts.AllowCritical {
		return "", fmt.Errorf("critical skill %q rejects unattended edit; rerun with --allow-critical only for human-supervised changes", skill)
	}

	pathspecs := skillEditPathspecs(repoRoot, skill)
	status, err := gitOutput(repoRoot, append([]string{"status", "--porcelain", "--"}, pathspecs...)...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "", fmt.Errorf("no live skill changes to seal for %q", skill)
	}

	subject := strings.TrimSpace(opts.Message)
	if subject == "" {
		subject = fmt.Sprintf("chore(skills): update %s via live edit", skill)
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = firstNonEmptySkillEdit(os.Getenv("AGENT_NAME"), os.Getenv("USER"), "agent")
	}
	body := fmt.Sprintf("Skill-Edit: %s\nSkill-Edit-Actor: %s\nSkill-Edit-Critical: %t\nSkill-Edit-Policy: %s",
		skill, actor, critical[skill], criticalPolicyDisplay(opts.CriticalPolicyPath))

	if opts.DryRun {
		return fmt.Sprintf("DRY-RUN: would commit %s (%d pathspecs)", skill, len(pathspecs)), nil
	}

	if _, err := gitOutput(repoRoot, append([]string{"add", "--"}, pathspecs...)...); err != nil {
		return "", err
	}
	commitEnv := []string{}
	if os.Getenv("AGENT_NAME") == "" && actor != "" {
		commitEnv = append(commitEnv, "AGENT_NAME="+actor)
	}
	if _, err := gitOutputEnv(repoRoot, commitEnv, "commit", "-m", subject, "-m", body); err != nil {
		return "", err
	}
	hash, err := gitOutput(repoRoot, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sealed skill edit %s at %s", skill, strings.TrimSpace(hash)), nil
}

func digestSkillEdits(opts skillEditDigestOptions) ([]skillEditDigestEntry, error) {
	since := strings.TrimSpace(opts.Since)
	if since == "" {
		since = "24 hours ago"
	}
	critical, err := loadCriticalSkills(opts.RepoRoot, opts.CriticalPolicyPath)
	if err != nil {
		return nil, err
	}
	out, err := gitOutput(opts.RepoRoot, "log", "--since", since, "--format=__AO_COMMIT__%x09%H%x09%aI%x09%an%x09%s", "--name-only", "--", "skills")
	if err != nil {
		return nil, err
	}
	var entries []skillEditDigestEntry
	var current *skillEditDigestEntry
	flush := func() {
		if current == nil || len(current.Files) == 0 {
			return
		}
		skills := map[string]bool{}
		for _, file := range current.Files {
			if skill := skillSlugFromPath(file); skill != "" {
				skills[skill] = true
				if critical[skill] {
					current.Critical = true
				}
			}
		}
		for skill := range skills {
			current.Skills = append(current.Skills, skill)
		}
		sort.Strings(current.Skills)
		if len(current.Skills) > 0 {
			entries = append(entries, *current)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "__AO_COMMIT__\t") {
			flush()
			parts := strings.SplitN(line, "\t", 5)
			if len(parts) != 5 {
				current = nil
				continue
			}
			current = &skillEditDigestEntry{
				Hash:    parts[1],
				When:    parts[2],
				Author:  parts[3],
				Subject: parts[4],
			}
			continue
		}
		if current != nil && strings.HasPrefix(line, "skills/") {
			current.Files = append(current.Files, line)
		}
	}
	flush()
	return entries, nil
}

func loadCriticalSkills(repoRoot, policyPath string) (map[string]bool, error) {
	if strings.TrimSpace(policyPath) == "" {
		policyPath = filepath.Join(repoRoot, "docs", "contracts", "critical-skills.txt")
	} else if !filepath.IsAbs(policyPath) {
		policyPath = filepath.Join(repoRoot, policyPath)
	}
	result := map[string]bool{}
	data, err := os.ReadFile(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("read critical skills policy: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line != "" {
			result[line] = true
		}
	}
	return result, nil
}

func skillEditPathspecs(repoRoot, skill string) []string {
	pathspecs := []string{filepath.ToSlash(filepath.Join("skills", skill))}
	for _, root := range []string{"skills-codex", "skills-codex-overrides"} {
		if isDir(filepath.Join(repoRoot, root, skill)) {
			pathspecs = append(pathspecs, filepath.ToSlash(filepath.Join(root, skill)))
		}
	}
	return pathspecs
}

func skillSlugFromPath(path string) string {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "skills" {
		return parts[1]
	}
	return ""
}

func gitOutput(repoRoot string, args ...string) (string, error) {
	return gitOutputEnv(repoRoot, nil, args...)
}

func gitOutputEnv(repoRoot string, extraEnv []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func resolveRepoRootForSkills() (string, error) {
	skillsDir, _ := resolveSkillsRoots()
	if !isDir(skillsDir) {
		return "", fmt.Errorf("skills directory not found")
	}
	return filepath.Dir(skillsDir), nil
}

func criticalPolicyDisplay(path string) string {
	if strings.TrimSpace(path) == "" {
		return "docs/contracts/critical-skills.txt"
	}
	return path
}

func firstNonEmptySkillEdit(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shortSkillEditHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
