package evalsubstrate

import (
	"strings"
	"testing"
)

func TestCanonicalizeText_StripsBOM(t *testing.T) {
	in := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello\n")...)
	out := CanonicalizeText(in)
	if string(out) != "hello\n" {
		t.Fatalf("BOM not stripped: got %q", out)
	}
}

func TestCanonicalizeText_DropsZeroWidth(t *testing.T) {
	in := []byte(string([]rune{'a', zwsp, 'b', zwnj, 'c', zwj, 'd', zwnbsp, 'e'}))
	out := CanonicalizeText(in)
	if string(out) != "abcde" {
		t.Fatalf("zero-width not dropped: got %q", out)
	}
}

func TestCanonicalizeText_NFCNormalization(t *testing.T) {
	composed := []byte(string([]rune{'c', 'a', 'f', 0x00E9}))
	decomposed := []byte(string([]rune{'c', 'a', 'f', 'e', 0x0301}))
	cc := CanonicalizeText(composed)
	dc := CanonicalizeText(decomposed)
	if string(cc) != string(dc) {
		t.Fatalf("NFC normalization mismatch: %q vs %q", cc, dc)
	}
}

func TestCanonicalizeYAML_SortsKeys(t *testing.T) {
	in := []byte("z: 1\na: 2\n")
	out, err := CanonicalizeYAML(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "a: 2") {
		t.Fatalf("keys not sorted: %s", out)
	}
}

func TestCanonicalizeYAML_StripsComments(t *testing.T) {
	in := []byte("# leading\nfoo: bar  # inline\n")
	out, err := CanonicalizeYAML(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "leading") || strings.Contains(string(out), "inline") {
		t.Fatalf("comments not stripped: %q", out)
	}
}

func TestCanonicalizeYAML_LFNormalize(t *testing.T) {
	lf := []byte("foo: bar\nbaz: qux\n")
	crlf := []byte("foo: bar\r\nbaz: qux\r\n")
	a, err := CanonicalizeYAML(lf)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalizeYAML(crlf)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("CRLF not normalized: lf=%q crlf=%q", a, b)
	}
	if ContentHash(a) != ContentHash(b) {
		t.Fatalf("hashes differ: %s vs %s", ContentHash(a), ContentHash(b))
	}
}

func TestCanonicalizeYAML_SingleTrailingNewline(t *testing.T) {
	in := []byte("foo: bar\n\n\n\n")
	out, err := CanonicalizeYAML(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(out), "\n") {
		t.Fatalf("missing trailing LF: %q", out)
	}
	if strings.HasSuffix(string(out), "\n\n") {
		t.Fatalf("extra trailing LFs: %q", out)
	}
}

func TestCanonicalizeJSON_SortsKeysAndStripsWhitespace(t *testing.T) {
	in := []byte(`{ "z": 1, "a": [3, 2, 1] }`)
	out, err := CanonicalizeJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":[3,2,1],"z":1}` + "\n"
	if string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestCanonicalizeMarkdown_StripsTrailingWhitespace(t *testing.T) {
	in := []byte("# Title  \n\nbody  \r\n  \n")
	out := CanonicalizeMarkdown(in)
	want := "# Title\n\nbody\n"
	if string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestContentHash_Stable(t *testing.T) {
	a := ContentHash([]byte("hello\n"))
	b := ContentHash([]byte("hello\n"))
	if a != b {
		t.Fatalf("hash not deterministic: %s vs %s", a, b)
	}
	if !strings.HasPrefix(a, "sha256:") {
		t.Fatalf("hash prefix missing: %s", a)
	}
}

func TestCRLFEdit_ProducesIdenticalHash(t *testing.T) {
	mac := []byte("foo: bar\nbaz: qux\n")
	wsl := []byte("foo: bar\r\nbaz: qux\r\n")
	macHash, err := canonAndHash(mac, ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	wslHash, err := canonAndHash(wsl, ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	if macHash != wslHash {
		t.Fatalf("CRLF crash-bug: %s vs %s", macHash, wslHash)
	}
}

func TestBOMPrefixedSkill_ProducesSameHashAsNoBOM(t *testing.T) {
	plain := []byte("# Skill\n\nbody\n")
	bom := append([]byte{0xEF, 0xBB, 0xBF}, plain...)
	plainHash, err := canonAndHash(plain, ".md")
	if err != nil {
		t.Fatal(err)
	}
	bomHash, err := canonAndHash(bom, ".md")
	if err != nil {
		t.Fatal(err)
	}
	if plainHash != bomHash {
		t.Fatalf("BOM should be stripped: %s vs %s", plainHash, bomHash)
	}
}

func TestZeroWidthChars_DontChangeHash(t *testing.T) {
	plain := []byte("# Skill\n\nbody\n")
	zw := []byte(string([]rune{'#', ' ', 'S', 'k', 'i', 'l', 'l', '\n', '\n', 'b', 'o', zwsp, 'd', 'y', '\n'}))
	plainHash, err := canonAndHash(plain, ".md")
	if err != nil {
		t.Fatal(err)
	}
	zwHash, err := canonAndHash(zw, ".md")
	if err != nil {
		t.Fatal(err)
	}
	if plainHash != zwHash {
		t.Fatalf("zero-width should be normalized: %s vs %s", plainHash, zwHash)
	}
}

func canonAndHash(in []byte, suffix string) (string, error) {
	switch suffix {
	case ".yaml", ".yml":
		c, err := CanonicalizeYAML(in)
		if err != nil {
			return "", err
		}
		return ContentHash(c), nil
	case ".json":
		c, err := CanonicalizeJSON(in)
		if err != nil {
			return "", err
		}
		return ContentHash(c), nil
	case ".md", ".markdown", ".txt":
		return ContentHash(CanonicalizeMarkdown(in)), nil
	default:
		return ContentHash(CanonicalizeText(in)), nil
	}
}
