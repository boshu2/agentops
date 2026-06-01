// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ProductionHarness satisfies HarnessPort by scanning the
// canonical skills/ tree and the skills-codex/ tree, computing SHA256
// content hashes for each SKILL.md, and reporting per-(skill, harness)
// sync state. A skill is "out of sync" when the codex copy's hash
// differs from the canonical (Claude) copy's hash.
//
// rootDir is the repository root containing both skills/ and
// skills-codex/ subdirectories. The adapter is read-only — it does
// not modify either tree (re-sync work belongs to
// scripts/regen-codex-hashes.sh).
type ProductionHarness struct {
	rootDir string
}

// NewProductionHarness returns an adapter rooted at rootDir.
func NewProductionHarness(rootDir string) *ProductionHarness {
	return &ProductionHarness{rootDir: rootDir}
}

// scanAll walks both trees and returns all (skill, harness) entries
// it discovered, with OutOfSync computed by comparing the codex hash
// against the canonical Claude hash for the same skill.
func (h *ProductionHarness) scanAll(ctx context.Context) ([]HarnessSkillSync, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if h.rootDir == "" {
		return nil, fmt.Errorf("ProductionHarness: rootDir required")
	}
	claude, err := h.scanTree(filepath.Join(h.rootDir, "skills"), HarnessClaude, "skills")
	if err != nil {
		return nil, err
	}
	codex, err := h.scanTree(filepath.Join(h.rootDir, "skills-codex"), HarnessCodex, "skills-codex")
	if err != nil {
		return nil, err
	}
	canonicalHash := make(map[string]string, len(claude))
	for _, e := range claude {
		canonicalHash[e.Skill] = e.ContentHash
	}
	for i := range codex {
		canon := canonicalHash[codex[i].Skill]
		codex[i].OutOfSync = canon != "" && codex[i].ContentHash != canon
	}
	out := append(claude, codex...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Skill != out[j].Skill {
			return out[i].Skill < out[j].Skill
		}
		return string(out[i].Harness) < string(out[j].Harness)
	})
	return out, nil
}

// scanTree walks <root> looking for <skill>/SKILL.md files and hashes
// each one. Missing root is not an error (returns empty slice).
func (h *ProductionHarness) scanTree(root string, harness HarnessName, rel string) ([]HarnessSkillSync, error) {
	out := make([]HarnessSkillSync, 0)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("ProductionHarness scan %q: %w", root, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillName := e.Name()
		skillPath := filepath.Join(root, skillName, "SKILL.md")
		body, err := os.ReadFile(skillPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("ProductionHarness read %q: %w", skillPath, err)
		}
		sum := sha256.Sum256(body)
		out = append(out, HarnessSkillSync{
			Harness:     harness,
			Skill:       skillName,
			Path:        filepath.Join(rel, skillName, "SKILL.md"),
			ContentHash: hex.EncodeToString(sum[:]),
			OutOfSync:   false, // computed by caller after comparing trees
		})
	}
	return out, nil
}

// Status returns the full inventory across both harnesses.
func (h *ProductionHarness) Status(ctx context.Context) ([]HarnessSkillSync, error) {
	return h.scanAll(ctx)
}

// StatusForSkill returns entries for one skill across both harnesses.
func (h *ProductionHarness) StatusForSkill(ctx context.Context, skill string) ([]HarnessSkillSync, error) {
	if skill == "" {
		return nil, fmt.Errorf("ProductionHarness: skill required")
	}
	all, err := h.scanAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HarnessSkillSync, 0)
	for _, e := range all {
		if e.Skill == skill {
			out = append(out, e)
		}
	}
	return out, nil
}

// Compile-time assertion: ProductionHarness satisfies the port.
var _ HarnessPort = (*ProductionHarness)(nil)
