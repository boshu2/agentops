package gates

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// UncommittedPaths is the worktree state that is not yet a commit: tracked
// paths that differ from HEAD (staged or unstaged) and untracked paths git does
// not ignore.
type UncommittedPaths struct {
	Modified  []string
	Untracked []string
}

// UncommittedFilesPort reports the worktree's uncommitted paths. The
// orchestrator uses it to say which uncommitted paths a fast run's routing
// scope left out; a ChangedFilesPort that does not implement it produces no
// such note.
type UncommittedFilesPort interface {
	Uncommitted(ctx context.Context) (UncommittedPaths, error)
}

// Uncommitted lists the worktree's modified and untracked paths. It reads
// `git status` with --no-optional-locks so a gate run never takes the index
// lock, and it works before the first commit.
func (g *GitChangedFiles) Uncommitted(ctx context.Context) (UncommittedPaths, error) {
	out, err := g.exec(ctx, "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=normal")
	if err != nil {
		return UncommittedPaths{}, err
	}
	return parsePorcelainZ(out), nil
}

// parsePorcelainZ splits `git status --porcelain=v1 -z` output. Each entry is
// "XY <path>" terminated by NUL; a rename or copy entry is followed by one
// extra NUL-terminated field holding the source path, which is not an entry.
// Paths are taken verbatim: whitespace and newlines are valid filename bytes.
func parsePorcelainZ(out string) UncommittedPaths {
	var paths UncommittedPaths
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 || entry[2] != ' ' {
			continue
		}
		code, path := entry[:2], entry[3:]
		if strings.ContainsAny(code, "RC") {
			i++ // skip the rename/copy source field
		}
		switch code {
		case "??":
			paths.Untracked = append(paths.Untracked, path)
		case "!!":
			// ignored paths are never requested; skip one if git reports it anyway
		default:
			paths.Modified = append(paths.Modified, path)
		}
	}
	return paths
}

// OutsideScope is the uncommitted worktree state that a fast run's routing did
// not consider. Fast mode selects checks from the scope's changed-file list, so
// an edit outside that list selects nothing: without this note a run can print
// green while the checks for the edit never ran.
type OutsideScope struct {
	// Modified are tracked paths that differ from HEAD and are absent from the
	// routed change set.
	Modified []string
	// Untracked are paths git neither tracks nor ignores; no scope routes on
	// them until they are added.
	Untracked []string
	// Err is set when the worktree could not be inspected. The counts are then
	// unknown, which is reported as such and never as zero.
	Err string
}

// outsideScope computes the note for a run. It returns nil when there is
// nothing to say: full mode (every check runs, routing is unused), a port that
// cannot list uncommitted paths, or a worktree with no path outside the scope.
func (o *Orchestrator) outsideScope(ctx context.Context, opts RunOptions, changed []string) *OutsideScope {
	if opts.Mode == Full {
		return nil
	}
	port, ok := o.files.(UncommittedFilesPort)
	if !ok {
		return nil
	}
	paths, err := port.Uncommitted(ctx)
	if err != nil {
		return &OutsideScope{Err: err.Error()}
	}
	routed := make(map[string]bool, len(changed))
	for _, f := range changed {
		routed[f] = true
	}
	note := &OutsideScope{}
	for _, f := range FilterInstalledSkillCopies(paths.Modified) {
		if !routed[f] {
			note.Modified = append(note.Modified, f)
		}
	}
	for _, f := range FilterInstalledSkillCopies(paths.Untracked) {
		if !routed[f] {
			note.Untracked = append(note.Untracked, f)
		}
	}
	if len(note.Modified) == 0 && len(note.Untracked) == 0 {
		return nil
	}
	return note
}

// outsideScopeExamples is how many paths the note names.
const outsideScopeExamples = 3

// examples returns the first few paths, modified before untracked.
func (s *OutsideScope) examples() []string {
	all := append(append([]string{}, s.Modified...), s.Untracked...)
	if len(all) > outsideScopeExamples {
		all = all[:outsideScopeExamples]
	}
	return all
}

// human writes the one-line note. A nil receiver writes nothing.
func (s *OutsideScope) human(w io.Writer, scope Scope) {
	if s == nil {
		return
	}
	if s.Err != "" {
		fmt.Fprintf(w, "routing coverage unknown: could not list uncommitted paths (%s)\n", s.Err)
		return
	}
	var remedy []string
	if len(s.Modified) > 0 && scope != ScopeWorktree {
		remedy = append(remedy, "--scope worktree routes on modified paths")
	}
	if len(s.Untracked) > 0 {
		remedy = append(remedy, "git add untracked paths to route on them")
	}
	total, noun := len(s.Modified)+len(s.Untracked), "paths"
	if total == 1 {
		noun = "path"
	}
	line := fmt.Sprintf("not considered by routing: %d uncommitted %s outside scope %s (%d modified, %d untracked), first: %s",
		total, noun, scope, len(s.Modified), len(s.Untracked),
		strings.Join(quoteAll(s.examples()), ", "))
	if len(remedy) > 0 {
		line += "; " + strings.Join(remedy, "; ")
	}
	fmt.Fprintln(w, line)
}

func quoteAll(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = fmt.Sprintf("%q", p)
	}
	return out
}

// jsonOutsideScope is the wire form of OutsideScope. It is omitted when the
// note is absent, so a run with nothing outside its scope renders as before.
type jsonOutsideScope struct {
	ModifiedCount  int      `json:"modified_count"`
	UntrackedCount int      `json:"untracked_count"`
	Examples       []string `json:"examples,omitempty"`
	Error          string   `json:"error,omitempty"`
}

func (s *OutsideScope) wire() *jsonOutsideScope {
	if s == nil {
		return nil
	}
	return &jsonOutsideScope{
		ModifiedCount:  len(s.Modified),
		UntrackedCount: len(s.Untracked),
		Examples:       s.examples(),
		Error:          s.Err,
	}
}

var _ UncommittedFilesPort = (*GitChangedFiles)(nil)
