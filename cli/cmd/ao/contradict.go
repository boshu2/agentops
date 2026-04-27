package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// ContradictionPair represents two learnings that may contradict each other.
type ContradictionPair struct {
	FileA      string  `json:"file_a"`
	FileB      string  `json:"file_b"`
	Similarity float64 `json:"similarity"`
	Reason     string  `json:"reason"`
	SnippetA   string  `json:"snippet_a,omitempty"`
	SnippetB   string  `json:"snippet_b,omitempty"`
}

// ContradictResult is the output of the contradiction scan.
type ContradictResult struct {
	TotalFiles     int                 `json:"total_files"`
	PairsChecked   int                 `json:"pairs_checked"`
	Contradictions int                 `json:"contradictions"`
	Pairs          []ContradictionPair `json:"pairs,omitempty"`
}

type contradictOptions struct {
	Cwd    string
	Output string
	Writer io.Writer
}

var contradictCmd = &cobra.Command{
	Use:   "contradict",
	Short: "Detect potentially contradictory learnings",
	Long: `Scan learnings and patterns for potential contradictions using keyword overlap heuristics.

Reads all files (.md and .jsonl) from .agents/learnings/ and .agents/patterns/,
extracts body content, and compares each pair using Jaccard similarity on word sets.
Pairs with high topic overlap but opposing sentiment indicators (e.g., "always" vs "never",
"do" vs "don't") are flagged as potential contradictions.

This is a heuristic tool — false positives are expected. Use it as a review aid.

Examples:
  ao contradict
  ao contradict --json`,
	RunE: runContradict,
}

func init() {
	contradictCmd.GroupID = "core"
	rootCmd.AddCommand(contradictCmd)
}

// negationWords are words that indicate negation or avoidance.
var negationWords = map[string]bool{
	"not": true, "never": true, "don't": true, "dont": true,
	"avoid": true, "anti-pattern": true, "antipattern": true,
	"shouldn't": true, "shouldnt": true, "cannot": true,
	"won't": true, "wont": true, "isn't": true, "isnt": true,
}

// oppositionPairs maps words to their opposites for contradiction detection.
var oppositionPairs = map[string]string{
	"always":    "never",
	"never":     "always",
	"do":        "don't",
	"don't":     "do",
	"use":       "avoid",
	"avoid":     "use",
	"enable":    "disable",
	"disable":   "enable",
	"required":  "optional",
	"optional":  "required",
	"must":      "never",
	"should":    "shouldn't",
	"shouldn't": "should",
}

// learningEntry holds extracted content for contradiction analysis.
type learningEntry struct {
	File    string
	Body    string
	Words   map[string]bool
	Snippet string
}

func runContradict(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	writer := io.Writer(os.Stdout)
	if cmd != nil {
		writer = cmd.OutOrStdout()
	}

	return runContradictWithOptions(contradictOptions{
		Cwd:    cwd,
		Output: GetOutput(),
		Writer: writer,
	})
}

func runContradictWithOptions(opts contradictOptions) error {
	if opts.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		opts.Cwd = cwd
	}
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}

	files, emptyMessage := collectContradictFiles(opts.Cwd)
	if emptyMessage != "" {
		result := ContradictResult{}
		if opts.Output == "json" {
			return writeContradictJSON(opts.Writer, result)
		}
		fmt.Fprintln(opts.Writer, emptyMessage)
		return nil
	}

	result := buildContradictResult(opts.Cwd, files)
	if opts.Output == "json" {
		return writeContradictJSON(opts.Writer, result)
	}

	writeContradictHuman(opts.Writer, result)
	return nil
}

func collectContradictFiles(cwd string) ([]string, string) {
	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	patternsDir := filepath.Join(cwd, ".agents", "patterns")
	dirs := []string{learningsDir, patternsDir}

	hasSourceDir := false
	var files []string
	for _, dir := range dirs {
		if !dirExists(dir) {
			continue
		}
		hasSourceDir = true
		jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		files = append(files, jsonlFiles...)
		files = append(files, mdFiles...)
	}

	if !hasSourceDir {
		return nil, "No learnings or patterns directory found."
	}
	if len(files) == 0 {
		return nil, "No learning or pattern files found."
	}
	return files, ""
}

func buildContradictResult(cwd string, files []string) ContradictResult {
	entries := make([]learningEntry, 0, len(files))
	for _, f := range files {
		body := extractLearningBody(f)
		if body == "" {
			continue
		}
		words := tokenize(body)
		if len(words) == 0 {
			continue
		}
		snippet := truncateSnippet(body, 120)
		entries = append(entries, learningEntry{
			File:    f,
			Body:    body,
			Words:   words,
			Snippet: snippet,
		})
	}

	result := ContradictResult{TotalFiles: len(files)}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			result.PairsChecked++
			if pair, ok := compareContradictEntries(cwd, entries[i], entries[j]); ok {
				result.Contradictions++
				result.Pairs = append(result.Pairs, pair)
			}
		}
	}

	return result
}

func compareContradictEntries(cwd string, a, b learningEntry) (ContradictionPair, bool) {
	sim := jaccardSimilarity(a.Words, b.Words)
	if sim < 0.4 {
		return ContradictionPair{}, false
	}

	reason := detectContradiction(a.Body, b.Body)
	if reason == "" {
		return ContradictionPair{}, false
	}

	relA := relativePathOrOriginal(cwd, a.File)
	relB := relativePathOrOriginal(cwd, b.File)
	return ContradictionPair{
		FileA:      relA,
		FileB:      relB,
		Similarity: sim,
		Reason:     reason,
		SnippetA:   a.Snippet,
		SnippetB:   b.Snippet,
	}, true
}

func relativePathOrOriginal(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}

func writeContradictHuman(w io.Writer, result ContradictResult) {
	fmt.Fprintln(w, "Contradiction Scan Results")
	fmt.Fprintln(w, "==========================")
	fmt.Fprintf(w, "Total files:         %d\n", result.TotalFiles)
	fmt.Fprintf(w, "Pairs checked:       %d\n", result.PairsChecked)
	fmt.Fprintf(w, "Contradictions found: %d\n", result.Contradictions)
	if result.Contradictions > 0 {
		fmt.Fprintln(w, "\nPotential Contradictions:")
		for i, p := range result.Pairs {
			fmt.Fprintf(w, "\n  %d. Similarity: %.1f%% — %s\n", i+1, p.Similarity*100, p.Reason)
			fmt.Fprintf(w, "     A: %s\n", p.FileA)
			fmt.Fprintf(w, "        %s\n", p.SnippetA)
			fmt.Fprintf(w, "     B: %s\n", p.FileB)
			fmt.Fprintf(w, "        %s\n", p.SnippetB)
		}
	} else {
		fmt.Fprintln(w, "\nNo potential contradictions found.")
	}
}

func writeContradictJSON(w io.Writer, result ContradictResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// tokenize extracts a set of lowercase words from text, filtering out short words and punctuation.
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	lower := strings.ToLower(text)
	// Split on non-letter, non-apostrophe boundaries
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\''
	})
	for _, w := range tokens {
		w = strings.Trim(w, "'")
		if len(w) >= 3 { // Skip very short words (a, an, is, to, etc.)
			words[w] = true
		}
	}
	return words
}

// jaccardSimilarity computes the Jaccard similarity coefficient between two word sets.
// Returns a value between 0.0 (no overlap) and 1.0 (identical sets).
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}

	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// detectContradiction checks if two texts show signs of contradicting each other.
// Returns a reason string if contradiction is detected, empty string otherwise.
func detectContradiction(textA, textB string) string {
	lowerA := strings.ToLower(textA)
	lowerB := strings.ToLower(textB)

	// Check for negation asymmetry: one has negation words, the other doesn't
	negA := countNegations(lowerA)
	negB := countNegations(lowerB)

	if negA > 0 && negB == 0 {
		return "negation asymmetry: first text contains negation words, second does not"
	}
	if negB > 0 && negA == 0 {
		return "negation asymmetry: second text contains negation words, first does not"
	}

	// Check for opposition pairs
	wordsA := strings.Fields(lowerA)
	wordsB := strings.Fields(lowerB)
	wordSetA := make(map[string]bool, len(wordsA))
	wordSetB := make(map[string]bool, len(wordsB))
	for _, w := range wordsA {
		wordSetA[w] = true
	}
	for _, w := range wordsB {
		wordSetB[w] = true
	}

	for word, opposite := range oppositionPairs {
		if wordSetA[word] && wordSetB[opposite] && !wordSetA[opposite] && !wordSetB[word] {
			return fmt.Sprintf("opposing terms: %q vs %q", word, opposite)
		}
	}

	return ""
}

// countNegations counts how many negation words appear in the text.
func countNegations(lower string) int {
	count := 0
	words := strings.Fields(lower)
	for _, w := range words {
		// Strip trailing punctuation for matching
		w = strings.TrimRight(w, ".,;:!?)")
		if negationWords[w] {
			count++
		}
	}
	return count
}

// truncateSnippet returns the first n characters of text, adding "..." if truncated.
func truncateSnippet(text string, n int) string {
	// Collapse whitespace first
	s := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	// Fast path: byte length is an upper bound on rune count.
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
