package evalsubstrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

const (
	zwsp   rune = 0x200B
	zwnj   rune = 0x200C
	zwj    rune = 0x200D
	zwnbsp rune = 0xFEFF
)

func CanonicalizeText(in []byte) []byte {
	in = bytes.TrimPrefix(in, []byte{0xEF, 0xBB, 0xBF})
	s := string(in)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case zwsp, zwnj, zwj, zwnbsp:
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.Bytes([]byte(b.String()))
}

func CanonicalizeYAML(in []byte) ([]byte, error) {
	in = CanonicalizeText(in)
	var doc interface{}
	if err := yaml.Unmarshal(in, &doc); err != nil {
		return nil, fmt.Errorf("canonicalize yaml: parse: %w", err)
	}
	doc = sortMap(doc)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("canonicalize yaml: encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("canonicalize yaml: close encoder: %w", err)
	}
	out := buf.Bytes()
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	out = bytes.ReplaceAll(out, []byte("\t"), []byte("  "))
	out = bytes.TrimRight(out, "\n")
	out = append(out, '\n')
	return out, nil
}

func CanonicalizeJSON(in []byte) ([]byte, error) {
	in = CanonicalizeText(in)
	var doc interface{}
	dec := json.NewDecoder(bytes.NewReader(in))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("canonicalize json: parse: %w", err)
	}
	out, err := jsonMarshalCanonical(doc)
	if err != nil {
		return nil, fmt.Errorf("canonicalize json: marshal: %w", err)
	}
	out = append(out, '\n')
	return out, nil
}

func CanonicalizeMarkdown(in []byte) []byte {
	in = CanonicalizeText(in)
	in = bytes.ReplaceAll(in, []byte("\r\n"), []byte("\n"))
	in = bytes.ReplaceAll(in, []byte("\r"), []byte("\n"))
	lines := bytes.Split(in, []byte("\n"))
	for i, ln := range lines {
		lines[i] = bytes.TrimRightFunc(ln, unicode.IsSpace)
	}
	out := bytes.Join(lines, []byte("\n"))
	out = bytes.TrimRight(out, "\n")
	out = append(out, '\n')
	return out
}

func CanonicalizePath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("canonicalize path: home: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("canonicalize path: abs: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs), nil
	}
	return filepath.Clean(resolved), nil
}

func ContentHash(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ContentHashFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	suffix := strings.ToLower(filepath.Ext(path))
	var canon []byte
	switch suffix {
	case ".yaml", ".yml":
		canon, err = CanonicalizeYAML(raw)
	case ".json":
		canon, err = CanonicalizeJSON(raw)
	case ".md", ".markdown", ".txt":
		canon = CanonicalizeMarkdown(raw)
	default:
		canon = CanonicalizeText(raw)
	}
	if err != nil {
		return "", err
	}
	return ContentHash(canon), nil
}

func ContentHashDirectory(root string) (string, error) {
	var entries []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base == "harness.lock.json" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := ContentHashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, rel+"\x00"+h)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %q: %w", root, err)
	}
	sort.Strings(entries)
	hh := sha256.New()
	for _, e := range entries {
		hh.Write([]byte(e))
		hh.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(hh.Sum(nil)), nil
}

func sortMap(v interface{}) interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[k] = sortMap(val)
		}
		return sortedStringMap(out)
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = sortMap(val)
		}
		return sortedStringMap(out)
	case []interface{}:
		for i := range m {
			m[i] = sortMap(m[i])
		}
		return m
	}
	return v
}

func sortedStringMap(m map[string]interface{}) *yaml.Node {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range keys {
		kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
		vn := &yaml.Node{}
		if err := vn.Encode(m[k]); err != nil {
			vn = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
		}
		node.Content = append(node.Content, kn, vn)
	}
	return node
}

func jsonMarshalCanonical(v interface{}) ([]byte, error) {
	switch x := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kk, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf.Write(kk)
			buf.WriteByte(':')
			vv, err := jsonMarshalCanonical(x[k])
			if err != nil {
				return nil, err
			}
			buf.Write(vv)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []interface{}:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, el := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			vv, err := jsonMarshalCanonical(el)
			if err != nil {
				return nil, err
			}
			buf.Write(vv)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		return json.Marshal(v)
	}
}
