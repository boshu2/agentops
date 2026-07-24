package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestOperationInventoryCoversEveryAdvertisedRunnable(t *testing.T) {
	if err := applyOperationContracts(rootCmd); err != nil {
		t.Fatalf("operation inventory is incomplete: %v", err)
	}
	for path, contract := range commandOperationContracts {
		if err := clicontract.ValidateOperationContract(contract); err != nil {
			t.Errorf("%s: invalid operation contract: %v", path, err)
		}
		if clicontract.RequiresDryRunControl(contract) &&
			contract.DryRun != clicontract.DryRunRejects &&
			contract.DryRun != clicontract.DryRunSuppresses {
			t.Errorf("%s: controlled effect has no fail-safe dry-run behavior", path)
		}
	}
}

func TestDryRunRejectsUnsupportedEffectBeforeHandler(t *testing.T) {
	var handled bool
	root := &cobra.Command{
		Use:           "ao",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(command *cobra.Command, _ []string) error {
			return enforceOperationContract(command)
		},
	}
	root.PersistentFlags().Bool("dry-run", false, "")
	command := &cobra.Command{
		Use: "mutate",
		Run: func(*cobra.Command, []string) {
			handled = true
		},
	}
	if err := clicontract.AttachOperation(command, rejectingOperation(textOutput,
		clicontract.EffectFilesystemWrite, clicontract.EffectProcessStart)); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(command)
	root.SetArgs([]string{"mutate", "--dry-run"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "does not support --dry-run") {
		t.Fatalf("unsupported dry-run error = %v", err)
	}
	if handled {
		t.Fatal("effectful handler ran after dry-run rejection")
	}
}

func TestEvalFileOutputFlagsDoNotShadowGlobalOutput(t *testing.T) {
	evalCommand := newEvalCommand()
	for _, path := range []string{"scenario-ab", "scenario-moat"} {
		command, _, err := evalCommand.Find([]string{path})
		if err != nil {
			t.Fatalf("find eval %s: %v", path, err)
		}
		if flag := command.LocalNonPersistentFlags().Lookup("output"); flag != nil {
			t.Errorf("eval %s still shadows global --output", path)
		}
		if flag := command.LocalNonPersistentFlags().Lookup("out"); flag == nil {
			t.Errorf("eval %s has no --out file destination", path)
		}
	}
}

func TestAdvertisedReadJSONFormsAreEquivalent(t *testing.T) {
	type probe struct {
		prepare func(*testing.T, string) commandProbe
		reason  string
	}
	repoProbe := func(args ...string) func(*testing.T, string) commandProbe {
		return func(t *testing.T, repo string) commandProbe {
			t.Helper()
			return commandProbe{Args: args, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}
	}
	tempProbe := func(args ...string) func(*testing.T, string) commandProbe {
		return func(t *testing.T, _ string) commandProbe {
			t.Helper()
			dir := t.TempDir()
			return commandProbe{Args: args, Dir: dir, Env: map[string]string{"HOME": dir}}
		}
	}
	probes := map[string]probe{
		"ao capabilities":        {prepare: repoProbe("capabilities")},
		"ao config":              {prepare: tempProbe("config")},
		"ao doctor capabilities": {prepare: tempProbe("doctor", "capabilities")},
		"ao doctor diff": {
			reason: "doctor diff resolves target_sha by starting git; its contract mechanically declares process.start",
		},
		"ao doctor explain": {prepare: prepareDoctorExplainProbe},
		"ao doctor health":  {prepare: tempProbe("doctor", "health")},
		"ao doctor ls":      {prepare: tempProbe("doctor", "ls")},
		"ao eval baseline-audit": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"eval", "baseline-audit", "--root", filepath.Join(repo, "evals", "agentops-core")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao eval coverage": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"eval", "coverage", "--root", filepath.Join(repo, "evals", "agentops-core")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao eval outcomes compile": {prepare: prepareOutcomesCompileProbe},
		"ao eval scenario list":    {prepare: tempProbe("eval", "scenario", "list")},
		"ao eval suite n-required": {
			reason: "n-required delegates statistical computation to Python; its contract mechanically declares process.start",
		},
		"ao eval suite verdict": {
			reason: "suite verdict delegates bootstrap computation to Python; its contract mechanically declares process.start",
		},
		"ao eval task list": {prepare: func(t *testing.T, _ string) commandProbe {
			dir := t.TempDir()
			return commandProbe{Args: []string{"eval", "task", "list"}, Dir: dir, Env: map[string]string{"HOME": dir, "AGENTOPS_EVALS_ROOT": filepath.Join(dir, "evals")}}
		}},
		"ao eval task show":   {prepare: prepareTaskShowProbe},
		"ao flywheel compare": {prepare: tempProbe("flywheel", "compare")},
		"ao flywheel status":  {prepare: tempProbe("flywheel", "status")},
		"ao goals history":    {prepare: tempProbe("goals", "history")},
		"ao goals scenarios": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"goals", "scenarios", "--file", filepath.Join(repo, "GOALS.md")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao goals validate": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"goals", "validate", "--file", filepath.Join(repo, "GOALS.md")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao provenance export":   {prepare: repoProbe("provenance", "export")},
		"ao provenance list":     {prepare: repoProbe("provenance", "list")},
		"ao provenance position": {prepare: repoProbe("provenance", "position")},
		"ao provenance show":     {prepare: prepareProvenanceShowProbe},
		"ao provenance trace": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"provenance", "trace", "--orphans", "--graph", filepath.Join(repo, "tests", "fixtures", "provenance", "orphan-stale-65-jobs.jsonl")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao provenance verify": {prepare: repoProbe("provenance", "verify")},
		"ao session bootstrap": {prepare: tempProbe("session", "bootstrap")},
		"ao session rehydrate": {prepare: tempProbe("session", "rehydrate")},
		"ao skills check":      {prepare: repoProbe("skills", "check")},
		"ao skills consumers":  {prepare: repoProbe("skills", "consumers", "rpi")},
		"ao skills find":       {prepare: repoProbe("skills", "find", "rpi")},
		"ao skills graph":      {prepare: repoProbe("skills", "graph")},
		"ao skills list":       {prepare: repoProbe("skills", "list")},
		"ao skills producers":  {prepare: repoProbe("skills", "producers", "verdict-ledger")},
		"ao skills resolve":    {prepare: repoProbe("skills", "resolve")},
		"ao status":            {prepare: tempProbe("status")},
		"ao version":           {prepare: tempProbe("version")},
	}

	var advertised []string
	for path, contract := range commandOperationContracts {
		if contract.ReadOnly && clicontract.SupportsOutput(contract, clicontract.FormatJSON) {
			advertised = append(advertised, path)
		}
	}
	sort.Strings(advertised)
	for path := range probes {
		contract, ok := commandOperationContracts[path]
		if !ok || !contract.ReadOnly || !clicontract.SupportsOutput(contract, clicontract.FormatJSON) {
			t.Errorf("JSON probe is stale or not advertised by a read-only command: %s", path)
		}
	}
	if len(advertised) != len(probes) {
		t.Errorf("read-only JSON inventory has %d commands but probe inventory has %d", len(advertised), len(probes))
	}

	bin := aoBinary(t)
	repo := findRepoRoot(t)
	for _, path := range advertised {
		entry, ok := probes[path]
		if !ok {
			t.Errorf("%s advertises read-only JSON but has no equivalence probe or reason", path)
			continue
		}
		t.Run(strings.TrimPrefix(path, "ao "), func(t *testing.T) {
			contract := commandOperationContracts[path]
			if entry.reason != "" {
				if entry.prepare != nil {
					t.Fatal("reason-only JSON probe also has an executable setup")
				}
				if !hasOperationEffect(contract, clicontract.EffectProcessStart) {
					t.Fatalf("reason-only JSON probe is allowed only for a declared process.start effect: %s", entry.reason)
				}
				return
			}
			if entry.prepare == nil {
				t.Fatal("JSON probe has neither executable setup nor reason")
			}
			probe := entry.prepare(t, repo)
			prefixJSON := probe
			prefixJSON.Args = append([]string{"--json"}, prefixJSON.Args...)
			prefixOutput := probe
			prefixOutput.Args = append([]string{"-o", "json"}, prefixOutput.Args...)
			results := []struct {
				label  string
				result commandProbeResult
			}{
				{label: "suffix --json", result: runCommandProbe(t, bin, probe, "--json")},
				{label: "prefix --json", result: runCommandProbe(t, bin, prefixJSON)},
				{label: "suffix -o json", result: runCommandProbe(t, bin, probe, "-o", "json")},
				{label: "prefix -o json", result: runCommandProbe(t, bin, prefixOutput)},
			}
			var baseline any
			for index, candidate := range results {
				if candidate.result.ExitCode != results[0].result.ExitCode {
					t.Fatalf("%s exited %d, want %d\nstderr:\n%s",
						candidate.label, candidate.result.ExitCode, results[0].result.ExitCode, candidate.result.Stderr)
				}
				var decoded any
				if err := json.Unmarshal(candidate.result.Stdout, &decoded); err != nil {
					t.Fatalf("%s emitted invalid JSON: %v\nstdout:\n%s\nstderr:\n%s",
						candidate.label, err, candidate.result.Stdout, candidate.result.Stderr)
				}
				normalizeVolatileJSON(decoded)
				if index == 0 {
					baseline = decoded
				} else if !reflect.DeepEqual(baseline, decoded) {
					t.Fatalf("%s differs from suffix --json\n%s:\n%s\nsuffix --json:\n%s",
						candidate.label, candidate.label, candidate.result.Stdout, results[0].result.Stdout)
				}
			}
		})
	}
}

func TestJSONFlagCollisionInventoryIsCovered(t *testing.T) {
	type collisionProbe struct {
		prepare   func(*testing.T, string) commandProbe
		freshEach bool
	}
	repoProbe := func(args ...string) func(*testing.T, string) commandProbe {
		return func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: args, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}
	}
	effectful := map[string]collisionProbe{
		"ao doctor": {prepare: func(t *testing.T, _ string) commandProbe {
			dir := t.TempDir()
			return commandProbe{Args: []string{"doctor"}, Dir: dir, Env: map[string]string{"HOME": dir, "PATH": filepath.Join(dir, "no-programs")}}
		}},
		"ao eval scenario evaluate": {prepare: func(t *testing.T, _ string) commandProbe {
			dir := t.TempDir()
			mustWriteFile(t, filepath.Join(dir, "GOALS.md"), []byte("# Goals\n\n## Directives\n\n### 1. Fixture\n\n**Directive ID:** d-fixture\n"))
			return commandProbe{Args: []string{"eval", "scenario", "evaluate", "--all"}, Dir: dir, Env: map[string]string{"HOME": dir}}
		}},
		"ao gate check": {prepare: repoProbe("gate", "check", "--scope", "range:HEAD..HEAD")},
		"ao provenance add": {freshEach: true, prepare: func(t *testing.T, _ string) commandProbe {
			dir := t.TempDir()
			return commandProbe{
				Args: []string{"provenance", "add", "ag-x31t.4", "cli/cmd/ao/provenance_add.go", "--relation", "wasGeneratedBy", "--ts", "2026-07-24T00:00:00Z"},
				Dir:  dir, Env: map[string]string{"HOME": dir},
			}
		}},
		"ao skills link": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"skills", "link", "--dest", filepath.Join(t.TempDir(), "skills")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao skills unlink": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"skills", "unlink", "--dest", filepath.Join(t.TempDir(), "skills")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao workflows link": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"workflows", "link", "--into", filepath.Join(t.TempDir(), "workflows")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
		"ao workflows unlink": {prepare: func(t *testing.T, repo string) commandProbe {
			return commandProbe{Args: []string{"workflows", "unlink", "--into", filepath.Join(t.TempDir(), "workflows")}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
		}},
	}

	if err := applyOperationContracts(rootCmd); err != nil {
		t.Fatal(err)
	}
	var collisions []string
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, command := range parent.Commands() {
			if command.Runnable() && command.LocalNonPersistentFlags().Lookup("json") != nil {
				collisions = append(collisions, command.CommandPath())
			}
			walk(command)
		}
	}
	walk(rootCmd)
	sort.Strings(collisions)
	for path := range effectful {
		if !containsString(collisions, path) {
			t.Errorf("effectful JSON collision probe is stale: %s", path)
		}
	}
	for _, path := range collisions {
		contract := commandOperationContracts[path]
		if contract.ReadOnly {
			if !clicontract.SupportsOutput(contract, clicontract.FormatJSON) {
				t.Errorf("read-only JSON collision does not advertise JSON: %s", path)
			}
			// TestAdvertisedReadJSONFormsAreEquivalent derives every read-only
			// JSON command and executes both prefix/suffix forms.
			continue
		}
		if _, ok := effectful[path]; !ok {
			t.Errorf("effectful local/root JSON collision has no emitted-equivalence probe: %s", path)
		}
	}

	bin := aoBinary(t)
	repo := findRepoRoot(t)
	for _, path := range collisions {
		entry, ok := effectful[path]
		if !ok {
			continue
		}
		t.Run(strings.TrimPrefix(path, "ao "), func(t *testing.T) {
			base := entry.prepare(t, repo)
			run := func(prefix []string, suffix []string) commandProbeResult {
				probe := base
				if entry.freshEach {
					probe = entry.prepare(t, repo)
				}
				args := append(append([]string(nil), prefix...), probe.Args...)
				probe.Args = append(args, suffix...)
				return runCommandProbe(t, bin, probe)
			}
			dryRun := commandOperationContracts[path].DryRun == clicontract.DryRunSuppresses
			basePrefix := []string(nil)
			if dryRun {
				basePrefix = []string{"--dry-run"}
			}
			results := []struct {
				label  string
				result commandProbeResult
			}{
				{label: "prefix --json", result: run(append(append([]string(nil), basePrefix...), "--json"), nil)},
				{label: "suffix --json", result: run(basePrefix, []string{"--json"})},
				{label: "prefix -o json", result: run(append(append([]string(nil), basePrefix...), "-o", "json"), nil)},
				{label: "suffix -o json", result: run(basePrefix, []string{"-o", "json"})},
			}
			var baseline any
			for index, candidate := range results {
				if candidate.result.ExitCode != results[0].result.ExitCode {
					t.Fatalf("%s exited %d, want %d\nstdout:\n%s\nstderr:\n%s",
						candidate.label, candidate.result.ExitCode, results[0].result.ExitCode,
						candidate.result.Stdout, candidate.result.Stderr)
				}
				var decoded any
				if err := json.Unmarshal(candidate.result.Stdout, &decoded); err != nil {
					t.Fatalf("%s emitted invalid JSON: %v\nstdout:\n%s\nstderr:\n%s",
						candidate.label, err, candidate.result.Stdout, candidate.result.Stderr)
				}
				normalizeVolatileJSON(decoded)
				if index == 0 {
					baseline = decoded
				} else if !reflect.DeepEqual(baseline, decoded) {
					t.Fatalf("%s differs from prefix --json\n%s:\n%s\nprefix --json:\n%s",
						candidate.label, candidate.label, candidate.result.Stdout, results[0].result.Stdout)
				}
			}
		})
	}
}

func TestMineSessionGlobalJSONFormsEmitOneArray(t *testing.T) {
	dir := t.TempDir()
	session := filepath.Join(dir, "session.jsonl")
	mustWriteFile(t, session, []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{}}]}}
{"type":"tool_use","tool_name":"Bash","tool_input":{"command":"pwd"}}
`))
	base := commandProbe{
		Args: []string{"provenance", "mine-session", "--file", session},
		Dir:  dir, Env: map[string]string{"HOME": dir},
	}
	bin := aoBinary(t)
	jsonl := runCommandProbe(t, bin, base)
	if jsonl.ExitCode != 0 {
		t.Fatalf("default JSONL failed\nstdout:\n%s\nstderr:\n%s", jsonl.Stdout, jsonl.Stderr)
	}
	if json.Valid(jsonl.Stdout) {
		t.Fatalf("default mine-session unexpectedly stopped being JSONL:\n%s", jsonl.Stdout)
	}

	prefixJSON := base
	prefixJSON.Args = append([]string{"--json"}, prefixJSON.Args...)
	prefixOutput := base
	prefixOutput.Args = append([]string{"-o", "json"}, prefixOutput.Args...)
	results := []commandProbeResult{
		runCommandProbe(t, bin, base, "--json"),
		runCommandProbe(t, bin, prefixJSON),
		runCommandProbe(t, bin, base, "-o", "json"),
		runCommandProbe(t, bin, prefixOutput),
	}
	var baseline any
	for index, result := range results {
		if result.ExitCode != 0 {
			t.Fatalf("JSON form %d exited %d\nstdout:\n%s\nstderr:\n%s", index, result.ExitCode, result.Stdout, result.Stderr)
		}
		var decoded any
		if err := json.Unmarshal(result.Stdout, &decoded); err != nil {
			t.Fatalf("JSON form %d emitted invalid array: %v\n%s", index, err, result.Stdout)
		}
		values, ok := decoded.([]any)
		if !ok || len(values) != 2 {
			t.Fatalf("JSON form %d = %#v, want two-event array", index, decoded)
		}
		if index == 0 {
			baseline = decoded
		} else if !reflect.DeepEqual(baseline, decoded) {
			t.Fatalf("JSON form %d differs from suffix --json", index)
		}
	}
}

func TestEverySuppressingDryRunHasNoDeclaredEffects(t *testing.T) {
	type dryRunProbe struct {
		prepare func(*testing.T, string) effectProbe
	}
	probes := map[string]dryRunProbe{
		"ao doctor gc": {prepare: func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			mustMkdirAll(t, filepath.Join(dir, ".doctor", "runs"))
			return effectProbe{Command: commandProbe{Args: []string{"doctor", "gc", "--before", "2030-01-01", "--yes"}, Dir: dir, Env: map[string]string{"HOME": dir}}, Targets: []string{dir}}
		}},
		"ao doctor undo": {prepare: func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			runDir := filepath.Join(dir, ".doctor", "runs", "run-1")
			mustMkdirAll(t, runDir)
			mustWriteFile(t, filepath.Join(runDir, "actions.jsonl"), nil)
			return effectProbe{Command: commandProbe{Args: []string{"doctor", "undo", "run-1"}, Dir: dir, Env: map[string]string{"HOME": dir}}, Targets: []string{dir}}
		}},
		"ao init": {prepare: func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			return effectProbe{Command: commandProbe{Args: []string{"init"}, Dir: dir, Env: map[string]string{"HOME": dir}}, Targets: []string{dir}}
		}},
		"ao session handoff": {prepare: func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			return effectProbe{Command: commandProbe{Args: []string{"session", "handoff", "summary", "--collect"}, Dir: dir, Env: map[string]string{"HOME": dir, "PATH": filepath.Join(dir, "no-programs")}}, Targets: []string{dir}}
		}},
		"ao skills link": {prepare: func(t *testing.T, repo string) effectProbe {
			dest := filepath.Join(t.TempDir(), "skills")
			return effectProbe{Command: commandProbe{Args: []string{"skills", "link", "--dest", dest}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}, Targets: []string{dest}}
		}},
		"ao skills unlink": {prepare: func(t *testing.T, repo string) effectProbe {
			dest := filepath.Join(t.TempDir(), "skills")
			return effectProbe{Command: commandProbe{Args: []string{"skills", "unlink", "--dest", dest}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}, Targets: []string{dest}}
		}},
		"ao workflows link": {prepare: func(t *testing.T, repo string) effectProbe {
			dest := filepath.Join(repo, ".claude", "workflows")
			return effectProbe{Command: commandProbe{Args: []string{"workflows", "link"}, Dir: repo, Env: map[string]string{"HOME": t.TempDir(), "PATH": filepath.Join(t.TempDir(), "no-programs")}}, Targets: []string{dest}}
		}},
		"ao workflows unlink": {prepare: func(t *testing.T, repo string) effectProbe {
			dest := filepath.Join(repo, ".claude", "workflows")
			return effectProbe{Command: commandProbe{Args: []string{"workflows", "unlink"}, Dir: repo, Env: map[string]string{"HOME": t.TempDir(), "PATH": filepath.Join(t.TempDir(), "no-programs")}}, Targets: []string{dest}}
		}},
	}

	var suppressing []string
	for path, contract := range commandOperationContracts {
		if contract.DryRun == clicontract.DryRunSuppresses {
			suppressing = append(suppressing, path)
		}
	}
	sort.Strings(suppressing)
	for path := range probes {
		if contract, ok := commandOperationContracts[path]; !ok || contract.DryRun != clicontract.DryRunSuppresses {
			t.Errorf("dry-run probe is stale or command no longer suppresses effects: %s", path)
		}
	}
	if len(suppressing) != len(probes) {
		t.Errorf("suppressing inventory has %d commands but probe inventory has %d", len(suppressing), len(probes))
	}

	bin := aoBinary(t)
	repo := findRepoRoot(t)
	for _, path := range suppressing {
		entry, ok := probes[path]
		if !ok {
			t.Errorf("%s suppresses effects but has no no-effect proof", path)
			continue
		}
		t.Run(strings.TrimPrefix(path, "ao "), func(t *testing.T) {
			probe := entry.prepare(t, repo)
			before := snapshotTargets(t, probe.Targets)
			dryCommand := probe.Command
			dryCommand.Args = append([]string{"--dry-run"}, dryCommand.Args...)
			result := runCommandProbe(t, bin, dryCommand)
			if result.ExitCode != 0 {
				t.Fatalf("dry-run exited %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
			}
			after := snapshotTargets(t, probe.Targets)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("dry-run changed an owned target\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

func TestDryRunFlagCollisionsHonorPrefixAndSuffix(t *testing.T) {
	prepare := map[string]func(*testing.T, string) effectProbe{
		"ao doctor": func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			return effectProbe{Command: commandProbe{Args: []string{"doctor"}, Dir: dir, Env: map[string]string{"HOME": dir, "PATH": filepath.Join(dir, "no-programs")}}, Targets: []string{dir}}
		},
		"ao doctor undo": func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			runDir := filepath.Join(dir, ".doctor", "runs", "run-1")
			mustMkdirAll(t, runDir)
			mustWriteFile(t, filepath.Join(runDir, "actions.jsonl"), nil)
			return effectProbe{Command: commandProbe{Args: []string{"doctor", "undo", "run-1"}, Dir: dir, Env: map[string]string{"HOME": dir}}, Targets: []string{dir}}
		},
		"ao eval cleanup": func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			root := filepath.Join(dir, "evals")
			mustMkdirAll(t, root)
			return effectProbe{Command: commandProbe{Args: []string{"eval", "cleanup"}, Dir: dir, Env: map[string]string{"HOME": dir, "AGENTOPS_EVALS_ROOT": root}}, Targets: []string{root}}
		},
		"ao eval task run": func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			root := filepath.Join(dir, "evals")
			mustMkdirAll(t, root)
			return effectProbe{Command: commandProbe{Args: []string{"eval", "task", "run", "task-1"}, Dir: dir, Env: map[string]string{"HOME": dir, "AGENTOPS_EVALS_ROOT": root}}, Targets: []string{root}}
		},
		"ao session handoff": func(t *testing.T, _ string) effectProbe {
			dir := t.TempDir()
			return effectProbe{Command: commandProbe{Args: []string{"session", "handoff", "summary", "--collect"}, Dir: dir, Env: map[string]string{"HOME": dir, "PATH": filepath.Join(dir, "no-programs")}}, Targets: []string{dir}}
		},
	}

	if err := applyOperationContracts(rootCmd); err != nil {
		t.Fatal(err)
	}
	var collisions []string
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, command := range parent.Commands() {
			if command.Runnable() && command.LocalNonPersistentFlags().Lookup("dry-run") != nil {
				collisions = append(collisions, command.CommandPath())
			}
			walk(command)
		}
	}
	walk(rootCmd)
	sort.Strings(collisions)
	for path := range prepare {
		if !containsString(collisions, path) {
			t.Errorf("dry-run collision probe is stale: %s", path)
		}
	}
	if len(collisions) != len(prepare) {
		t.Errorf("found %d local/root dry-run collisions but have %d probes: %v", len(collisions), len(prepare), collisions)
	}

	bin := aoBinary(t)
	repo := findRepoRoot(t)
	for _, path := range collisions {
		setup, ok := prepare[path]
		if !ok {
			t.Errorf("dry-run collision has no prefix/suffix probe: %s", path)
			continue
		}
		t.Run(strings.TrimPrefix(path, "ao "), func(t *testing.T) {
			contract := commandOperationContracts[path]
			for _, placement := range []string{"prefix", "suffix"} {
				t.Run(placement, func(t *testing.T) {
					probe := setup(t, repo)
					before := snapshotTargets(t, probe.Targets)
					command := probe.Command
					if placement == "prefix" {
						command.Args = append([]string{"--dry-run"}, command.Args...)
					} else {
						command.Args = append(command.Args, "--dry-run")
					}
					result := runCommandProbe(t, bin, command)
					switch contract.DryRun {
					case clicontract.DryRunRejects:
						if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "does not support --dry-run") {
							t.Fatalf("%s dry-run did not fail closed\nexit=%d\nstdout:\n%s\nstderr:\n%s",
								placement, result.ExitCode, result.Stdout, result.Stderr)
						}
					case clicontract.DryRunSuppresses:
						if result.ExitCode != 0 {
							t.Fatalf("%s dry-run exited %d\nstdout:\n%s\nstderr:\n%s",
								placement, result.ExitCode, result.Stdout, result.Stderr)
						}
					default:
						t.Fatalf("colliding effectful command has unsafe dry-run policy %q", contract.DryRun)
					}
					after := snapshotTargets(t, probe.Targets)
					if !reflect.DeepEqual(before, after) {
						t.Fatalf("%s dry-run changed an owned target\nbefore: %#v\nafter: %#v", placement, before, after)
					}
				})
			}
		})
	}
}

func TestHiddenRunnableCommandsAreFailClosed(t *testing.T) {
	if err := applyOperationContracts(rootCmd); err != nil {
		t.Fatal(err)
	}
	var hidden []string
	var walk func(*cobra.Command, bool)
	walk = func(parent *cobra.Command, hiddenAncestor bool) {
		for _, command := range parent.Commands() {
			isHidden := hiddenAncestor || command.Hidden
			if isHidden && command.Runnable() && command.Name() != "help" && command.Name() != "completion" {
				hidden = append(hidden, command.CommandPath())
				if _, attached := clicontract.OperationFor(command); attached {
					t.Errorf("hidden runnable %s unexpectedly has an advertised operation contract", command.CommandPath())
				}
				err := enforceOperationContract(command)
				if err == nil || !strings.Contains(err.Error(), "no executable effect/output contract") {
					t.Errorf("hidden runnable %s was not explicitly refused: %v", command.CommandPath(), err)
				}
			}
			walk(command, isHidden)
		}
	}
	walk(rootCmd, false)
	sort.Strings(hidden)

	// Keep the fail-closed policy executable even while the shipped inventory
	// contains no hidden runnable leaves.
	synthetic := &cobra.Command{Use: "hidden", Hidden: true, Run: func(*cobra.Command, []string) {}}
	host := &cobra.Command{Use: "ao"}
	host.AddCommand(synthetic)
	if err := enforceOperationContract(synthetic); err == nil ||
		!strings.Contains(err.Error(), "no executable effect/output contract") {
		t.Fatalf("synthetic hidden runnable was not explicitly refused: %v", err)
	}
}

type commandProbe struct {
	Args []string
	Dir  string
	Env  map[string]string
}

type commandProbeResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type effectProbe struct {
	Command commandProbe
	Targets []string
}

func runCommandProbe(t *testing.T, bin string, probe commandProbe, trailing ...string) commandProbeResult {
	t.Helper()
	args := append(append([]string(nil), probe.Args...), trailing...)
	command := exec.Command(bin, args...)
	command.Dir = probe.Dir
	command.Env = environmentWith(probe.Env)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("start %v: %v", args, err)
		}
		exitCode = exitErr.ExitCode()
	}
	return commandProbeResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode}
}

func environmentWith(overrides map[string]string) []string {
	keys := make(map[string]bool, len(overrides))
	for key := range overrides {
		keys[key] = true
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !keys[key] {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func prepareDoctorExplainProbe(t *testing.T, _ string) commandProbe {
	t.Helper()
	dir := t.TempDir()
	runDir := filepath.Join(dir, ".doctor", "runs", "run-1")
	mustMkdirAll(t, runDir)
	report := map[string]any{
		"schema_version": "doctor-report.v1",
		"findings": []map[string]any{{
			"id": "f1", "severity": "P1", "subsystem": "test", "title": "fixture",
			"confidence": 1,
			"evidence":   map[string]any{},
			"remediation": map[string]any{
				"command": "inspect", "auto_fixable": false, "estimated_actions": 0,
			},
		}},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(runDir, "report.json"), raw)
	mustMkdirAll(t, filepath.Join(dir, ".doctor"))
	if err := os.Symlink(filepath.Join("runs", "run-1"), filepath.Join(dir, ".doctor", "latest")); err != nil {
		t.Fatal(err)
	}
	return commandProbe{Args: []string{"doctor", "explain", "f1"}, Dir: dir, Env: map[string]string{"HOME": dir}}
}

func prepareOutcomesCompileProbe(t *testing.T, _ string) commandProbe {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.json")
	mustWriteFile(t, path, []byte(`{
  "task": {"id":"task-1","description":"fixture","stats":{"min_n_samples":3}},
  "criteria": [{"id":"c1","description":"criterion","weight":1}],
  "judge_content_hash": "hash"
}`))
	return commandProbe{Args: []string{"eval", "outcomes", "compile", path}, Dir: dir, Env: map[string]string{"HOME": dir}}
}

func prepareTaskShowProbe(t *testing.T, _ string) commandProbe {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "evals")
	path := filepath.Join(root, "tasks", "task-1", "task.yaml")
	mustMkdirAll(t, filepath.Dir(path))
	mustWriteFile(t, path, []byte("id: task-1\nschema_version: 1\ndomain: test\ndescription: fixture\nharness_ref: fixture\nstats:\n  min_n_samples: 3\n"))
	return commandProbe{
		Args: []string{"eval", "task", "show", "task-1"}, Dir: dir,
		Env: map[string]string{"HOME": dir, "AGENTOPS_EVALS_ROOT": root},
	}
}

func prepareProvenanceShowProbe(t *testing.T, repo string) commandProbe {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, "docs", "provenance", "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var first map[string]any
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &first); err != nil {
			t.Fatal(err)
		}
		break
	}
	id, _ := first["from_id"].(string)
	if id == "" {
		t.Fatal("provenance ledger first record has no from_id")
	}
	return commandProbe{Args: []string{"provenance", "show", id}, Dir: repo, Env: map[string]string{"HOME": t.TempDir()}}
}

func normalizeVolatileJSON(value any) {
	volatile := map[string]bool{
		"checked_at": true, "created_at": true, "duration_ms": true, "elapsed_ms": true,
		"finished_at": true, "generated": true, "generated_at": true,
		"period_end": true, "period_start": true, "started_at": true,
		"timestamp": true,
	}
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				if volatile[key] {
					delete(typed, key)
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
}

func hasOperationEffect(contract clicontract.OperationContract, effect clicontract.Effect) bool {
	for _, candidate := range contract.Effects {
		if candidate == effect {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func snapshotTargets(t *testing.T, targets []string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	for index, target := range targets {
		prefix := fmt.Sprintf("%d:", index)
		info, err := os.Lstat(target)
		if os.IsNotExist(err) {
			snapshot[prefix+"<absent>"] = ""
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			snapshot[prefix+"."] = snapshotEntry(t, target, info)
			continue
		}
		err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(target, path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			snapshot[prefix+filepath.ToSlash(rel)] = snapshotEntry(t, path, info)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func snapshotEntry(t *testing.T, path string, info fs.FileInfo) string {
	t.Helper()
	base := info.Mode().String()
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			t.Fatal(err)
		}
		return base + "|link=" + target
	}
	if !info.Mode().IsRegular() {
		return base
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s|size=%d|sha256=%x", base, len(raw), sha256.Sum256(raw))
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHelpCommandReferencesResolve(t *testing.T) {
	var checked []string
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		if command != rootCmd {
			for _, line := range strings.Split(command.Long+"\n"+command.Example, "\n") {
				if ref := commandReferenceLine(line); ref != "" {
					assertCommandReferenceResolves(t, ref)
					checked = append(checked, ref)
				}
			}
		}
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(rootCmd)

	// Flag descriptions are normally fragments, so only inspect descriptions
	// that explicitly name an `ao ...` command.
	var walkFlags func(*cobra.Command)
	walkFlags = func(command *cobra.Command) {
		command.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if index := strings.Index(flag.Usage, "ao "); index >= 0 {
				ref := commandReferenceLine(flag.Usage[index:])
				if ref != "" {
					assertCommandReferenceResolves(t, ref)
					checked = append(checked, ref)
				}
			}
		})
		for _, child := range command.Commands() {
			walkFlags(child)
		}
	}
	walkFlags(rootCmd)
	if len(checked) == 0 {
		t.Fatal("help reference coverage found no executable references")
	}
	sort.Strings(checked)
}

func commandReferenceLine(line string) string {
	line = strings.TrimSpace(strings.Trim(line, "`'\""))
	if !strings.HasPrefix(line, "ao ") {
		return ""
	}
	return line
}

func assertCommandReferenceResolves(t *testing.T, reference string) {
	t.Helper()
	fields := strings.Fields(reference)
	node := rootCmd
	consumed := 1
	for consumed < len(fields) {
		token := strings.Trim(fields[consumed], "`'\";,()")
		if strings.HasPrefix(token, "-") {
			break
		}
		var next *cobra.Command
		for _, child := range node.Commands() {
			if child.Name() == token {
				next = child
				break
			}
			for _, alias := range child.Aliases {
				if alias == token {
					next = child
					break
				}
			}
		}
		if next == nil {
			structural := !node.Runnable() || node.Annotations[groupGuardAnnotation] == "true"
			if structural {
				t.Errorf("help names unregistered command path at %q in %q", token, reference)
			}
			break
		}
		node = next
		consumed++
	}
}
