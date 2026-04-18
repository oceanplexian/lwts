package board

import (
	"strings"
	"unicode"
)

// snippetLen is the target length in characters of the rendered snippet. Small
// enough to be cheap for agents burning context windows on many search calls;
// big enough to show meaningful surrounding context.
const snippetLen = 140

// extractSnippet returns a short excerpt of `body` that best explains why a
// search for `query` matched the card. Deterministic, cheap, no embeddings:
//
//  1. Try case-insensitive whole-word matches for each query token and return
//     a window centered on the first hit. This is the common case for both
//     lexical matches and semantic matches where the topic terms appear
//     verbatim.
//  2. Fall back to the leading chunk of the body (skipping markdown fluff)
//     so the agent at least sees what the card is about.
//
// Returned text has surrounding whitespace normalized to single spaces.
func extractSnippet(query, body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Token-window search
	bodyLower := strings.ToLower(body)
	for _, tok := range tokenize(query) {
		tokLower := strings.ToLower(tok)
		idx := findWord(bodyLower, tokLower)
		if idx >= 0 {
			return windowAround(body, idx, len(tok))
		}
	}

	// Fallback: first non-trivial chunk.
	return leadingChunk(body)
}

// tokenize splits a query into alphanumeric tokens at least 3 chars long.
// Short tokens like "a" or "is" would match everywhere and aren't useful for
// anchoring a snippet. We keep the raw casing for nicer rendering in logs —
// the caller lowercases before comparison.
func tokenize(q string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() >= 3 {
			out = append(out, cur.String())
		}
		cur.Reset()
	}
	if cur.Len() >= 3 {
		out = append(out, cur.String())
	}
	return out
}

// findWord returns the index of the first whole-word occurrence of `tok` in
// `haystack`, or -1. Both inputs should already be lowercased.
func findWord(haystack, tok string) int {
	start := 0
	for {
		idx := strings.Index(haystack[start:], tok)
		if idx < 0 {
			return -1
		}
		pos := start + idx
		// Boundary checks examine the *adjacent* characters, not the match
		// itself: pos-1 to the left, pos+len(tok) to the right. A position
		// just past the end of the string counts as a boundary.
		leftOK := pos == 0 || !isWordRune(haystack[pos-1])
		rightEnd := pos + len(tok)
		rightOK := rightEnd >= len(haystack) || !isWordRune(haystack[rightEnd])
		if leftOK && rightOK {
			return pos
		}
		start = pos + 1
	}
}

func isWordRune(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b >= 0x80
}

// windowAround returns a ~snippetLen-char window of `body` centered on the
// match at `matchIdx`. Trims to word boundaries, collapses whitespace, and
// adds leading/trailing ellipses when the window doesn't reach the edges.
func windowAround(body string, matchIdx, matchLen int) string {
	const halfWindow = snippetLen / 2
	startRaw := matchIdx - halfWindow
	endRaw := matchIdx + matchLen + halfWindow

	leadEllipsis := startRaw > 0
	trailEllipsis := endRaw < len(body)
	if startRaw < 0 {
		startRaw = 0
	}
	if endRaw > len(body) {
		endRaw = len(body)
	}

	// Nudge to nearest whitespace so we don't cut words in half.
	start := startRaw
	if leadEllipsis {
		for start < matchIdx && !unicode.IsSpace(rune(body[start])) {
			start++
		}
		if start < len(body) && unicode.IsSpace(rune(body[start])) {
			start++
		}
	}
	end := endRaw
	if trailEllipsis {
		for end > matchIdx+matchLen && !unicode.IsSpace(rune(body[end-1])) {
			end--
		}
		if end > 0 && unicode.IsSpace(rune(body[end-1])) {
			end--
		}
	}
	if end <= start {
		end = endRaw
	}

	out := collapseSpaces(body[start:end])
	if leadEllipsis {
		out = "…" + out
	}
	if trailEllipsis {
		out = out + "…"
	}
	return out
}

// leadingChunk returns the first ~snippetLen chars of `body` with markdown
// headers and consecutive whitespace stripped. Used when the query tokens
// don't appear in the body (common for paraphrased semantic hits).
func leadingChunk(body string) string {
	clean := collapseSpaces(body)
	if len(clean) <= snippetLen {
		return clean
	}
	cut := snippetLen
	// Prefer a word boundary for the ellipsis.
	for i := cut; i > snippetLen-30 && i < len(clean); i-- {
		if unicode.IsSpace(rune(clean[i])) {
			cut = i
			break
		}
	}
	return clean[:cut] + "…"
}

// collapseSpaces turns all runs of whitespace (including newlines) into a
// single space and trims.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}
