package auth

import (
	"testing"
	"time"
)

const testSecret = "test-jwt-secret-key-1234567890"

func TestIssueTokens(t *testing.T) {
	pair, jti, err := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	if err != nil {
		t.Fatalf("IssueTokens failed: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("access token is empty")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token is empty")
	}
	if jti == "" {
		t.Error("jti is empty")
	}
}

func TestParseAccessToken(t *testing.T) {
	pair, _, err := IssueTokens(testSecret, "user-123", "alice@test.com", "admin")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseAccessToken(testSecret, pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken failed: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want user-123", claims.Subject)
	}
	if claims.Email != "alice@test.com" {
		t.Errorf("Email = %q", claims.Email)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q", claims.Role)
	}
	if claims.Type != "access" {
		t.Errorf("Type = %q", claims.Type)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt is nil")
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < 14*time.Minute || ttl > 16*time.Minute {
		t.Errorf("TTL = %v, want ~15min", ttl)
	}
}

func TestParseAccessToken_WrongSecret(t *testing.T) {
	pair, _, _ := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	_, err := ParseAccessToken("wrong-secret", pair.AccessToken)
	if err == nil {
		t.Error("expected error with wrong secret")
	}
}

func TestParseRefreshToken(t *testing.T) {
	pair, jti, err := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseRefreshToken(testSecret, pair.RefreshToken)
	if err != nil {
		t.Fatalf("ParseRefreshToken failed: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q", claims.Subject)
	}
	if claims.Type != "refresh" {
		t.Errorf("Type = %q", claims.Type)
	}
	if claims.ID != jti {
		t.Errorf("JTI = %q, want %q", claims.ID, jti)
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < 6*24*time.Hour || ttl > 8*24*time.Hour {
		t.Errorf("TTL = %v, want ~7 days", ttl)
	}
}

func TestParseAccessToken_RejectsRefreshToken(t *testing.T) {
	pair, _, _ := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	_, err := ParseAccessToken(testSecret, pair.RefreshToken)
	if err == nil {
		t.Error("expected error when parsing refresh token as access token")
	}
}

func TestParseRefreshToken_RejectsAccessToken(t *testing.T) {
	pair, _, _ := IssueTokens(testSecret, "user-123", "alice@test.com", "member")
	_, err := ParseRefreshToken(testSecret, pair.AccessToken)
	if err == nil {
		t.Error("expected error when parsing access token as refresh token")
	}
}
