package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

type evalBenchAdapter struct{}

func (evalBenchAdapter) Bench(_ context.Context, request aoeval.BenchRequest) (aoeval.AliasOutput, error) {
	if strings.TrimSpace(request.SearchEval) != "" {
		return evalSearchBench(request)
	}
	if request.Live {
		return evalLiveBench(request)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return aoeval.AliasOutput{}, fmt.Errorf("get working directory: %w", err)
	}
	corpus, err := resolveRetrievalBenchCorpus(cwd, request.Corpus)
	if err != nil {
		return aoeval.AliasOutput{}, err
	}
	report, err := buildBenchReport(corpus, corpus, request.K)
	if err != nil {
		return aoeval.AliasOutput{}, err
	}
	if request.JSON {
		return jsonAliasOutput(report)
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "Retrieval Quality Report\n========================")
	fmt.Fprintf(&out, "Queries:     %d\nPrecision@%d: %.2f (target: %.2f)\nMRR:         %.2f (target: %.2f)\n", report.Queries, request.K, report.AvgPAtK, report.TargetPAtK, report.AvgMRR, report.TargetMRR)
	if len(report.Splits) > 0 {
		fmt.Fprintln(&out, "Splits:")
		names := make([]string, 0, len(report.Splits))
		for name := range report.Splits {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			summary := report.Splits[name]
			fmt.Fprintf(&out, "  %-10s cases=%d  P@%d=%.2f  MRR=%.2f", name, summary.Cases, request.K, summary.AvgPAtK, summary.AvgMRR)
			if summary.SectionCases > 0 {
				fmt.Fprintf(&out, "  section-MRR=%.2f", summary.AvgSectionMRR)
			}
			fmt.Fprintln(&out)
		}
	}
	fmt.Fprintln(&out, "\nPer-query breakdown:")
	for _, result := range report.Results {
		status := "PASS"
		if !result.Pass {
			status = "WARN (below target)"
		}
		fmt.Fprintf(&out, "  %-30s P@%d=%.2f  MRR=%.2f", fmt.Sprintf("%q", result.Query), request.K, result.PAtK, result.MRR)
		if result.ExpectedSection != "" {
			fmt.Fprintf(&out, "  section-MRR=%.2f", result.SectionMRR)
		}
		fmt.Fprintf(&out, "  %s\n", status)
	}
	return aoeval.AliasOutput{Stdout: out.String()}, nil
}

func evalLiveBench(request aoeval.BenchRequest) (aoeval.AliasOutput, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return aoeval.AliasOutput{}, fmt.Errorf("getting working directory: %w", err)
	}
	globalDir, mode, storeLabel, modeLabel, benchCwd := "", "live-local", ".agents/learnings/", "Live (local)", cwd
	if request.Corpus != "" {
		if _, err := os.Stat(request.Corpus); err != nil {
			return aoeval.AliasOutput{}, fmt.Errorf("live benchmark corpus not found at %s", request.Corpus)
		}
		tmp, err := os.MkdirTemp("", "retrieval-live-*")
		if err != nil {
			return aoeval.AliasOutput{}, err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		benchCwd, globalDir, mode, storeLabel, modeLabel = tmp, request.Corpus, "live-corpus", request.Corpus, "Live (fixture)"
	} else if request.Global {
		home, err := os.UserHomeDir()
		if err != nil {
			return aoeval.AliasOutput{}, err
		}
		globalDir = filepath.Join(home, ".agents", "learnings")
		if _, err := os.Stat(globalDir); err != nil {
			return aoeval.AliasOutput{}, fmt.Errorf("global knowledge store not found at %s — run 'ao harvest' first", globalDir)
		}
		mode, storeLabel, modeLabel = "live-global", "~/.agents/learnings/ (cross-rig)", "Live (global)"
	}
	report, err := buildLiveReport(benchCwd, globalDir, mode, request.K)
	if err != nil {
		return aoeval.AliasOutput{}, err
	}
	if request.JSON {
		return jsonAliasOutput(report)
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "Retrieval Quality Report (%s)\n================================\nCorpus:      %d learnings in %s\nQueries:     %d\nK:           %d\nCoverage:    %.0f%% (%d/%d queries with hits)\n\n", modeLabel, report.TotalLearnings, storeLabel, report.Queries, request.K, report.Coverage*100, report.QueriesWithHits, report.Queries)
	if report.TotalLearnings == 0 {
		fmt.Fprintln(&out, "No learnings found. Run /retro or /post-mortem to populate the knowledge base.")
		return aoeval.AliasOutput{Stdout: out.String()}, nil
	}
	fmt.Fprintln(&out, "Per-query breakdown:")
	for _, result := range report.Results {
		if result.Count == 0 {
			fmt.Fprintf(&out, "  %-25s  hits=0  (no matches)\n", fmt.Sprintf("%q", result.Query))
		} else {
			fmt.Fprintf(&out, "  %-25s  hits=%-3d  score=[%.2f, %.2f]  mean=%.2f  top=%v\n", fmt.Sprintf("%q", result.Query), result.Count, result.MinScore, result.MaxScore, result.MeanScore, result.TopIDs)
		}
	}
	return aoeval.AliasOutput{Stdout: out.String()}, nil
}

func evalSearchBench(request aoeval.BenchRequest) (aoeval.AliasOutput, error) {
	k := request.K
	if !request.KChanged {
		k = defaultSearchEvalK
	}
	backends, compare, err := resolveSearchEvalRunBackends(request.SearchBackend, request.CompareBackends)
	if err != nil {
		return aoeval.AliasOutput{}, err
	}
	if compare {
		report, err := buildSearchEvalComparisonReport(request.SearchRoot, request.SearchEval, k, backends)
		if err != nil {
			return aoeval.AliasOutput{}, err
		}
		if request.JSON {
			return jsonAliasOutput(report)
		}
		var out bytes.Buffer
		fmt.Fprintf(&out, "AO Search Retrieval Eval Comparison\n===================================\nEval set: %s\nQueries:  %d\nK:        %d\n", report.ID, report.Queries, report.K)
		for _, backend := range report.Backends {
			fmt.Fprintf(&out, "  %-20s any@k=%.0f%% precision=%.2f MRR=%.2f\n", backend.Backend, backend.AnyRelevantAtK*100, backend.AvgPrecisionAtK, backend.MeanReciprocalRank)
		}
		return aoeval.AliasOutput{Stdout: out.String()}, nil
	}
	report, err := buildSearchEvalReportForBackend(request.SearchRoot, request.SearchEval, k, backends[0])
	if err != nil {
		return aoeval.AliasOutput{}, err
	}
	if request.JSON {
		return jsonAliasOutput(report)
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "AO Search Retrieval Eval\n========================")
	fmt.Fprintf(&out, "Eval set:       %s\nBackend:        %s\nManifest:       %s\nSearch root:    %s\nQueries:        %d\nK:              %d\n", report.ID, report.Backend, report.ManifestPath, report.SearchRoot, report.Queries, report.K)
	if report.MissingGroundTruth > 0 {
		fmt.Fprintf(&out, "Missing labels: %d ground-truth path(s)\n", report.MissingGroundTruth)
	}
	fmt.Fprintf(&out, "Any-relevant@%d: %.0f%% (%d/%d)\nAvg precision@%d: %.2f\nMRR:            %.2f\n", report.K, report.AnyRelevantAtK*100, report.Hits, report.Queries, report.K, report.AvgPrecisionAtK, report.MeanReciprocalRank)
	return aoeval.AliasOutput{Stdout: out.String()}, nil
}

func jsonAliasOutput(value any) (aoeval.AliasOutput, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return aoeval.AliasOutput{}, err
	}
	return aoeval.AliasOutput{Stdout: out.String()}, nil
}
