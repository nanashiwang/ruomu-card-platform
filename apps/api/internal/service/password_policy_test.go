package service

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/config"
)

func TestUserPasswordPolicyAllowsLowercaseOnly(t *testing.T) {
	policy := config.PasswordPolicyConfig{
		MinLength:     8,
		RequireLower:  true,
		RequireUpper:  false,
		RequireNumber: false,
	}

	if err := validatePassword(policy, "password"); err != nil {
		t.Fatalf("expected lowercase-only password to pass user policy, got %v", err)
	}
}

func TestAdminPasswordPolicyStillRequiresUppercaseAndNumber(t *testing.T) {
	policy := config.PasswordPolicyConfig{
		MinLength:     8,
		RequireUpper:  true,
		RequireLower:  true,
		RequireNumber: true,
	}

	for _, password := range []string{"password1", "Password"} {
		if err := validatePassword(policy, password); !errors.Is(err, ErrWeakPassword) {
			t.Fatalf("expected %q to fail admin policy, got %v", password, err)
		}
	}
}
