package inputscope

import (
	"reflect"
	"testing"
)

func TestOrdinaryChange(t *testing.T) {
	got := Select([]string{"scripts/check-one.sh"}, nil, []string{"scripts/check-one.sh", "scripts/check-other.sh"})
	if !reflect.DeepEqual(got, []string{"scripts/check-one.sh"}) {
		t.Fatalf("Select = %v", got)
	}
}
