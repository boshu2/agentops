package vibecheck

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ParseTimeline runs git log in repoPath for commits since the given time
// and returns a slice of TimelineEvents sorted newest-first.
func ParseTimeline(repoPath string, since time.Time) ([]TimelineEvent, error) {
	sinceStr := since.Format(time.RFC3339)

	// Use a delimiter unlikely to appear in commit messages.
	const delim = "|||"
	format := "%H" + delim + "%aI" + delim + "%an" + delim + "%s"

	cmd := exec.Command("git", "log",
		"--format="+format,
		"--numstat",
		"--since="+sinceStr,
	)
	cmd.Dir = repoPath
	cmd.Env = gitDiscoveryEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, stderr.String())
	}

	return parseGitLog(stdout.String(), delim)
}

func gitDiscoveryEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "GIT_DIR="):
			continue
		case strings.HasPrefix(entry, "GIT_WORK_TREE="):
			continue
		case strings.HasPrefix(entry, "GIT_COMMON_DIR="):
			continue
		default:
			env = append(env, entry)
		}
	}
	return env
}

// gitLogParser accumulates events while scanning git log output line by line.
type gitLogParser struct {
	events  []TimelineEvent
	current *TimelineEvent
	delim   string
}

// flush appends the current event (if any) to the events slice and resets current.
func (g *gitLogParser) flush() {
	if g.current != nil {
		g.events = append(g.events, *g.current)
		g.current = nil
	}
}

// processLine handles a single line of git log output.
func (g *gitLogParser) processLine(line string) error {
	if strings.TrimSpace(line) == "" {
		// `git log --format=… --numstat` emits a blank line AFTER each header,
		// before that commit's numstat block; commits are delimited by the next
		// header (tryParseHeader below), not by blank lines. So a blank line is
		// intra-commit — skip it WITHOUT flushing. Flushing here drops the event
		// with an empty Files list before its numstat lines are parsed (the bug
		// behind vibe-check's tests-passing-lie false positives: every commit had
		// empty Files, so filesRelated's empty/empty case over-correlated
		// temporally-adjacent but unrelated claim→fix commits). age-mr1c.
		return nil
	}

	if ev, err := tryParseHeader(line, g.delim); ev != nil {
		g.flush()
		g.current = ev
		return nil
	} else if err != nil {
		return err
	}

	if g.current != nil {
		parseNumstat(line, g.current)
	}
	return nil
}

// parseGitLog parses the combined --format + --numstat output from git log.
//
// The format alternates between a header line (fields separated by delim) and
// zero or more numstat lines (tab-separated: insertions, deletions, filename).
// Commits are separated by blank lines.
func parseGitLog(raw string, delim string) ([]TimelineEvent, error) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	g := &gitLogParser{delim: delim}

	for scanner.Scan() {
		if err := g.processLine(scanner.Text()); err != nil {
			return nil, err
		}
	}

	g.flush()

	slices.SortFunc(g.events, func(a, b TimelineEvent) int {
		return b.Timestamp.Compare(a.Timestamp)
	})

	return g.events, nil
}

// tryParseHeader attempts to parse a git log header line. Returns (event, nil) on success,
// (nil, nil) if not a header, or (nil, error) on parse failure.
func tryParseHeader(line, delim string) (*TimelineEvent, error) {
	parts := strings.SplitN(line, delim, 4)
	if len(parts) != 4 {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp %q: %w", parts[1], err)
	}
	return &TimelineEvent{
		SHA:       parts[0],
		Timestamp: ts,
		Author:    parts[2],
		Message:   parts[3],
	}, nil
}

// parseNumstat parses a numstat line and adds file stats to the event.
func parseNumstat(line string, event *TimelineEvent) {
	fields := strings.SplitN(line, "\t", 3)
	if len(fields) != 3 {
		return
	}
	ins, _ := strconv.Atoi(fields[0])
	del, _ := strconv.Atoi(fields[1])
	event.Insertions += ins
	event.Deletions += del
	event.FilesChanged++
	event.Files = append(event.Files, fields[2])
}
