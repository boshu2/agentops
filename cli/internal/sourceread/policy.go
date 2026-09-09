// Package sourceread reads explicit, authorized byte spans without interpreting
// transcript records or claiming host delivery or semantic processing.
package sourceread

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	configadapter "github.com/boshu2/agentops/cli/internal/adapters/config"
	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// OutputProfile is selected through independently supplied T05 policy, never
// through a source record. Its receipt is a caller observation, not OS isolation
// or evidence of this particular command's delivery to a model.
type OutputProfile struct {
	ID                 string `json:"id"`
	ModelRef           string `json:"model_ref"`
	DestinationRef     string `json:"destination_ref"`
	MaxSerializedBytes int64  `json:"max_serialized_bytes"`
	ObservationRef     string `json:"observation_ref"`
	ObservationSHA256  string `json:"observation_sha256"`
}

type SourcePermission struct {
	Path         string `json:"path"`
	ContentScope string `json:"content_scope"`
}

// Policy is the source-read-policy.v1 document at the resolved TaskPolicyRef.
// The T05 access policy and native maintenance fact select this document.
type Policy struct {
	SchemaVersion  string             `json:"schema_version"`
	SourceID       string             `json:"source_id"`
	ProjectID      string             `json:"project_id"`
	OwnerScope     string             `json:"owner_scope"`
	TaskRef        string             `json:"task_ref"`
	ModelRef       string             `json:"model_ref"`
	DestinationRef string             `json:"destination_ref"`
	Files          []SourcePermission `json:"files"`
	OutputProfile  OutputProfile      `json:"output_profile"`
}

type authorization struct {
	route        config.ContextResult
	policy       Policy
	policyDigest string
	permission   SourcePermission
}

// Service's only public dependency is the existing native configuration port.
// Zero-value Service uses the production native BD gateway. Private operation
// seams let tests observe source-open ordering and races without global hooks.
type Service struct {
	Gateway    config.ContextGateway
	openSource func(string) (*os.File, error)
	afterRead  func()
}

func (s Service) authorize(ctx context.Context, o Options) (authorization, error) {
	var a authorization
	if strings.TrimSpace(o.Context.Overrides.AccessPolicyRef) == "" {
		return a, fmt.Errorf("read-source denied: explicit access policy is required")
	}
	gateway := s.Gateway
	if gateway == nil {
		gateway = configadapter.Gateway{}
	}
	result, err := config.ResolveContext(ctx, gateway, o.Context)
	if err != nil {
		return a, fmt.Errorf("read-source denied: context authorization failed")
	}
	data, err := readPolicyFile(result.Route.TaskPolicyRef)
	if err != nil {
		return a, fmt.Errorf("read-source denied: source policy unavailable")
	}
	var p Policy
	if err = strictJSON(data, &p); err != nil {
		return a, fmt.Errorf("read-source denied: malformed source policy")
	}
	if !policyMatches(p, o.Context) {
		return a, fmt.Errorf("read-source denied: source policy identity mismatch")
	}
	permission, err := permittedFile(p.Files, o.File)
	if err != nil {
		return a, err
	}
	if err := checkProfile(p.OutputProfile, o.Context); err != nil {
		return a, err
	}
	// Validate the selected file's canonical location before opening bytes.
	canonical, err := filepath.EvalSymlinks(o.File)
	if err != nil || canonical != o.File {
		return a, fmt.Errorf("read-source denied: source canonical identity unavailable or changed")
	}
	return authorization{route: result, policy: p, policyDigest: digest(data), permission: permission}, nil
}

func policyMatches(p Policy, r config.ContextRequest) bool {
	return p.SchemaVersion == "source-read-policy.v1" && p.SourceID == r.SourceID && p.ProjectID == r.ProjectID && p.OwnerScope == r.OwnerScope && p.TaskRef == r.TaskRef && p.ModelRef == r.ModelRef && p.DestinationRef == r.DestinationRef
}

func permittedFile(files []SourcePermission, path string) (SourcePermission, error) {
	var permission SourcePermission
	// Compare the caller's exact lexical path before touching its metadata.
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return permission, fmt.Errorf("read-source denied: file must be an exact absolute path")
	}
	seen := map[string]bool{}
	for _, f := range files {
		if !filepath.IsAbs(f.Path) || filepath.Clean(f.Path) != f.Path || seen[f.Path] {
			return permission, fmt.Errorf("read-source denied: invalid source allowlist")
		}
		seen[f.Path] = true
		if f.ContentScope != "synthetic" && f.ContentScope != "already-cleared" && f.ContentScope != "restricted" {
			return permission, fmt.Errorf("read-source denied: unsupported content scope")
		}
		if f.Path == path {
			permission = f
		}
	}
	if permission.Path == "" {
		return permission, fmt.Errorf("read-source denied: file is not authorized")
	}
	// T05 only selects policy today. No supplied string can assert the missing
	// native T39 enforcement. Restricted processing remains unavailable.
	if permission.ContentScope == "restricted" {
		return permission, fmt.Errorf("read-source denied: restricted-source native enforcement is unavailable")
	}
	return permission, nil
}

func checkProfile(profile OutputProfile, r config.ContextRequest) error {
	if strings.TrimSpace(profile.ID) == "" || profile.ModelRef != r.ModelRef || profile.DestinationRef != r.DestinationRef || profile.MaxSerializedBytes <= 0 || !verdictcheck.ValidDigest(profile.ObservationSHA256) {
		return fmt.Errorf("read-source denied: measured output profile is missing or mismatched")
	}
	observation, err := readPolicyFile(profile.ObservationRef)
	if err != nil || digest(observation) != profile.ObservationSHA256 {
		return fmt.Errorf("read-source denied: output profile observation integrity failed")
	}
	return nil
}

func strictJSON(data []byte, v any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("policy is not UTF-8")
	}
	if _, err := verdictcheck.DecodeObject(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

func readPolicyFile(path string) (data []byte, err error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("absolute policy path required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("regular policy file required")
	}
	// A parser resource bound, not a measured host-output profile.
	data, err = io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("policy exceeds parser resource bound")
	}
	return data, nil
}
func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
