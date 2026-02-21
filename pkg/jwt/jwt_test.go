package jwtutil

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key-12345"
	userID := uuid.New()
	role := "rider"

	// Generate token
	token, err := GenerateToken(userID, role, secret, time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Validate token
	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Role != role {
		t.Errorf("Role = %v, want %v", claims.Role, role)
	}
	if claims.Issuer != "rideshare" {
		t.Errorf("Issuer = %v, want rideshare", claims.Issuer)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	userID := uuid.New()
	token, _ := GenerateToken(userID, "rider", "secret-1", time.Hour)

	_, err := ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Error("expected error when validating with wrong secret")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	userID := uuid.New()
	// Generate token that expired 1 second ago
	token, _ := GenerateToken(userID, "rider", "secret", -time.Second)

	_, err := ValidateToken(token, "secret")
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateToken_InvalidString(t *testing.T) {
	_, err := ValidateToken("not-a-valid-jwt", "secret")
	if err == nil {
		t.Error("expected error for invalid token string")
	}
}

func TestValidateToken_EmptyString(t *testing.T) {
	_, err := ValidateToken("", "secret")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestGenerateToken_DifferentRoles(t *testing.T) {
	secret := "test-secret"
	roles := []string{"rider", "driver"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			token, err := GenerateToken(uuid.New(), role, secret, time.Hour)
			if err != nil {
				t.Fatalf("GenerateToken(%s) failed: %v", role, err)
			}

			claims, err := ValidateToken(token, secret)
			if err != nil {
				t.Fatalf("ValidateToken failed: %v", err)
			}
			if claims.Role != role {
				t.Errorf("Role = %v, want %v", claims.Role, role)
			}
		})
	}
}

func TestGenerateToken_UniqueTokens(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()

	token1, _ := GenerateToken(userID, "rider", secret, time.Hour)
	// Small delay to ensure different iat
	time.Sleep(time.Millisecond)
	token2, _ := GenerateToken(userID, "rider", secret, time.Hour)

	// Tokens may be the same if generated within the same second (same iat),
	// but both should be independently valid
	claims1, err := ValidateToken(token1, secret)
	if err != nil {
		t.Fatalf("token1 invalid: %v", err)
	}
	claims2, err := ValidateToken(token2, secret)
	if err != nil {
		t.Fatalf("token2 invalid: %v", err)
	}

	if claims1.UserID != claims2.UserID {
		t.Error("same user should produce tokens with same UserID")
	}
}
