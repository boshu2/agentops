package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// VerifyJudgments checks the consumer-selected required legs over independently
// supplied exact subject/acceptance. It preserves every supplied verdict and
// fails closed on unverified native identity, omissions or missing legs. It
// attests available runtime reporting, not provider weights or OS isolation.
func VerifyJudgments(o JudgmentOptions) (*JudgmentResult, error) {
	if err := checkJudgePolicy(o); err != nil {
		return nil, err
	} // before candidate/source retrieval
	root, err := judgmentRoot(o.EvidenceRoot)
	if err != nil {
		return nil, err
	}
	manifest, err := LoadManifest(o.Manifest)
	if err != nil {
		return nil, err
	}
	var base *Manifest
	if o.BaseManifest != "" {
		base, err = LoadManifest(o.BaseManifest)
		if err != nil {
			return nil, err
		}
	}
	if err = VerifyManifest(o.Root, manifest, base); err != nil {
		return nil, err
	}
	intent, err := os.ReadFile(o.Intent)
	if err != nil {
		return nil, err
	}
	result := &JudgmentResult{Satisfied: true, SubjectManifestDigest: manifest.Digest, AcceptanceDigest: Hash(intent), Legs: []JudgmentLeg{}, Problems: []string{}}
	state := judgmentState{options: o, root: root, result: result, seen: map[string]bool{}, contexts: map[string]string{}}
	for _, path := range o.Verdicts {
		state.inspectVerdict(path)
	}
	for _, profile := range o.Required {
		if !state.seen[profile.ID] {
			result.Legs = append(result.Legs, JudgmentLeg{ID: profile.ID, Problems: []string{"missing_required_leg"}})
			result.Satisfied = false
		}
	}
	return result, nil
}

func checkJudgePolicy(o JudgmentOptions) error {
	if len(o.Required) == 0 || !knownJudgeIdentity(o.AuthorContextID) {
		return fmt.Errorf("required profiles and independent author identity must be nonempty")
	}
	ids := map[string]bool{}
	for _, p := range o.Required {
		if !knownJudgeIdentity(p.ID) || !knownJudgeIdentity(p.Model) || ids[p.ID] {
			return fmt.Errorf("required profile IDs/models must be nonempty and IDs unique")
		}
		ids[p.ID] = true
		family := map[string]string{"codex": "openai", "claude": "anthropic"}[p.Runtime]
		if family == "" || p.Family != family {
			return fmt.Errorf("unsupported required runtime/family profile")
		}
		if !slices.Contains(o.AllowedProviders, p.Family) {
			return fmt.Errorf("provider_denied: required leg %s remains unsatisfied", p.ID)
		}
	}
	return nil
}

type judgmentState struct {
	options  JudgmentOptions
	root     string
	result   *JudgmentResult
	seen     map[string]bool
	contexts map[string]string
}

func (s *judgmentState) problem(problem string) {
	s.result.Problems = append(s.result.Problems, problem)
	s.result.Satisfied = false
}

func (s *judgmentState) inspectVerdict(path string) {
	b, err := privateRead(s.root, path)
	if err != nil {
		s.problem("verdict_read: " + err.Error())
		return
	}
	digest := strings.TrimSuffix(filepath.Base(path), ".json")
	if err = verdictcheck.VerifyArtifact(b, digest); err != nil {
		s.problem("verdict_integrity: " + err.Error())
		return
	}
	var verdict verdictcheck.Verdict
	if err = json.Unmarshal(b, &verdict); err != nil {
		s.problem("verdict_decode: " + err.Error())
		return
	}
	found := false
	for _, ref := range verdict.EvidenceRefs {
		if !strings.HasPrefix(ref, judgmentPrefix) {
			continue
		}
		found = true
		receipt, err := readJudgmentReceipt(s.root, ref)
		if err != nil {
			s.problem("receipt_integrity: " + err.Error())
			continue
		}
		s.inspectLeg(path, ref, &verdict, receipt)
	}
	if !found {
		s.problem("verdict_missing_judgment_receipt")
	}
}

func (s *judgmentState) inspectLeg(path, ref string, v *verdictcheck.Verdict, r *JudgmentReceipt) {
	leg := JudgmentLeg{ID: r.Requested.ID, VerdictPath: path, Verdict: v.Verdict, ReceiptRef: ref, Problems: []string{}}
	index := slices.IndexFunc(s.options.Required, func(p JudgeProfile) bool { return p.ID == r.Requested.ID })
	if index < 0 {
		s.problem("unexpected_judgment_leg: " + r.Requested.ID)
		return
	}
	if s.seen[leg.ID] {
		leg.Problems = append(leg.Problems, "duplicate_required_leg")
	}
	s.seen[leg.ID] = true
	profile := s.options.Required[index]
	if r.Requested != profile {
		leg.Problems = append(leg.Problems, "requested_profile_mismatch")
	}
	leg.Problems = append(leg.Problems, receiptBindingProblems(s.options, s.result, v, r)...)
	// Runtime/family identity is chosen by the independent profile. A receipt
	// cannot route parsing or retrieval to a substituted/disallowed provider.
	if r.Requested.Runtime == profile.Runtime && r.Requested.Family == profile.Family {
		s.inspectNative(&leg, profile, v, r)
	}
	leg.Satisfied = len(leg.Problems) == 0
	if !leg.Satisfied {
		s.result.Satisfied = false
	}
	s.result.Legs = append(s.result.Legs, leg)
}

func receiptBindingProblems(o JudgmentOptions, result *JudgmentResult, v *verdictcheck.Verdict, r *JudgmentReceipt) []string {
	problems := []string{}
	if r.SubjectManifestDigest != result.SubjectManifestDigest || v.SubjectManifestDigest != result.SubjectManifestDigest {
		problems = append(problems, "subject_mismatch")
	}
	if r.AcceptanceDigest != result.AcceptanceDigest || v.AcceptanceDigest != result.AcceptanceDigest {
		problems = append(problems, "acceptance_mismatch")
	}
	if r.AuthorContextID != o.AuthorContextID || v.AuthorContextID == nil || *v.AuthorContextID != o.AuthorContextID {
		problems = append(problems, "author_context_mismatch")
	}
	if v.FreshnessAttestation == nil {
		problems = append(problems, "freshness_unverified")
	}
	if v.Verdict != "PASS" {
		problems = append(problems, "verdict_not_pass")
	}
	if len(r.Omissions) > 0 {
		problems = append(problems, "omitted_evidence")
	}
	if !runtimeComplete(r) {
		problems = append(problems, "incomplete_runtime")
	}
	return problems
}

func runtimeComplete(r *JudgmentReceipt) bool {
	return r.ExitCode != nil && *r.ExitCode == 0 && r.TimedOut != nil && !*r.TimedOut && r.Truncated != nil && !*r.Truncated && r.CleanupVerified != nil && *r.CleanupVerified
}

func (s *judgmentState) inspectNative(leg *JudgmentLeg, p JudgeProfile, v *verdictcheck.Verdict, r *JudgmentReceipt) {
	b, err := readNativeSpan(s.root, r.Transcript)
	if err != nil {
		leg.Problems = append(leg.Problems, err.Error())
		return
	}
	native, err := parser.NewParser().ParseRuntime(b, p.Runtime)
	if err != nil {
		leg.Problems = append(leg.Problems, "native_parse: "+err.Error())
		return
	}
	// Parser offsets are relative to its input; reports use absolute file spans.
	for i := range native.Spans {
		native.Spans[i].Start += r.Transcript.Start
		native.Spans[i].End += r.Transcript.Start
	}
	leg.Native = native
	leg.Problems = append(leg.Problems, nativeProblems(p, s.options.AuthorContextID, v, native)...)
	if native.ContextID != "" {
		if previous, ok := s.contexts[native.ContextID]; ok {
			leg.Problems = append(leg.Problems, "reused_peer_context: "+previous)
		}
		s.contexts[native.ContextID] = leg.ID
	}
}

func nativeProblems(p JudgeProfile, author string, v *verdictcheck.Verdict, n *parser.RuntimeMetadata) []string {
	problems := []string{}
	if !knownJudgeIdentity(n.Model) || !knownJudgeIdentity(n.ContextID) || !knownJudgeIdentity(n.Provider) {
		problems = append(problems, "identity_unverified")
	}
	if n.Model != "" && n.Model != p.Model {
		problems = append(problems, "runtime_model_mismatch")
	}
	if n.Provider != "" && n.Provider != p.Family {
		problems = append(problems, "runtime_family_mismatch")
	}
	if p.Effort != "" && n.Effort != p.Effort {
		problems = append(problems, "runtime_effort_mismatch")
	}
	if n.ContextID == author {
		problems = append(problems, "reused_author_context")
	}
	if v.ValidatorContextID == nil || *v.ValidatorContextID != n.ContextID {
		problems = append(problems, "validator_context_mismatch")
	}
	if !n.Completed {
		problems = append(problems, "native_termination_unverified")
	}
	return problems
}

func knownJudgeIdentity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown", "unavailable", "unverified", "identity_unverified", "null", "default":
		return false
	default:
		return value == strings.TrimSpace(value)
	}
}
