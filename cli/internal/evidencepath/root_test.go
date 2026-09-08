package evidencepath

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestExplicitNonGitRoots(t *testing.T) {
	external := t.TempDir()
	for _, kind := range []string{"ordinary", "linked", "bare"} {
		t.Run(kind, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repo")
			if err := os.Mkdir(repo, 0700); err != nil {
				t.Fatal(err)
			}
			// Synthetic native Git layouts: no Git command/object ingestion is needed.
			if kind == "ordinary" {
				if err := os.Mkdir(filepath.Join(repo, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "linked" {
				if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /opaque/main/.git/worktrees/linked\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "bare" {
				for _, name := range []string{"objects", "refs"} {
					if err := os.Mkdir(filepath.Join(repo, name), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(repo, "HEAD"), []byte("ref: refs/heads/main\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			nested := filepath.Join(repo, "evidence")
			if err := os.Mkdir(nested, 0700); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(external, kind)
			if err := os.Symlink(nested, alias); err != nil {
				t.Fatal(err)
			}
			for _, root := range []string{repo, nested, alias, filepath.Join(nested, "missing")} {
				if _, err := Validate(root); err == nil {
					t.Fatalf("admitted %s", root)
				}
			}
			if _, err := os.Stat(filepath.Join(nested, "missing")); !os.IsNotExist(err) {
				t.Fatal("guard created missing path")
			}
		})
	}
	for _, root := range []string{"", filepath.Join(external, "missing")} {
		if _, err := Validate(root); err == nil {
			t.Fatalf("admitted %q", root)
		}
	}
	if _, err := Validate(external); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(external, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(alias); err != nil {
		t.Fatal(err)
	}
}

func clearGitBindings(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestActiveGitStorageBindings(t *testing.T) {
	for _, kind := range []string{"relocated-objects", "split-common", "common-without-env"} {
		t.Run(kind, func(t *testing.T) {
			clearGitBindings(t)
			boundary := t.TempDir()
			if kind != "relocated-objects" {
				for _, name := range []string{"objects", "refs"} {
					if err := os.Mkdir(filepath.Join(boundary, name), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(boundary, "config"), []byte("[core]\nrepositoryformatversion = 0\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "relocated-objects" {
				t.Setenv("GIT_OBJECT_DIRECTORY", boundary)
			}
			if kind == "split-common" {
				admin := t.TempDir()
				if err := os.WriteFile(filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/main\n"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_DIR", admin)
				t.Setenv("GIT_COMMON_DIR", boundary)
			}
			child := filepath.Join(boundary, "child")
			if err := os.Mkdir(child, 0700); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(t.TempDir(), "alias")
			if err := os.Symlink(boundary, alias); err != nil {
				t.Fatal(err)
			}
			for _, target := range []string{boundary, child, alias, filepath.Join(alias, "child")} {
				if _, err := Validate(target); err == nil {
					t.Errorf("admitted active Git storage %s", target)
				}
			}
		})
	}
}

func TestDeclaredGitBoundaries(t *testing.T) {
	clearGitBindings(t)
	pool := t.TempDir()
	outside := t.TempDir()
	child := filepath.Join(pool, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(pool, alias); err != nil {
		t.Fatal(err)
	}
	for _, excluded := range []string{pool, alias} {
		for _, target := range []string{pool, child, alias, filepath.Join(alias, "child")} {
			if _, err := Validate(target, excluded); err == nil {
				t.Fatalf("admitted excluded storage %s via %s", target, excluded)
			}
		}
		if _, err := Validate(outside, excluded); err != nil {
			t.Fatalf("unrelated root rejected: %v", err)
		}
	}
	// A storage subtree of the proposed evidence root is not safe either.
	if _, err := Validate(pool, child); err == nil {
		t.Fatal("admitted a root containing declared Git storage")
	}
	missing := filepath.Join(outside, "missing")
	dangling := filepath.Join(outside, "dangling")
	if err := os.Symlink(missing, dangling); err != nil {
		t.Fatal(err)
	}
	for _, excluded := range []string{"", missing, dangling} {
		if _, err := Validate(pool, excluded); err == nil {
			t.Fatal("unresolved exclusion admitted")
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("guard created a missing boundary")
	}
}

func TestGitBindingResolution(t *testing.T) {
	for _, kind := range []string{"commondir", "objects-symlink", "alternates", "quoted-alternates", "worktree", "index"} {
		t.Run(kind, func(t *testing.T) {
			clearGitBindings(t)
			storage := t.TempDir()
			outside := t.TempDir()
			switch kind {
			case "commondir", "objects-symlink":
				base := t.TempDir()
				admin := filepath.Join(base, "admin")
				if err := os.Mkdir(admin, 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_DIR", admin)
				if kind == "commondir" {
					common := filepath.Join(base, "common")
					if err := os.Mkdir(common, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(filepath.Join(common, "objects"), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(admin, "commondir"), []byte("../common\n"), 0600); err != nil {
						t.Fatal(err)
					}
					storage = common
				} else {
					if err := os.Symlink(storage, filepath.Join(admin, "objects")); err != nil {
						t.Fatal(err)
					}
				}
			case "alternates":
				t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", outside+string(os.PathListSeparator)+storage)
			case "quoted-alternates":
				storage = filepath.Join(storage, "pool:with-colon")
				if err := os.Mkdir(storage, 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", strconv.Quote(storage))
			case "worktree":
				t.Setenv("GIT_WORK_TREE", storage)
			case "index":
				t.Setenv("GIT_INDEX_FILE", filepath.Join(storage, "not-created-index"))
			}
			if _, err := Validate(storage); err == nil {
				t.Fatal("active binding admitted")
			}
			if _, err := Validate(t.TempDir()); err != nil {
				t.Fatalf("resolved binding should permit unrelated root: %v", err)
			}
		})
	}
}

func TestUnresolvedGitBindingsFailClosed(t *testing.T) {
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		t.Run(key, func(t *testing.T) {
			clearGitBindings(t)
			root := t.TempDir()
			missing := filepath.Join(root, "missing")
			value := missing
			if key == "GIT_INDEX_FILE" {
				value = filepath.Join(missing, "index")
			}
			t.Setenv(key, value)
			if _, err := Validate(t.TempDir()); err == nil {
				t.Fatal("unresolved active Git binding admitted")
			}
			if _, err := os.Stat(missing); !os.IsNotExist(err) {
				t.Fatal("missing binding was created")
			}
		})
	}
	t.Run("dangling-commondir", func(t *testing.T) {
		clearGitBindings(t)
		admin := t.TempDir()
		t.Setenv("GIT_DIR", admin)
		if err := os.WriteFile(filepath.Join(admin, "commondir"), []byte("missing\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Validate(t.TempDir()); err == nil {
			t.Fatal("unresolved commondir admitted")
		}
	})
	t.Run("malformed-alternate", func(t *testing.T) {
		clearGitBindings(t)
		t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", `"unterminated`)
		if _, err := Validate(t.TempDir()); err == nil {
			t.Fatal("malformed alternate admitted")
		}
	})
}
