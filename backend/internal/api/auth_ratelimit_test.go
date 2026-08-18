package api

import (
	"net/http"
	"testing"
)

func TestLoginLimiterBlocksAfterMaxFailures(t *testing.T) {
	l := newLoginLimiter()
	key := "203.0.113.9"
	for i := 0; i < 10; i++ {
		if !l.deny(key) {
			t.Fatalf("failure %d should be within budget", i+1)
		}
	}
	if l.deny(key) {
		t.Fatal("11th failure should be blocked")
	}
	// a different IP keeps its own budget
	if !l.deny("198.51.100.7") {
		t.Fatal("fresh IP should not be blocked")
	}
}

// TestManySuccessfulLoginsNotRateLimited simulates a whole class logging in at
// once from the same IP: successful logins must not consume the failure budget.
func TestManySuccessfulLoginsNotRateLimited(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "stu001", "pass12345", "user")
	for i := 0; i < 15; i++ {
		rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"pass12345"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("successful login %d should not be rate limited, got %d", i+1, rec.Code)
		}
	}
}

func TestFailedLoginsTrigger429(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "stu001", "pass12345", "user")
	for i := 0; i < 10; i++ {
		rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"wrong"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("failed login %d should be 401, got %d", i+1, rec.Code)
		}
	}
	rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"wrong"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th failure should be 429, got %d", rec.Code)
	}
	// correct password while blocked still succeeds (success is not counted)
	rec = s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"pass12345"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct login after block should succeed, got %d", rec.Code)
	}
}
