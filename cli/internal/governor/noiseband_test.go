// practices: [design-by-contract, ai-assisted-dev]
package governor

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// gvDomain appends a gate-verdict (with a domain) via the production writer.
func gvDomain(t *testing.T, root, run, bead, disposition, domain string, attempt int) {
	t.Helper()
	w := yieldledger.Writer{}
	headSHA := bead + "-headsha0"
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID:          bead,
		RunID:           run,
		TS:              time.Date(2026, 6, 22, 14, 0, attempt, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     disposition,
		AuthorContextID: "ctx-" + bead,
		AuthorFamily:    "claude",
		HeadSHA:         headSHA,
		Attempt:         attempt,
		Domain:          domain,
	}); err != nil {
		t.Fatalf("append %s %s: %v", disposition, bead, err)
	}
}

// domainEscape writes a CONFIRMED (in domain) then a higher-attempt REFUTED — one
// escape attributed to that domain.
func domainEscape(t *testing.T, root, run, bead, domain string) {
	gvDomain(t, root, run, bead, yieldledger.DispositionConfirmed, domain, 1)
	gvDomain(t, root, run, bead, yieldledger.DispositionRefuted, domain, 2)
}

func TestShouldAdjust_SpecialCausePatternAdjusts(t *testing.T) {
	root := t.TempDir()
	// 3 escapes in the SAME domain within the window -> special-cause -> adjust.
	domainEscape(t, root, "r", "c1", "concurrency")
	domainEscape(t, root, "r", "c2", "concurrency")
	domainEscape(t, root, "r", "c3", "concurrency")

	v := ShouldAdjust(load(t, root), DefaultNoiseBandConfig())
	if v.Decision != Adjust {
		t.Fatalf("decision = %q, want adjust (%+v)", v.Decision, v)
	}
	if len(v.SpecialCauseDomains) != 1 || v.SpecialCauseDomains[0] != "concurrency" {
		t.Fatalf("special-cause domains = %v, want [concurrency]", v.SpecialCauseDomains)
	}
}

func TestShouldAdjust_BelowLimitHolds(t *testing.T) {
	root := t.TempDir()
	// 2 escapes in one domain (< K=3) -> common-cause -> hold (no tampering).
	domainEscape(t, root, "r", "c1", "concurrency")
	domainEscape(t, root, "r", "c2", "concurrency")

	v := ShouldAdjust(load(t, root), DefaultNoiseBandConfig())
	if v.Decision != Hold {
		t.Fatalf("decision = %q, want hold — 2 escapes is common-cause, adjusting would be tampering (%+v)", v.Decision, v)
	}
}

func TestShouldAdjust_SpreadAcrossDomainsHolds(t *testing.T) {
	root := t.TempDir()
	// 3 escapes but in 3 DIFFERENT domains (1 each) -> no domain reaches the limit
	// -> hold. The pattern must be a REPEATED one, not scattered noise.
	domainEscape(t, root, "r", "a1", "concurrency")
	domainEscape(t, root, "r", "b1", "gate-policy")
	domainEscape(t, root, "r", "c1", "docs")

	v := ShouldAdjust(load(t, root), DefaultNoiseBandConfig())
	if v.Decision != Hold {
		t.Fatalf("decision = %q, want hold — escapes scattered across domains are not a special-cause pattern (%+v)", v.Decision, v)
	}
}

func TestShouldAdjust_NilLedgerHolds(t *testing.T) {
	if v := ShouldAdjust(nil, DefaultNoiseBandConfig()); v.Decision != Hold {
		t.Fatalf("nil ledger decision = %q, want hold", v.Decision)
	}
}

func TestFitnessAdmits_TwoSided(t *testing.T) {
	tests := []struct {
		name          string
		before, after FitnessSnapshot
		wantAdmit     bool
	}{
		{"catch up, fa unchanged -> admit", FitnessSnapshot{0.50, 0.10}, FitnessSnapshot{0.60, 0.10}, true},
		{"catch up, fa down -> admit", FitnessSnapshot{0.50, 0.20}, FitnessSnapshot{0.60, 0.10}, true},
		{"catch up but fa up -> reject (cry-wolf)", FitnessSnapshot{0.50, 0.10}, FitnessSnapshot{0.70, 0.20}, false},
		{"catch unchanged -> reject (no benefit)", FitnessSnapshot{0.50, 0.10}, FitnessSnapshot{0.50, 0.05}, false},
		{"catch down -> reject", FitnessSnapshot{0.50, 0.10}, FitnessSnapshot{0.40, 0.10}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FitnessAdmits(tt.before, tt.after)
			if got.Admit != tt.wantAdmit {
				t.Fatalf("FitnessAdmits(%+v, %+v).Admit = %v, want %v (%s)", tt.before, tt.after, got.Admit, tt.wantAdmit, got.Reason)
			}
		})
	}
}
