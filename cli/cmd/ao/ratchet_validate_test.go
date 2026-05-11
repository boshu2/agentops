// practices: [dora-metrics, continuous-delivery]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetValidate_UnknownStep(t *testing.T) {
	step := ratchet.ParseStep("nonsense")
	if step != "" {
		t.Errorf("ParseStep(nonsense) = %q, want empty", step)
	}
}

func TestBuildValidateOptions_Default(t *testing.T) {
	origLenient := ratchetLenient
	origDays := ratchetLenientDays
	ratchetLenient = false
	ratchetLenientDays = 90
	defer func() {
		ratchetLenient = origLenient
		ratchetLenientDays = origDays
	}()

	opts := buildValidateOptions()

	if opts.Lenient {
		t.Error("default should be strict (Lenient=false)")
	}
	if opts.LenientExpiryDate != nil {
		t.Error("non-lenient should have nil expiry date")
	}
}

func TestBuildValidateOptions_Lenient(t *testing.T) {
	origLenient := ratchetLenient
	origDays := ratchetLenientDays
	ratchetLenient = true
	ratchetLenientDays = 90
	defer func() {
		ratchetLenient = origLenient
		ratchetLenientDays = origDays
	}()

	opts := buildValidateOptions()

	if !opts.Lenient {
		t.Error("expected Lenient=true")
	}
	if opts.LenientExpiryDate == nil {
		t.Fatal("expected non-nil expiry date in lenient mode")
	}

	// Expiry should be ~90 days from now
	expected := time.Now().AddDate(0, 0, 90)
	diff := opts.LenientExpiryDate.Sub(expected)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("expiry date off by %v", diff)
	}
}

func TestBuildValidateOptions_LenientCustomDays(t *testing.T) {
	origLenient := ratchetLenient
	origDays := ratchetLenientDays
	ratchetLenient = true
	ratchetLenientDays = 180
	defer func() {
		ratchetLenient = origLenient
		ratchetLenientDays = origDays
	}()

	opts := buildValidateOptions()

	expected := time.Now().AddDate(0, 0, 180)
	diff := opts.LenientExpiryDate.Sub(expected)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("custom expiry off by %v", diff)
	}
}

func TestBuildValidateOptions_LenientZeroDays(t *testing.T) {
	origLenient := ratchetLenient
	origDays := ratchetLenientDays
	ratchetLenient = true
	ratchetLenientDays = 0
	defer func() {
		ratchetLenient = origLenient
		ratchetLenientDays = origDays
	}()

	opts := buildValidateOptions()

	if opts.LenientExpiryDate != nil {
		t.Error("zero days should produce nil expiry date")
	}
}

func TestResolveValidationFiles_ExplicitChanges(t *testing.T) {
	origFiles := ratchetFiles
	ratchetFiles = []string{"file1.md", "file2.md"}
	defer func() { ratchetFiles = origFiles }()

	files := resolveValidationFiles(t.TempDir(), ratchet.StepResearch)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != "file1.md" || files[1] != "file2.md" {
		t.Errorf("files = %v", files)
	}
}

func TestResolveValidationFiles_NoChangesNoOutput(t *testing.T) {
	origFiles := ratchetFiles
	ratchetFiles = nil
	defer func() { ratchetFiles = origFiles }()

	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	files := resolveValidationFiles(tmp, ratchet.StepResearch)

	// With no --changes and no matching output file, should return nil
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d: %v", len(files), files)
	}
}

func TestFormatValidationStatus_Valid(t *testing.T) {
	var buf bytes.Buffer
	result := &ratchet.ValidationResult{Valid: true}
	allValid := true

	formatValidationStatus(&buf, result, &allValid)

	if !strings.Contains(buf.String(), "VALID") {
		t.Errorf("output missing VALID\nGot: %s", buf.String())
	}
	if !allValid {
		t.Error("allValid should still be true")
	}
}

func TestFormatValidationStatus_Invalid(t *testing.T) {
	var buf bytes.Buffer
	result := &ratchet.ValidationResult{Valid: false}
	allValid := true

	formatValidationStatus(&buf, result, &allValid)

	if !strings.Contains(buf.String(), "INVALID") {
		t.Errorf("output missing INVALID\nGot: %s", buf.String())
	}
	if allValid {
		t.Error("allValid should be false after invalid result")
	}
}

func TestFormatLenientInfo_NotLenient(t *testing.T) {
	var buf bytes.Buffer
	result := &ratchet.ValidationResult{Lenient: false}

	formatLenientInfo(&buf, result)

	if buf.Len() != 0 {
		t.Errorf("non-lenient should produce no output, got: %s", buf.String())
	}
}

func TestFormatLenientInfo_Lenient(t *testing.T) {
	var buf bytes.Buffer
	expiryDate := "2025-04-15T00:00:00Z"
	result := &ratchet.ValidationResult{
		Lenient:           true,
		LenientExpiryDate: &expiryDate,
	}

	formatLenientInfo(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "LENIENT") {
		t.Errorf("output missing LENIENT\nGot: %s", out)
	}
	if !strings.Contains(out, "2025-04-15") {
		t.Errorf("output missing expiry date\nGot: %s", out)
	}
}

func TestFormatLenientInfo_ExpiringSoon(t *testing.T) {
	var buf bytes.Buffer
	expiryDate := "2025-02-01T00:00:00Z"
	result := &ratchet.ValidationResult{
		Lenient:             true,
		LenientExpiryDate:   &expiryDate,
		LenientExpiringSoon: true,
	}

	formatLenientInfo(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "Expiring soon") {
		t.Errorf("output missing expiring soon warning\nGot: %s", out)
	}
}

func TestFormatStringList_Empty(t *testing.T) {
	var buf bytes.Buffer
	formatStringList(&buf, "Issues", nil)

	if buf.Len() != 0 {
		t.Errorf("empty list should produce no output, got: %s", buf.String())
	}
}

func TestFormatStringList_WithItems(t *testing.T) {
	var buf bytes.Buffer
	formatStringList(&buf, "Issues", []string{"issue one", "issue two"})

	out := buf.String()
	if !strings.Contains(out, "Issues:") {
		t.Errorf("output missing label\nGot: %s", out)
	}
	if !strings.Contains(out, "issue one") || !strings.Contains(out, "issue two") {
		t.Errorf("output missing items\nGot: %s", out)
	}
}

func TestFormatValidationResult_FullOutput(t *testing.T) {
	var buf bytes.Buffer
	tier := ratchet.TierLearning
	result := &ratchet.ValidationResult{
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{"minor warning"},
		Tier:     &tier,
	}
	allValid := true

	formatValidationResult(&buf, "test.md", result, &allValid)

	out := buf.String()
	if !strings.Contains(out, "test.md") {
		t.Errorf("output missing filename\nGot: %s", out)
	}
	if !strings.Contains(out, "VALID") {
		t.Errorf("output missing VALID\nGot: %s", out)
	}
	if !strings.Contains(out, "Tier: 1") {
		t.Errorf("output missing tier\nGot: %s", out)
	}
}

func TestValidateFiles_AllValid(t *testing.T) {
	tmp := t.TempDir()

	// Create a valid research artifact
	artifactPath := filepath.Join(tmp, "research.md")
	content := "---\nschema_version: 1\n---\n# Research\n\n## Summary\nSummary here.\n\n## Key Findings\nFindings.\n\n## Recommendations\nRecommendations.\n\nSource: http://example.com\n\n" + strings.Repeat("word ", 120)
	if err := os.WriteFile(artifactPath, []byte(content), 0644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	validator, err := ratchet.NewValidator(tmp)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	origLenient := ratchetLenient
	ratchetLenient = false
	defer func() { ratchetLenient = origLenient }()

	var buf bytes.Buffer
	err = validateFiles(&buf, validator, ratchet.StepResearch, []string{artifactPath})
	if err != nil {
		t.Errorf("validateFiles should pass for valid artifact: %v", err)
	}
}

func TestValidateFiles_MissingFile(t *testing.T) {
	tmp := t.TempDir()

	validator, err := ratchet.NewValidator(tmp)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	err = validateFiles(&buf, validator, ratchet.StepResearch, []string{filepath.Join(tmp, "nonexistent.md")})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestOutputValidationResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := &ratchet.ValidationResult{
		Step:  ratchet.StepResearch,
		Valid: true,
	}
	allValid := true

	var buf bytes.Buffer
	err := outputValidationResult(&buf, "test.md", result, &allValid)
	if err != nil {
		t.Fatalf("outputValidationResult: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON output, got: %s", out)
	}
	if !strings.Contains(out, "research") {
		t.Errorf("JSON missing step name\nGot: %s", out)
	}
}

func TestOutputValidationResult_Table(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := &ratchet.ValidationResult{
		Step:   ratchet.StepPlan,
		Valid:  false,
		Issues: []string{"missing objective"},
	}
	allValid := true

	var buf bytes.Buffer
	err := outputValidationResult(&buf, "plan.md", result, &allValid)
	if err != nil {
		t.Fatalf("outputValidationResult: %v", err)
	}

	if allValid {
		t.Error("allValid should be false after invalid result")
	}
}
