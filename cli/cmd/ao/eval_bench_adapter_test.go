package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

func TestEvalBenchAdapterStandardJSON(t *testing.T) {
	corpus := filepath.Join("testdata", "retrieval-bench")
	output, err := (evalBenchAdapter{}).Bench(context.Background(), aoeval.BenchRequest{Corpus: corpus, JSON: true, K: 3})
	if err != nil {
		t.Fatalf("Bench: %v", err)
	}
	if !json.Valid([]byte(output.Stdout)) {
		t.Fatalf("stdout is not JSON: %q", output.Stdout)
	}
}
