package docker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestSecretGenNext(t *testing.T) {
	g := &SecretGen{}
	s, err := g.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 24 {
		t.Fatalf("expected 24 chars, got %d: %s", len(s), s)
	}
	valid := regexp.MustCompile(`^[abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789]{24}$`)
	if !valid.MatchString(s) {
		t.Fatalf("invalid charset in secret: %s", s)
	}
	// two calls differ
	s2, _ := g.Next()
	if s == s2 {
		t.Fatal("two secrets should differ")
	}
}

func TestProbeHTTPSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ok, err := ProbeHTTP(context.Background(), "127.0.0.1", 0)
	// 0 won't match the test server port, so this should fail
	_ = err
	_ = ok

	// extract port from srv.URL and test properly
	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	ok, err = ProbeHTTP(context.Background(), "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected probe to succeed")
	}
}

func TestProbeHTTPFailure(t *testing.T) {
	ok, err := ProbeHTTP(context.Background(), "127.0.0.1", 1)
	if err == nil && ok {
		t.Fatal("expected probe to fail on port 1")
	}
}

func TestClientAvailable(t *testing.T) {
	var nilClient *Client
	if nilClient.Available() {
		t.Fatal("nil client should not be available")
	}
	zeroClient := &Client{}
	if zeroClient.Available() {
		t.Fatal("zero-value client should not be available")
	}
}
