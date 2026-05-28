// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// InMemoryHarness is a HarnessPort backed by a fixed slice of
// HarnessSkillSync records. Intended for tests; the production
// adapter (FilesystemHarness or similar) computes hashes via real
// disk reads.
type InMemoryHarness struct {
	entries []HarnessSkillSync
}

// NewInMemoryHarness returns an adapter over the given entries. The
// caller's slice is retained (not copied) so callers must not mutate
// it after construction. Nil entries is safe.
func NewInMemoryHarness(entries []HarnessSkillSync) *InMemoryHarness {
	return &InMemoryHarness{entries: entries}
}

// Status returns all entries unchanged. The returned slice is a fresh
// copy so callers can sort/mutate without affecting adapter state.
func (h *InMemoryHarness) Status(ctx context.Context) ([]HarnessSkillSync, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]HarnessSkillSync, len(h.entries))
	copy(out, h.entries)
	return out, nil
}

// StatusForSkill returns entries whose Skill field matches the given
// skill. Empty skill is rejected as a structural error.
func (h *InMemoryHarness) StatusForSkill(ctx context.Context, skill string) ([]HarnessSkillSync, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if skill == "" {
		return nil, errors.New("ports: HarnessPort.StatusForSkill skill required")
	}
	out := make([]HarnessSkillSync, 0)
	for _, e := range h.entries {
		if e.Skill == skill {
			out = append(out, e)
		}
	}
	return out, nil
}

// Compile-time assertion: InMemoryHarness satisfies the port.
var _ HarnessPort = (*InMemoryHarness)(nil)
