package doctor

// Skills subsystem detectors and fixers.
//
// This file implements the six skills failure modes from the Phase 2 analysis.
// Five are auto-fixable (one partially), one is detect-only:
//
//	fm-skills-missing            (auto)    — mirror repo skills/<name>/** into ~/.claude/skills
//	fm-skills-stale-codex-sync   (auto)    — re-sync drift surfaces into the Codex native cache
//	fm-skills-hash-drift         (auto)    — rewrite drifted hash fields in codex metadata JSON
//	fm-skills-stale-command-refs (auto)    — substitute deprecated `ao` namespace commands
//	fm-skills-integrity-hygiene  (partial) — append links for unlinked references/ files
//	fm-skills-duplicate-install  (detect)  — overlapping installs; needs an operator decision
//
// Detectors are PURE: they stat and read only. Every fixer disk write flows
// through Mutate — there is no os.WriteFile/os.Remove/os.Rename/os.Create in
// this file. WriteFile is create-or-overwrite and Mutate's executeAtomic
// MkdirAll's the parent directory, so mirroring into a fresh tree is safe.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/quality"
)

// skillsSubsystem is the canonical subsystem name for every skills FM.
const skillsSubsystem = "skills"

// init registers all six skills detectors and six skills fixers.
func init() {
	RegisterDetector(skillsMissingDetector{})
	RegisterDetector(skillsStaleCodexSyncDetector{})
	RegisterDetector(skillsHashDriftDetector{})
	RegisterDetector(skillsStaleCommandRefsDetector{})
	RegisterDetector(skillsIntegrityHygieneDetector{})
	RegisterDetector(skillsDuplicateInstallDetector{})

	RegisterFixer(skillsMissingFixer{})
	RegisterFixer(skillsStaleCodexSyncFixer{})
	RegisterFixer(skillsHashDriftFixer{})
	RegisterFixer(skillsStaleCommandRefsFixer{})
	RegisterFixer(skillsIntegrityHygieneFixer{})
	RegisterFixer(skillsDuplicateInstallFixer{})
}

// ---------------------------------------------------------------------------
// Shared helpers (pure).
// ---------------------------------------------------------------------------

// hashHex returns the hex-encoded SHA-256 of b. It is used for codex manifest
// and metadata hash comparison and stamping.
func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// codexNativeRoot returns the Codex native plugin cache root under home.
func codexNativeRoot(home string) string {
	return filepath.Join(home, ".codex", "plugins", "cache",
		"agentops-marketplace", "agentops", "local")
}

// skillInstallDirs returns the four candidate skill install roots, in priority
// order: Codex native plugin cache, raw Codex install, Claude install, legacy.
func skillInstallDirs(home string) []string {
	return []string{
		filepath.Join(codexNativeRoot(home), "skills-codex"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}
}

// countSkillMDSubdirs counts immediate subdirectories of dir that contain a
// SKILL.md file. A missing dir yields zero. It is a read-only walk.
func countSkillMDSubdirs(dir string) int {
	return len(skillMDSubdirNames(dir))
}

// skillMDSubdirNames returns the sorted names of immediate subdirectories of
// dir that contain a SKILL.md file. A missing dir yields nil.
func skillMDSubdirNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if st, serr := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); serr == nil && !st.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// listSubdirs returns the sorted names of immediate subdirectories of dir.
func listSubdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// walkRelFiles returns every regular file under root, as paths relative to
// root, sorted. A missing root yields nil. It is a read-only walk.
func walkRelFiles(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, de os.DirEntry, werr error) error {
		if werr != nil || de.IsDir() {
			return nil //nolint:nilerr // missing tree is benign
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr == nil {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// fileMode returns the permission bits of path, defaulting to 0o644 if absent.
func fileMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

// remediation builds a Remediation pointing at the doctor for finding id.
func remediation(id string, autoFixable bool, actions int) Remediation {
	return Remediation{
		Command:          "ao doctor --fix --only " + id,
		ExplainCommand:   "ao doctor explain " + id,
		AutoFixable:      autoFixable,
		EstimatedActions: actions,
	}
}

// ---------------------------------------------------------------------------
// FM: fm-skills-missing (auto-fixable; refuses when no repo skills/ source)
// ---------------------------------------------------------------------------

// skillsMissingDetector flags a machine where none of the four skill install
// roots contains a SKILL.md-bearing subdirectory.
type skillsMissingDetector struct{}

func (skillsMissingDetector) ID() string           { return "fm-skills-missing" }
func (skillsMissingDetector) Subsystem() string    { return skillsSubsystem }
func (skillsMissingDetector) Severity() string     { return "P1" }
func (skillsMissingDetector) EstimatedCostMS() int { return 6 }
func (skillsMissingDetector) OnlineRequired() bool { return false }
func (skillsMissingDetector) QuickPath() bool      { return true }
func (skillsMissingDetector) Describe() string {
	return "no installed skills found in any of the four known install roots"
}

// anyRootPopulated reports whether at least one skill install root has a
// SKILL.md-bearing subdirectory.
func anyRootPopulated(home string) bool {
	for _, d := range skillInstallDirs(home) {
		if countSkillMDSubdirs(d) > 0 {
			return true
		}
	}
	return false
}

func (d skillsMissingDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if anyRootPopulated(env.HomeDir) {
		return nil, nil
	}
	// Auto-fixable only if a repo skills/ source tree is resolvable.
	src := filepath.Join(env.RepoRoot, "skills")
	autoFixable := countSkillMDSubdirs(src) > 0
	var scanned []string
	for _, dir := range skillInstallDirs(env.HomeDir) {
		scanned = append(scanned, fmt.Sprintf("%s=%d", dir, countSkillMDSubdirs(dir)))
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      "no installed skills found in any known install location",
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".claude/skills",
			Query: "scan of 4 SkillInstallDirs found 0 SKILL.md subdirs: " + strings.Join(scanned, " "),
		},
		Remediation: remediation(d.ID(), autoFixable, countSkillMDSubdirs(src)),
	}}, nil
}

// skillsMissingFixer mirrors the repo skills/<name>/** tree into the canonical
// Claude install root (~/.claude/skills) through Mutate. It refuses if no repo
// skills/ source tree is resolvable.
type skillsMissingFixer struct{}

func (skillsMissingFixer) ID() string { return "fm-skills-missing" }
func (skillsMissingFixer) Preconditions() []string {
	return []string{
		"repo.root/skills/ contains at least one SKILL.md-bearing dir",
		"~/.claude/skills is inside write_scopes and writable",
	}
}
func (skillsMissingFixer) WritesTo() []string { return []string{"~/.claude/skills"} }
func (skillsMissingFixer) Ops() []string      { return []string{"WriteFile"} }
func (skillsMissingFixer) Reversible() bool   { return true }
func (skillsMissingFixer) Idempotent() bool   { return true }
func (skillsMissingFixer) AutoFixable() bool  { return true }

func (f skillsMissingFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	// 1. Re-read current state; do not trust the detector snapshot.
	if anyRootPopulated(env.HomeDir) {
		res.Fixed = true
		return res, nil
	}
	// 2. Precondition: a repo skills/ source tree must exist.
	src := filepath.Join(env.RepoRoot, "skills")
	if countSkillMDSubdirs(src) == 0 {
		res.Err = fmt.Errorf("doctor: %s: no skills/ source tree to install from (refused_unsafe)", f.ID())
		return res, res.Err
	}
	// 3/4. Mirror every skills/<name>/** file into ~/.claude/skills.
	targetRoot := filepath.Join(ctx.HomeDir, ".claude", "skills")
	for _, rel := range walkRelFiles(src) {
		content, err := os.ReadFile(filepath.Join(src, rel))
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: read source %s: %w", f.ID(), rel, err)
			return res, res.Err
		}
		dest := filepath.Join(targetRoot, rel)
		r, err := Mutate(ctx, dest, WriteFile{Content: content, Mode: fileMode(filepath.Join(src, rel))})
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: install %s: %w", f.ID(), rel, err)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	// 5. Verify post-state.
	if !ctx.DryRun && !anyRootPopulated(env.HomeDir) {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-skills-stale-codex-sync (auto-fixable)
// ---------------------------------------------------------------------------

// codexInstallMeta is the minimal projection of ~/.codex/.agentops-codex-install.json.
type codexInstallMeta struct {
	ManifestHash string `json:"manifest_hash"`
	Version      string `json:"version"`
}

// skillsStaleCodexSyncDetector flags an installed Codex plugin that has drifted
// from the local repo checkout: manifest-hash mismatch or version mismatch.
type skillsStaleCodexSyncDetector struct{}

func (skillsStaleCodexSyncDetector) ID() string           { return "fm-skills-stale-codex-sync" }
func (skillsStaleCodexSyncDetector) Subsystem() string    { return skillsSubsystem }
func (skillsStaleCodexSyncDetector) Severity() string     { return "P1" }
func (skillsStaleCodexSyncDetector) EstimatedCostMS() int { return 8 }
func (skillsStaleCodexSyncDetector) OnlineRequired() bool { return false }
func (skillsStaleCodexSyncDetector) QuickPath() bool      { return false }
func (skillsStaleCodexSyncDetector) Describe() string {
	return "installed Codex plugin drifts from the local repo skills-codex/ checkout"
}

// codexInstallMetaPath returns ~/.codex/.agentops-codex-install.json.
func codexInstallMetaPath(home string) string {
	return filepath.Join(home, ".codex", ".agentops-codex-install.json")
}

// repoCodexManifestPath returns repo/skills-codex/.agentops-manifest.json.
func repoCodexManifestPath(repo string) string {
	return filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
}

// codexSyncDrift reports the two drift signals (hash, version).
// It is pure: stat + read only. ok reports whether a comparison was possible.
func codexSyncDrift(env *DetectEnv) (hashDrift, versionDrift, ok bool) {
	metaPath := codexInstallMetaPath(env.HomeDir)
	manifestPath := repoCodexManifestPath(env.RepoRoot)
	metaRaw, err := os.ReadFile(metaPath)
	if err != nil {
		return false, false, false
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, false, false
	}
	var meta codexInstallMeta
	if json.Unmarshal(metaRaw, &meta) != nil {
		return false, false, false
	}
	hashDrift = meta.ManifestHash != hashHex(manifestRaw)
	versionDrift = env.TargetSHA != "" && meta.Version != env.TargetSHA
	return hashDrift, versionDrift, true
}

// fileExists reports whether path exists as a regular file.
func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func (d skillsStaleCodexSyncDetector) Detect(env *DetectEnv) ([]Finding, error) {
	hashDrift, versionDrift, ok := codexSyncDrift(env)
	if !ok || (!hashDrift && !versionDrift) {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      "installed Codex plugin drifts from the repo checkout",
		Confidence: 1.0,
		Evidence: Evidence{
			File: ".codex/.agentops-codex-install.json",
			Query: fmt.Sprintf("hash_drift=%t version_drift=%t",
				hashDrift, versionDrift),
		},
		Remediation: remediation(d.ID(), true, 1),
	}}, nil
}

// skillsStaleCodexSyncFixer mirrors the drift surfaces (skills-codex/** and the
// install-metadata JSON) from the repo into the Codex native cache through
// Mutate. The metadata stamp is written last so a crash leaves the install
// marked stale (safe) rather than falsely fresh.
type skillsStaleCodexSyncFixer struct{}

func (skillsStaleCodexSyncFixer) ID() string { return "fm-skills-stale-codex-sync" }
func (skillsStaleCodexSyncFixer) Preconditions() []string {
	return []string{
		"repo.root/skills-codex/ exists",
		"~/.codex/ exists with a valid .agentops-codex-install.json",
	}
}
func (skillsStaleCodexSyncFixer) WritesTo() []string {
	return []string{
		"~/.codex/plugins/cache/agentops-marketplace",
		"~/.codex/.agentops-codex-install.json",
	}
}
func (skillsStaleCodexSyncFixer) Ops() []string     { return []string{"WriteFile"} }
func (skillsStaleCodexSyncFixer) Reversible() bool  { return true }
func (skillsStaleCodexSyncFixer) Idempotent() bool  { return true }
func (skillsStaleCodexSyncFixer) AutoFixable() bool { return true }

// mirrorTree mirrors every file under srcRoot into destRoot through Mutate,
// adding to res.ActionsTaken. It returns the first error encountered.
func mirrorTree(ctx *MutateContext, fixerID, srcRoot, destRoot string, res *FixResult) error {
	for _, rel := range walkRelFiles(srcRoot) {
		content, err := os.ReadFile(filepath.Join(srcRoot, rel))
		if err != nil {
			return fmt.Errorf("doctor: %s: read %s: %w", fixerID, rel, err)
		}
		dest := filepath.Join(destRoot, rel)
		r, err := Mutate(ctx, dest, WriteFile{Content: content, Mode: fileMode(filepath.Join(srcRoot, rel))})
		if err != nil {
			return fmt.Errorf("doctor: %s: mirror %s: %w", fixerID, rel, err)
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	return nil
}

func (f skillsStaleCodexSyncFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}

	needsFix, err := f.codexSyncNeedsFix(env)
	if err != nil {
		res.Err = err
		return res, err
	}
	if !needsFix {
		res.Fixed = true
		return res, nil
	}

	sources, err := f.codexSyncSources(ctx, env)
	if err != nil {
		res.Err = err
		return res, err
	}

	if err := f.mirrorCodexSyncSources(ctx, sources, &res); err != nil {
		res.Err = err
		return res, err
	}

	if err := f.stampCodexInstallMeta(ctx, env, &res); err != nil {
		res.Err = err
		return res, err
	}

	if err := f.verifyCodexSyncFixed(ctx, env); err != nil {
		res.Err = err
		return res, err
	}
	res.Fixed = true
	return res, nil
}

type codexSyncSources struct {
	SkillsCodex string
	CodexRoot   string
}

func (f skillsStaleCodexSyncFixer) codexSyncNeedsFix(env *DetectEnv) (bool, error) {
	hashDrift, versionDrift, ok := codexSyncDrift(env)
	if !ok {
		return false, fmt.Errorf("doctor: %s: no Codex install / repo manifest to sync (refused_unsafe)", f.ID())
	}
	return hashDrift || versionDrift, nil
}

func (f skillsStaleCodexSyncFixer) codexSyncSources(ctx *MutateContext, env *DetectEnv) (codexSyncSources, error) {
	sources := codexSyncSources{
		SkillsCodex: filepath.Join(env.RepoRoot, "skills-codex"),
		CodexRoot:   codexNativeRoot(ctx.HomeDir),
	}
	if !dirExists(sources.SkillsCodex) {
		return codexSyncSources{}, fmt.Errorf("doctor: %s: no skills-codex/ source (refused_unsafe)", f.ID())
	}
	if !dirExists(filepath.Join(ctx.HomeDir, ".codex")) {
		return codexSyncSources{}, fmt.Errorf("doctor: %s: no Codex install present (refused_unsafe)", f.ID())
	}
	return sources, nil
}

func (f skillsStaleCodexSyncFixer) mirrorCodexSyncSources(ctx *MutateContext, sources codexSyncSources, res *FixResult) error {
	return mirrorTree(ctx, f.ID(), sources.SkillsCodex, filepath.Join(sources.CodexRoot, "skills-codex"), res)
}

func (f skillsStaleCodexSyncFixer) stampCodexInstallMeta(ctx *MutateContext, env *DetectEnv, res *FixResult) error {
	newMeta, err := f.stampInstallMeta(env)
	if err != nil {
		return err
	}
	r, err := Mutate(ctx, codexInstallMetaPath(ctx.HomeDir), WriteFile{Content: newMeta, Mode: 0o644})
	if err != nil {
		return fmt.Errorf("doctor: %s: stamp install metadata: %w", f.ID(), err)
	}
	if r.OK {
		res.ActionsTaken++
	}
	return nil
}

func (f skillsStaleCodexSyncFixer) verifyCodexSyncFixed(ctx *MutateContext, env *DetectEnv) error {
	if ctx.DryRun {
		return nil
	}
	hashDrift, versionDrift, _ := codexSyncDrift(env)
	if hashDrift || versionDrift {
		return fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
	}
	return nil
}

// stampInstallMeta returns the install-metadata JSON with manifest_hash and
// version rewritten, preserving every other key verbatim.
func (f skillsStaleCodexSyncFixer) stampInstallMeta(env *DetectEnv) ([]byte, error) {
	metaRaw, err := os.ReadFile(codexInstallMetaPath(env.HomeDir))
	if err != nil {
		return nil, fmt.Errorf("doctor: %s: read install metadata: %w", f.ID(), err)
	}
	manifestRaw, err := os.ReadFile(repoCodexManifestPath(env.RepoRoot))
	if err != nil {
		return nil, fmt.Errorf("doctor: %s: read repo manifest: %w", f.ID(), err)
	}
	return jsonSetFields(metaRaw, map[string]any{
		"manifest_hash": hashHex(manifestRaw),
		"version":       env.TargetSHA,
	})
}

// dirExists reports whether path exists as a directory.
func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// jsonSetFields parses raw JSON object bytes, sets the given top-level keys,
// and re-marshals with stable two-space indentation. Other keys are preserved.
func jsonSetFields(raw []byte, fields map[string]any) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("doctor: parse JSON object: %w", err)
	}
	for k, v := range fields {
		enc, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("doctor: marshal field %s: %w", k, err)
		}
		obj[k] = enc
	}
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("doctor: marshal JSON object: %w", err)
	}
	return append(out, '\n'), nil
}

// ---------------------------------------------------------------------------
// FM: fm-skills-hash-drift (auto-fixable)
// ---------------------------------------------------------------------------

// skillsHashDriftDetector flags Codex skill artifact metadata whose recorded
// source/generated/catalog hashes drift from the recomputed values.
type skillsHashDriftDetector struct{}

func (skillsHashDriftDetector) ID() string           { return "fm-skills-hash-drift" }
func (skillsHashDriftDetector) Subsystem() string    { return skillsSubsystem }
func (skillsHashDriftDetector) Severity() string     { return "P2" }
func (skillsHashDriftDetector) EstimatedCostMS() int { return 14 }
func (skillsHashDriftDetector) OnlineRequired() bool { return false }
func (skillsHashDriftDetector) QuickPath() bool      { return false }
func (skillsHashDriftDetector) Describe() string {
	return "Codex skill artifact hashes drift from the skills-codex source"
}

// generatedMeta is the minimal projection of a .agentops-generated.json file.
type generatedMeta struct {
	SourceHash    string `json:"source_hash"`
	GeneratedHash string `json:"generated_hash"`
}

// hashDirRecursive returns a deterministic SHA-256 over the sorted set of
// (relpath, content) pairs of every regular file under root, excluding the
// named basenames. A missing root hashes the empty input. It is pure.
func hashDirRecursive(root string, exclude ...string) string {
	skip := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		skip[e] = true
	}
	h := sha256.New()
	for _, rel := range walkRelFiles(root) {
		if skip[filepath.Base(rel)] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// hashDrift is one drifted hash artifact.
type hashDrift struct {
	skill string
	path  string
	field string
}

// generatedJSONPaths returns the sorted skills-codex/*/.agentops-generated.json
// paths under repo.
func generatedJSONPaths(repo string) []string {
	codexRoot := filepath.Join(repo, "skills-codex")
	var out []string
	for _, name := range listSubdirs(codexRoot) {
		p := filepath.Join(codexRoot, name, ".agentops-generated.json")
		if fileExists(p) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// computeHashDrift returns the drifted hash artifacts for repo. It is pure.
func computeHashDrift(repo string) []hashDrift {
	var drifted []hashDrift
	for _, genPath := range generatedJSONPaths(repo) {
		raw, err := os.ReadFile(genPath)
		if err != nil {
			continue
		}
		var g generatedMeta
		if json.Unmarshal(raw, &g) != nil {
			continue
		}
		name := filepath.Base(filepath.Dir(genPath))
		srcH := hashDirRecursive(filepath.Join(repo, "skills", name))
		genH := hashDirRecursive(filepath.Dir(genPath), ".agentops-generated.json")
		if g.SourceHash != srcH || g.GeneratedHash != genH {
			drifted = append(drifted, hashDrift{skill: name, path: genPath, field: "source_hash|generated_hash"})
		}
	}
	manifestPath := repoCodexManifestPath(repo)
	if raw, err := os.ReadFile(manifestPath); err == nil {
		var manifest map[string]json.RawMessage
		if json.Unmarshal(raw, &manifest) == nil {
			catalogH := hashDirRecursive(filepath.Join(repo, "skills-codex-overrides"))
			var recorded string
			if rc, ok := manifest["codex_override_catalog_hash"]; ok {
				_ = json.Unmarshal(rc, &recorded)
			}
			if recorded != catalogH {
				drifted = append(drifted, hashDrift{skill: "<catalog>", path: manifestPath, field: "codex_override_catalog_hash"})
			}
		}
	}
	return drifted
}

func (d skillsHashDriftDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if !fileExists(repoCodexManifestPath(env.RepoRoot)) {
		return nil, nil
	}
	drifted := computeHashDrift(env.RepoRoot)
	if len(drifted) == 0 {
		return nil, nil
	}
	var names []string
	for _, dr := range drifted {
		names = append(names, dr.skill)
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d Codex skill artifact hash(es) drifted", len(drifted)),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  "skills-codex/.agentops-manifest.json",
			Query: "drifted: " + strings.Join(names, ", "),
		},
		Remediation: remediation(d.ID(), true, len(drifted)),
	}}, nil
}

// skillsHashDriftFixer rewrites only the drifted hash fields in each affected
// metadata JSON file through Mutate; all other keys are preserved verbatim.
type skillsHashDriftFixer struct{}

func (skillsHashDriftFixer) ID() string { return "fm-skills-hash-drift" }
func (skillsHashDriftFixer) Preconditions() []string {
	return []string{
		"repo.root/skills-codex/ catalog exists and is valid JSON",
		"every drifted .agentops-generated.json is valid JSON",
	}
}
func (skillsHashDriftFixer) WritesTo() []string { return []string{"skills-codex"} }
func (skillsHashDriftFixer) Ops() []string      { return []string{"WriteFile"} }
func (skillsHashDriftFixer) Reversible() bool   { return true }
func (skillsHashDriftFixer) Idempotent() bool   { return true }
func (skillsHashDriftFixer) AutoFixable() bool  { return true }

func (f skillsHashDriftFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	// 1. Re-read current state.
	if len(computeHashDrift(env.RepoRoot)) == 0 {
		res.Fixed = true
		return res, nil
	}
	// 2. Precondition.
	if !dirExists(filepath.Join(env.RepoRoot, "skills-codex")) {
		res.Err = fmt.Errorf("doctor: %s: no skills-codex/ catalog (refused_unsafe)", f.ID())
		return res, res.Err
	}
	// 3/4. Rewrite each drifted JSON file.
	for _, genPath := range generatedJSONPaths(env.RepoRoot) {
		raw, err := os.ReadFile(genPath)
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: read %s: %w", f.ID(), genPath, err)
			return res, res.Err
		}
		var g generatedMeta
		if json.Unmarshal(raw, &g) != nil {
			continue
		}
		name := filepath.Base(filepath.Dir(genPath))
		srcH := hashDirRecursive(filepath.Join(env.RepoRoot, "skills", name))
		genH := hashDirRecursive(filepath.Dir(genPath), ".agentops-generated.json")
		if g.SourceHash == srcH && g.GeneratedHash == genH {
			continue
		}
		newRaw, err := jsonSetFields(raw, map[string]any{
			"source_hash":    srcH,
			"generated_hash": genH,
		})
		if err != nil {
			res.Err = err
			return res, err
		}
		if r, merr := Mutate(ctx, genPath, WriteFile{Content: newRaw, Mode: 0o644}); merr != nil {
			res.Err = fmt.Errorf("doctor: %s: rewrite %s: %w", f.ID(), genPath, merr)
			return res, res.Err
		} else if r.OK {
			res.ActionsTaken++
		}
	}
	if err := f.fixCatalogHash(ctx, env, &res); err != nil {
		res.Err = err
		return res, err
	}
	// 5. Verify post-state.
	if !ctx.DryRun && len(computeHashDrift(env.RepoRoot)) != 0 {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// fixCatalogHash rewrites the manifest's codex_override_catalog_hash if drifted.
func (f skillsHashDriftFixer) fixCatalogHash(ctx *MutateContext, env *DetectEnv, res *FixResult) error {
	manifestPath := repoCodexManifestPath(env.RepoRoot)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil //nolint:nilerr // no manifest => nothing to rewrite
	}
	var manifest map[string]json.RawMessage
	if json.Unmarshal(raw, &manifest) != nil {
		return nil
	}
	catalogH := hashDirRecursive(filepath.Join(env.RepoRoot, "skills-codex-overrides"))
	var recorded string
	if rc, ok := manifest["codex_override_catalog_hash"]; ok {
		_ = json.Unmarshal(rc, &recorded)
	}
	if recorded == catalogH {
		return nil
	}
	newRaw, err := jsonSetFields(raw, map[string]any{"codex_override_catalog_hash": catalogH})
	if err != nil {
		return err
	}
	r, merr := Mutate(ctx, manifestPath, WriteFile{Content: newRaw, Mode: 0o644})
	if merr != nil {
		return fmt.Errorf("doctor: %s: rewrite manifest: %w", f.ID(), merr)
	}
	if r.OK {
		res.ActionsTaken++
	}
	return nil
}

// ---------------------------------------------------------------------------
// FM: fm-skills-stale-command-refs (auto-fixable)
// ---------------------------------------------------------------------------

// skillsStaleCommandRefsDetector flags skill/doc/script files that
// reference deprecated `ao` namespace-qualified commands.
type skillsStaleCommandRefsDetector struct{}

func (skillsStaleCommandRefsDetector) ID() string           { return "fm-skills-stale-command-refs" }
func (skillsStaleCommandRefsDetector) Subsystem() string    { return skillsSubsystem }
func (skillsStaleCommandRefsDetector) Severity() string     { return "P2" }
func (skillsStaleCommandRefsDetector) EstimatedCostMS() int { return 18 }
func (skillsStaleCommandRefsDetector) OnlineRequired() bool { return false }
func (skillsStaleCommandRefsDetector) QuickPath() bool      { return false }
func (skillsStaleCommandRefsDetector) Describe() string {
	return "skill and doc files reference deprecated `ao` namespace commands"
}

// staleRefScanGlobs returns the glob set scanned for deprecated command refs,
// rooted at repo.
func staleRefScanGlobs(repo string) []string {
	return []string{
		filepath.Join(repo, "skills", "*", "SKILL.md"),
		filepath.Join(repo, "skills", "*", "references", "*.md"),
		filepath.Join(repo, "skills-codex", "*", "SKILL.md"),
		filepath.Join(repo, "skills-codex-overrides", "*", "*.md"),
		filepath.Join(repo, "docs", "*.md"),
		filepath.Join(repo, "docs", "*", "*.md"),
		filepath.Join(repo, "scripts", "*.sh"),
	}
}

// scanStaleRefFiles returns the sorted set of files with ≥1 deprecated command
// reference. It reuses quality.ScanFileForDeprecatedCommands so detector and
// fixer agree exactly on what counts as a stale reference.
func scanStaleRefFiles(repo string) []string {
	seen := make(map[string]bool)
	for _, pattern := range staleRefScanGlobs(repo) {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			if len(quality.ScanFileForDeprecatedCommands(file)) > 0 {
				seen[file] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for file := range seen {
		out = append(out, file)
	}
	sort.Strings(out)
	return out
}

func (d skillsStaleCommandRefsDetector) Detect(env *DetectEnv) ([]Finding, error) {
	files := scanStaleRefFiles(env.RepoRoot)
	if len(files) == 0 {
		return nil, nil
	}
	rels := make([]string, 0, len(files))
	for _, f := range files {
		if rel, err := filepath.Rel(env.RepoRoot, f); err == nil {
			rels = append(rels, rel)
		} else {
			rels = append(rels, f)
		}
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("deprecated `ao` command refs in %d file(s)", len(files)),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  rels[0],
			Query: "files with stale refs: " + strings.Join(rels, ", "),
		},
		Remediation: remediation(d.ID(), true, len(files)),
	}}, nil
}

// deprecatedCommandsByLenDesc returns the DeprecatedCommands keys sorted longest
// first, so a longer command is substituted before a shorter prefix of it.
func deprecatedCommandsByLenDesc() []string {
	keys := make([]string, 0, len(quality.DeprecatedCommands))
	for k := range quality.DeprecatedCommands {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}

// rewriteStaleRefLine substitutes every word-boundary-bounded deprecated command
// on a single line. A trailing alphanumeric or '-' suppresses the match, mirroring
// quality.ScanFileForDeprecatedCommands so detector and fixer stay consistent.
func rewriteStaleRefLine(line string) string {
	for _, old := range deprecatedCommandsByLenDesc() {
		newCmd := quality.DeprecatedCommands[old]
		var b strings.Builder
		rest := line
		for {
			idx := strings.Index(rest, old)
			if idx < 0 {
				b.WriteString(rest)
				break
			}
			after := idx + len(old)
			boundaryOK := after >= len(rest) || !isCmdWordChar(rest[after])
			b.WriteString(rest[:idx])
			if boundaryOK {
				b.WriteString(newCmd)
			} else {
				b.WriteString(old)
			}
			rest = rest[after:]
		}
		line = b.String()
	}
	return line
}

// isCmdWordChar reports whether c continues a command word (letter or hyphen).
func isCmdWordChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-'
}

// isRenameDocLine reports whether a line documents a command rename (contains
// → or " -> "). It mirrors quality.isRenameDocLine — that function is
// unexported, so detector and fixer keep this predicate identical by hand. Such
// lines intentionally reference deprecated commands and are never rewritten.
func isRenameDocLine(line string) bool {
	return strings.Contains(line, "→") || strings.Contains(line, " -> ")
}

// rewriteStaleRefs returns the file content with every deprecated command on a
// non-rename-doc line substituted. Rename-doc lines (containing → or " -> ")
// are preserved verbatim.
func rewriteStaleRefs(raw []byte) []byte {
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if isRenameDocLine(line) {
			continue
		}
		lines[i] = rewriteStaleRefLine(line)
	}
	return []byte(strings.Join(lines, "\n"))
}

// skillsStaleCommandRefsFixer rewrites each affected file, substituting every
// deprecated command for its replacement, atomically through Mutate.
type skillsStaleCommandRefsFixer struct{}

func (skillsStaleCommandRefsFixer) ID() string { return "fm-skills-stale-command-refs" }
func (skillsStaleCommandRefsFixer) Preconditions() []string {
	return []string{
		"DeprecatedCommands map is the 1:1 source of truth",
		"every file to rewrite is valid UTF-8 text",
	}
}
func (skillsStaleCommandRefsFixer) WritesTo() []string {
	return []string{"skills", "skills-codex", "docs", "scripts"}
}
func (skillsStaleCommandRefsFixer) Ops() []string     { return []string{"WriteFile"} }
func (skillsStaleCommandRefsFixer) Reversible() bool  { return true }
func (skillsStaleCommandRefsFixer) Idempotent() bool  { return true }
func (skillsStaleCommandRefsFixer) AutoFixable() bool { return true }

func (f skillsStaleCommandRefsFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	// 1. Re-scan; do not trust the detector snapshot.
	files := scanStaleRefFiles(env.RepoRoot)
	if len(files) == 0 {
		res.Fixed = true
		return res, nil
	}
	// 2/3/4. Rewrite each affected file atomically.
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: read %s: %w", f.ID(), file, err)
			return res, res.Err
		}
		desired := rewriteStaleRefs(raw)
		if string(desired) == string(raw) {
			continue
		}
		r, merr := Mutate(ctx, file, WriteFile{Content: desired, Mode: fileMode(file)})
		if merr != nil {
			res.Err = fmt.Errorf("doctor: %s: rewrite %s: %w", f.ID(), file, merr)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	// 5. Verify post-state.
	if !ctx.DryRun && len(scanStaleRefFiles(env.RepoRoot)) != 0 {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-skills-integrity-hygiene (partial auto-fix: unlinked references only)
// ---------------------------------------------------------------------------

// hygieneFinding is one skill-hygiene violation. Kind is one of MISSING_NAME,
// MISSING_DESC, MISSING_TIER, UNLINKED, DEAD_REF.
type hygieneFinding struct {
	Skill   string
	Kind    string
	RefFile string
}

// skillsIntegrityHygieneDetector flags skill hygiene violations: missing
// frontmatter fields, unlinked references, and dead reference links.
type skillsIntegrityHygieneDetector struct{}

func (skillsIntegrityHygieneDetector) ID() string           { return "fm-skills-integrity-hygiene" }
func (skillsIntegrityHygieneDetector) Subsystem() string    { return skillsSubsystem }
func (skillsIntegrityHygieneDetector) Severity() string     { return "P2" }
func (skillsIntegrityHygieneDetector) EstimatedCostMS() int { return 16 }
func (skillsIntegrityHygieneDetector) OnlineRequired() bool { return false }
func (skillsIntegrityHygieneDetector) QuickPath() bool      { return false }
func (skillsIntegrityHygieneDetector) Describe() string {
	return "skill hygiene violations: missing frontmatter, unlinked or dead references"
}

// frontmatterHas reports whether the YAML frontmatter block at the top of text
// contains a top-level key. It is a best-effort line scan, not a full parser.
func frontmatterHas(text, key string) bool {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for _, ln := range lines[1:] {
		if strings.TrimSpace(ln) == "---" {
			return false
		}
		trimmed := strings.TrimLeft(ln, " \t")
		if strings.HasPrefix(trimmed, key+":") {
			return true
		}
	}
	return false
}

// scanSkillHygiene returns every skill-hygiene violation under repo/skills. It
// is pure: stat + read only.
func scanSkillHygiene(repo string) ([]hygieneFinding, error) {
	skillsRoot := filepath.Join(repo, "skills")
	var out []hygieneFinding
	for _, name := range listSubdirs(skillsRoot) {
		skillDir := filepath.Join(skillsRoot, name)
		skillMD := filepath.Join(skillDir, "SKILL.md")
		raw, err := os.ReadFile(skillMD)
		if err != nil {
			continue // not a skill dir
		}
		text := string(raw)
		if !frontmatterHas(text, "name") {
			out = append(out, hygieneFinding{Skill: name, Kind: "MISSING_NAME"})
		}
		if !frontmatterHas(text, "description") {
			out = append(out, hygieneFinding{Skill: name, Kind: "MISSING_DESC"})
		}
		if !frontmatterHas(text, "tier") && !strings.Contains(text, "tier:") {
			out = append(out, hygieneFinding{Skill: name, Kind: "MISSING_TIER"})
		}
		refsDir := filepath.Join(skillDir, "references")
		for _, rf := range listRefFiles(refsDir) {
			rel := "references/" + rf
			if !strings.Contains(text, "]("+rel+")") && !strings.Contains(text, "Read "+rel) {
				out = append(out, hygieneFinding{Skill: name, Kind: "UNLINKED", RefFile: rel})
			}
		}
		for _, linked := range extractReferenceLinks(text) {
			if !fileExists(filepath.Join(skillDir, linked)) {
				out = append(out, hygieneFinding{Skill: name, Kind: "DEAD_REF", RefFile: linked})
			}
		}
	}
	return out, nil
}

// listRefFiles returns the sorted *.md basenames directly inside dir.
func listRefFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// extractReferenceLinks returns the distinct "references/<f>.md" tokens linked
// from text via Markdown link syntax.
func extractReferenceLinks(text string) []string {
	seen := make(map[string]bool)
	var out []string
	rest := text
	for {
		idx := strings.Index(rest, "](references/")
		if idx < 0 {
			break
		}
		rest = rest[idx+2:]
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			break
		}
		token := rest[:end]
		if strings.HasSuffix(token, ".md") && !seen[token] {
			seen[token] = true
			out = append(out, token)
		}
		rest = rest[end:]
	}
	sort.Strings(out)
	return out
}

func (d skillsIntegrityHygieneDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if !dirExists(filepath.Join(env.RepoRoot, "skills")) {
		return nil, nil
	}
	hygiene, err := scanSkillHygiene(env.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("doctor: scan skill hygiene: %w", err)
	}
	if len(hygiene) == 0 {
		return nil, nil
	}
	unlinked := 0
	var kinds []string
	for _, h := range hygiene {
		if h.Kind == "UNLINKED" {
			unlinked++
		}
		kinds = append(kinds, h.Skill+":"+h.Kind)
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d skill hygiene violation(s) (%d auto-fixable unlinked)", len(hygiene), unlinked),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  "skills",
			Query: "hygiene: " + strings.Join(kinds, ", "),
		},
		Remediation: remediation(d.ID(), true, unlinked),
	}}, nil
}

// skillsIntegrityHygieneFixer is a PARTIAL fixer: it appends a deterministic
// link block for unlinked references/ files through Mutate. Missing frontmatter
// and dead references are report-only — the detector keeps reporting them.
type skillsIntegrityHygieneFixer struct{}

func (skillsIntegrityHygieneFixer) ID() string { return "fm-skills-integrity-hygiene" }
func (skillsIntegrityHygieneFixer) Preconditions() []string {
	return []string{
		"repo.root/skills/ exists and is readable",
		"every SKILL.md to rewrite is valid UTF-8",
		"partial fix: only unlinked references are auto-fixed",
	}
}
func (skillsIntegrityHygieneFixer) WritesTo() []string { return []string{"skills"} }
func (skillsIntegrityHygieneFixer) Ops() []string      { return []string{"WriteFile"} }
func (skillsIntegrityHygieneFixer) Reversible() bool   { return true }
func (skillsIntegrityHygieneFixer) Idempotent() bool   { return true }
func (skillsIntegrityHygieneFixer) AutoFixable() bool  { return true }

// renderReferencesBlock builds the deterministic "## References" block for a
// sorted set of unlinked reference paths.
func renderReferencesBlock(refs []string) string {
	sorted := append([]string(nil), refs...)
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString("\n## References\n\n")
	for _, rel := range sorted {
		name := strings.TrimSuffix(filepath.Base(rel), ".md")
		fmt.Fprintf(&b, "- [%s](%s)\n", name, rel)
	}
	return b.String()
}

func (f skillsIntegrityHygieneFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	// 1. Re-scan.
	hygiene, err := scanSkillHygiene(env.RepoRoot)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: %w", f.ID(), err)
		return res, res.Err
	}
	if len(hygiene) == 0 {
		res.Fixed = true
		return res, nil
	}
	// 2. Partition into auto-fixable (UNLINKED) vs report-only.
	bySkill := make(map[string][]string)
	for _, h := range hygiene {
		if h.Kind == "UNLINKED" {
			bySkill[h.Skill] = append(bySkill[h.Skill], h.RefFile)
		}
	}
	if len(bySkill) == 0 {
		// Nothing safely fixable; report-only findings remain. A successful
		// run with nothing to fix, not a refusal.
		res.Fixed = true
		return res, nil
	}
	// 3/4. Append a references block to each affected SKILL.md.
	skills := make([]string, 0, len(bySkill))
	for name := range bySkill {
		skills = append(skills, name)
	}
	sort.Strings(skills)
	for _, name := range skills {
		skillMD := filepath.Join(env.RepoRoot, "skills", name, "SKILL.md")
		raw, err := os.ReadFile(skillMD)
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: read %s: %w", f.ID(), skillMD, err)
			return res, res.Err
		}
		text := string(raw)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		desired := []byte(text + renderReferencesBlock(bySkill[name]))
		r, merr := Mutate(ctx, skillMD, WriteFile{Content: desired, Mode: fileMode(skillMD)})
		if merr != nil {
			res.Err = fmt.Errorf("doctor: %s: rewrite %s: %w", f.ID(), skillMD, merr)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	// 5. Verify: no UNLINKED finding may remain. Report-only findings may.
	if !ctx.DryRun {
		post, _ := scanSkillHygiene(env.RepoRoot)
		for _, h := range post {
			if h.Kind == "UNLINKED" {
				res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the unlinked-reference findings", f.ID())
				return res, res.Err
			}
		}
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-skills-duplicate-install (detect-only)
// ---------------------------------------------------------------------------

// skillsDuplicateInstallDetector flags overlapping skill installs across the
// Codex/Claude/legacy install roots.
type skillsDuplicateInstallDetector struct{}

func (skillsDuplicateInstallDetector) ID() string           { return "fm-skills-duplicate-install" }
func (skillsDuplicateInstallDetector) Subsystem() string    { return skillsSubsystem }
func (skillsDuplicateInstallDetector) Severity() string     { return "P3" }
func (skillsDuplicateInstallDetector) EstimatedCostMS() int { return 7 }
func (skillsDuplicateInstallDetector) OnlineRequired() bool { return false }
func (skillsDuplicateInstallDetector) QuickPath() bool      { return false }
func (skillsDuplicateInstallDetector) Describe() string {
	return "overlapping skill installs across Codex/Claude/legacy install roots"
}

// intersectNames returns the sorted intersection of two skill-name slices.
func intersectNames(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, n := range a {
		set[n] = true
	}
	var out []string
	for _, n := range b {
		if set[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func (d skillsDuplicateInstallDetector) Detect(env *DetectEnv) ([]Finding, error) {
	dirs := skillInstallDirs(env.HomeDir)
	named := make([][]string, len(dirs))
	for i, dir := range dirs {
		named[i] = skillMDSubdirNames(dir)
	}
	// Primary = first non-empty root. None => fm-skills-missing's job.
	primaryIdx := -1
	for i := range named {
		if len(named[i]) > 0 {
			primaryIdx = i
			break
		}
	}
	if primaryIdx < 0 {
		return nil, nil
	}
	rawCodexOverlap := intersectNames(named[0], named[1])
	legacyOverlap := intersectNames(named[primaryIdx], named[3])
	if len(rawCodexOverlap) == 0 && len(legacyOverlap) == 0 {
		return nil, nil
	}
	overlapSet := make(map[string]bool)
	for _, n := range rawCodexOverlap {
		overlapSet[n] = true
	}
	for _, n := range legacyOverlap {
		overlapSet[n] = true
	}
	allOverlap := make([]string, 0, len(overlapSet))
	for n := range overlapSet {
		allOverlap = append(allOverlap, n)
	}
	sort.Strings(allOverlap)
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d skill(s) installed in overlapping roots", len(allOverlap)),
		Confidence: 1.0,
		Evidence: Evidence{
			File: dirs[primaryIdx],
			Query: fmt.Sprintf("primary=%s overlap=[%s]",
				dirs[primaryIdx], strings.Join(allOverlap, ", ")),
		},
		Remediation: Remediation{
			Command:        "ao doctor explain " + d.ID(),
			ExplainCommand: "ao doctor explain " + d.ID(),
			AutoFixable:    false,
		},
	}}, nil
}

// skillsDuplicateInstallFixer is a detect-only refuser: resolving a duplicate
// install means choosing which copy is authoritative — an operator decision the
// doctor must not make. The default --fix path always refuses.
type skillsDuplicateInstallFixer struct{}

func (skillsDuplicateInstallFixer) ID() string { return "fm-skills-duplicate-install" }
func (skillsDuplicateInstallFixer) Preconditions() []string {
	return []string{"detect-only: duplicate-install resolution requires an operator decision"}
}
func (skillsDuplicateInstallFixer) WritesTo() []string { return nil }
func (skillsDuplicateInstallFixer) Ops() []string      { return nil }
func (skillsDuplicateInstallFixer) Reversible() bool   { return true }
func (skillsDuplicateInstallFixer) Idempotent() bool   { return true }
func (skillsDuplicateInstallFixer) AutoFixable() bool  { return false }

func (f skillsDuplicateInstallFixer) Fix(_ *MutateContext, _ *DetectEnv, _ []Finding) (FixResult, error) {
	err := fmt.Errorf("doctor: %s: detect-only — duplicate install resolution requires an "+
		"operator decision; the doctor cannot determine which install root is authoritative", f.ID())
	return FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}, Fixed: false, Err: err}, err
}
