package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("Password123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Error("hash is empty")
	}

	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost failed: %v", err)
	}
	if cost != 12 {
		t.Errorf("cost = %d, want 12", cost)
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("Password123")

	if !CheckPassword(hash, "Password123") {
		t.Error("CheckPassword returned false for correct password")
	}
	if CheckPassword(hash, "WrongPassword1") {
		t.Error("CheckPassword returned true for wrong password")
	}
}
