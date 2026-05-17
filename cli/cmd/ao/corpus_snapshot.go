// practices: [wiki-knowledge-surface, resilience-patterns, ai-assisted-dev]
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultSnapshotDirEnv = "AGENTOPS_CORPUS_SNAPSHOT_DIR"
	defaultSnapshotSubdir = ".agentops/corpus-snapshots"
	corpusSourceDir       = ".agents"
)

var maxSnapshotExtractBytes int64 = 1 << 30

var (
	snapshotOutputDir string
	snapshotJSON      bool
	restoreFrom       string
	restoreLatest     bool
	restoreInto       string
	restoreOverwrite  bool
	restoreJSON       bool
)

var corpusSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Write a tar.gz snapshot of the local .agents/ corpus to a durable path",
	Long: `Writes the entire .agents/ tree as a tar.gz to a durable directory outside the repo,
along with a sidecar manifest containing file count, total bytes, sha256, and ISO-8601 timestamp.

Default output dir: $AGENTOPS_CORPUS_SNAPSHOT_DIR, falling back to ~/.agentops/corpus-snapshots/.
Snapshot filename: <repo-basename>-<RFC3339-utc>.tar.gz.

Intent: routine cleanup periodically wipes .agents/. A snapshot is the durable copy that
ao corpus restore can rehydrate from.`,
	RunE: runCorpusSnapshot,
}

var corpusRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore .agents/ from a durable snapshot",
	Long: `Untars a snapshot produced by ao corpus snapshot. By default refuses to overwrite an
existing .agents/ directory; use --overwrite to replace it (the existing tree is moved to
.agents.bak-<timestamp>/ first as a safety net, then removed only after a successful extract).

Snapshot source resolution:
  --from <path>     explicit tarball path
  --latest          newest tarball in the snapshot dir
  (neither)         errors out (no ambiguous default)`,
	RunE: runCorpusRestore,
}

func init() {
	corpusCmd.AddCommand(corpusSnapshotCmd)
	corpusCmd.AddCommand(corpusRestoreCmd)

	corpusSnapshotCmd.Flags().StringVar(&snapshotOutputDir, "output-dir", "", "Override snapshot dir (default: $AGENTOPS_CORPUS_SNAPSHOT_DIR or ~/.agentops/corpus-snapshots)")
	corpusSnapshotCmd.Flags().BoolVar(&snapshotJSON, "json", false, "Emit the manifest as JSON to stdout")

	corpusRestoreCmd.Flags().StringVar(&restoreFrom, "from", "", "Explicit snapshot tarball path")
	corpusRestoreCmd.Flags().BoolVar(&restoreLatest, "latest", false, "Pick the newest tarball in the snapshot dir")
	corpusRestoreCmd.Flags().StringVar(&restoreInto, "into", corpusSourceDir, "Destination directory (default: .agents)")
	corpusRestoreCmd.Flags().BoolVar(&restoreOverwrite, "overwrite", false, "Replace an existing destination directory (with .bak rescue)")
	corpusRestoreCmd.Flags().BoolVar(&restoreJSON, "json", false, "Emit the result as JSON to stdout")
}

type snapshotManifest struct {
	SnapshotPath string    `json:"snapshot_path"`
	Repo         string    `json:"repo"`
	Source       string    `json:"source"`
	FileCount    int       `json:"file_count"`
	TotalBytes   int64     `json:"total_bytes"`
	SHA256       string    `json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
}

func runCorpusSnapshot(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("corpus snapshot: cwd: %w", err)
	}
	srcAbs := filepath.Join(cwd, corpusSourceDir)
	info, err := os.Stat(srcAbs)
	if err != nil {
		return fmt.Errorf("corpus snapshot: %s not found at %s: %w", corpusSourceDir, cwd, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("corpus snapshot: %s is not a directory", srcAbs)
	}

	outDir, err := resolveSnapshotDir(snapshotOutputDir)
	if err != nil {
		return fmt.Errorf("corpus snapshot: resolving output dir: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("corpus snapshot: mkdir %s: %w", outDir, err)
	}

	now := time.Now().UTC()
	repoName := filepath.Base(cwd)
	stamp := now.Format("20060102T150405Z")
	snapPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.tar.gz", repoName, stamp))
	tmpPath := snapPath + ".tmp"

	count, total, sum, err := writeSnapshot(tmpPath, srcAbs)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("corpus snapshot: writing tarball: %w", err)
	}
	if err := os.Rename(tmpPath, snapPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("corpus snapshot: rename: %w", err)
	}

	manifest := snapshotManifest{
		SnapshotPath: snapPath,
		Repo:         repoName,
		Source:       srcAbs,
		FileCount:    count,
		TotalBytes:   total,
		SHA256:       sum,
		CreatedAt:    now,
	}
	manifestPath := snapPath + ".manifest.json"
	if err := writeCorpusManifestFile(manifestPath, manifest); err != nil {
		return fmt.Errorf("corpus snapshot: writing manifest: %w", err)
	}

	if snapshotJSON {
		return json.NewEncoder(os.Stdout).Encode(manifest)
	}
	fmt.Printf("Corpus snapshot written:\n")
	fmt.Printf("  path:        %s\n", manifest.SnapshotPath)
	fmt.Printf("  manifest:    %s\n", manifestPath)
	fmt.Printf("  files:       %d\n", manifest.FileCount)
	fmt.Printf("  bytes:       %d\n", manifest.TotalBytes)
	fmt.Printf("  sha256:      %s\n", manifest.SHA256)
	fmt.Printf("  created_at:  %s\n", manifest.CreatedAt.Format(time.RFC3339))
	return nil
}

type restoreResult struct {
	From       string    `json:"from"`
	Into       string    `json:"into"`
	FileCount  int       `json:"file_count"`
	TotalBytes int64     `json:"total_bytes"`
	RestoredAt time.Time `json:"restored_at"`
	BackupPath string    `json:"backup_path,omitempty"`
}

func runCorpusRestore(cmd *cobra.Command, args []string) error {
	source := restoreFrom
	if source == "" && restoreLatest {
		dir, err := resolveSnapshotDir(snapshotOutputDir)
		if err != nil {
			return fmt.Errorf("corpus restore: resolving snapshot dir: %w", err)
		}
		latest, err := findLatestSnapshot(dir)
		if err != nil {
			return fmt.Errorf("corpus restore: %w", err)
		}
		source = latest
	}
	if source == "" {
		return fmt.Errorf("corpus restore: provide --from <path> or --latest")
	}

	dest := restoreInto
	if dest == "" {
		dest = corpusSourceDir
	}
	backupPath := ""
	if _, err := os.Stat(dest); err == nil {
		if !restoreOverwrite {
			return fmt.Errorf("corpus restore: %s already exists; pass --overwrite to replace it", dest)
		}
		backupPath = fmt.Sprintf("%s.bak-%s", strings.TrimRight(dest, string(os.PathSeparator)), time.Now().UTC().Format("20060102T150405Z"))
		if err := os.Rename(dest, backupPath); err != nil {
			return fmt.Errorf("corpus restore: backing up existing dest %s: %w", dest, err)
		}
	}

	count, total, err := extractSnapshot(source, dest)
	if err != nil {
		if backupPath != "" {
			_ = os.RemoveAll(dest)
			_ = os.Rename(backupPath, dest)
		}
		return fmt.Errorf("corpus restore: extracting: %w", err)
	}

	result := restoreResult{
		From:       source,
		Into:       dest,
		FileCount:  count,
		TotalBytes: total,
		RestoredAt: time.Now().UTC(),
		BackupPath: backupPath,
	}
	if restoreJSON {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	fmt.Printf("Corpus restored:\n")
	fmt.Printf("  from:        %s\n", result.From)
	fmt.Printf("  into:        %s\n", result.Into)
	fmt.Printf("  files:       %d\n", result.FileCount)
	fmt.Printf("  bytes:       %d\n", result.TotalBytes)
	fmt.Printf("  restored_at: %s\n", result.RestoredAt.Format(time.RFC3339))
	if result.BackupPath != "" {
		fmt.Printf("  prior tree:  %s (safe to remove once verified)\n", result.BackupPath)
	}
	return nil
}

func resolveSnapshotDir(override string) (string, error) {
	if override != "" {
		return expandHome(override)
	}
	if env := os.Getenv(defaultSnapshotDirEnv); env != "" {
		return expandHome(env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultSnapshotSubdir), nil
}

func expandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}

func writeSnapshot(tarPath, srcRoot string) (int, int64, string, error) {
	f, err := os.Create(tarPath)
	if err != nil {
		return 0, 0, "", err
	}
	defer func() { _ = f.Close() }()

	hash := sha256.New()
	mw := io.MultiWriter(f, hash)
	gz := gzip.NewWriter(mw)
	tw := tar.NewWriter(gz)

	var count int
	var total int64
	srcRoot = filepath.Clean(srcRoot)
	parent := filepath.Dir(srcRoot)
	root, err := os.OpenRoot(srcRoot)
	if err != nil {
		return 0, 0, "", err
	}
	defer func() { _ = root.Close() }()
	walkErr := filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(parent, path)
		if rerr != nil {
			return rerr
		}
		rootRel, rerr := filepath.Rel(srcRoot, path)
		if rerr != nil {
			return rerr
		}
		header, herr := tar.FileInfoHeader(info, "")
		if herr != nil {
			return herr
		}
		header.Name = rel
		if werr := tw.WriteHeader(header); werr != nil {
			return werr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		fh, oerr := root.Open(rootRel)
		if oerr != nil {
			return oerr
		}
		openedInfo, serr := fh.Stat()
		if serr != nil {
			_ = fh.Close()
			return serr
		}
		if !openedInfo.Mode().IsRegular() {
			_ = fh.Close()
			return fmt.Errorf("snapshot source changed while reading: %s", rel)
		}
		n, cperr := io.CopyN(tw, fh, info.Size())
		_ = fh.Close()
		if cperr != nil {
			return cperr
		}
		count++
		total += n
		return nil
	})
	if walkErr != nil {
		return 0, 0, "", walkErr
	}
	if err := tw.Close(); err != nil {
		return 0, 0, "", err
	}
	if err := gz.Close(); err != nil {
		return 0, 0, "", err
	}
	if err := f.Sync(); err != nil {
		return 0, 0, "", err
	}
	return count, total, hex.EncodeToString(hash.Sum(nil)), nil
}

func extractSnapshot(tarPath, destParent string) (int, int64, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	if err := os.MkdirAll(filepath.Dir(destParent), 0o755); err != nil {
		return 0, 0, err
	}
	parentDir := filepath.Dir(destParent)
	if parentDir == "" {
		parentDir = "."
	}

	var count int
	var total int64
	for {
		hdr, herr := tr.Next()
		if herr == io.EOF {
			break
		}
		if herr != nil {
			return 0, 0, herr
		}
		// Defend against path traversal
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(os.PathSeparator)+"..") {
			return 0, 0, fmt.Errorf("refusing path traversal entry: %q", hdr.Name)
		}
		target := filepath.Join(parentDir, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)&0o777); err != nil {
				return 0, 0, err
			}
		case tar.TypeReg:
			if hdr.Size < 0 || total+hdr.Size > maxSnapshotExtractBytes {
				return 0, 0, fmt.Errorf("snapshot extract exceeds byte limit: %d", maxSnapshotExtractBytes)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return 0, 0, err
			}
			out, oerr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if oerr != nil {
				return 0, 0, oerr
			}
			n, cperr := io.CopyN(out, tr, hdr.Size)
			cerr := out.Close()
			if cperr != nil {
				return 0, 0, cperr
			}
			if cerr != nil {
				return 0, 0, cerr
			}
			count++
			total += n
		}
	}
	return count, total, nil
}

func findLatestSnapshot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", dir, err)
	}
	type pair struct {
		path string
		t    time.Time
	}
	var pairs []pair
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		pairs = append(pairs, pair{filepath.Join(dir, name), info.ModTime()})
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("no *.tar.gz snapshots found under %s", dir)
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].t.After(pairs[j].t) })
	return pairs[0].path, nil
}

func writeCorpusManifestFile(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
