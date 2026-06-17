package checks

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"

	"gopkg.in/yaml.v3"
)

func init() {
	gates.Register(gates.Check{
		ID:       "claim.registry-drift",
		Tiers:    gates.Fast | gates.Full,
		Match:    claimRegistryPaths,
		Blocking: true,
		Run:      runClaimRegistryDrift,
	})
	gates.Register(gates.Check{
		ID:         "claim.pmf-evidence",
		Tiers:      gates.Fast | gates.Full,
		Match:      claimPMFPaths,
		Blocking:   false,
		Backing:    "check-pmf-evidence.sh",
		Args:       []string{"--list"},
		RepairHint: "bash scripts/check-pmf-evidence.sh --list",
	})
}

var claimRegistryPaths = []string{
	"docs/contracts/claim-registry.yaml",
	"schemas/claim-registry.v1.schema.json",
	"PRODUCT.md", "README.md", "GOALS.md",
	"docs/**",
}

var claimPMFPaths = []string{
	"PRODUCT.md", "README.md", "docs/launch/**",
}

var claimMarkerRe = regexp.MustCompile(`agentops:claim:(AOP-CLAIM-[A-Z0-9-]+)`)

type claimRegistryFile struct {
	Version string                      `yaml:"version"`
	Claims  map[string]claimRegistryRow `yaml:"claims"`
}

type claimRegistryRow struct {
	Tier     string   `yaml:"tier"`
	Surfaces []string `yaml:"surfaces"`
}

func runClaimRegistryDrift(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	registryPath := filepath.Join(rc.RepoRoot, "docs", "contracts", "claim-registry.yaml")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: "claim-registry.yaml not found: " + err.Error(),
		}, nil
	}

	var reg claimRegistryFile
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: "claim-registry.yaml parse error: " + err.Error(),
		}, nil
	}

	markers, err := scanClaimMarkers(ctx, rc.RepoRoot)
	if err != nil {
		return ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: "marker scan error: " + err.Error(),
		}, nil
	}

	var problems []string

	for id := range markers {
		if _, ok := reg.Claims[id]; !ok {
			problems = append(problems, fmt.Sprintf("marker %s has no registry entry", id))
		}
	}

	for id := range reg.Claims {
		if _, ok := markers[id]; !ok {
			problems = append(problems, fmt.Sprintf("registry entry %s has no marker in source", id))
		}
	}

	sort.Strings(problems)

	if len(problems) > 0 {
		return ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: fmt.Sprintf("claim registry drift: %d parity issue(s)", len(problems)),
			LogTail: tail(strings.Join(problems, "\n"), 4096),
		}, nil
	}

	return ports.GateVerdict{
		Status: ports.GateStatusPass,
		Reason: fmt.Sprintf("claim registry in sync: %d claim(s)", len(reg.Claims)),
	}, nil
}

func scanClaimMarkers(ctx context.Context, repoRoot string) (map[string][]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	result := make(map[string][]string)
	for _, relPath := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if relPath == "" {
			continue
		}
		if strings.HasPrefix(relPath, ".agents/") || strings.HasPrefix(relPath, "_beads/") {
			continue
		}
		if strings.HasSuffix(relPath, "_test.go") || strings.Contains(relPath, "/testdata/") {
			continue
		}
		if relPath == "docs/contracts/claim-registry.yaml" || relPath == "docs/contracts/claim-eval-promote.md" {
			continue
		}
		if !looksLikeClaimHost(relPath) {
			continue
		}
		absPath := filepath.Join(repoRoot, relPath)
		ids, err := extractClaimIDs(absPath)
		if err != nil {
			continue
		}
		for _, id := range ids {
			result[id] = append(result[id], relPath)
		}
	}
	return result, nil
}

func looksLikeClaimHost(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".md", ".yaml", ".yml", ".go":
		return true
	}
	return false
}

func extractClaimIDs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]bool)
	var ids []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		matches := claimMarkerRe.FindAllStringSubmatch(scanner.Text(), -1)
		for _, m := range matches {
			id := m[1]
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, scanner.Err()
}
