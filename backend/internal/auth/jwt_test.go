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

func TestRefreshTokenRejectedAsAccess(t *testing.T) {
	tm := NewTokenManager("test-secret")
	_, refresh, err := tm.Issue("u1", "stu001", "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tm.ParseAccess(refresh); err == nil {
		t.Fatal("expected refresh token to be rejected by ParseAccess")
	}
	c, err := tm.ParseRefresh(refresh)
	if err != nil || c.UserID != "u1" {
		t.Fatalf("refresh token should parse via ParseRefresh: %v %+v", err, c)
	}
}

func TestAccessTokenRejectedAsRefresh(t *testing.T) {
	tm := NewTokenManager("test-secret")
	access, _, err := tm.Issue("u1", "stu001", "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tm.ParseRefresh(access); err == nil {
		t.Fatal("expected access token to be rejected by ParseRefresh")
	}
	c, err := tm.ParseAccess(access)
	if err != nil || c.UserID != "u1" {
		t.Fatalf("access token should parse via ParseAccess: %v %+v", err, c)
	}
}

func TestTokensIssuedWithinSameSecondAreDistinct(t *testing.T) {
	tm := NewTokenManager("test-secret")
	a1, r1, err := tm.Issue("u1", "stu001", "user")
	if err != nil {
		t.Fatal(err)
	}
	a2, r2, err := tm.Issue("u1", "stu001", "user")
	if err != nil {
		t.Fatal(err)
	}
	// identical payloads (same iat/exp) must still yield distinct tokens,
	// otherwise a refresh within the same second would return the same token
	if a1 == a2 {
		t.Fatal("expected distinct access tokens")
	}
	if r1 == r2 {
		t.Fatal("expected distinct refresh tokens")
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
