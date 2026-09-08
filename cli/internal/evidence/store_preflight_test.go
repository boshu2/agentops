package evidence

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Discover the actual addresses through successful public operations, then
// reject those same destinations before a missing intent can be recreated.
func TestStorePreflightChecksSelectedFinalAddresses(t *testing.T) {
	for _, kind := range []string{"primary symlink", "recovery symlink", "recovery collision", "recovery permissions"} {
		t.Run(kind, func(t *testing.T) {
			o, _ := fixture(t)
			first, err := StoreVerdict(o)
			if err != nil {
				t.Fatal(err)
			}
			target := first.Path
			want := "symlink"
			if strings.HasPrefix(kind, "recovery") {
				write(t, first.Path, []byte("preserved corrupt primary"))
				recovered, err := StoreVerdict(o)
				if err != nil || recovered.Verdict != "NOT_PROVEN" {
					t.Fatalf("derive recovery: %+v %v", recovered, err)
				}
				target = recovered.Path
			}
			outside := filepath.Join(t.TempDir(), "untouched")
			write(t, outside, []byte("outside target stays unchanged"))
			switch kind {
			case "primary symlink", "recovery symlink":
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, target); err != nil {
					t.Fatal(err)
				}
			case "recovery collision":
				write(t, target, []byte("preserved corrupt recovery"))
				want = "integrity collision"
			case "recovery permissions":
				if err := os.Chmod(target, 0644); err != nil {
					t.Fatal(err)
				}
				want = "integrity collision"
			}
			if err := os.RemoveAll(filepath.Join(o.EvidenceRoot, "intents")); err != nil {
				t.Fatal(err)
			}
			before := tree(t, o.EvidenceRoot)
			_, err = StoreVerdict(o)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want %s rejection, got %v", want, err)
			}
			if !bytes.Equal(before, tree(t, o.EvidenceRoot)) {
				t.Fatal("rejected final destination changed evidence tree")
			}
			got, err := os.ReadFile(outside)
			if err != nil || string(got) != "outside target stays unchanged" {
				t.Fatalf("outside target changed: %q %v", got, err)
			}
		})
	}
}
