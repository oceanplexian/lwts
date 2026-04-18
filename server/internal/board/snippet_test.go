package board

import (
	"strings"
	"testing"
)

func TestExtractSnippet_MatchedToken(t *testing.T) {
	body := "The header and filter bar currently scroll with the page content " +
		"when the user moves down the list. We need the header fixed so that " +
		"only the content area scrolls."
	got := extractSnippet("broken page scroll", body)
	if !strings.Contains(strings.ToLower(got), "scroll") {
		t.Fatalf("expected snippet to center on matched token, got: %q", got)
	}
	if len(got) > snippetLen+10 { // +10 for potential ellipses
		t.Fatalf("snippet too long: %d chars: %q", len(got), got)
	}
}

func TestExtractSnippet_FallsBackToLeadingChunk(t *testing.T) {
	body := "This is a relatively short description with no matching terms at all. " +
		"It has multiple sentences for realism but nothing that lines up."
	got := extractSnippet("kubernetes deployment manifest", body)
	if got == "" {
		t.Fatal("expected non-empty fallback snippet")
	}
	if !strings.HasPrefix(got, "This is") {
		t.Fatalf("expected leading chunk, got: %q", got)
	}
}

func TestExtractSnippet_CollapsesWhitespace(t *testing.T) {
	body := "Line one\n\n  Line two  \twith\ttabs\n\nLine three"
	got := extractSnippet("line two", body)
	if strings.Contains(got, "\n") || strings.Contains(got, "\t") {
		t.Fatalf("expected whitespace collapsed, got: %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("expected single spaces only, got: %q", got)
	}
}

func TestExtractSnippet_EllipsesAtBoundaries(t *testing.T) {
	body := strings.Repeat("abc def ", 50) + " TARGET " + strings.Repeat("ghi jkl ", 50)
	got := extractSnippet("target", body)
	if !strings.HasPrefix(got, "…") {
		t.Errorf("expected leading ellipsis for mid-body match, got: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected trailing ellipsis for mid-body match, got: %q", got)
	}
}

func TestExtractSnippet_EmptyBody(t *testing.T) {
	if got := extractSnippet("x", ""); got != "" {
		t.Errorf("expected empty, got: %q", got)
	}
}

func TestTokenize_SkipsShortTokens(t *testing.T) {
	toks := tokenize("a is i x modal bug")
	for _, tok := range toks {
		if len(tok) < 3 {
			t.Errorf("tokenize returned short token: %q", tok)
		}
	}
	// a, is, i, x are all too short; only modal + bug survive.
	if len(toks) != 2 {
		t.Fatalf("expected 2 tokens (modal, bug), got %d: %v", len(toks), toks)
	}
}

func TestFindWord_RequiresBoundary(t *testing.T) {
	// "modal" should NOT match "modals" as a whole word
	if findWord("i have modals here", "modal") >= 0 {
		t.Error("expected no match: 'modal' inside 'modals'")
	}
	if findWord("the modal is broken", "modal") != 4 {
		t.Error("expected match at position 4")
	}
}

func TestLeadingChunk_StopsAtWordBoundary(t *testing.T) {
	body := strings.Repeat("averylongwordwithoutwhitespace", 20)
	got := leadingChunk(body)
	if len(got) > snippetLen+3 { // +3 for "…"
		t.Fatalf("fallback exceeded cap: %d", len(got))
	}
}

func TestScoreLexical_TitleBeatsDescription(t *testing.T) {
	ts, _ := scoreLexical("foo", "foo bar", "unrelated")
	ds, _ := scoreLexical("foo", "bar baz", "foo somewhere")
	if ts <= ds {
		t.Errorf("title match score %.2f should exceed description-only %.2f", ts, ds)
	}
}

func TestIsDoneColumn(t *testing.T) {
	for _, tt := range []struct {
		id   string
		want bool
	}{
		{"done", true},
		{"Done", true},
		{"cleared", true},
		{"complete", true},
		{"completed", true},
		{"resolved", true},
		{"in-progress", false},
		{"todo", false},
		{"backlog", false},
		{"", false},
	} {
		if got := isDoneColumn(tt.id); got != tt.want {
			t.Errorf("isDoneColumn(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}
