package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"devcapsule/backend/internal/auth"
	"devcapsule/backend/internal/config"
	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/model"
	"devcapsule/backend/internal/proxy"
	"devcapsule/backend/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Memory, *auth.TokenManager) {
	t.Helper()
	st := store.NewMemory()
	tm := auth.NewTokenManager("test-secret")
	cfg := config.Config{
		JWTSecret:   "test-secret",
		NetworkName: "testnet",
	}
	dc := &docker.Client{}
	orch := docker.NewOrchestrator(dc, st, cfg.NetworkName, 5)
	px := proxy.New(tm, st, orch)
	srv := &Server{
		cfg:        cfg,
		st:         st,
		tm:         tm,
		docker:     dc,
		orch:       orch,
		px:         px,
		logger:     log.New(io.Discard, "", 0),
		loginLimit: newLoginLimiter(),
	}
	return srv, st, tm
}

func addUser(t *testing.T, st store.Store, username, password, role string) *model.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	u := &model.User{
		ID:           model.NewID(),
		Username:     username,
		PasswordHash: hash,
		Role:         model.Role(role),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := st.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func (s *Server) do(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, r)
	return rec
}

func (s *Server) login(t *testing.T, username, password string) string {
	t.Helper()
	rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"`+username+`","password":"`+password+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d %s", username, rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Access string `json:"access"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	return out.Data.Access
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode json: %v body=%s", err, rec.Body.String())
	}
}

func issueToken(tm *auth.TokenManager, u *model.User) string {
	access, _, err := tm.Issue(u.ID, u.Username, string(u.Role))
	if err != nil {
		panic(err)
	}
	return access
}
