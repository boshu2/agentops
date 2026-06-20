// Package statememory owns admission and verification for durable AgentOps
// state findings.
package statememory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	FindingKind   = "state_finding"
	AdmissionKind = "state_admission"

	findingSchemaRel   = "schemas/state-finding.v1.schema.json"
	admissionSchemaRel = "schemas/state-admission.v1.schema.json"
	stateFindingDirRel = ".agents/state/findings"
	defaultMaxAge      = 30 * 24 * time.Hour
)

// Finding mirrors schemas/state-finding.v1.schema.json.
type Finding struct {
	SchemaVersion int           `json:"schema_version"`
	Kind          string        `json:"kind"`
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Status        string        `json:"status"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
	Source        FindingSource `json:"source"`
	Review        FindingReview `json:"review"`
	Severity      string        `json:"severity"`
	Category      string        `json:"category"`
	Summary       string        `json:"summary"`
	Evidence      []string      `json:"evidence"`
	Body          string        `json:"body"`
}

type FindingSource struct {
	BeadID   string `json:"bead_id"`
	RunID    string `json:"run_id"`
	AuthorID string `json:"author_id"`
	Repo     string `json:"repo"`
	Path     string `json:"path"`
}

type FindingReview struct {
	ReviewerID string `json:"reviewer_id"`
	ReviewedAt string `json:"reviewed_at"`
	Verdict    string `json:"verdict"`
	HeadSHA    string `json:"head_sha"`
}

// AdmissionRequest mirrors schemas/state-admission.v1.schema.json.
type AdmissionRequest struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	CandidatePath string `json:"candidate_path"`
	Destination   string `json:"destination"`
	OperatorID    string `json:"operator_id"`
	Reason        string `json:"reason"`
}

type AdmissionOptions struct {
	Root   string
	Now    time.Time
	MaxAge time.Duration
	Write  bool
}

type AdmissionReport struct {
	Verdict     string   `json:"verdict"`
	FindingID   string   `json:"finding_id"`
	Destination string   `json:"destination"`
	Reasons     []string `json:"reasons"`
	Wrote       bool     `json:"wrote"`
}

type VerifyReport struct {
	Verdict             string   `json:"verdict"`
	Schemas             int      `json:"schemas"`
	GoodFixtures        int      `json:"good_fixtures"`
	BadFixturesRejected int      `json:"bad_fixtures_rejected"`
	StateFindings       int      `json:"state_findings"`
	Failures            []string `json:"failures"`
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)sk-[a-z0-9]{12,}`),
	regexp.MustCompile(`(?i)aws_secret_access_key`),
	regexp.MustCompile(`(?i)(password|token|secret)\s*[:=]\s*["']?[^"'\s]{8,}`),
	regexp.MustCompile(`-----BEGIN (?:OPENSSH |RSA |EC |DSA )?PRIVATE KEY-----`),
}

// CompileSchemaFile compiles a repo JSON schema from disk.
func CompileSchemaFile(path string) (*jsonschema.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", path, err)
	}
	compiler := jsonschema.NewCompiler()
	resource := filepath.ToSlash(path)
	if err := compiler.AddResource(resource, doc); err != nil {
		return nil, fmt.Errorf("add schema resource %s: %w", path, err)
	}
	schema, err := compiler.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", path, err)
	}
	return schema, nil
}

// ValidateJSON validates raw JSON bytes against a compiled schema.
func ValidateJSON(schema *jsonschema.Schema, data []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	if err := schema.Validate(inst); err != nil {
		return err
	}
	return nil
}

// ValidateStateFile validates a state JSON file by its kind field.
func ValidateStateFile(root, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	var schemaPath string
	switch header.Kind {
	case FindingKind:
		schemaPath = filepath.Join(root, filepath.FromSlash(findingSchemaRel))
	case AdmissionKind:
		schemaPath = filepath.Join(root, filepath.FromSlash(admissionSchemaRel))
	default:
		return fmt.Errorf("%s: unknown state kind %q", path, header.Kind)
	}
	schema, err := CompileSchemaFile(schemaPath)
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("%s violates %s: %w", path, filepath.Base(schemaPath), err)
	}
	return nil
}

// AdmitFinding validates and optionally writes a finding into .agents/state.
func AdmitFinding(ctx context.Context, findingBytes []byte, req AdmissionRequest, opts AdmissionOptions) (AdmissionReport, error) {
	if err := ctx.Err(); err != nil {
		return AdmissionReport{}, err
	}
	root, now, maxAge := normalizeOptions(opts)
	if req.Destination != "" {
		if _, _, err := resolveDestination(root, req.Destination, "placeholder"); err != nil {
			return AdmissionReport{}, err
		}
	}
	if err := validateFindingBytes(root, findingBytes); err != nil {
		return AdmissionReport{}, err
	}

	var finding Finding
	if err := json.Unmarshal(findingBytes, &finding); err != nil {
		return AdmissionReport{}, fmt.Errorf("decode finding: %w", err)
	}
	if strings.TrimSpace(req.Destination) == "" {
		req.Destination = defaultDestinationForFinding(finding.ID)
	}
	if err := validateAdmissionRequest(root, req); err != nil {
		return AdmissionReport{}, err
	}
	if err := checkFindingAdmission(findingBytes, finding, now, maxAge); err != nil {
		return AdmissionReport{}, err
	}

	destRel, destAbs, err := resolveDestination(root, req.Destination, finding.ID)
	if err != nil {
		return AdmissionReport{}, err
	}
	report := AdmissionReport{
		Verdict:     "ADMITTED",
		FindingID:   finding.ID,
		Destination: destRel,
		Reasons: []string{
			"schema valid",
			"non-author review",
			"fresh enough",
			"leak scan clean",
			"path confined",
		},
	}
	if opts.Write {
		if err := writeFileAtomic(destAbs, findingBytes, 0o644); err != nil {
			return AdmissionReport{}, fmt.Errorf("write admitted finding: %w", err)
		}
		report.Wrote = true
	}
	return report, nil
}

func defaultDestinationForFinding(findingID string) string {
	return filepath.ToSlash(filepath.Join(stateFindingDirRel, findingID+".json"))
}

func normalizeOptions(opts AdmissionOptions) (string, time.Time, time.Duration) {
	root := opts.Root
	if root == "" {
		root, _ = os.Getwd()
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = defaultMaxAge
	}
	return root, now, maxAge
}

func validateAdmissionRequest(root string, req AdmissionRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal admission request: %w", err)
	}
	schema, err := CompileSchemaFile(filepath.Join(root, filepath.FromSlash(admissionSchemaRel)))
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("admission request violates %s: %w", admissionSchemaRel, err)
	}
	return nil
}

func validateFindingBytes(root string, data []byte) error {
	schema, err := CompileSchemaFile(filepath.Join(root, filepath.FromSlash(findingSchemaRel)))
	if err != nil {
		return err
	}
	if err := ValidateJSON(schema, data); err != nil {
		return fmt.Errorf("finding violates %s: %w", findingSchemaRel, err)
	}
	return nil
}

func checkFindingAdmission(raw []byte, finding Finding, now time.Time, maxAge time.Duration) error {
	if finding.Kind != FindingKind {
		return fmt.Errorf("finding kind must be %q", FindingKind)
	}
	if normalizeID(finding.Source.AuthorID) == normalizeID(finding.Review.ReviewerID) {
		return fmt.Errorf("self-review rejected: author_id and reviewer_id are both %q", finding.Source.AuthorID)
	}
	if finding.Review.Verdict != "CONFIRMED" {
		return fmt.Errorf("review verdict must be CONFIRMED, got %q", finding.Review.Verdict)
	}
	if err := rejectStale(finding, now, maxAge); err != nil {
		return err
	}
	if err := rejectLeaks(raw); err != nil {
		return err
	}
	return nil
}

func normalizeID(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func rejectStale(f Finding, now time.Time, maxAge time.Duration) error {
	updated, err := time.Parse(time.RFC3339, f.UpdatedAt)
	if err != nil {
		return fmt.Errorf("updated_at invalid: %w", err)
	}
	reviewed, err := time.Parse(time.RFC3339, f.Review.ReviewedAt)
	if err != nil {
		return fmt.Errorf("reviewed_at invalid: %w", err)
	}
	latest := updated
	if reviewed.After(latest) {
		latest = reviewed
	}
	if now.Sub(latest) > maxAge {
		return fmt.Errorf("stale finding rejected: latest timestamp %s is older than %s", latest.Format(time.RFC3339), maxAge)
	}
	if latest.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("stale finding rejected: latest timestamp %s is in the future", latest.Format(time.RFC3339))
	}
	return nil
}

func rejectLeaks(raw []byte) error {
	text := string(raw)
	for _, pattern := range secretPatterns {
		if pattern.MatchString(text) {
			return fmt.Errorf("leak detected by state admission secret scan")
		}
	}
	return nil
}

func resolveDestination(root, dest, findingID string) (string, string, error) {
	if strings.TrimSpace(dest) == "" {
		dest = filepath.ToSlash(filepath.Join(stateFindingDirRel, findingID+".json"))
	}
	if filepath.IsAbs(dest) {
		return "", "", fmt.Errorf("destination path escapes state directory: absolute path %q", dest)
	}
	clean := filepath.ToSlash(filepath.Clean(dest))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", "", fmt.Errorf("destination path escapes state directory: %q", dest)
	}
	requiredPrefix := stateFindingDirRel + "/"
	if !strings.HasPrefix(clean, requiredPrefix) || strings.Contains(clean, "/../") {
		return "", "", fmt.Errorf("destination path escapes state directory: %q", dest)
	}
	if filepath.Base(clean) == "" || !strings.HasSuffix(clean, ".json") {
		return "", "", fmt.Errorf("destination must be a .json file under %s", stateFindingDirRel)
	}
	abs := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", fmt.Errorf("resolve destination: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("destination path escapes repo root: %q", dest)
	}
	return clean, abs, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-state-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// VerifyRepo validates the state schemas, fixtures, and any local admitted
// state findings.
func VerifyRepo(ctx context.Context, root string) (VerifyReport, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return VerifyReport{}, err
		}
	}
	report := VerifyReport{Verdict: "PASS"}
	addFailure := func(format string, args ...any) {
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
		report.Verdict = "FAIL"
	}

	findingSchema, err := CompileSchemaFile(filepath.Join(root, filepath.FromSlash(findingSchemaRel)))
	if err != nil {
		addFailure("%v", err)
	} else {
		report.Schemas++
	}
	admissionSchema, err := CompileSchemaFile(filepath.Join(root, filepath.FromSlash(admissionSchemaRel)))
	if err != nil {
		addFailure("%v", err)
	} else {
		report.Schemas++
	}

	validateKnownFixtures(root, findingSchema, admissionSchema, &report, addFailure)
	validateStateStore(ctx, root, &report, addFailure)
	sort.Strings(report.Failures)
	return report, nil
}

func validateKnownFixtures(root string, findingSchema, admissionSchema *jsonschema.Schema, report *VerifyReport, addFailure func(string, ...any)) {
	fixtures := filepath.Join(root, "schemas", "fixtures", "state-memory")
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		addFailure("read state-memory fixtures: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()
		path := filepath.Join(fixtures, name)
		data, err := os.ReadFile(path)
		if err != nil {
			addFailure("read fixture %s: %v", name, err)
			continue
		}
		schema := schemaForFixture(name, findingSchema, admissionSchema)
		if schema == nil {
			addFailure("fixture %s has no schema mapping", name)
			continue
		}
		err = ValidateJSON(schema, data)
		if strings.HasPrefix(name, "bad-") {
			if err == nil {
				addFailure("bad fixture %s was accepted", name)
				continue
			}
			report.BadFixturesRejected++
			continue
		}
		if err != nil {
			addFailure("valid fixture %s rejected: %v", name, err)
			continue
		}
		report.GoodFixtures++
	}
}

func schemaForFixture(name string, findingSchema, admissionSchema *jsonschema.Schema) *jsonschema.Schema {
	if strings.Contains(name, "admission") {
		return admissionSchema
	}
	return findingSchema
}

func validateStateStore(ctx context.Context, root string, report *VerifyReport, addFailure func(string, ...any)) {
	stateDir := filepath.Join(root, filepath.FromSlash(stateFindingDirRel))
	entries, err := os.ReadDir(stateDir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		addFailure("read state store: %v", err)
		return
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			addFailure("state verify canceled: %v", err)
			return
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(stateDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			addFailure("read state finding %s: %v", entry.Name(), err)
			continue
		}
		if err := ValidateStateFile(root, path); err != nil {
			addFailure("%v", err)
			continue
		}
		var finding Finding
		if err := json.Unmarshal(data, &finding); err != nil {
			addFailure("decode state finding %s: %v", entry.Name(), err)
			continue
		}
		if err := checkFindingAdmission(data, finding, time.Now().UTC(), defaultMaxAge); err != nil {
			addFailure("state finding %s failed admission invariant: %v", entry.Name(), err)
			continue
		}
		report.StateFindings++
	}
}
