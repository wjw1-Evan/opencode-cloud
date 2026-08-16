package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenRoundTrip(t *testing.T) {
	tm := NewTokenManager("test-secret")
	access, refresh, err := tm.Issue("u1", "stu001", "user")
	if err != nil {
		t.Fatal(err)
	}
	if access == "" || refresh == "" {
		t.Fatal("expected tokens")
	}
	c, err := tm.Parse(access)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != "u1" || c.Username != "stu001" || c.Role != "user" {
		t.Fatalf("unexpected claims: %+v", c)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	tm := NewTokenManager("test-secret")
	token := signExpired(tm, "u1")
	if _, err := tm.Parse(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestWrongSecretRejected(t *testing.T) {
	tm1 := NewTokenManager("secret-1")
	tm2 := NewTokenManager("secret-2")
	access, _, err := tm1.Issue("u1", "stu001", "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tm2.Parse(access); err == nil {
		t.Fatal("expected token signed with different secret to fail")
	}
}

func signExpired(tm *TokenManager, uid string) string {
	now := time.Now()
	c := Claims{
		UserID: uid, Username: "x", Role: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
	}
	s, _ := tm.sign(c)
	return s
}
