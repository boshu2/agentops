package verdictcheck

import "testing"

func TestStrictJSONInput(t *testing.T) {
	for _, input := range []string{`{"":"x","":"y"}`, `{"nested":{"a":1,"a":2}}`, `{"k":"\ud800"}`, `{"k":"\udc00"}`, `{} {}`, `null`, `[]`} {
		if _, err := DecodeObject([]byte(input)); err == nil {
			t.Errorf("admitted %s", input)
		}
	}
	for _, input := range []string{`{"k":"\ud83d\ude00"}`, `{"k":"\\ud800"}`} {
		if _, err := DecodeObject([]byte(input)); err != nil {
			t.Errorf("rejected %s: %v", input, err)
		}
	}
}
func TestCanonicalNumericJSON(t *testing.T) {
	input := []byte(`{"a":1e2,"b":1e-5,"c":1e16,"d":-0.0,"e":-0,"f":123456789012345678901234567890,"g":1.2345}`)
	raw, err := DecodeObject(input)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":100.0,"b":1e-05,"c":1e+16,"d":-0.0,"e":0,"f":123456789012345678901234567890,"g":1.2345}`
	if string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
}
