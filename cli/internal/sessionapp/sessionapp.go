// Package sessionapp owns the filesystem effects behind the `ao session`
// evidence commands. It reports local orientation files (bootstrap) and reads
// the latest caller-authored handoff without consuming it (rehydrate), keeping
// the session command module a thin Cobra presentation seam that performs no
// direct filesystem effect.
package sessionapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// orientationCandidates is the fixed, ordered set of local orientation files a
// bootstrap report probes. The order is the reported order.
var orientationCandidates = []string{"AGENTS.md", "README.md", "PRODUCT.md", "GOALS.md", "PROGRAM.md"}

// BootstrapStatus is the machine-readable bootstrap report. It carries only the
// workspace directory and the orientation files that exist there; it never
// starts runtimes, probes trackers, selects work, or inspects queues.
type BootstrapStatus struct {
	Workspace        string   `json:"workspace"`
	OrientationFiles []string `json:"orientation_files"`
}

// BootstrapOptions carries the presentation choices resolved by the command
// module. The working directory is resolved inside Bootstrap so the module
// never performs a direct filesystem effect.
type BootstrapOptions struct {
	// JSON selects machine-readable output when true.
	JSON bool
	// Stdout receives the rendered report.
	Stdout io.Writer
}

// Bootstrap reports the local orientation files available in the working
// directory. It renders JSON under opts.JSON, otherwise a human list.
func Bootstrap(opts BootstrapOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	status := BootstrapStatus{Workspace: cwd, OrientationFiles: []string{}}
	for _, relative := range orientationCandidates {
		if info, err := os.Stat(filepath.Join(cwd, relative)); err == nil && info.Mode().IsRegular() {
			status.OrientationFiles = append(status.OrientationFiles, relative)
		}
	}
	if opts.JSON {
		encoder := json.NewEncoder(opts.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}
	fmt.Fprintf(opts.Stdout, "workspace: %s\n", status.Workspace)
	for _, relative := range status.OrientationFiles {
		fmt.Fprintf(opts.Stdout, "- %s\n", relative)
	}
	return nil
}

// storedHandoff decodes the caller-authored handoff artifact for the brief. It
// mirrors the on-disk handoff shape's read side; rehydrate never writes, so the
// writer-side type stays with the `ao session handoff` command.
type storedHandoff struct {
	SchemaVersion     *int                `json:"schema_version"`
	ID                *string             `json:"id"`
	CreatedAt         *string             `json:"created_at"`
	Type              *string             `json:"type,omitempty"`
	Goal              string              `json:"goal,omitempty"`
	Summary           string              `json:"summary,omitempty"`
	Continuation      string              `json:"continuation,omitempty"`
	ArtifactsProduced []string            `json:"artifacts_produced,omitempty"`
	DecisionsMade     []string            `json:"decisions_made,omitempty"`
	OpenRisks         []string            `json:"open_risks,omitempty"`
	RPI               *storedHandoffRPI   `json:"rpi,omitempty"`
	State             *storedHandoffState `json:"state,omitempty"`
	Consumed          *bool               `json:"consumed,omitempty"`
	ConsumedAt        *string             `json:"consumed_at,omitempty"`
	ConsumedBy        *string             `json:"consumed_by,omitempty"`
}

// storedHandoffState is the optional read-only Git observation block. Only the
// branch is surfaced in the human brief.
type storedHandoffState struct {
	GitBranch      string   `json:"git_branch,omitempty"`
	GitDirty       *bool    `json:"git_dirty"`
	ModifiedFiles  []string `json:"modified_files,omitempty"`
	ActiveBead     string   `json:"active_bead,omitempty"`
	OpenBeadsCount *int     `json:"open_beads_count,omitempty"`
	RecentCommits  []string `json:"recent_commits,omitempty"`
}

type storedHandoffRPI struct {
	Phase     *int              `json:"phase"`
	PhaseName *string           `json:"phase_name"`
	EpicID    string            `json:"epic_id,omitempty"`
	RunID     string            `json:"run_id,omitempty"`
	Verdicts  map[string]string `json:"verdicts,omitempty"`
}

var handoffIDPattern = regexp.MustCompile(`^handoff-[0-9]{8}T[0-9]{6}(\.[0-9]+)?Z$`)

// handoffReadTestHook is a deterministic race seam for package-local tests.
// Production leaves it nil.
var handoffReadTestHook func(stage string)

// errNoHandoffArtifacts is the only discovery result Rehydrate renders as an
// honest empty state. Filesystem errors and unsafe/corrupt handoff shapes are
// not absence: they fail closed so existing evidence is never silently
// stranded behind `{}`.
var errNoHandoffArtifacts = errors.New("no handoff artifacts")

// RehydrateOptions carries the presentation choices resolved by the command
// module. The working directory is resolved inside Rehydrate so the module
// never performs a direct filesystem effect.
type RehydrateOptions struct {
	// JSON emits the stored artifact bytes verbatim (or `{}` for the empty
	// state) so `ao session rehydrate --json | jq` never breaks.
	JSON bool
	// Stdout receives the artifact or brief.
	Stdout io.Writer
	// Stderr receives the human "no handoff found" hint under JSON.
	Stderr io.Writer
}

// Rehydrate reads the latest caller-authored handoff without consuming it,
// claiming work, or choosing a next action. Under JSON the empty state is
// exactly one `{}` document on stdout with the hint on stderr. Only an honest
// no-artifacts result takes that success path; discovery, read, and parse
// failures surface as errors.
func Rehydrate(opts RehydrateOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	candidate, err := pickLatestHandoff(cwd)
	if err != nil {
		if errors.Is(err, errNoHandoffArtifacts) {
			// Under --json, stdout must be exactly one JSON document (`{}` for the
			// empty state) so `ao session rehydrate --json | jq` never breaks; the
			// human hint goes to stderr. Exit 0 either way.
			if opts.JSON {
				fmt.Fprintln(opts.Stderr, "rehydrate: no handoff found")
				fmt.Fprintln(opts.Stdout, "{}")
				return nil
			}
			fmt.Fprintln(opts.Stdout, "rehydrate: no handoff found")
			return nil
		}
		return fmt.Errorf("discover handoff: %w", err)
	}
	defer func() { _ = candidate.root.Close() }()
	data, err := readRegularHandoff(cwd, candidate)
	if err != nil {
		return fmt.Errorf("read handoff: %w", err)
	}
	var artifact storedHandoff
	if err := decodeStoredHandoff(data, candidate.name, &artifact); err != nil {
		return fmt.Errorf("parse handoff: %w", err)
	}
	if opts.JSON {
		_, err = opts.Stdout.Write(data)
		return err
	}
	fmt.Fprintln(opts.Stdout, renderBrief(&artifact))
	return nil
}

// pickLatestHandoff returns the newest handoff artifact by lexical name order.
// Current writers use .agents/ao/handoff; the legacy directory remains a
// read-only compatibility source so an upgrade does not strand existing
// caller-authored evidence. If the same artifact name exists in both places,
// the canonical directory wins.
type handoffCandidate struct {
	name       string
	displayDir string
	components []string
	priority   int
	root       *os.Root
}

func pickLatestHandoff(cwd string) (*handoffCandidate, error) {
	var latest *handoffCandidate
	roots := [][]string{
		{".agents", "ao", "handoff"},
		{".agents", "handoff"},
	}
	for priority, components := range roots {
		candidate, err := pickLatestHandoffInRoot(cwd, components, priority)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if latest != nil {
				_ = latest.root.Close()
			}
			return nil, err
		}
		if candidate == nil {
			continue
		}
		if latest == nil || candidate.name > latest.name || (candidate.name == latest.name && candidate.priority < latest.priority) {
			if latest != nil {
				_ = latest.root.Close()
			}
			latest = candidate
		} else {
			_ = candidate.root.Close()
		}
	}
	if latest == nil {
		return nil, errNoHandoffArtifacts
	}
	return latest, nil
}

func pickLatestHandoffInRoot(cwd string, components []string, priority int) (*handoffCandidate, error) {
	root, dir, err := openRealHandoffRoot(cwd, components...)
	if err != nil {
		return nil, err
	}
	dirFile, err := root.Open(".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("open handoff root %s: %w", dir, err)
	}
	entries, err := dirFile.ReadDir(-1)
	closeErr := dirFile.Close()
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("read handoff root %s: %w", dir, err)
	}
	if closeErr != nil {
		_ = root.Close()
		return nil, fmt.Errorf("close handoff root %s: %w", dir, closeErr)
	}

	localName := ""
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "handoff-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := root.Lstat(name)
		if err != nil {
			_ = root.Close()
			return nil, fmt.Errorf("inspect handoff artifact %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			_ = root.Close()
			return nil, fmt.Errorf("handoff artifact %s is not a real regular file", path)
		}
		if name > localName {
			localName = name
		}
	}
	if localName == "" {
		_ = root.Close()
		return nil, nil
	}
	return &handoffCandidate{
		name:       localName,
		displayDir: dir,
		components: append([]string(nil), components...),
		priority:   priority,
		root:       root,
	}, nil
}

// requireRealHandoffRoot resolves one configured handoff root without allowing
// a symlink or non-directory at any existing component. Component-wise Lstat is
// required for the nested canonical root: checking only its leaf would follow
// a symlinked `.agents/ao` parent before reporting the leaf's type.
func openRealHandoffRoot(cwd string, components ...string) (*os.Root, string, error) {
	root, err := os.OpenRoot(cwd)
	if err != nil {
		return nil, "", fmt.Errorf("open workspace root: %w", err)
	}
	currentRoot := root
	currentPath := cwd
	for _, component := range components {
		currentPath = filepath.Join(currentPath, component)
		before, err := currentRoot.Lstat(component)
		if err != nil {
			_ = currentRoot.Close()
			return nil, "", fmt.Errorf("inspect handoff root component %s: %w", currentPath, err)
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			_ = currentRoot.Close()
			return nil, "", fmt.Errorf("handoff root component %s is not a real directory", currentPath)
		}
		next, err := currentRoot.OpenRoot(component)
		if err != nil {
			_ = currentRoot.Close()
			return nil, "", fmt.Errorf("open handoff root component %s: %w", currentPath, err)
		}
		opened, openedErr := next.Stat(".")
		after, afterErr := currentRoot.Lstat(component)
		if openedErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
			_ = next.Close()
			_ = currentRoot.Close()
			return nil, "", fmt.Errorf("handoff root component %s changed identity while opening", currentPath)
		}
		_ = currentRoot.Close()
		currentRoot = next
	}
	return currentRoot, currentPath, nil
}

// readRegularHandoff revalidates the selected artifact immediately before the
// read. The descriptor identity checks ensure a path swapped between Lstat and
// Open cannot redirect the read through a symlink: bytes are read only after
// both the opened descriptor and the current path still identify the same real
// regular file.
func readRegularHandoff(cwd string, candidate *handoffCandidate) ([]byte, error) {
	path := filepath.Join(candidate.displayDir, candidate.name)
	file, opened, err := openRegularHandoff(candidate, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	first, err := readStableHandoff(file, candidate, path, opened)
	if err != nil {
		return nil, err
	}
	if err := verifyHandoffRootIdentity(cwd, candidate); err != nil {
		return nil, err
	}
	return first, nil
}

func openRegularHandoff(candidate *handoffCandidate, path string) (*os.File, os.FileInfo, error) {
	if handoffReadTestHook != nil {
		handoffReadTestHook("before-artifact-open")
	}
	before, err := candidate.root.Lstat(candidate.name)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("handoff artifact %s is not a real regular file", path)
	}

	file, err := candidate.root.Open(candidate.name)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	after, err := candidate.root.Lstat(candidate.name)
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("handoff artifact %s changed identity during read", path)
	}
	return file, opened, nil
}

func readStableHandoff(file *os.File, candidate *handoffCandidate, path string, opened os.FileInfo) ([]byte, error) {
	if handoffReadTestHook != nil {
		handoffReadTestHook("before-first-read")
	}
	first, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if handoffReadTestHook != nil {
		handoffReadTestHook("after-first-read")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	second, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	openedAfter, err := file.Stat()
	if err != nil {
		return nil, err
	}
	pathAfter, err := candidate.root.Lstat(candidate.name)
	if err != nil {
		return nil, err
	}
	if pathAfter.Mode()&os.ModeSymlink != 0 || !pathAfter.Mode().IsRegular() || !os.SameFile(opened, openedAfter) || !os.SameFile(pathAfter, openedAfter) || opened.Size() != openedAfter.Size() || !opened.ModTime().Equal(openedAfter.ModTime()) || !bytes.Equal(first, second) {
		return nil, fmt.Errorf("handoff artifact %s changed while reading", path)
	}
	return first, nil
}

func verifyHandoffRootIdentity(cwd string, candidate *handoffCandidate) error {
	current, _, err := openRealHandoffRoot(cwd, candidate.components...)
	if err != nil {
		return fmt.Errorf("verify handoff root %s: %w", candidate.displayDir, err)
	}
	defer func() { _ = current.Close() }()
	wantRoot, err := candidate.root.Stat(".")
	if err != nil {
		return err
	}
	gotRoot, err := current.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(wantRoot, gotRoot) {
		return fmt.Errorf("handoff root %s changed identity while reading", candidate.displayDir)
	}
	return nil
}

func decodeStoredHandoff(data []byte, filename string, artifact *storedHandoff) error {
	if err := rejectHandoffSchemaNulls(data); err != nil {
		return err
	}
	if err := decodeOneStoredHandoff(data, artifact); err != nil {
		return err
	}
	if err := validateHandoffIdentity(filename, artifact); err != nil {
		return err
	}
	if err := validateHandoffMetadata(artifact); err != nil {
		return err
	}
	if err := validateHandoffRPI(artifact.RPI); err != nil {
		return err
	}
	return validateHandoffState(artifact.State)
}

func decodeOneStoredHandoff(data []byte, artifact *storedHandoff) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(artifact); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateHandoffIdentity(filename string, artifact *storedHandoff) error {
	if artifact.SchemaVersion == nil || *artifact.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1")
	}
	if artifact.ID == nil || !handoffIDPattern.MatchString(*artifact.ID) {
		return fmt.Errorf("id does not satisfy handoff.v1")
	}
	if filename != *artifact.ID+".json" {
		return fmt.Errorf("filename %s does not match artifact id %s", filename, *artifact.ID)
	}
	return nil
}

func validateHandoffMetadata(artifact *storedHandoff) error {
	if artifact.CreatedAt == nil {
		return fmt.Errorf("created_at is required")
	}
	if _, err := time.Parse(time.RFC3339Nano, *artifact.CreatedAt); err != nil {
		return fmt.Errorf("created_at is not a date-time: %w", err)
	}
	if artifact.Type != nil && *artifact.Type != "manual" && *artifact.Type != "auto" && *artifact.Type != "rpi" {
		return fmt.Errorf("type is outside the handoff.v1 enum")
	}
	if artifact.ConsumedAt != nil {
		if _, err := time.Parse(time.RFC3339Nano, *artifact.ConsumedAt); err != nil {
			return fmt.Errorf("consumed_at is not a date-time: %w", err)
		}
	}
	return nil
}

func validateHandoffRPI(rpi *storedHandoffRPI) error {
	if rpi != nil {
		if rpi.Phase == nil || *rpi.Phase < 1 || *rpi.Phase > 3 {
			return fmt.Errorf("rpi.phase must be an integer from 1 through 3")
		}
		if rpi.PhaseName == nil || (*rpi.PhaseName != "discovery" && *rpi.PhaseName != "implementation" && *rpi.PhaseName != "validation") {
			return fmt.Errorf("rpi.phase_name is outside the handoff.v1 enum")
		}
	}
	return nil
}

func validateHandoffState(state *storedHandoffState) error {
	if state != nil {
		if state.GitDirty == nil {
			return fmt.Errorf("state.git_dirty is required")
		}
		if state.OpenBeadsCount != nil && *state.OpenBeadsCount < 0 {
			return fmt.Errorf("state.open_beads_count must be non-negative")
		}
	}
	return nil
}

// rejectHandoffSchemaNulls closes encoding/json's permissive null-to-zero
// conversion for optional scalar, array, and object fields. handoff.v1 allows
// null only for rpi, state, consumed_at, and consumed_by; every other present
// property must retain its declared JSON type.
func rejectHandoffSchemaNulls(data []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return err
	}
	for _, name := range []string{
		"schema_version", "id", "created_at", "type", "goal", "summary", "continuation",
		"artifacts_produced", "decisions_made", "open_risks", "consumed",
	} {
		if raw, ok := top[name]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("%s must not be null", name)
		}
	}
	for _, nestedName := range []string{"rpi", "state"} {
		raw, ok := top[nestedName]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(raw, &nested); err != nil {
			return err
		}
		for name, value := range nested {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s.%s must not be null", nestedName, name)
			}
		}
		if nestedName == "rpi" {
			if verdictsRaw, ok := nested["verdicts"]; ok {
				var verdicts map[string]json.RawMessage
				if err := json.Unmarshal(verdictsRaw, &verdicts); err != nil {
					return err
				}
				for key, value := range verdicts {
					if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
						return fmt.Errorf("rpi.verdicts.%s must not be null", key)
					}
				}
			}
		}
	}
	return nil
}

// renderBrief renders the caller-authored brief. It surfaces only the goal,
// summary, continuation, and observed branch; it never invents lifecycle state.
func renderBrief(artifact *storedHandoff) string {
	var lines []string
	if artifact.Goal != "" {
		lines = append(lines, "Goal: "+artifact.Goal)
	}
	if artifact.Summary != "" {
		lines = append(lines, "Summary: "+artifact.Summary)
	}
	if artifact.Continuation != "" {
		lines = append(lines, "Caller continuation: "+artifact.Continuation)
	}
	if artifact.State != nil && artifact.State.GitBranch != "" {
		lines = append(lines, "Observed branch: "+artifact.State.GitBranch)
	}
	if len(lines) == 0 {
		return "Handoff contains no caller-authored brief."
	}
	return strings.Join(lines, "\n")
}
