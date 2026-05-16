package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is an L2 test helper: it creates parent dirs and writes content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// findByID returns the single Finding with id, or fails the test.
func findByID(t *testing.T, fs []Finding, id string) Finding {
	t.Helper()
	for _, f := range fs {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("expected finding %q, got %d findings: %+v", id, len(fs), fs)
	return Finding{}
}

// --- FM 1: invalid-config-yaml-swallowed ----------------------------------

func TestCliConfigInvalidConfigYAML_DetectsBrokenHomeConfig(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	// Unterminated double-quoted scalar — fails yaml.Unmarshal at a fixed point.
	writeFile(t, filepath.Join(home, ".agentops", "config.yaml"),
		"models:\n  default_tier: \"haiku\n")

	env := &DetectEnv{HomeDir: home, CWD: filepath.Join(tmp, "cwd")}
	fs, err := invalidConfigYAMLDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	f := findByID(t, fs, fmInvalidConfigYAML)
	if f.Severity != "P1" {
		t.Errorf("Severity = %q, want P1", f.Severity)
	}
	if f.Subsystem != "cli-config" {
		t.Errorf("Subsystem = %q, want cli-config", f.Subsystem)
	}
	if f.Remediation.AutoFixable {
		t.Error("AutoFixable = true, want false")
	}
	wantPath := filepath.Join(home, ".agentops", "config.yaml")
	if f.Evidence.File != wantPath {
		t.Errorf("Evidence.File = %q, want %q", f.Evidence.File, wantPath)
	}
	if !strings.Contains(f.Remediation.Command, wantPath) {
		t.Errorf("Remediation.Command does not name the file: %q", f.Remediation.Command)
	}
	if !strings.Contains(f.Remediation.Command, "yaml.safe_load") {
		t.Errorf("Remediation.Command lacks verify command: %q", f.Remediation.Command)
	}
}

func TestCliConfigInvalidConfigYAML_CleanConfigYieldsNoFinding(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	writeFile(t, filepath.Join(home, ".agentops", "config.yaml"),
		"models:\n  default_tier: haiku\n")

	env := &DetectEnv{HomeDir: home, CWD: filepath.Join(tmp, "cwd")}
	fs, err := invalidConfigYAMLDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected 0 findings for valid YAML, got %d: %+v", len(fs), fs)
	}
}

func TestCliConfigInvalidConfigYAML_FixerRefuses(t *testing.T) {
	fx := FixerByID(fmInvalidConfigYAML)
	if fx == nil {
		t.Fatal("fixer not registered")
	}
	if fx.AutoFixable() {
		t.Fatal("AutoFixable() = true, want false")
	}
	res, err := fx.Fix(nil, nil, []Finding{{ID: fmInvalidConfigYAML}})
	if err == nil {
		t.Fatal("Fix() err = nil, want refusal error")
	}
	if res.Fixed {
		t.Error("res.Fixed = true, want false")
	}
	if res.ActionsTaken != 0 {
		t.Errorf("res.ActionsTaken = %d, want 0", res.ActionsTaken)
	}
	if !strings.Contains(err.Error(), "refused_unsafe") {
		t.Errorf("error lacks refused_unsafe: %v", err)
	}
	if !strings.Contains(err.Error(), "ao doctor explain "+fmInvalidConfigYAML) {
		t.Errorf("error does not name the operator command: %v", err)
	}
}

// --- FM 2: config-flag-not-threaded ---------------------------------------

func TestCliConfigConfigFlagNotThreaded_DetectsBuggySource(t *testing.T) {
	repo := t.TempDir()
	// Buggy shape: root.go uses syncConfigFlagToEnv + AGENTOPS_CONFIG;
	// config.go has homeConfigPath but never references AGENTOPS_CONFIG.
	writeFile(t, filepath.Join(repo, "cli", "cmd", "ao", "root.go"),
		"package main\nfunc syncConfigFlagToEnv() { os.Setenv(\"AGENTOPS_CONFIG\", path) }\n")
	writeFile(t, filepath.Join(repo, "cli", "internal", "config", "config.go"),
		"package config\nfunc homeConfigPath() string { return \"~/.agentops/config.yaml\" }\n")

	if !probeConfigSourceShape(repo) {
		t.Fatal("probeConfigSourceShape = false, want true for buggy shape")
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo}
	fs, err := configFlagNotThreadedDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	f := findByID(t, fs, fmConfigFlagNotThreaded)
	if f.Severity != "P2" {
		t.Errorf("Severity = %q, want P2", f.Severity)
	}
	if f.Remediation.AutoFixable {
		t.Error("AutoFixable = true, want false")
	}
	if !strings.Contains(f.Remediation.Command, "config.Load") {
		t.Errorf("Remediation.Command does not name the code fix: %q", f.Remediation.Command)
	}
}

func TestCliConfigConfigFlagNotThreaded_FixedSourceYieldsNoFinding(t *testing.T) {
	repo := t.TempDir()
	// Fixed shape: config.go references AGENTOPS_CONFIG near homeConfigPath.
	writeFile(t, filepath.Join(repo, "cli", "cmd", "ao", "root.go"),
		"package main\nfunc syncConfigFlagToEnv() { os.Setenv(\"AGENTOPS_CONFIG\", path) }\n")
	writeFile(t, filepath.Join(repo, "cli", "internal", "config", "config.go"),
		"package config\nfunc homeConfigPath() string { return os.Getenv(\"AGENTOPS_CONFIG\") }\n")

	if probeConfigSourceShape(repo) {
		t.Fatal("probeConfigSourceShape = true, want false for fixed shape")
	}
}

func TestCliConfigConfigFlagNotThreaded_FixerRefuses(t *testing.T) {
	fx := FixerByID(fmConfigFlagNotThreaded)
	if fx == nil {
		t.Fatal("fixer not registered")
	}
	if fx.AutoFixable() {
		t.Fatal("AutoFixable() = true, want false")
	}
	res, err := fx.Fix(nil, nil, []Finding{{ID: fmConfigFlagNotThreaded}})
	if err == nil {
		t.Fatal("Fix() err = nil, want refusal")
	}
	if res.Fixed || res.ActionsTaken != 0 {
		t.Errorf("res = %+v, want unfixed/zero-actions", res)
	}
	if !strings.Contains(err.Error(), "ao-code defect") {
		t.Errorf("error does not name it as ao-code defect: %v", err)
	}
}

// --- FM 3: missing-required-cli -------------------------------------------

func TestCliConfigMissingRequiredCLI_DetectsMissingBd(t *testing.T) {
	tmp := t.TempDir()
	fakebin := filepath.Join(tmp, "fakebin")
	// Only git present, no bd.
	writeFile(t, filepath.Join(fakebin, "git"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(fakebin, "git"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv("PATH", fakebin)

	fs, err := missingRequiredCLIDetector{}.Detect(&DetectEnv{})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	f := findByID(t, fs, fmMissingRequiredCLI)
	if f.Severity != "P1" {
		t.Errorf("Severity = %q, want P1", f.Severity)
	}
	if !strings.Contains(f.Evidence.Query, "missing_clis=bd") {
		t.Errorf("Evidence.Query missing bd: %q", f.Evidence.Query)
	}
	if strings.Contains(f.Evidence.Query, "missing_clis=bd,git") {
		t.Errorf("git wrongly reported missing: %q", f.Evidence.Query)
	}
	if !strings.Contains(f.Remediation.Command, "beads") {
		t.Errorf("Remediation.Command lacks bd install hint: %q", f.Remediation.Command)
	}
}

func TestCliConfigMissingRequiredCLI_AllPresentYieldsNoFinding(t *testing.T) {
	tmp := t.TempDir()
	fakebin := filepath.Join(tmp, "fakebin")
	for _, name := range []string{"bd", "git"} {
		p := filepath.Join(fakebin, name)
		writeFile(t, p, "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
	}
	t.Setenv("PATH", fakebin)

	fs, err := missingRequiredCLIDetector{}.Detect(&DetectEnv{})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(fs), fs)
	}
}

func TestCliConfigMissingRequiredCLI_FixerRefuses(t *testing.T) {
	fx := FixerByID(fmMissingRequiredCLI)
	if fx == nil {
		t.Fatal("fixer not registered")
	}
	res, err := fx.Fix(nil, nil, []Finding{{ID: fmMissingRequiredCLI}})
	if err == nil || res.Fixed {
		t.Fatalf("Fix() = (%+v, %v), want refusal", res, err)
	}
	if !strings.Contains(err.Error(), "does not install software") {
		t.Errorf("error lacks 'does not install software': %v", err)
	}
}

// --- FM 4: optional-codex-cli-absent --------------------------------------

func TestCliConfigOptionalCodexAbsent_DetectsAbsence(t *testing.T) {
	tmp := t.TempDir()
	fakebin := filepath.Join(tmp, "fakebin")
	for _, name := range []string{"bd", "git"} {
		p := filepath.Join(fakebin, name)
		writeFile(t, p, "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
	}
	t.Setenv("PATH", fakebin) // no codex on PATH

	fs, err := optionalCodexAbsentDetector{}.Detect(&DetectEnv{})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	f := findByID(t, fs, fmOptionalCodexAbsent)
	if f.Severity != "P3" {
		t.Errorf("Severity = %q, want P3", f.Severity)
	}
	if !strings.Contains(f.Evidence.Query, "state=absent") {
		t.Errorf("Evidence.Query lacks state=absent: %q", f.Evidence.Query)
	}
	if !strings.Contains(f.Remediation.Command, "codex") {
		t.Errorf("Remediation.Command lacks codex install hint: %q", f.Remediation.Command)
	}
}

func TestCliConfigOptionalCodexAbsent_PresentYieldsNoFinding(t *testing.T) {
	tmp := t.TempDir()
	fakebin := filepath.Join(tmp, "fakebin")
	p := filepath.Join(fakebin, "codex")
	writeFile(t, p, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv("PATH", fakebin)

	fs, err := optionalCodexAbsentDetector{}.Detect(&DetectEnv{})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected 0 findings when codex present, got %d: %+v", len(fs), fs)
	}
}

func TestCliConfigOptionalCodexAbsent_FixerRefuses(t *testing.T) {
	fx := FixerByID(fmOptionalCodexAbsent)
	if fx == nil {
		t.Fatal("fixer not registered")
	}
	res, err := fx.Fix(nil, nil, []Finding{{ID: fmOptionalCodexAbsent}})
	if err == nil || res.Fixed {
		t.Fatalf("Fix() = (%+v, %v), want refusal", res, err)
	}
	if !strings.Contains(err.Error(), "refused_unsafe") {
		t.Errorf("error lacks refused_unsafe: %v", err)
	}
}

// --- FM 5: dev-version-build-integrity ------------------------------------

func TestCliConfigDevVersion_DetectsDevBinary(t *testing.T) {
	tmp := t.TempDir()
	fakebin := filepath.Join(tmp, "fakebin")
	// Stub `ao` that reports a dev version.
	writeFile(t, filepath.Join(fakebin, "ao"), "#!/bin/sh\necho 'ao version dev'\n")
	if err := os.Chmod(filepath.Join(fakebin, "ao"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv("PATH", fakebin)

	fs, err := devVersionBuildIntegrityDetector{}.Detect(&DetectEnv{})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	f := findByID(t, fs, fmDevVersionBuildIntegrity)
	if f.Severity != "P2" {
		t.Errorf("Severity = %q, want P2", f.Severity)
	}
	if !strings.Contains(f.Evidence.Query, `reported_version="dev"`) {
		t.Errorf("Evidence.Query lacks reported_version=dev: %q", f.Evidence.Query)
	}
	if !strings.Contains(f.Evidence.Query, "suspect_version=true") {
		t.Errorf("Evidence.Query lacks suspect_version=true: %q", f.Evidence.Query)
	}
	if !strings.Contains(f.Remediation.Command, "make build") {
		t.Errorf("Remediation.Command lacks rebuild command: %q", f.Remediation.Command)
	}
}

func TestCliConfigDevVersion_ReleaseBinaryYieldsNoFinding(t *testing.T) {
	tmp := t.TempDir()
	fakebin := filepath.Join(tmp, "fakebin")
	writeFile(t, filepath.Join(fakebin, "ao"), "#!/bin/sh\necho 'ao version v2.40.0'\n")
	if err := os.Chmod(filepath.Join(fakebin, "ao"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv("PATH", fakebin)

	fs, err := devVersionBuildIntegrityDetector{}.Detect(&DetectEnv{})
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected 0 findings for release version, got %d: %+v", len(fs), fs)
	}
}

func TestCliConfigDevVersion_FixerRefuses(t *testing.T) {
	fx := FixerByID(fmDevVersionBuildIntegrity)
	if fx == nil {
		t.Fatal("fixer not registered")
	}
	res, err := fx.Fix(nil, nil, []Finding{{ID: fmDevVersionBuildIntegrity}})
	if err == nil || res.Fixed {
		t.Fatalf("Fix() = (%+v, %v), want refusal", res, err)
	}
	if !strings.Contains(err.Error(), "does not recompile or replace binaries") {
		t.Errorf("error lacks recompile refusal text: %v", err)
	}
}

// --- FM 6: stale-project-config-shadows-home ------------------------------

func TestCliConfigStaleProjectConfig_DetectsShadowing(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	cwd := filepath.Join(tmp, "work")
	writeFile(t, filepath.Join(home, ".agentops", "config.yaml"),
		"models:\n  default_tier: sonnet\n")
	writeFile(t, filepath.Join(cwd, ".agentops", "config.yaml"),
		"models:\n  default_tier: haiku\n  deprecated_tier: opus\n")

	env := &DetectEnv{HomeDir: home, CWD: cwd}
	fs, err := staleProjectConfigDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	f := findByID(t, fs, fmStaleProjectConfig)
	if f.Severity != "P3" {
		t.Errorf("Severity = %q, want P3", f.Severity)
	}
	wantPath := filepath.Join(cwd, ".agentops", "config.yaml")
	if f.Evidence.File != wantPath {
		t.Errorf("Evidence.File = %q, want %q", f.Evidence.File, wantPath)
	}
	if !strings.Contains(f.Evidence.Query, "models.default_tier") {
		t.Errorf("Evidence.Query lacks shadowed key models.default_tier: %q", f.Evidence.Query)
	}
	if !strings.Contains(f.Evidence.Query, "models.deprecated_tier") {
		t.Errorf("Evidence.Query lacks shadowed key models.deprecated_tier: %q", f.Evidence.Query)
	}
	if !strings.Contains(f.Remediation.Command, wantPath) {
		t.Errorf("Remediation.Command does not name the stray file: %q", f.Remediation.Command)
	}
	if f.Remediation.AutoFixable {
		t.Error("AutoFixable = true, want false")
	}
}

func TestCliConfigStaleProjectConfig_NoProjectFileYieldsNoFinding(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	cwd := filepath.Join(tmp, "work")
	writeFile(t, filepath.Join(home, ".agentops", "config.yaml"),
		"models:\n  default_tier: sonnet\n")

	env := &DetectEnv{HomeDir: home, CWD: cwd}
	fs, err := staleProjectConfigDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected 0 findings with no project config, got %d: %+v", len(fs), fs)
	}
}

func TestCliConfigStaleProjectConfig_InertProjectFileYieldsNoFinding(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	cwd := filepath.Join(tmp, "work")
	// Project file matches home exactly — inert, not shadowing.
	writeFile(t, filepath.Join(home, ".agentops", "config.yaml"),
		"models:\n  default_tier: sonnet\n")
	writeFile(t, filepath.Join(cwd, ".agentops", "config.yaml"),
		"models:\n  default_tier: sonnet\n")

	env := &DetectEnv{HomeDir: home, CWD: cwd}
	fs, err := staleProjectConfigDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected 0 findings for inert project config, got %d: %+v", len(fs), fs)
	}
}

func TestCliConfigStaleProjectConfig_FixerRefuses(t *testing.T) {
	fx := FixerByID(fmStaleProjectConfig)
	if fx == nil {
		t.Fatal("fixer not registered")
	}
	res, err := fx.Fix(nil, nil, []Finding{{ID: fmStaleProjectConfig}})
	if err == nil || res.Fixed {
		t.Fatalf("Fix() = (%+v, %v), want refusal", res, err)
	}
	if !strings.Contains(err.Error(), "will not move a possibly-intentional user file") {
		t.Errorf("error lacks file-move refusal text: %v", err)
	}
}

// --- Registration sanity --------------------------------------------------

func TestCliConfigAllSixRegistered(t *testing.T) {
	want := []string{
		fmInvalidConfigYAML,
		fmConfigFlagNotThreaded,
		fmMissingRequiredCLI,
		fmOptionalCodexAbsent,
		fmDevVersionBuildIntegrity,
		fmStaleProjectConfig,
	}
	dets := make(map[string]bool)
	for _, d := range Detectors() {
		dets[d.ID()] = true
	}
	for _, id := range want {
		if !dets[id] {
			t.Errorf("detector %q not registered", id)
		}
		fx := FixerByID(id)
		if fx == nil {
			t.Errorf("fixer %q not registered", id)
			continue
		}
		if fx.AutoFixable() {
			t.Errorf("fixer %q AutoFixable() = true, want false (all cli-config FMs are detect-only)", id)
		}
	}
}
