package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/archcheck"
)

func main() {
	root := flag.String("root", ".", "repository root")
	family := flag.String("family", "", "migrated command family to validate")
	selfTest := flag.Bool("self-test", false, "run induced positive and negative fixtures")
	allMigrated := flag.Bool("all-migrated", false, "validate every frozen family with a migrated module")
	inventory := flag.Bool("inventory", false, "emit the deterministic pre-migration family inventory")
	out := flag.String("out", "", "write inventory JSON to this path instead of stdout")
	verifyScope := flag.String("verify-scope", "", "verify a reviewed scope JSON against lineage and ownership")
	candidateSHA := flag.String("candidate-sha", "", "externally resolved exact candidate Git SHA")
	semanticProductionGate := flag.Bool("semantic-production-gate", false, "execute every registered semantic rule canary")
	flag.Parse()
	if *semanticProductionGate {
		rules, err := archcheck.SemanticProductionGate(*candidateSHA)
		if err != nil {
			fmt.Fprintln(os.Stderr, "go-cli semantic production gate FAIL:", err)
			os.Exit(1)
		}
		for index, rule := range rules {
			if index > 0 {
				fmt.Print(",")
			}
			fmt.Print(rule)
		}
		fmt.Println()
		return
	}
	if *selfTest {
		if err := archcheck.SelfTest(); err != nil {
			fmt.Fprintln(os.Stderr, "go-cli-architecture self-test FAIL:", err)
			os.Exit(1)
		}
		fmt.Println("go-cli-architecture self-test PASS")
		return
	}
	if *inventory {
		packet, err := archcheck.BuildInventory(*root, *family)
		if err != nil {
			fmt.Fprintln(os.Stderr, "go-cli-architecture inventory:", err)
			os.Exit(1)
		}
		encoded, err := json.MarshalIndent(packet, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "go-cli-architecture inventory:", err)
			os.Exit(1)
		}
		encoded = append(encoded, '\n')
		if *out == "" {
			_, _ = os.Stdout.Write(encoded)
			return
		}
		path := *out
		if !filepath.IsAbs(path) {
			path = filepath.Join(*root, path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "go-cli-architecture inventory:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "go-cli-architecture inventory:", err)
			os.Exit(1)
		}
		fmt.Println(path)
		return
	}
	violations, err := archcheck.Check(archcheck.Options{Root: *root, Family: *family, AllMigrated: *allMigrated, VerifyScope: *verifyScope, CandidateSHA: *candidateSHA})
	if err != nil {
		fmt.Fprintln(os.Stderr, "go-cli-architecture:", err)
		os.Exit(1)
	}
	if len(violations) != 0 {
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, violation.String())
		}
		fmt.Fprintf(os.Stderr, "go-cli-architecture FAIL: %d violation(s)\n", len(violations))
		os.Exit(1)
	}
	if *family != "" {
		fmt.Printf("go-cli-architecture PASS: family=%s\n", *family)
	} else {
		fmt.Println("go-cli-architecture PASS")
	}
}
