package trackerresolve

import (
	"errors"
	"testing"
)

func TestResolveSelectedTrackerDoesNotFallback(t *testing.T) {
	look := func(name string) (string, error) {
		if name == BD {
			return "/fake/bd", nil
		}
		return "", errors.New("missing")
	}
	got, err := ResolveWithLookPath(t.TempDir(), []string{"AGENTOPS_TRACKER=br"}, look)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BR || got.Binary != BR {
		t.Fatalf("selected BR silently fell back: %+v", got)
	}
}
