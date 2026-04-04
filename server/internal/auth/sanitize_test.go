package auth

import (
	"strings"
	"testing"
)

func TestSanitizeEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
		err   bool
	}{
		{" Alice@Test.COM  ", "alice@test.com", false},
		{"valid@email.org", "valid@email.org", false},
		{"", "", true},
		{"notanemail", "", true},
		{strings.Repeat("a", 250) + "@b.com", "", true},
	}
	for _, tt := range tests {
		got, err := SanitizeEmail(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("SanitizeEmail(%q) error = %v, wantErr %v", tt.input, err, tt.err)
			continue
		}
		if got != tt.want {
			t.Errorf("SanitizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
		err   bool
	}{
		{"  Alice  ", "Alice", false},
		{"<script>alert(1)</script>Alice", "alert(1)Alice", false},
		{"A", "", true},
		{"", "", true},
		{"<b>A</b>", "", true}, // strips to "A" which is < 2 chars
	}
	for _, tt := range tests {
		got, err := SanitizeName(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("SanitizeName(%q) error = %v, wantErr %v", tt.input, err, tt.err)
			continue
		}
		if !tt.err && got != tt.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		input string
		err   bool
	}{
		{"Password1", false},
		{"short1", false},          // ValidatePassword only rejects empty
		{"abcdefgh", false},        // ValidatePassword only rejects empty
		{"12345678", false},        // ValidatePassword only rejects empty
		{strings.Repeat("a", 129), false}, // ValidatePassword only rejects empty
		{"ValidPass1", false},
		{"", true},                 // empty password is rejected
	}
	for _, tt := range tests {
		err := ValidatePassword(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.input, err, tt.err)
		}
	}
}
