package packet

import (
	"fmt"
	"strings"
)

var requiredLifecycleReceipts = []struct {
	phase  string
	skill  string
	status string
}{
	{phase: "discovery", skill: "discovery", status: "DONE"},
	{phase: "crank", skill: "crank", status: "DONE"},
	{phase: "validate", skill: "validate", status: "PASS"},
	{phase: "learn", skill: "learn", status: "DONE"},
}

// ValidateLifecycleReceipts proves that a completed RPI tick crossed the four
// umbrella boundaries exactly once and in order. In-progress packets may carry
// a prefix; callers invoke this method only at completion/handoff.
func (p ExecutionPacket) ValidateLifecycleReceipts() error {
	if len(p.PhaseReceipts) != len(requiredLifecycleReceipts) {
		return fmt.Errorf("phase_receipts must contain discovery, crank, validate, learn in order; got %d receipts", len(p.PhaseReceipts))
	}
	loaded := make(map[string]struct{}, len(p.SkillsLoaded))
	for index, entry := range p.SkillsLoaded {
		name := strings.TrimSpace(entry.Name)
		if name == "" || strings.TrimSpace(entry.Reason) == "" {
			return fmt.Errorf("skills_loaded[%d] requires nonempty name and reason", index)
		}
		if _, duplicate := loaded[name]; duplicate {
			return fmt.Errorf("skills_loaded contains duplicate skill %q", name)
		}
		loaded[name] = struct{}{}
	}
	if _, ok := loaded["rpi"]; !ok {
		return fmt.Errorf("skills_loaded must include rpi")
	}
	for index, expected := range requiredLifecycleReceipts {
		receipt := p.PhaseReceipts[index]
		if receipt.Phase != expected.phase || receipt.Skill != expected.skill {
			return fmt.Errorf("phase_receipts[%d] must be phase %q with skill %q; got phase %q skill %q", index, expected.phase, expected.skill, receipt.Phase, receipt.Skill)
		}
		if receipt.Status != expected.status {
			return fmt.Errorf("phase_receipts[%d] status %q, want %q", index, receipt.Status, expected.status)
		}
		if strings.TrimSpace(receipt.Artifact) == "" {
			return fmt.Errorf("phase_receipts[%d].artifact must be nonempty", index)
		}
		if _, ok := loaded[receipt.Skill]; !ok {
			return fmt.Errorf("phase_receipts[%d].skill %q is absent from skills_loaded", index, receipt.Skill)
		}
	}
	return nil
}
