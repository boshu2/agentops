package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestManifestGlobReversedRange(t *testing.T) {
	// Python fnmatch removes empty ranges before applying class negation.
	for _, name := range []string{"a", "z", "-", "/", "\n", "é"} {
		if !matches(name, "[!z-a]") {
			t.Errorf("matches(%q, [!z-a]) = false; Python fnmatchcase = true", name)
		}
		if matches(name, "[z-a]") {
			t.Errorf("matches(%q, [z-a]) = true; Python fnmatchcase = false", name)
		}
	}
	for _, name := range []string{"", "aa"} {
		if matches(name, "[!z-a]") {
			t.Errorf("negated empty range must consume exactly one character: %q", name)
		}
	}
}

// Python is a developer-only compatibility oracle, never a runtime dependency.
func globPython(t *testing.T, script string, input any, output any) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable; cross-language glob parity not checked")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-B", "-c", script)
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Python oracle: %v", err)
	}
	if err := json.Unmarshal(out, output); err != nil {
		t.Fatalf("Python oracle output: %v", err)
	}
}

func TestManifestGlobPythonClassParity(t *testing.T) {
	patterns := []string{
		"[z-a]", "[!z-a]", "[a-z]", "[!a-z]", "[a-a]", "[!a-a]",
		"[az-a]", "[!az-a]", "[z-ab]", "[!z-ab]", "[a-cz-a]", "[!a-cz-a]",
		"[z-a0-9]", "[!z-a0-9]", "[z-ab-c]", "[!z-ab-c]", "[z-a-z]", "[!z-a-z]",
		"[-az]", "[az-]", "[!--]", "[a--b]", "[!a--b]", "[--a]", "[a-b-c]",
		"[]a]", "[!]]", "[^a]", "[!^a]", "[[]", "[[:alpha:]]",
		"[a&&b]", "[a||b]", "[a~~b]", `[\a]`, "[é-ê]", "[!ê-é]",
		"[", "[]", "[!]", "[!", "prefix[!z-a]suffix", "*[!z-a]?",
	}
	names := []string{"", "a", "b", "c", "m", "z", "0", "9", "-", "!", "^", "[", "]", "&", "|", "~", `\`, "/", "\n", "é", "ê", "aa", "[]", "[!]", "prefixasuffix", "dir/a"}
	var want [][]bool
	globPython(t, `import fnmatch,json,sys
patterns,names=json.load(sys.stdin)
json.dump([[fnmatch.fnmatchcase(n,p) for n in names] for p in patterns],sys.stdout)
`, [2][]string{patterns, names}, &want)
	if len(want) != len(patterns) {
		t.Fatal("incomplete oracle pattern coverage")
	}
	for i, pattern := range patterns {
		if len(want[i]) != len(names) {
			t.Fatal("incomplete oracle name coverage")
		}
		for j, name := range names {
			if got := matches(name, pattern); got != want[i][j] {
				t.Errorf("matches(%q, %q) = %v; Python = %v", name, pattern, got, want[i][j])
			}
		}
	}
}

func TestManifestGlobSelection(t *testing.T) {
	for _, tc := range []struct {
		name, include string
		excludes      []string
		remove        bool
		want          []Entry
	}{
		{"live exclusion", "a", []string{"[!z-a]"}, false, []Entry{}},
		{"base deletion membership", "[!z-a]", nil, true, []Entry{{Path: "a", Kind: "deletion"}}},
		{"base deletion exclusion", ".", []string{"[!z-a]"}, true, []Entry{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "a")
			if err := os.WriteFile(file, []byte("subject\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var base *Manifest
			if tc.remove {
				var err error
				base, err = BuildManifest(root, []string{"."}, nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			got, err := BuildManifest(root, []string{tc.include}, tc.excludes, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Entries, tc.want) {
				t.Errorf("entries = %+v; Python-compatible selection = %+v", got.Entries, tc.want)
			}
			var oracle Manifest
			globPython(t, `import importlib.util,json,sys
from pathlib import Path
d=json.load(sys.stdin)
spec=importlib.util.spec_from_file_location("reference",d["reference"])
m=importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
json.dump(m.build_manifest(Path(d["root"]),d["includes"],d["excludes"] or [],d["base"]),sys.stdout)
`, map[string]any{
				"reference": filepath.Join("..", "..", "..", "skills", "validate", "tests", "validate.py"),
				"root":      root, "includes": []string{tc.include}, "excludes": tc.excludes, "base": base,
			}, &oracle)
			goBytes, err := Canonical(got)
			if err != nil {
				t.Fatal(err)
			}
			pyBytes, err := Canonical(&oracle)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(goBytes, pyBytes) {
				t.Errorf("manifest parity mismatch\nGo: %s\nPython: %s", goBytes, pyBytes)
			}
			if err := VerifyManifest(root, &oracle, base); err != nil {
				t.Errorf("Go rejected Python manifest: %v", err)
			}
		})
	}
}

func TestManifestDeclaredDirectoryExclusionParity(t *testing.T) {
	for _, tc := range []struct{ include, exclude string }{
		{".", "?"}, {".", "[!z-a]"}, {"docs", "d?cs"},
	} {
		t.Run(tc.include+"/"+tc.exclude, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "docs"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"README.md", "docs/guide.md"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("still present\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			base, err := BuildManifest(root, []string{"."}, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := BuildManifest(root, []string{tc.include}, []string{tc.exclude}, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range got.Entries {
				if entry.Kind != "file" {
					t.Errorf("existing file reported as %s: %s", entry.Kind, entry.Path)
				}
			}
			var oracle Manifest
			globPython(t, `import importlib.util,json,sys
from pathlib import Path
d=json.load(sys.stdin)
spec=importlib.util.spec_from_file_location("reference",d["reference"])
m=importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
json.dump(m.build_manifest(Path(d["root"]),d["includes"],d["excludes"],d["base"]),sys.stdout)
`, map[string]any{
				"reference": filepath.Join("..", "..", "..", "skills", "validate", "tests", "validate.py"),
				"root":      root, "includes": []string{tc.include}, "excludes": []string{tc.exclude}, "base": base,
			}, &oracle)
			if !reflect.DeepEqual(got, &oracle) {
				t.Errorf("directory exclusion parity mismatch\nGo: %+v\nPython: %+v", got, &oracle)
			}
			if err := VerifyManifest(root, &oracle, base); err != nil {
				t.Errorf("rejected unchanged live subject: %v", err)
			}
			// A self-consistent address cannot make a false deletion true.
			for i := range got.Entries {
				got.Entries[i].Kind = "deletion"
				got.Entries[i].Digest = ""
			}
			got.Digest, err = manifestDigest(got)
			if err != nil {
				t.Fatal(err)
			}
			if err := VerifyManifest(root, got, base); err == nil {
				t.Error("accepted deletion manifest while all declared files still exist")
			}
		})
	}
}
