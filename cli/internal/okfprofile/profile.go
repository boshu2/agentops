// Package okfprofile checks the explicitly selected AgentOps page profile of
// pinned OKF v0.2. It establishes structure, never factual support, permission,
// applicability, independent review, admission, or usefulness.
package okfprofile

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	Profile             = "agentops-okf-v0.2/v1"
	OKFVersion          = "0.2"
	SpecCommit          = "ad30107c31c06aec8a7d5636e0d1058118604e6f"
	MaxPageBytes        = 1 << 20
	MaxFrontmatterBytes = 64 << 10
	Assurance           = "Structure only; factual support, destination disclosure, independent review, current applicability and demonstrated usefulness are not established."
)

// Issue contains fixed field names and codes, never candidate text or locators.
type Issue struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type Result struct {
	Profile           string  `json:"profile"`
	OKFVersion        string  `json:"okf_version"`
	SpecCommit        string  `json:"spec_commit"`
	ContentSHA256     string  `json:"content_sha256,omitempty"`
	StructurallyValid bool    `json:"structurally_valid"`
	Issues            []Issue `json:"issues"`
	Assurance         string  `json:"assurance"`
}

func checkProfile(profile string) error {
	if profile != Profile {
		return fmt.Errorf("unsupported profile; supported profile is %s", Profile)
	}
	return nil
}

// Check validates one page without resolving any source, link or other file.
// The caller's profile selection is checked before inspecting candidate bytes.
// A structural rejection is a Result; an unsupported operation is an error.
func Check(payload []byte, profile string) (Result, error) {
	result := Result{Profile: Profile, OKFVersion: OKFVersion, SpecCommit: SpecCommit, Issues: []Issue{}, Assurance: Assurance}
	if err := checkProfile(profile); err != nil {
		return result, err
	}
	if len(payload) > MaxPageBytes {
		result.add("page", "exceeds_1_mib")
		return result, nil
	}
	result.ContentSHA256 = fmt.Sprintf("%x", sha256.Sum256(payload))
	if !utf8.Valid(payload) {
		result.add("page", "invalid_utf8")
		return result, nil
	}
	metadata, body, code := frontmatter(payload)
	if code != "" {
		result.add("frontmatter", code)
		return result, nil
	}

	result.metadata(metadata)
	result.requiredText(metadata, bodySections(body))
	result.StructurallyValid = len(result.Issues) == 0
	return result, nil
}

func (result *Result) metadata(metadata map[string]*yaml.Node) {
	for _, key := range []string{"type", "title", "description"} {
		result.requireString(metadata[key], key)
	}
	result.requireEnum(metadata["status"], "status", "draft", "stable", "deprecated")
	result.requireEnum(metadata["knowledge_use"], "knowledge_use", "maintained-reference", "promotion-candidate")
	if n := metadata["agentops_profile"]; n != nil {
		result.requireEnum(n, "agentops_profile", Profile)
	}
	if n := metadata["okf_version"]; n != nil {
		result.requireEnum(n, "okf_version", OKFVersion)
	}
	result.sources(metadata["sources"])
	if n := metadata["generated"]; n != nil {
		result.actorEvent(n, "generated", false)
	}
	if n := metadata["verified"]; n != nil {
		items := n.Content
		if n.Kind == yaml.MappingNode {
			items = []*yaml.Node{n}
		} else if n.Kind != yaml.SequenceNode {
			result.add("verified", "expected_mapping_or_list")
			items = nil
		}
		for i, item := range items {
			result.actorEvent(item, fmt.Sprintf("verified[%d]", i), true)
		}
	}
	if n := metadata["stale_after"]; n != nil {
		result.timestamp(n, "stale_after")
	}

}

func (result *Result) requiredText(metadata map[string]*yaml.Node, sections map[string]bool) {
	for _, key := range []string{"applicability", "limitations", "consumer", "retirement_condition"} {
		if n := metadata[key]; n != nil {
			result.requireString(n, key)
		} else if !sections[key] {
			result.add(key, "required_nonempty_text")
		}
	}
	claim := false
	for _, key := range []string{"claim", "action"} {
		if n := metadata[key]; n != nil {
			claim = result.requireString(n, key) || claim
		} else {
			claim = sections[key] || claim
		}
	}
	if !claim {
		result.add("claim/action", "required_nonempty_text")
	}
}

func (r *Result) add(field, code string) {
	r.Issues = append(r.Issues, Issue{Field: field, Code: code})
}

func stringValue(n *yaml.Node) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" || strings.TrimSpace(n.Value) == "" {
		return "", false
	}
	return n.Value, true
}

func (r *Result) requireString(n *yaml.Node, field string) bool {
	_, ok := stringValue(n)
	if !ok {
		r.add(field, "required_nonempty_string")
	}
	return ok
}

func (r *Result) requireEnum(n *yaml.Node, field string, values ...string) {
	value, ok := stringValue(n)
	if !ok {
		r.add(field, "required_explicit_value")
		return
	}
	for _, allowed := range values {
		if value == allowed {
			return
		}
	}
	r.add(field, "unsupported_value")
}

func mapping(n *yaml.Node) map[string]*yaml.Node {
	fields := map[string]*yaml.Node{}
	if n != nil && n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			fields[n.Content[i].Value] = n.Content[i+1]
		}
	}
	return fields
}

func (r *Result) sources(n *yaml.Node) {
	if n == nil || n.Kind != yaml.SequenceNode || len(n.Content) == 0 {
		r.add("sources", "required_nonempty_list")
		return
	}
	ids := map[string]bool{}
	for i, entry := range n.Content {
		field := fmt.Sprintf("sources[%d]", i)
		if entry.Kind != yaml.MappingNode {
			r.add(field, "expected_mapping")
			continue
		}
		fields := mapping(entry)
		r.requireString(fields["resource"], field+".resource")
		for _, key := range []string{"id", "title", "author"} {
			if item := fields[key]; item != nil {
				r.requireString(item, field+"."+key)
			}
		}
		if id, ok := stringValue(fields["id"]); ok {
			if ids[id] {
				r.add(field+".id", "duplicate_source_id")
			}
			ids[id] = true
		}
		if n := fields["last_modified"]; n != nil {
			r.timestamp(n, field+".last_modified")
		}
	}
}

func (r *Result) actorEvent(n *yaml.Node, field string, requireTime bool) {
	if n.Kind != yaml.MappingNode {
		r.add(field, "expected_mapping")
		return
	}
	fields := mapping(n)
	actor, ok := stringValue(fields["by"])
	if !ok {
		r.add(field+".by", "required_actor")
	} else if !validActor(actor) {
		r.add(field+".by", "invalid_actor_convention")
	}
	if fields["at"] != nil || requireTime {
		r.timestamp(fields["at"], field+".at")
	}
}

func validActor(actor string) bool {
	if strings.IndexFunc(actor, unicode.IsSpace) >= 0 {
		return false
	}
	for _, prefix := range []string{"human:", "process:"} {
		if strings.HasPrefix(actor, prefix) {
			return len(actor) > len(prefix)
		}
	}
	producer, version, ok := strings.Cut(actor, "/")
	return ok && producer != "" && version != ""
}

func (r *Result) timestamp(n *yaml.Node, field string) {
	if n == nil || n.Kind != yaml.ScalarNode || (n.Tag != "!!str" && n.Tag != "!!timestamp") {
		r.add(field, "expected_datetime_with_offset")
		return
	}
	if _, err := time.Parse(time.RFC3339, n.Value); err != nil {
		r.add(field, "expected_datetime_with_offset")
	}
}

func frontmatter(payload []byte) (map[string]*yaml.Node, string, string) {
	header, body, code := splitFrontmatter(payload)
	if code != "" {
		return nil, "", code
	}
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(header))
	if err := decoder.Decode(&doc); err != nil {
		return nil, "", "invalid_yaml"
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, "", "multiple_yaml_documents"
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, "", "expected_mapping"
	}
	count := 0
	if !unambiguousYAML(doc.Content[0], 0, &count) {
		return nil, "", "ambiguous_or_excessive_yaml"
	}
	return mapping(doc.Content[0]), strings.ReplaceAll(string(body), "\r\n", "\n"), ""
}

func splitFrontmatter(payload []byte) ([]byte, []byte, string) {
	opening := bytes.IndexByte(payload, '\n')
	if opening < 0 || string(bytes.TrimSuffix(payload[:opening], []byte{'\r'})) != "---" {
		return nil, nil, "missing_opening_delimiter"
	}
	start := opening + 1
	for position := start; position < len(payload); {
		if position > MaxFrontmatterBytes {
			return nil, nil, "exceeds_64_kib"
		}
		length := bytes.IndexByte(payload[position:], '\n')
		if length < 0 {
			length = len(payload) - position
		}
		end := position + length
		if string(bytes.TrimSuffix(payload[position:end], []byte{'\r'})) == "---" {
			body := payload[end:]
			return payload[start:position], bytes.TrimPrefix(body, []byte{'\n'}), ""
		}
		position = end + 1
	}
	return nil, nil, "missing_closing_delimiter"
}

// Explicit keys prevent aliases, merge inheritance and duplicate keys from
// concealing missing metadata. Bound nesting and node count without expansion.
func unambiguousYAML(n *yaml.Node, depth int, count *int) bool {
	*count++
	if depth > 32 || *count > 8192 || n.Kind == yaml.AliasNode {
		return false
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return false
			}
			seen[key.Value] = true
		}
	}
	for _, child := range n.Content {
		if !unambiguousYAML(child, depth+1, count) {
			return false
		}
	}
	return true
}

// bodySections recognizes ordinary ATX headings outside comments and code
// examples. It checks presence of content, not the meaning of that content.
func bodySections(body string) map[string]bool {
	sections := map[string]bool{}
	current, level := "", 0
	fence := markdownFence{}
	comment := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimLeft(raw, " ")
		indent := len(raw) - len(line)
		if fence.marker == 0 {
			line = stripComments(line, &comment)
		}
		if indent <= 3 && fence.delimiter(line) {
			continue
		}
		if fence.marker == 0 && indent <= 3 {
			if key, n := atxHeading(line); n != 0 {
				switch key {
				case "applicability", "claim", "action", "limitations", "consumer", "retirement_condition":
					current, level = key, n
				default:
					if n <= level {
						current, level = "", 0
					}
				}
				continue
			}
		}
		if current != "" && len(bytes.TrimSpace([]byte(line))) > 0 {
			sections[current] = true
		}
	}
	return sections
}

type markdownFence struct {
	marker byte
	length int
}

func (f *markdownFence) delimiter(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return false
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < 3 {
		return false
	}
	if f.marker == 0 {
		f.marker, f.length = line[0], n
		return true
	}
	if line[0] == f.marker && n >= f.length && strings.TrimSpace(line[n:]) == "" {
		f.marker, f.length = 0, 0
		return true
	}
	return false
}

func atxHeading(line string) (string, int) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(line) && line[n] != ' ' && line[n] != '\t') {
		return "", 0
	}
	title := strings.TrimSpace(line[n:])
	withoutClosing := strings.TrimRight(title, "#")
	if strings.HasSuffix(withoutClosing, " ") || strings.HasSuffix(withoutClosing, "\t") {
		title = strings.TrimSpace(withoutClosing)
	}
	return strings.ReplaceAll(strings.ToLower(title), " ", "_"), n
}

func stripComments(line string, inComment *bool) string {
	var out strings.Builder
	for line != "" {
		if *inComment {
			_, rest, found := strings.Cut(line, "-->")
			if !found {
				break
			}
			line, *inComment = rest, false
		} else {
			before, rest, found := strings.Cut(line, "<!--")
			out.WriteString(before)
			if !found {
				break
			}
			line, *inComment = rest, true
		}
	}
	return out.String()
}
