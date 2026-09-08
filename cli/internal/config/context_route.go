package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/evidencepath"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// ContextRequest supplies the native identity and invocation purpose separately
// from mutable route configuration. SourceID is the canonical native beads_dir.
type ContextRequest struct {
	Overrides                         ContextConfig
	SourceID, ProjectID, OwnerScope   string
	TaskRef, ModelRef, DestinationRef string
	ConsumerRoot, NativeDirectory     string
	Recover                           bool
}

// ContextPolicy selects an exact scope. It is not native access enforcement or
// semantic clearance, and is never supplied by a candidate knowledge page.
type ContextPolicy struct {
	MaintenanceWorkRef   string `json:"maintenance_work_ref"`
	SchemaVersion        int    `json:"schema_version"`
	SourceID             string `json:"source_id"`
	ProjectID            string `json:"project_id"`
	OwnerScope           string `json:"owner_scope"`
	TaskRef              string `json:"task_ref"`
	ModelRef             string `json:"model_ref"`
	DestinationRef       string `json:"destination_ref"`
	BundleID             string `json:"bundle_id"`
	BundleRoot           string `json:"bundle_root"`
	EvidenceRoot         string `json:"evidence_root"`
	StagingRoot          string `json:"staging_root"`
	OwnerPolicyRef       string `json:"owner_policy_ref"`
	TaskPolicyRef        string `json:"task_policy_ref"`
	ModelPolicyRef       string `json:"model_policy_ref"`
	DestinationPolicyRef string `json:"destination_policy_ref"`
}

type NativeContext struct {
	BDVersion     string `json:"bd_version"`
	SchemaVersion int    `json:"schema_version"`
	Backend       string `json:"backend"`
	ProjectID     string `json:"project_id"`
	BeadsDir      string `json:"beads_dir"`
}

type AnchorComment struct {
	ID      string `json:"id"`
	IssueID string `json:"issue_id"`
	Text    string `json:"text"`
}

type ContextGateway interface {
	NativeContext(context.Context, string) (NativeContext, error)
	AnchorComments(context.Context, string, string) ([]AnchorComment, error)
}

type ContextResult struct {
	Route             ContextConfig     `json:"context"`
	Sources           map[string]Source `json:"sources"`
	Recovered         bool              `json:"recovered"`
	CommentsRead      int               `json:"comments_read"`
	Facts             []ContextFact     `json:"facts"`
	AccessEnforcement string            `json:"access_enforcement"`
}

// ResolveContext resolves configuration and verifies the exact native anchor.
// It creates no directory, config, bundle, index, object, anchor or work item.
func ResolveContext(ctx context.Context, gateway ContextGateway, req ContextRequest) (ContextResult, error) {
	result := ContextResult{AccessEnforcement: "not_attested"}
	for name, value := range map[string]string{"source_id": req.SourceID, "project_id": req.ProjectID, "owner_scope": req.OwnerScope, "task_ref": req.TaskRef, "model_ref": req.ModelRef, "destination_ref": req.DestinationRef, "consumer_root": req.ConsumerRoot, "native_directory": req.NativeDirectory} {
		if strings.TrimSpace(value) == "" {
			return result, fmt.Errorf("explicit context %s is required", name)
		}
	}
	route, sources, err := LoadContext(req.Overrides)
	if err != nil {
		return result, err
	}
	policy, err := loadContextPolicy(route, req)
	if err != nil {
		return result, err
	}
	facts, count, err := readContextAnchor(ctx, gateway, req, route.MaintenanceWorkRef)
	if err != nil {
		return result, err
	}
	recorded, err := recordedContextRoute(facts)
	if err != nil {
		return result, err
	}
	if req.Recover {
		for key, value := range route.fields() {
			if *value != "" && *value != *recorded.fields()[key] {
				return result, fmt.Errorf("recovery conflicts with selected context.%s", key)
			}
		}
		route = *recorded
		result.Recovered = true
		for key := range route.fields() {
			if _, ok := sources[key]; !ok {
				sources[key] = Source("maintenance-anchor")
			}
		}
	}
	if route != *recorded {
		return result, fmt.Errorf("selected route conflicts with maintenance recovery fact")
	}
	if err = validateRoute(&route, policy, req); err != nil {
		return result, err
	}
	for _, fact := range facts {
		if fact.BundleID != "" && fact.BundleID != route.BundleID {
			return result, fmt.Errorf("anchor fact belongs to a different bundle")
		}
	}
	result.Route = route
	result.Sources = sources
	result.CommentsRead = count
	result.Facts = facts
	return result, nil
}

func policyMatchesRequest(p ContextPolicy, r ContextRequest) error {
	if p.SchemaVersion != 1 || p.SourceID != r.SourceID || p.ProjectID != r.ProjectID || p.OwnerScope != r.OwnerScope || p.TaskRef != r.TaskRef || p.ModelRef != r.ModelRef || p.DestinationRef != r.DestinationRef {
		return fmt.Errorf("context policy denies source/owner/task/model/destination or is unsupported")
	}
	return nil
}
func validateRoute(r *ContextConfig, p ContextPolicy, req ContextRequest) error {
	for key, value := range r.fields() {
		if strings.TrimSpace(*value) == "" {
			return fmt.Errorf("context.%s is required", key)
		}
	}
	if r.SourceID != req.SourceID || r.ProjectID != req.ProjectID || r.OwnerScope != req.OwnerScope || r.BundleID != p.BundleID {
		return fmt.Errorf("context route identity conflicts with selected policy")
	}
	consumer, err := canonical(req.ConsumerRoot, true)
	if err != nil {
		return err
	}
	for _, pair := range []struct {
		value     *string
		allowed   string
		directory bool
	}{
		{&r.BundleRoot, p.BundleRoot, true}, {&r.EvidenceRoot, p.EvidenceRoot, true}, {&r.StagingRoot, p.StagingRoot, true},
		{&r.OwnerPolicyRef, p.OwnerPolicyRef, false}, {&r.TaskPolicyRef, p.TaskPolicyRef, false}, {&r.ModelPolicyRef, p.ModelPolicyRef, false}, {&r.DestinationPolicyRef, p.DestinationPolicyRef, false},
	} {
		actual, err := canonical(*pair.value, pair.directory)
		if err != nil {
			return err
		}
		allowed, err := canonical(pair.allowed, pair.directory)
		if err != nil {
			return err
		}
		if filepath.Clean(pair.allowed) != allowed {
			return fmt.Errorf("policy path must name its canonical destination, not an alias")
		}
		if actual != allowed {
			return fmt.Errorf("context path escapes selected policy")
		}
		*pair.value = actual
	}
	if overlap(r.BundleRoot, consumer) {
		return fmt.Errorf("context bundle overlaps consumer checkout")
	}
	for _, root := range []string{r.StagingRoot, r.EvidenceRoot} {
		if _, err := evidencepath.Validate(root, r.BundleRoot, consumer); err != nil {
			return err
		}
	}
	if overlap(r.StagingRoot, r.EvidenceRoot) {
		return fmt.Errorf("context staging and evidence roots overlap")
	}
	return nil
}
func canonical(path string, directory bool) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("context path must be absolute")
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if directory && !info.IsDir() || !directory && !info.Mode().IsRegular() {
		return "", fmt.Errorf("context path has wrong type")
	}
	return real, nil
}
func overlap(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return true
	}
	bi, err := os.Stat(b)
	if err != nil {
		return true
	}
	for _, pair := range []struct {
		path   string
		target os.FileInfo
	}{{a, bi}, {b, ai}} {
		for dir := pair.path; ; dir = filepath.Dir(dir) {
			info, err := os.Stat(dir)
			if err != nil {
				return true
			}
			if os.SameFile(info, pair.target) {
				return true
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return false
}
func readContextFile(path string) (data []byte, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("context policy must be a regular file")
	}
	data, err = io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("context policy exceeds 1 MiB")
	}
	return data, nil
}

// Every supplied locator is checked before private comments are opened; recovery
// may fill only absent locators and never override a conflicting owner.
func preflightRoute(route ContextConfig, p ContextPolicy, req ContextRequest) error {
	expected := ContextConfig{SourceID: req.SourceID, ProjectID: req.ProjectID, OwnerScope: req.OwnerScope, BundleID: p.BundleID, BundleRoot: p.BundleRoot, EvidenceRoot: p.EvidenceRoot, StagingRoot: p.StagingRoot, OwnerPolicyRef: p.OwnerPolicyRef, TaskPolicyRef: p.TaskPolicyRef, ModelPolicyRef: p.ModelPolicyRef, DestinationPolicyRef: p.DestinationPolicyRef}
	for key, field := range expected.fields() {
		if *field != "" && *route.fields()[key] != "" && *field != *route.fields()[key] {
			return fmt.Errorf("selected context.%s conflicts with policy", key)
		}
	}
	expected.AccessPolicyRef = route.AccessPolicyRef
	expected.MaintenanceWorkRef = route.MaintenanceWorkRef
	return validateRoute(&expected, p, req)
}

func loadContextPolicy(route ContextConfig, req ContextRequest) (ContextPolicy, error) {
	var policy ContextPolicy
	// Access policy and anchor must remain independently selected even in recovery.
	if route.AccessPolicyRef == "" || route.MaintenanceWorkRef == "" {
		return policy, fmt.Errorf("context access_policy_ref and maintenance_work_ref are required")
	}
	if strings.HasPrefix(route.MaintenanceWorkRef, "-") || strings.ContainsAny(route.MaintenanceWorkRef, " \t\r\n") {
		return policy, fmt.Errorf("invalid maintenance_work_ref")
	}
	policyPath, err := canonical(route.AccessPolicyRef, false)
	if err != nil {
		return policy, fmt.Errorf("access policy: %w", err)
	}
	data, err := readContextFile(policyPath)
	if err != nil {
		return policy, err
	}
	if err = decodeContextObject(data, &policy, []string{"schema_version", "source_id", "project_id", "owner_scope", "task_ref", "model_ref", "destination_ref", "bundle_id", "bundle_root", "evidence_root", "staging_root", "owner_policy_ref", "task_policy_ref", "model_policy_ref", "destination_policy_ref", "maintenance_work_ref"}, nil); err != nil {
		return policy, fmt.Errorf("invalid context policy: %w", err)
	}
	if err = policyMatchesRequest(policy, req); err != nil {
		return policy, err
	}
	if policy.MaintenanceWorkRef != route.MaintenanceWorkRef {
		return policy, fmt.Errorf("context policy denies maintenance anchor")
	}
	if err = preflightRoute(route, policy, req); err != nil {
		return policy, err
	}
	return policy, nil
}
func readContextAnchor(ctx context.Context, gateway ContextGateway, req ContextRequest, anchor string) ([]ContextFact, int, error) {
	// Policy is selected and purpose-bound before any private native comment read.
	native, err := gateway.NativeContext(ctx, req.NativeDirectory)
	if err != nil {
		return nil, 0, fmt.Errorf("native context unavailable: %w", err)
	}
	source, err := canonical(req.SourceID, true)
	if err != nil {
		return nil, 0, err
	}
	nativeSource, err := canonical(native.BeadsDir, true)
	if err != nil {
		return nil, 0, err
	}
	if native.BDVersion != "1.2.2" || native.SchemaVersion != 1 || native.Backend != "dolt" || native.ProjectID != req.ProjectID || nativeSource != source {
		return nil, 0, fmt.Errorf("native project/source identity mismatch or unsupported context")
	}
	comments, err := gateway.AnchorComments(ctx, req.NativeDirectory, anchor)
	if err != nil {
		return nil, 0, fmt.Errorf("maintenance anchor unavailable: %w", err)
	}
	facts, err := ParseContextFacts(comments, anchor)
	if err != nil {
		return nil, 0, err
	}
	return facts, len(comments), nil
}
func recordedContextRoute(facts []ContextFact) (*ContextConfig, error) {
	var recorded *ContextConfig
	for _, fact := range facts {
		if fact.Type == "context.route.v1" {
			candidate := *fact.Route
			if recorded != nil && *recorded != candidate {
				return nil, fmt.Errorf("conflicting maintenance route facts")
			}
			recorded = &candidate
		}
	}
	if recorded == nil {
		return nil, fmt.Errorf("maintenance anchor has no context.route.v1 recovery fact")
	}
	return recorded, nil
}

func decodeContextObject(data []byte, value any, required, optional []string) error {
	raw, err := verdictcheck.DecodeObject(data)
	if err != nil {
		return err
	}
	if err = verdictcheck.ExactFields(raw, required, optional); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}
