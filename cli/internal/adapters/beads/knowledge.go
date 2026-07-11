package beads

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

type KnowledgeRepository struct {
	workingDirectory func() (string, error)
}

func NewKnowledgeRepository() KnowledgeRepository {
	return KnowledgeRepository{workingDirectory: os.Getwd}
}

func NewKnowledgeRepositoryWithWorkingDirectory(workingDirectory func() (string, error)) KnowledgeRepository {
	return KnowledgeRepository{workingDirectory: workingDirectory}
}

func (repository KnowledgeRepository) VerifyCitations(citations []beadsapp.Citation) []beadsapp.Citation {
	cwd, err := repository.workingDirectory()
	if err != nil {
		for index := range citations {
			citations[index].Status = beadsapp.CitationUnknown
			citations[index].Reason = "working directory unavailable"
		}
		return citations
	}
	for index := range citations {
		repository.verifyCitation(&citations[index], cwd)
	}
	return citations
}

func (repository KnowledgeRepository) CreateLearning(path string, content []byte) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func (repository KnowledgeRepository) verifyCitation(citation *beadsapp.Citation, cwd string) {
	switch citation.Kind {
	case "file":
		repository.verifyFile(citation, cwd)
	case "function":
		repository.verifySymbol(citation, cwd, strings.TrimPrefix(citation.Raw, "func "), true)
	case "symbol":
		repository.verifySymbol(citation, cwd, strings.Trim(citation.Raw, "`"), false)
	default:
		citation.Status = beadsapp.CitationUnknown
		citation.Reason = "unrecognized citation kind"
	}
}

func (repository KnowledgeRepository) verifyFile(citation *beadsapp.Citation, cwd string) {
	path := citation.Raw
	if index := strings.LastIndex(path, ":"); index >= 0 {
		if _, err := fmt.Sscanf(path[index+1:], "%d", new(int)); err == nil {
			path = path[:index]
		}
	}
	if _, err := os.Stat(filepath.Join(cwd, path)); err == nil {
		citation.Status, citation.Reason = beadsapp.CitationFresh, "file exists at HEAD"
		return
	}
	if strings.Contains(path, "/") {
		citation.Status, citation.Reason = beadsapp.CitationStale, fmt.Sprintf("file %s not found at HEAD", path)
		return
	}
	matches := findFilesByBasename(cwd, path)
	switch len(matches) {
	case 0:
		citation.Status, citation.Reason = beadsapp.CitationStale, fmt.Sprintf("bare filename %q has zero matches at HEAD", path)
	case 1:
		citation.Status, citation.Reason, citation.Resolved = beadsapp.CitationFresh, "bare filename resolves uniquely", matches[0]
	default:
		citation.Status = beadsapp.CitationUnknown
		citation.Reason = fmt.Sprintf("bare filename %q is ambiguous (%d matches) — cite the full path", path, len(matches))
		citation.Resolved = strings.Join(matches[:min(3, len(matches))], ", ")
	}
}

func (repository KnowledgeRepository) verifySymbol(citation *beadsapp.Citation, cwd, symbol string, definition bool) {
	matches := grepSymbol(cwd, symbol)
	if len(matches) == 0 {
		citation.Status = beadsapp.CitationStale
		if definition {
			citation.Reason = fmt.Sprintf("function %q has zero definitions at HEAD", symbol)
		} else {
			citation.Reason = fmt.Sprintf("symbol %q has zero references at HEAD", symbol)
		}
		return
	}
	citation.Status = beadsapp.CitationFresh
	if definition {
		citation.Reason = fmt.Sprintf("function defined at %d location(s)", len(matches))
		if len(matches) == 1 {
			citation.Resolved = matches[0]
		}
	} else {
		citation.Reason = fmt.Sprintf("symbol found at %d location(s)", len(matches))
	}
}

func findFilesByBasename(cwd, name string) []string {
	var matches []string
	for _, root := range []string{"cli", "skills", "docs", "scripts", ".agents"} {
		rootPath := filepath.Join(cwd, root)
		if _, err := os.Stat(rootPath); err != nil {
			continue
		}
		_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				base := filepath.Base(path)
				if (strings.HasPrefix(base, ".") && path != rootPath) || base == "node_modules" || base == "vendor" || base == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Base(path) == name {
				if relative, relErr := filepath.Rel(cwd, path); relErr == nil {
					matches = append(matches, relative)
				}
			}
			return nil
		})
		if len(matches) >= 10 {
			return matches[:10]
		}
	}
	return matches
}

func grepSymbol(cwd, symbol string) []string {
	if symbol == "" {
		return nil
	}
	command := exec.Command("grep", "-rn", "-l", "--include=*.go", "--include=*.md", "--include=*.py", "--include=*.sh", "--include=*.yaml", "--include=*.yml", "--include=*.json", regexp.QuoteMeta(symbol), filepath.Join(cwd, "cli"), filepath.Join(cwd, "skills"), filepath.Join(cwd, "scripts")) // #nosec G204 -- fixed grep binary and bounded source roots.
	output, _ := command.Output()
	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			matches = append(matches, line)
		}
		if len(matches) == 10 {
			break
		}
	}
	return matches
}

var _ beadsapp.KnowledgeRepository = KnowledgeRepository{}
