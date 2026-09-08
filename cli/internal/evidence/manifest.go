// Package evidence implements explicit, deterministic evidence mechanics.
// Semantic judgment and configuration resolution belong to the caller.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

const HelperVersion = "1"

type Entry struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Executable bool   `json:"executable"`
	Digest     string `json:"digest,omitempty"`
}
type Manifest struct {
	SchemaVersion string             `json:"schema_version"`
	DeclaredRoots []string           `json:"declared_roots"`
	Exclusions    []string           `json:"exclusions"`
	Entries       []Entry            `json:"entries"`
	BaseDigest    string             `json:"base_manifest_digest,omitempty"`
	Digest        string             `json:"canonical_manifest_digest"`
	GitMetadata   map[string]*string `json:"git_metadata,omitempty"`
}

func Hash(payload []byte) string { sum := sha256.Sum256(payload); return hex.EncodeToString(sum[:]) }
func object(value any) (map[string]any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return verdictcheck.DecodeObject(payload)
}
func Canonical(value any) ([]byte, error) {
	raw, err := object(value)
	if err != nil {
		return nil, err
	}
	return verdictcheck.CanonicalJSON(raw)
}
func Digest(value any) (string, error) {
	b, err := Canonical(value)
	if err != nil {
		return "", err
	}
	return Hash(b), nil
}
func ReadObject(file string) (map[string]any, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return verdictcheck.DecodeObject(b)
}
func manifestDigest(m *Manifest) (string, error) {
	raw, err := object(m)
	if err != nil {
		return "", err
	}
	delete(raw, "canonical_manifest_digest")
	delete(raw, "git_metadata")
	return Digest(raw)
}
func normalize(raw string) (string, error) {
	raw = strings.ReplaceAll(raw, `\`, "/")
	if strings.HasPrefix(raw, "/") || strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("path escapes subject root: %s", raw)
	}
	for _, part := range strings.Split(raw, "/") {
		if part == ".." {
			return "", fmt.Errorf("path escapes subject root: %s", raw)
		}
	}
	return path.Clean(raw), nil
}
func normalized(values []string) ([]string, error) {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		n, err := normalize(value)
		if err != nil {
			return nil, err
		}
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	sort.Strings(result)
	return result, nil
}

// matches follows fnmatchcase: wildcards may match slashes, unlike path.Match.
func matches(name, pattern string) bool {
	if pattern == "." {
		return true
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return name == pattern || strings.HasPrefix(name, strings.TrimRight(pattern, "/")+"/")
	}
	var re strings.Builder
	re.WriteString("(?s)^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			re.WriteString(".*")
		case '?':
			re.WriteByte('.')
		case '[':
			j := i + 1
			if j < len(pattern) && pattern[j] == '!' {
				j++
			}
			if j < len(pattern) && pattern[j] == ']' {
				j++
			}
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			if j == len(pattern) {
				re.WriteString(`\[`)
				continue
			}
			re.WriteString(globClass(pattern[i+1 : j]))
			i = j
		default:
			re.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	re.WriteByte('$')
	ok, err := regexp.MatchString(re.String(), name)
	return err == nil && ok
}

// globClass follows fnmatch's range splitting and removes reversed ranges
// before negation. A negated empty range matches any single character.
func globClass(body string) string {
	runes := []rune(body)
	chunks := [][]rune{}
	start, search := 0, 1
	if runes[0] == '!' {
		search++
	}
	for k := search; k < len(runes); k++ {
		if runes[k] == '-' {
			chunks = append(chunks, runes[start:k])
			start = k + 1
			// A range consumes its right endpoint; the next hyphen is literal.
			k += 2
		}
	}
	if start < len(runes) {
		chunks = append(chunks, runes[start:])
	} else {
		chunks[len(chunks)-1] = append(chunks[len(chunks)-1], '-')
	}
	for k := len(chunks) - 1; k > 0; k-- {
		left, right := chunks[k-1], chunks[k]
		if left[len(left)-1] > right[0] {
			chunks[k-1] = append(left[:len(left)-1], right[1:]...)
			chunks = append(chunks[:k], chunks[k+1:]...)
		}
	}
	if len(chunks) == 1 && len(chunks[0]) == 0 {
		// RE2 has no negative lookahead; this class matches no Unicode rune.
		return `[^\x{0}-\x{10ffff}]`
	}
	negated := len(chunks[0]) > 0 && chunks[0][0] == '!'
	if negated {
		chunks[0] = chunks[0][1:]
		if len(chunks) == 1 && len(chunks[0]) == 0 {
			return "."
		}
	}
	var re strings.Builder
	re.WriteByte('[')
	if negated {
		re.WriteByte('^')
	}
	for i, chunk := range chunks {
		if i > 0 {
			re.WriteByte('-')
		}
		for _, r := range chunk {
			// Hex escapes keep literal hyphens, brackets, carets, backslashes
			// and POSIX-looking class text out of RE2's syntax.
			fmt.Fprintf(&re, `\x{%x}`, r)
		}
	}
	re.WriteByte(']')
	return re.String()
}

func anyMatch(name string, patterns []string) bool {
	for _, p := range patterns {
		if matches(name, p) {
			return true
		}
	}
	return false
}

// Live declared roots are literal paths, matching BuildManifest's traversal.
// Exclusions and legacy base-deletion membership retain pattern semantics.
func inDeclaredRoots(name string, roots []string) bool {
	for _, root := range roots {
		if root == "." || name == root || strings.HasPrefix(name, root+"/") {
			return true
		}
	}
	return false
}

// ParseManifest enforces the unchanged schema plus canonical identity before use.
func ParseManifest(raw map[string]any) (*Manifest, error) {
	if err := verdictcheck.ExactFields(raw, []string{"schema_version", "declared_roots", "exclusions", "entries", "canonical_manifest_digest"}, []string{"base_manifest_digest", "git_metadata"}); err != nil {
		return nil, err
	}
	for _, key := range []string{"declared_roots", "exclusions", "entries"} {
		if _, ok := raw[key].([]any); !ok {
			return nil, fmt.Errorf("manifest %s must be an array", key)
		}
	}
	if err := validateManifestEntries(raw["entries"].([]any)); err != nil {
		return nil, err
	}
	if meta, exists := raw["git_metadata"]; exists {
		if _, ok := meta.(map[string]any); !ok {
			return nil, fmt.Errorf("git_metadata must be an object")
		}
	}
	b, err := verdictcheck.CanonicalJSON(raw)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.SchemaVersion != "subject-manifest.v1" || len(m.DeclaredRoots) == 0 {
		return nil, fmt.Errorf("invalid manifest version or missing declared roots")
	}
	if err := validateManifestPaths(&m); err != nil {
		return nil, err
	}
	if base, ok := raw["base_manifest_digest"]; ok {
		s, ok := base.(string)
		if !ok || !verdictcheck.ValidDigest(s) {
			return nil, fmt.Errorf("base manifest digest invalid")
		}
	}
	identity := map[string]any{}
	for k, v := range raw {
		if k != "canonical_manifest_digest" && k != "git_metadata" {
			identity[k] = v
		}
	}
	d, err := Digest(identity)
	if err != nil || !verdictcheck.ValidDigest(m.Digest) || d != m.Digest {
		return nil, fmt.Errorf("manifest canonical digest is invalid")
	}
	return &m, nil
}
func LoadManifest(file string) (*Manifest, error) {
	raw, err := ReadObject(file)
	if err != nil {
		return nil, err
	}
	return ParseManifest(raw)
}

// BuildManifest never follows a symlink directory, including intermediate
// components of an explicitly declared path. Symlinks bind their target bytes.
func BuildManifest(root string, includes, excludes []string, base *Manifest, metadata map[string]*string) (manifest *Manifest, err error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("subject root required")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { err = joinCleanupError(err, handle.Close()) }()
	declared, err := normalized(includes)
	if err != nil {
		return nil, err
	}
	if len(declared) == 0 {
		return nil, fmt.Errorf("at least one declared root is required")
	}
	excluded, err := normalized(excludes)
	if err != nil {
		return nil, err
	}
	entries := map[string]Entry{}
	if err := collectManifestEntries(handle, declared, excluded, entries); err != nil {
		return nil, err
	}
	m := &Manifest{SchemaVersion: "subject-manifest.v1", DeclaredRoots: declared, Exclusions: excluded, Entries: []Entry{}, GitMetadata: metadata}
	if base != nil {
		raw, err := object(base)
		if err != nil {
			return nil, err
		}
		if _, err = ParseManifest(raw); err != nil {
			return nil, err
		}
		m.BaseDigest = base.Digest
		for _, prior := range base.Entries {
			// Preserve the Python reference's canonical base-deletion membership.
			// Live traversal and file/symlink scope still use literal roots.
			if _, ok := entries[prior.Path]; !ok && anyMatch(prior.Path, declared) && !anyMatch(prior.Path, excluded) {
				entries[prior.Path] = Entry{Path: prior.Path, Kind: "deletion", Executable: prior.Executable}
			}
		}
	}
	for _, e := range entries {
		m.Entries = append(m.Entries, e)
	}
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Path < m.Entries[j].Path })
	m.Digest, err = manifestDigest(m)
	return m, err
}
func VerifyManifest(root string, m, base *Manifest) error {
	raw, err := object(m)
	if err != nil {
		return err
	}
	if _, err = ParseManifest(raw); err != nil {
		return err
	}
	if (m.BaseDigest != "") != (base != nil) || (base != nil && m.BaseDigest != base.Digest) {
		return fmt.Errorf("base manifest mismatch")
	}
	rebuilt, err := BuildManifest(root, m.DeclaredRoots, m.Exclusions, base, m.GitMetadata)
	if err != nil {
		return err
	}
	if rebuilt.Digest != m.Digest {
		return fmt.Errorf("subject content no longer matches manifest")
	}
	return nil
}

func ParseMetadata(text string) (map[string]*string, error) {
	raw, err := verdictcheck.DecodeObject([]byte(text))
	if err != nil {
		return nil, err
	}
	result := map[string]*string{}
	for key, value := range raw {
		if value == nil {
			result[key] = nil
			continue
		}
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("git metadata values must be string or null")
		}
		result[key] = &s
	}
	return result, nil
}

func validateManifestEntries(entries []any) error {
	for _, item := range entries {
		e, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("manifest entry must be an object")
		}
		if err := verdictcheck.ExactFields(e, []string{"path", "kind", "executable"}, []string{"digest"}); err != nil {
			return err
		}
		if _, ok := e["executable"].(bool); !ok {
			return fmt.Errorf("entry executable must be boolean")
		}
		if e["kind"] == "deletion" {
			if _, ok := e["digest"]; ok {
				return fmt.Errorf("deletion has a digest")
			}
		} else {
			d, ok := e["digest"].(string)
			if !ok || !verdictcheck.ValidDigest(d) {
				return fmt.Errorf("entry digest invalid")
			}
		}
	}
	return nil
}

func validateManifestPaths(m *Manifest) error {
	for _, list := range [][]string{m.DeclaredRoots, m.Exclusions} {
		seen := map[string]bool{}
		for _, s := range list {
			n, err := normalize(s)
			if err != nil || s == "" || n != s || seen[s] {
				return fmt.Errorf("noncanonical or duplicate manifest path %q", s)
			}
			seen[s] = true
		}
	}
	seen := map[string]bool{}
	for _, e := range m.Entries {
		n, err := normalize(e.Path)
		if err != nil || n != e.Path || e.Path == "" || seen[e.Path] {
			return fmt.Errorf("invalid/duplicate entry path")
		}
		seen[e.Path] = true
		if e.Kind != "file" && e.Kind != "symlink" && e.Kind != "deletion" {
			return fmt.Errorf("unsupported entry kind")
		}
		inScope := inDeclaredRoots(e.Path, m.DeclaredRoots)
		if e.Kind == "deletion" {
			// Version 1 preserves Python's historical base-deletion membership.
			// VerifyManifest binds the exact base and recomputes every deletion.
			inScope = anyMatch(e.Path, m.DeclaredRoots)
		}
		if !inScope || anyMatch(e.Path, m.Exclusions) {
			return fmt.Errorf("entry outside declared manifest scope")
		}
	}
	return nil
}

func readManifestEntry(handle *os.Root, name string) (Entry, error) {
	info, err := handle.Lstat(name)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{Path: name, Executable: info.Mode().Perm()&0111 != 0}
	var payload []byte
	if info.Mode()&os.ModeSymlink != 0 {
		e.Kind = "symlink"
		target, err := handle.Readlink(name)
		if err != nil {
			return Entry{}, err
		}
		payload = []byte(target)
	} else if info.Mode().IsRegular() {
		e.Kind = "file"
		f, err := handle.Open(name)
		if err != nil {
			return Entry{}, err
		}
		opened, err := f.Stat()
		if err != nil {
			return Entry{}, errors.Join(err, f.Close())
		}
		if !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
			return Entry{}, errors.Join(fmt.Errorf("subject changed while reading"), f.Close())
		}
		payload, err = io.ReadAll(f)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return Entry{}, err
		}
	} else {
		return Entry{}, fmt.Errorf("unsupported subject kind: %s", name)
	}
	after, err := handle.Lstat(name)
	if err != nil {
		return Entry{}, err
	}
	if !os.SameFile(info, after) || info.Mode() != after.Mode() || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return Entry{}, fmt.Errorf("subject changed while reading")
	}
	e.Digest = Hash(payload)
	return e, nil
}

func collectManifestEntries(handle *os.Root, declared, excluded []string, entries map[string]Entry) error {
	for _, decl := range declared {
		parts := strings.Split(decl, "/")
		for i := 1; i < len(parts); i++ {
			info, err := handle.Lstat(strings.Join(parts[:i], "/"))
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("declared path crosses symlink directory")
			}
		}
		err := fs.WalkDir(handle.FS(), decl, func(name string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if name == decl && os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			// The declared directory starts traversal; the historical oracle
			// applies exclusions to its children, not to that starting point.
			if anyMatch(name, excluded) && (name != decl || !d.IsDir()) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			e, err := readManifestEntry(handle, name)
			if err != nil {
				return err
			}
			entries[name] = e
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
