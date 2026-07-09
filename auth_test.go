package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func newTestAuth(t *testing.T) *AuthService {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAuthService(db, "test-secret")
}

func TestRegister_Success(t *testing.T) {
	a := newTestAuth(t)

	user, token, err := a.Register("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected normalized email, got %q", user.Email)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}

	claims, err := a.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("token UserID %q != user ID %q", claims.UserID, user.ID)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	a := newTestAuth(t)

	if _, _, err := a.Register("alice@example.com", "password123"); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	// Same email, different case — must map the UNIQUE violation to ErrEmailTaken.
	_, _, err := a.Register("Alice@Example.com", "password456")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	a := newTestAuth(t)

	for _, email := range []string{"not-an-email", "a b@c.com", "@nouser.com"} {
		_, _, err := a.Register(email, "password123")
		if !errors.Is(err, ErrEmailInvalid) {
			t.Errorf("email %q: expected ErrEmailInvalid, got %v", email, err)
		}
	}
}

func TestLogin_WrongPasswordAndUnknownEmail(t *testing.T) {
	a := newTestAuth(t)

	if _, _, err := a.Register("alice@example.com", "password123"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, _, err := a.Login("alice@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCreds) {
		t.Errorf("wrong password: expected ErrInvalidCreds, got %v", err)
	}
	if _, _, err := a.Login("nobody@example.com", "password123"); !errors.Is(err, ErrInvalidCreds) {
		t.Errorf("unknown email: expected ErrInvalidCreds, got %v", err)
	}
	if _, _, err := a.Login("alice@example.com", "password123"); err != nil {
		t.Errorf("valid login failed: %v", err)
	}
}
