package embed

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Vector implements driver.Valuer so a []float32 can be written to a pgvector
// column via the standard sql/database interface (which pgx accepts).
//
// The pgvector text format is "[1.0,2.0,...]". pgx will accept this and the
// server will cast it to vector. We avoid pulling in the official pgvector-go
// driver to keep deps minimal — this whole package is opt-in.
type Vector []float32

func (v Vector) Value() (driver.Value, error) {
	if len(v) == 0 {
		return nil, nil
	}
	var b strings.Builder
	b.Grow(len(v) * 8)
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String(), nil
}

// String returns the pgvector text representation. Helpful for inline SQL or
// debugging.
func (v Vector) String() string {
	s, _ := v.Value()
	if s == nil {
		return ""
	}
	str, _ := s.(string)
	return str
}

// ParseVector turns a pgvector text representation back into a Vector. Used
// only by tests today; we never read embeddings out of the database in the
// hot path.
func ParseVector(s string) (Vector, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid vector literal: %q", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return Vector{}, nil
	}
	parts := strings.Split(inner, ",")
	v := make(Vector, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("invalid vector element %d: %w", i, err)
		}
		v[i] = float32(f)
	}
	return v, nil
}
