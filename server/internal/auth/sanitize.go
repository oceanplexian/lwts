package auth

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	htmlTagRe  = regexp.MustCompile(`<[^>]*>`)
)

// SanitizeEmail trims, lowercases, and validates an email address.
func SanitizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", fmt.Errorf("email is required")
	}
	if len(email) > 255 {
		return "", fmt.Errorf("email must be 255 characters or less")
	}
	if !emailRegex.MatchString(email) {
		return "", fmt.Errorf("invalid email format")
	}
	return email, nil
}

// SanitizeName trims whitespace and strips HTML tags from a name.
func SanitizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = htmlTagRe.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if len(name) < 2 {
		return "", fmt.Errorf("name must be at least 2 characters")
	}
	return name, nil
}

// ValidatePassword checks password requirements.
func ValidatePassword(password string) error {
	if len(password) == 0 {
		return fmt.Errorf("password is required")
	}
	return nil
}
