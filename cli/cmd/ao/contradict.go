package main

import (
	"encoding/json"
	"fmt"
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
	"always": "never",
	"never":  "always",
	"do":     "don't",
	"don't":  "do",
	"use":    "avoid",
	"avoid":  "use",
	"enable": "disable",
	"disable": "enable",
	"required": "optional",
	"optional": "required",
	"must":   "never",
	"should": "shouldn't",
	"shouldn't": "should",
}

// learningEntry holds extracted content for contradiction analysis.
type learningEntry struct {
	File    string
	Body    string
	Words   map[string]bool
	Snippet string
}

func runContradict(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	patternsDir := filepath.Join(cwd, ".agents", "patterns")

	learningsExists := dirExists(learningsDir)
	patternsExists := dirExists(patternsDir)

	if !learningsExists && !patternsExists {
		fmt.Println("No learnings or patterns directory found.")
		return nil
	}

	// Collect all files
	var files []string
	for _, dir := range []string{learningsDir, patternsDir} {
		if !dirExists(dir) {
			continue
		}
		jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		files = append(files, jsonlFiles...)
		files = append(files, mdFiles...)
	}

	if len(files) == 0 {
		fmt.Println("No learning or pattern files found.")
		return nil
	}

	// Parse all entries
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

	// Compare all pairs
	result := ContradictResult{
		TotalFiles: len(files),
	}

	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			result.PairsChecked++
			sim := jaccardSimilarity(entries[i].Words, entries[j].Words)
			if sim < 0.4 {
				continue // Not similar enough to be about the same topic
			}

			reason := detectContradiction(entries[i].Body, entries[j].Body)
			if reason == "" {
				continue
			}

			// Make paths relative for cleaner output
			relA, relErr := filepath.Rel(cwd, entries[i].File)
			if relErr != nil {
				relA = entries[i].File
			}
			relB, relErr := filepath.Rel(cwd, entries[j].File)
			if relErr != nil {
				relB = entries[j].File
			}

			result.Contradictions++
			result.Pairs = append(result.Pairs, ContradictionPair{
				FileA:      relA,
				FileB:      relB,
				Similarity: sim,
				Reason:     reason,
				SnippetA:   entries[i].Snippet,
				SnippetB:   entries[j].Snippet,
			})
		}
	}

	// Output
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Contradiction Scan Results\n")
	fmt.Printf("==========================\n")
	fmt.Printf("Total files:         %d\n", result.TotalFiles)
	fmt.Printf("Pairs checked:       %d\n", result.PairsChecked)
	fmt.Printf("Contradictions found: %d\n", result.Contradictions)

	if result.Contradictions > 0 {
		fmt.Println("\nPotential Contradictions:")
		for i, p := range result.Pairs {
			fmt.Printf("\n  %d. Similarity: %.1f%% — %s\n", i+1, p.Similarity*100, p.Reason)
			fmt.Printf("     A: %s\n", p.FileA)
			fmt.Printf("        %s\n", p.SnippetA)
			fmt.Printf("     B: %s\n", p.FileB)
			fmt.Printf("        %s\n", p.SnippetB)
		}
	} else {
		fmt.Println("\nNo potential contradictions found.")
	}

	return nil
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
