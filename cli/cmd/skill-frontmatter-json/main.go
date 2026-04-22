package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: skill-frontmatter-json <path-to-SKILL.md>")
		os.Exit(2)
	}

	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}

	frontmatter, ok, err := extractFrontmatter(string(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract frontmatter from %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	if !ok {
		fmt.Println("{}")
		return
	}

	var document any
	if err := yaml.Unmarshal([]byte(frontmatter), &document); err != nil {
		fmt.Fprintf(os.Stderr, "parse YAML frontmatter in %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}

	object, ok := normalize(document).(map[string]any)
	if !ok || object == nil {
		fmt.Println("{}")
		return
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(object); err != nil {
		fmt.Fprintf(os.Stderr, "encode JSON for %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

func extractFrontmatter(content string) (string, bool, error) {
	content = strings.TrimPrefix(content, "\ufeff")

	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return "", false, nil
	}

	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			return strings.Join(lines, "\n"), true, nil
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return "", false, errors.New("unterminated frontmatter block")
}

func normalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalize(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[fmt.Sprint(key)] = normalize(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	default:
		return v
	}
}
