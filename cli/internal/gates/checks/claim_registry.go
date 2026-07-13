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

	"github.com/boshu2/agentops/cli/internal/claimpolicy"
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
		ID:         "claim.tier-citation",
		Tiers:      gates.Fast,
		Match:      claimRegistryPaths,
		Blocking:   true,
		Run:        runClaimTierCitation,
		RepairHint: "ao claim check --changed --json",
	})
}

var claimRegistryPaths = []string{
	"docs/contracts/claim-registry.yaml",
	"schemas/claim-registry.v1.schema.json",
	"PRODUCT.md", "README.md", "GOALS.md",
	"docs/**",
}

var claimMarkerRe = regexp.MustCompile(`agentops:claim:(AOP-CLAIM-[A-Z0-9-]+)`)

type claimRegistryFile struct {
	Version string                       `yaml:"version"`
	Tiers   map[string]claimRegistryTier `yaml:"tiers"`
	Claims  map[string]claimRegistryRow  `yaml:"claims"`
}

type claimRegistryTier struct {
	CiteAllowed []string `yaml:"cite_allowed"`
}

type claimRegistryRow struct {
	Tier     string   `yaml:"tier"`
	Surfaces []string `yaml:"surfaces"`
}

func runClaimRegistryDrift(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	reg, verdict, ok := readClaimRegistry(rc.RepoRoot)
	if !ok {
		return verdict, nil
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
			Status:  ports.GateStatusFail,
			Reason:  fmt.Sprintf("claim registry drift: %d parity issue(s)", len(problems)),
			LogTail: tail(strings.Join(problems, "\n"), 4096),
		}, nil
	}

	return ports.GateVerdict{
		Status: ports.GateStatusPass,
		Reason: fmt.Sprintf("claim registry in sync: %d claim(s)", len(reg.Claims)),
	}, nil
}

func readClaimRegistry(repoRoot string) (claimRegistryFile, ports.GateVerdict, bool) {
	registryPath := filepath.Join(repoRoot, "docs", "contracts", "claim-registry.yaml")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return claimRegistryFile{}, ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: "claim-registry.yaml not found: " + err.Error(),
		}, false
	}

	var reg claimRegistryFile
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return claimRegistryFile{}, ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: "claim-registry.yaml parse error: " + err.Error(),
		}, false
	}
	if reg.Claims == nil {
		reg.Claims = map[string]claimRegistryRow{}
	}
	if reg.Tiers == nil {
		reg.Tiers = map[string]claimRegistryTier{}
	}
	return reg, ports.GateVerdict{}, true
}

func runClaimTierCitation(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	reg, verdict, ok := readClaimRegistry(rc.RepoRoot)
	if !ok {
		return verdict, nil
	}

	markers, err := scanClaimMarkersForCitation(ctx, rc)
	if err != nil {
		return ports.GateVerdict{
			Status: ports.GateStatusFail,
			Reason: "marker scan error: " + err.Error(),
		}, nil
	}
	if len(markers) == 0 {
		return ports.GateVerdict{
			Status: ports.GateStatusPass,
			Reason: "claim tier citations clean: no changed claim markers",
		}, nil
	}

	var problems []string
	for id, surfaces := range markers {
		row, ok := reg.Claims[id]
		if !ok {
			continue
		}
		tierName := claimpolicy.NormalizeTier(row.Tier)
		tier, ok := reg.Tiers[tierName]
		if !ok || len(tier.CiteAllowed) == 0 {
			problems = append(problems, fmt.Sprintf("%s tier %s has no cite_allowed policy", id, tierName))
			continue
		}
		for _, surface := range surfaces {
			if claimpolicy.SurfaceAllowed(tier.CiteAllowed, surface) {
				continue
			}
			problems = append(problems, fmt.Sprintf("%s tier %s cannot be cited from %s (allowed: %s)",
				id, tierName, surface, strings.Join(tier.CiteAllowed, ", ")))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		return ports.GateVerdict{
			Status:  ports.GateStatusFail,
			Reason:  fmt.Sprintf("claim tier citation ceiling: %d violation(s)", len(problems)),
			LogTail: tail(strings.Join(problems, "\n"), 4096),
		}, nil
	}

	return ports.GateVerdict{
		Status: ports.GateStatusPass,
		Reason: fmt.Sprintf("claim tier citations clean: %d changed claim(s)", len(markers)),
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

func scanClaimMarkersForCitation(ctx context.Context, rc gates.RunContext) (map[string][]string, error) {
	if changedIncludesClaimPolicy(rc.ChangedFiles) {
		return scanClaimMarkers(ctx, rc.RepoRoot)
	}
	return scanClaimMarkersInPaths(rc.RepoRoot, rc.ChangedFiles)
}

func changedIncludesClaimPolicy(paths []string) bool {
	for _, relPath := range paths {
		relPath = filepath.ToSlash(strings.TrimSpace(relPath))
		if relPath == "docs/contracts/claim-registry.yaml" || relPath == "schemas/claim-registry.v1.schema.json" {
			return true
		}
	}
	return false
}

func scanClaimMarkersInPaths(repoRoot string, paths []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, relPath := range paths {
		relPath = filepath.ToSlash(strings.TrimSpace(relPath))
		if relPath == "" || shouldSkipClaimPath(relPath) || !looksLikeClaimHost(relPath) {
			continue
		}
		ids, err := extractClaimIDs(filepath.Join(repoRoot, filepath.FromSlash(relPath)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, id := range ids {
			result[id] = appendUnique(result[id], relPath)
		}
	}
	return result, nil
}

func shouldSkipClaimPath(relPath string) bool {
	if strings.HasPrefix(relPath, ".agents/") || strings.HasPrefix(relPath, "_beads/") {
		return true
	}
	if strings.HasSuffix(relPath, "_test.go") || strings.Contains(relPath, "/testdata/") {
		return true
	}
	return relPath == "docs/contracts/claim-registry.yaml" || relPath == "docs/contracts/claim-eval-promote.md"
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
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
	// Read-only handle: a Close error cannot lose data, so it is safe to drop.
	defer func() { _ = f.Close() }()

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
