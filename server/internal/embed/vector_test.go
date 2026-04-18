package embed

import (
	"strings"
	"testing"
)

func TestVector_Value(t *testing.T) {
	v := Vector{0.1, 0.5, -0.25}
	got, err := v.Value()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		t.Fatalf("not bracketed: %q", s)
	}
	parsed, err := ParseVector(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 3 {
		t.Fatalf("len: %d", len(parsed))
	}
	if parsed[0] != 0.1 || parsed[1] != 0.5 || parsed[2] != -0.25 {
		t.Fatalf("roundtrip: %v", parsed)
	}
}

func TestVector_Empty(t *testing.T) {
	got, err := Vector{}.Value()
	if err != nil || got != nil {
		t.Fatalf("expected nil/nil, got %v %v", got, err)
	}
}

func TestParseVector_Errors(t *testing.T) {
	for _, s := range []string{"", "1,2,3", "[a]", "[1,b]"} {
		if _, err := ParseVector(s); err == nil {
			t.Errorf("expected error parsing %q", s)
		}
	}
}

func TestComposeCardText(t *testing.T) {
	if got := composeCardText("", ""); got != "" {
		t.Errorf("empty: %q", got)
	}
	if got := composeCardText("title", ""); got != "title" {
		t.Errorf("title only: %q", got)
	}
	if got := composeCardText("title", "desc"); got != "title. desc" {
		t.Errorf("both: %q", got)
	}
	long := strings.Repeat("a", maxInputChars+500)
	got := composeCardText("t", long)
	if len(got) != maxInputChars {
		t.Errorf("truncate: %d (want %d)", len(got), maxInputChars)
	}
}
