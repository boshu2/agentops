package provenance

import (
	"encoding/json"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/spf13/cobra"
)

func (m *Module) evidenceCommands() []*cobra.Command {
	names := []string{"snapshot-intent", "manifest", "verify-manifest", "digest", "store-verdict", "verify-verdict", "verify-subject"}
	commands := make([]*cobra.Command, 0, len(names))
	for _, name := range names {
		commands = append(commands, m.evidenceCommand(name))
	}
	return commands
}
func (m *Module) evidenceCommand(name string) *cobra.Command {
	o := &evidenceOptions{}
	cmd := &cobra.Command{Use: name, Short: map[string]string{
		"snapshot-intent": "Store exact immutable intent bytes in an explicit non-Git evidence root",
		"manifest":        "Compute subject-manifest.v1 from declared filesystem paths",
		"verify-manifest": "Recompute and compare exact subject identity",
		"digest":          "Hash a strict JSON object in canonical form",
		"store-verdict":   "Verify and atomically store a supplied verdict.v2 with runtime facts",
		"verify-verdict":  "Structurally verify a content-addressed verdict.v2",
		"verify-subject":  "Check a supplied PASS against exact content and independent expected intent",
	}[name], Args: cobra.NoArgs}
	cmd.Long = cmd.Short + `.

Evidence helper version 1 supports subject-manifest.v1 and verdict.v2.
Unknown/duplicate JSON fields, invalid identity, incompatible helper versions
and invalid paths fail closed. These operations do not run Python, Git, a
tracker or a semantic judge. Exit 0 means the requested mechanical operation
completed; exit 1 means invalid input, failed verification or a filesystem error.

Storage requires an existing explicit --evidence-root outside ordinary, bare
and linked Git repositories, active Git environment storage bindings and declared
--exclude-git-root boundaries, including aliases and descendants. Required
boundaries must resolve before any write. No Git subprocess or Git configuration
lookup is used. Unmarked storage referenced by an unrelated repository cannot be
discovered by ancestry; the caller must declare known boundaries. There is no workspace
fallback or configuration lookup. Files are private, content addressed and
atomically published; identical bytes are idempotent, corrupt verdict addresses
produce a separate NOT_PROVEN integrity artifact without overwriting the original.
--out is relative to --evidence-root and never silently overwrites content.
The caller supplies scope and freshness facts; these are attestations, not a
proof of runtime isolation. --dry-run rejects evidence writes before mutation.`
	flag := func(target *string, key, help string, required bool) {
		cmd.Flags().StringVar(target, key, "", help)
		if required {
			_ = cmd.MarkFlagRequired(key)
		}
	}
	flagRoot := func() { flag(&o.root, "root", "Explicit subject directory", true) }
	flagManifest := func() {
		flag(&o.manifest, "manifest", "subject-manifest.v1 file", true)
		flag(&o.base, "base-manifest", "Base manifest required for deletion identity", false)
	}
	switch name {
	case "snapshot-intent":
		flag(&o.source, "source", "Intent file, or - for stdin", true)
		flag(&o.evidenceRoot, "evidence-root", "Existing explicit non-Git evidence directory", true)
	case "manifest":
		flagRoot()
		cmd.Flags().StringArrayVar(&o.includes, "include", nil, "Declared relative path (repeatable)")
		_ = cmd.MarkFlagRequired("include")
		cmd.Flags().StringArrayVar(&o.excludes, "exclude", nil, "Excluded path or fnmatch pattern (repeatable)")
		flag(&o.base, "base-manifest", "Base manifest for deletions", false)
		flag(&o.metadata, "git-metadata-json", "Descriptive string/null metadata object; excluded from identity", false)
		flag(&o.output, "out", "Optional relative output path inside evidence root", false)
		flag(&o.evidenceRoot, "evidence-root", "Existing non-Git directory required with --out", false)
	case "verify-manifest":
		flagRoot()
		flagManifest()
	case "digest":
		cmd.Use = "digest <json-file>"
		cmd.Args = cobra.ExactArgs(1)
	case "store-verdict":
		flagRoot()
		flag(&o.evidenceRoot, "evidence-root", "Existing explicit non-Git evidence directory", true)
		flag(&o.draft, "draft", "Fresh judge's supplied draft JSON", true)
		flag(&o.intent, "intent-source", "Independently supplied immutable intent file", true)
		flag(&o.manifest, "subject-manifest", "Runtime-derived manifest", true)
		flag(&o.base, "base-manifest", "Base manifest for deletions", false)
		flag(&o.facts.AuthorContextID, "author-context-id", "Runtime author identity", true)
		flag(&o.facts.ValidatorContextID, "validator-context-id", "Fresh validator identity", true)
		flag(&o.facts.FreshnessSource, "freshness-source", "runtime or caller", true)
		flag(&o.facts.FreshnessAttesterID, "freshness-attester-id", "Freshness attester identity", true)
		flag(&o.facts.ScopeResult, "scope-result", "Runtime-derived PASS, FAIL or NOT_PROVEN scope fact", true)
	case "verify-verdict":
		flag(&o.verdict, "verdict", "Content-addressed verdict.v2 file", true)
	case "verify-subject":
		flagRoot()
		flagManifest()
		flag(&o.verdict, "verdict", "Content-addressed supplied verdict.v2 PASS", true)
		flag(&o.intent, "intent", "Independent expected immutable acceptance file", true)
	}
	if name == "snapshot-intent" || name == "manifest" || name == "store-verdict" {
		cmd.Flags().StringArrayVar(&o.excludedGitRoots, "exclude-git-root", nil, "Caller-known existing Git storage root to exclude (repeatable); unresolved roots fail before writes")
	}
	cmd.Flags().StringVar(&o.version, "helper-version", evidence.HelperVersion, "Require evidence helper version; incompatibility fails before mutation")
	cmd.Flags().BoolVar(&o.localJSON, "json", false, "Emit JSON (the default for evidence operations except digest)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return m.runEvidence(cmd, args, name, o) }
	contract := m.Contract()
	contract.ID = "ao.provenance." + name
	contract.Args = clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs}
	contract.Output = clicontract.OutputStructured
	contract.Effects = clicontract.EffectFilesystem
	if name == "snapshot-intent" || name == "manifest" || name == "store-verdict" {
		contract.Effects |= clicontract.EffectEnvironment
	}
	if name == "digest" {
		contract.Args = clicontract.ArgsPolicy{Name: "exact-1", Validate: cobra.ExactArgs(1)}
		contract.Output = clicontract.OutputText
	}
	if err := clicontract.Attach(cmd, contract); err != nil {
		panic(err)
	}
	return cmd
}

type evidenceOptions struct {
	root, evidenceRoot, source, manifest, base, draft, intent, verdict, output, metadata string
	includes, excludes, excludedGitRoots                                                 []string
	facts                                                                                evidence.RuntimeFacts
	localJSON                                                                            bool
	version                                                                              string
}

func (m *Module) runEvidence(cmd *cobra.Command, args []string, name string, o *evidenceOptions) error {
	if err := m.checkEvidenceRequest(cmd, name, o); err != nil {
		return err
	}
	var value any
	var err error
	switch name {
	case "snapshot-intent":
		value, err = evidence.SnapshotSource(o.evidenceRoot, o.source, cmd.InOrStdin(), o.excludedGitRoots...)
	case "manifest":
		value, err = o.buildManifest()
	case "verify-manifest":
		err = o.verifyManifest()
		value = map[string]string{"result": "PASS", "reason": "manifest matches subject"}
	case "digest":
		raw, e := evidence.ReadObject(args[0])
		if e != nil {
			return e
		}
		d, e := evidence.Digest(raw)
		if e != nil {
			return e
		}
		if mode := m.outputMode(); !o.localJSON && (mode == "" || mode == "table") {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), d)
			return err
		}
		value = map[string]string{"digest": d}
	case "store-verdict":
		if o.facts.ScopeResult != "PASS" && o.facts.ScopeResult != "FAIL" && o.facts.ScopeResult != "NOT_PROVEN" {
			return fmt.Errorf("invalid --scope-result")
		}
		if o.facts.FreshnessSource != "runtime" && o.facts.FreshnessSource != "caller" {
			return fmt.Errorf("invalid --freshness-source")
		}
		value, err = evidence.StoreVerdict(evidence.StoreOptions{ExcludedGitRoots: o.excludedGitRoots, EvidenceRoot: o.evidenceRoot, Root: o.root, Draft: o.draft, IntentSource: o.intent, SubjectManifest: o.manifest, BaseManifest: o.base, Facts: o.facts})
	case "verify-verdict":
		err = evidence.VerifyStored(o.verdict)
		value = map[string]string{"result": "PASS", "reason": "verdict structure and digest match"}
	case "verify-subject":
		err = evidence.VerifySubject(o.root, o.manifest, o.base, o.verdict, o.intent)
		value = map[string]string{"result": "PASS", "reason": "exact subject and independently supplied intent match the supplied PASS"}
	}
	if err != nil {
		return err
	}
	if !o.localJSON && m.outputMode() == "yaml" {
		return clicontract.WriteYAML(cmd.OutOrStdout(), value)
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (o *evidenceOptions) verifyManifest() error {
	subject, err := evidence.LoadManifest(o.manifest)
	if err != nil {
		return err
	}
	var prior *evidence.Manifest
	if o.base != "" {
		prior, err = evidence.LoadManifest(o.base)
		if err != nil {
			return err
		}
	}
	return evidence.VerifyManifest(o.root, subject, prior)
}

func (o *evidenceOptions) buildManifest() (value any, err error) {
	var prior *evidence.Manifest
	if o.base != "" {
		prior, err = evidence.LoadManifest(o.base)
		if err != nil {
			return nil, err
		}
	}
	var meta map[string]*string
	if o.metadata != "" { // strict object parsing also rejects duplicate metadata keys
		raw, e := evidence.ParseMetadata(o.metadata)
		if e != nil {
			return nil, e
		}
		meta = raw
	}
	value, err = evidence.BuildManifest(o.root, o.includes, o.excludes, prior, meta)
	if err == nil && o.output != "" {
		_, err = evidence.StoreDocument(o.evidenceRoot, o.output, value, o.excludedGitRoots...)
	}
	return value, err
}

func (m *Module) checkEvidenceRequest(cmd *cobra.Command, name string, o *evidenceOptions) error {
	cmd.SilenceUsage = true
	// The leaf also works without a composed root, so it owns --json.
	// Honor the root's explicit format-conflict policy before any effects.
	formatFlag := cmd.Root().PersistentFlags().Lookup("output")
	if o.localJSON && formatFlag != nil && formatFlag.Changed && m.outputMode() != "json" {
		return fmt.Errorf("conflicting output formats: --json requests json while --output requests %s", m.outputMode())
	}
	if o.version != evidence.HelperVersion {
		return fmt.Errorf("incompatible evidence helper version %q; supported: %s", o.version, evidence.HelperVersion)
	}
	writes := name == "snapshot-intent" || name == "store-verdict" || (name == "manifest" && o.output != "")
	if writes && m.host.DryRun != nil && m.host.DryRun() {
		return fmt.Errorf("evidence writes are disabled by --dry-run")
	}
	return nil
}
