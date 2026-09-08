package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// OrphanBinding records a historical binding exposed by a changed path or digest.
// A nil current digest means the legacy reader could not obtain that binding.
type OrphanBinding struct {
	Artifact       string  `json:"artifact"`
	Binds          string  `json:"binds"`
	Cause          string  `json:"cause"`
	CurrentSHA256  *string `json:"current_sha256"`
	RecordedSHA256 string  `json:"recorded_sha256"`
}

type OrphanReceipt struct {
	ArtifactCount int             `json:"artifact_count"`
	BindingCount  int             `json:"binding_count"`
	Changed       []string        `json:"changed"`
	Orphaned      []OrphanBinding `json:"orphaned"`
}

// OrphanBindings preserves the advisory receipt from evidence-orphans.sh. It
// reads only the established artifact kinds, never a ledger or session store.
// Like that reader, this is not a confinement or authorization boundary: bound
// evaluator paths may be absolute or contain parent components, artifact reads
// follow file symlinks, and absent scan directories produce an empty scan.
func OrphanBindings(root string, changed []string) (*OrphanReceipt, error) {
	result := &OrphanReceipt{Changed: []string{}, Orphaned: []OrphanBinding{}}
	// The shell interface stored one path per line, discarding blank lines. Keep
	// that representation (including order and duplicates) for existing callers.
	// Python opened that file with universal newlines, including bare CR.
	changedSet := map[string]bool{}
	for _, input := range changed {
		input = strings.ReplaceAll(strings.ReplaceAll(input, "\r\n", "\n"), "\r", "\n")
		for _, line := range strings.Split(input, "\n") {
			if strings.TrimSpace(line) != "" {
				result.Changed = append(result.Changed, line)
				changedSet[line] = true
			}
		}
	}
	scanner := &orphanScanner{root: root, changed: changedSet, seen: map[[2]string]bool{}, result: result}
	err := walkOrphanArtifacts(filepath.Join(root, "docs", "evals", "scorecards"), func(dir string, names []string) error {
		for _, name := range names {
			if strings.HasSuffix(name, ".json") {
				if err := scanner.scan(filepath.Join(dir, name), []string{"evaluator", "capture_evaluator"}, false); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = walkOrphanArtifacts(filepath.Join(root, "evals", "skill-probes"), func(dir string, names []string) error {
		present := map[string]bool{}
		for _, name := range names {
			present[name] = true
		}
		if present["fixture-set.json"] {
			if err := scanner.scan(filepath.Join(dir, "fixture-set.json"), []string{"capture_evaluator"}, true); err != nil {
				return err
			}
		}
		if present["capture-contract.json"] {
			return scanner.scan(filepath.Join(dir, "capture-contract.json"), nil, true)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	artifacts := map[string]bool{}
	for _, row := range result.Orphaned {
		artifacts[row.Artifact] = true
	}
	result.BindingCount = len(result.Orphaned)
	result.ArtifactCount = len(artifacts)
	return result, nil
}

func orphanRecord(record *orphanObject, label, artifact string) (string, string, error) {
	path, ok := record.values["path"].(string)
	if !ok || path == "" {
		return "", "", fmt.Errorf("malformed %s in %s: path is not a string", label, artifact)
	}
	digest, ok := record.values["sha256"].(string)
	if !ok || digest == "" {
		return "", "", fmt.Errorf("malformed %s in %s: sha256 is not a string", label, artifact)
	}
	return path, digest, nil
}

// Walk in native directory order, with sorted filenames, like os.walk plus
// sorted(filenames). Do not recurse through child directory symlinks.
func walkOrphanArtifacts(root string, visit func(string, []string) error) error {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	var walk func(string) error
	walk = func(dir string) error {
		f, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("cannot walk %s: %w", root, err)
		}
		entries, err := f.ReadDir(-1)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("cannot walk %s: %w", root, err)
		}
		var files, dirs []string
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				dirs = append(dirs, path)
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if target, err := os.Stat(path); err == nil && target.IsDir() {
					continue
				}
			}
			files = append(files, entry.Name())
		}
		sort.Strings(files)
		if err := visit(dir, files); err != nil {
			return err
		}
		for _, child := range dirs {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}

func orphanBoundDigest(root, path string) (result *string, err error) {
	absolute := path
	if !filepath.IsAbs(path) {
		absolute = filepath.Join(root, path)
	}
	info, err := os.Lstat(absolute)
	// Python's islink/isfile checks return false on a stat error. A file which
	// passed the regular-file check but cannot be read instead aborts the scan.
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil
	}
	f, err := os.Open(absolute)
	if err != nil {
		return nil, err
	}
	defer func() { err = joinCleanupError(err, f.Close()) }()
	digest := sha256.New()
	if _, err = io.Copy(digest, f); err != nil {
		return nil, err
	}
	sum := "sha256:" + hex.EncodeToString(digest.Sum(nil))
	return &sum, nil
}

var orphanSkillName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// This is the native counterpart of canonical_skill_path + digest_file, not
// a second artifact validator. All MetadataError equivalents produce nil, as
// the legacy scanner deliberately does for canonical-skill bindings.
func orphanSkillDigest(root, name string) (result *string) {
	if !orphanSkillName.MatchString(name) {
		return nil
	}
	skills := filepath.Join(root, "skills")
	dir := filepath.Join(skills, name)
	for _, p := range []string{skills, dir} {
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
	}
	path := filepath.Join(dir, "SKILL.md")
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() {
		// Like any failed canonical-skill read, a failed close leaves its
		// digest unavailable in the advisory receipt.
		if err := f.Close(); err != nil {
			result = nil
		}
	}()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil
	}
	digest := sha256.New()
	if _, err = io.Copy(digest, f); err != nil {
		return nil
	}
	after, err := f.Stat()
	if err != nil {
		return nil
	}
	named, err := os.Lstat(path)
	if err != nil || !named.Mode().IsRegular() || !os.SameFile(opened, after) || !os.SameFile(opened, named) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil
	}
	sum := "sha256:" + hex.EncodeToString(digest.Sum(nil))
	return &sum
}

// Keep JSON object insertion order and Python json.load's last-value duplicate
// semantics: the first orphaned binding for (artifact,path) wins. The strict
// verdict reader is deliberately not used for these older artifact formats.
type orphanObject struct {
	keys   []string
	values map[string]any
}

func readOrphanJSON(payload []byte) (any, error) {
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("artifact is not UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(orphanLegacyNumbers(payload)))
	decoder.UseNumber()
	var read func() (any, error)
	read = func() (any, error) {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if token == json.Delim('{') {
			object := &orphanObject{values: map[string]any{}}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not text")
				}
				value, err := read()
				if err != nil {
					return nil, err
				}
				if _, exists := object.values[name]; !exists {
					object.keys = append(object.keys, name)
				}
				object.values[name] = value
			}
			_, err = decoder.Token()
			return object, err
		}
		if token == json.Delim('[') {
			values := []any{}
			for decoder.More() {
				value, err := read()
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
			_, err = decoder.Token()
			return values, err
		}
		return token, nil
	}
	value, err := read()
	if err != nil {
		return nil, err
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("trailing JSON data")
		}
		return nil, err
	}
	return value, nil
}

// json.load accepts these nonfinite numeric constants. Artifact consumption
// tests only object/string/null shapes; mapping them to a number preserves
// those checks, including rejecting a numeric binding rather than treating it
// as null. Quoted strings are untouched. No numeric artifact value is emitted.
func orphanLegacyNumbers(input []byte) []byte {
	output := append([]byte(nil), input...)
	quoted, escaped := false, false
	for i := 0; i < len(output); i++ {
		if quoted {
			if escaped {
				escaped = false
			} else if output[i] == '\\' {
				escaped = true
			} else if output[i] == '"' {
				quoted = false
			}
			continue
		}
		if output[i] == '"' {
			quoted = true
			continue
		}
		if i > 0 && !bytes.ContainsRune([]byte("[{: ,\t\r\n"), rune(output[i-1])) {
			continue
		}
		for _, word := range []string{"-Infinity", "Infinity", "NaN"} {
			if bytes.HasPrefix(output[i:], []byte(word)) {
				end := i + len(word)
				if end < len(output) && !bytes.ContainsRune([]byte("]}, \t\r\n"), rune(output[end])) {
					continue
				}
				output[i] = '0'
				for j := i + 1; j < end; j++ {
					output[j] = ' '
				}
				i = end - 1
				break
			}
		}
	}
	return output
}

type orphanScanner struct {
	root    string
	changed map[string]bool
	seen    map[[2]string]bool
	result  *OrphanReceipt
}

func (s *orphanScanner) appendBinding(artifact, path, recorded string, current *string, skill bool) {
	key := [2]string{artifact, path}
	if s.seen[key] {
		return
	}
	drifted := current == nil || *current != recorded
	listed := s.changed[path]
	if !listed && !drifted {
		return
	}
	cause := "digest_drift"
	if skill {
		cause = "skill_changed"
	}
	if listed {
		cause = "changed_path"
		if drifted {
			cause = "both"
		}
	}
	s.seen[key] = true
	s.result.Orphaned = append(s.result.Orphaned, OrphanBinding{artifact, path, cause, current, recorded})
}

func (s *orphanScanner) scan(file string, evaluatorKeys []string, skill bool) error {
	relative, err := filepath.Rel(s.root, file)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	payload, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("malformed JSON in %s: %w", relative, err)
	}
	value, err := readOrphanJSON(payload)
	if err != nil {
		return fmt.Errorf("malformed JSON in %s: %w", relative, err)
	}
	object, ok := value.(*orphanObject)
	if !ok {
		return fmt.Errorf("malformed artifact %s: top level is not an object", relative)
	}
	for _, key := range evaluatorKeys {
		value := object.values[key]
		if value == nil {
			continue
		}
		block, ok := value.(*orphanObject)
		if !ok {
			return fmt.Errorf("malformed %s block in %s: not an object", key, relative)
		}
		for _, category := range block.keys {
			entry, ok := block.values[category].(*orphanObject)
			label := fmt.Sprintf("%s.%s", key, category)
			if !ok {
				return fmt.Errorf("malformed %s in %s: not an object", label, relative)
			}
			path, recorded, err := orphanRecord(entry, label, relative)
			if err != nil {
				return err
			}
			if s.seen[[2]string{relative, path}] {
				continue
			}
			current, err := orphanBoundDigest(s.root, path)
			if err != nil {
				return fmt.Errorf("cannot read bound file %s: %w", path, err)
			}
			s.appendBinding(relative, path, recorded, current, false)
		}
	}
	if !skill || object.values["canonical_skill"] == nil {
		return nil
	}
	record, ok := object.values["canonical_skill"].(*orphanObject)
	if !ok {
		return fmt.Errorf("malformed canonical_skill in %s: not an object", relative)
	}
	name, ok := record.values["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("malformed canonical_skill in %s: name is not a string", relative)
	}
	path, recorded, err := orphanRecord(record, "canonical_skill", relative)
	if err != nil {
		return err
	}
	if !s.seen[[2]string{relative, path}] {
		s.appendBinding(relative, path, recorded, orphanSkillDigest(s.root, name), true)
	}
	return nil
}
