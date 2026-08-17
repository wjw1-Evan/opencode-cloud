package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"devcapsule/backend/internal/model"
)

func TestFullStackUserAccess(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")

	// create a user + template + container record (running) so the proxy routes
	user := addUser(t, st, "stu001", "pass12345", "user")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "app on path %s auth=%s", r.URL.Path, r.Header.Get("Authorization"))
	}))
	defer upstream.Close()

	// point the proxy upstream at our fake container server
	s.px.SetHostForForTesting(func(rec *model.Container) string { return "127.0.0.1" })

	tplID := "tpl1"
	st.CreateTemplate(t.Context(), &model.Template{ID: tplID, Name: "student", Image: "x", InternalPort: portOf(upstream.URL)})
	st.CreateContainer(t.Context(), &model.Container{
		ID:            model.NewID(),
		UserID:        user.ID,
		TemplateID:    tplID,
		ContainerID:   "fake",
		ContainerName: "user-stu001",
		Status:        model.ContainerRunning,
		InternalPort:  portOf(upstream.URL),
		Secret:        "s3cret",
	})

	token := s.login(t, "stu001", "pass12345")
	rec := s.do(t, "GET", "/hello", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "app on path /hello") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Basic ") {
		t.Fatal("expected upstream basic auth injected")
	}

	// anonymous / admin requests never reach the container
	rec = s.do(t, "GET", "/hello", "", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "app on path") {
		t.Fatalf("anonymous should get SPA, got %d %s", rec.Code, rec.Body.String())
	}
	adminToken := s.login(t, "admin", "admin123")
	rec = s.do(t, "GET", "/hello", adminToken, "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "app on path") {
		t.Fatalf("admin should get SPA, got %d %s", rec.Code, rec.Body.String())
	}
}

// TestRootSilentRefresh simulates a full page load after the 30-min access
// token expired: only the refresh cookie (still valid for 24h) is usable.
// The root route must silently rotate both tokens and proxy through instead
// of bouncing the student to the login page.
func TestRootSilentRefresh(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "pass12345", "user")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "app on path %s auth=%s", r.URL.Path, r.Header.Get("Authorization"))
	}))
	defer upstream.Close()
	s.px.SetHostForForTesting(func(rec *model.Container) string { return "127.0.0.1" })

	tplID := "tpl1"
	st.CreateTemplate(t.Context(), &model.Template{ID: tplID, Name: "student", Image: "x", InternalPort: portOf(upstream.URL)})
	st.CreateContainer(t.Context(), &model.Container{
		ID:            model.NewID(),
		UserID:        user.ID,
		TemplateID:    tplID,
		ContainerID:   "fake",
		ContainerName: "user-stu001",
		Status:        model.ContainerRunning,
		InternalPort:  portOf(upstream.URL),
		Secret:        "s3cret",
	})

	// log in to obtain a real refresh cookie
	login := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"pass12345"}`)
	var refresh string
	for _, c := range login.Result().Cookies() {
		if c.Name == "refresh_token" {
			refresh = c.Value
		}
	}
	if refresh == "" {
		t.Fatal("no refresh cookie from login")
	}

	// expired/garbage access cookie + valid refresh cookie, full page load
	req := httptest.NewRequest("GET", "/hello", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "expired.junk.token"})
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "app on path") {
		t.Fatalf("expired access + valid refresh should proxy through, got %d %s", rr.Code, rr.Body.String())
	}
	rotated := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "access_token" && c.Value != "" && c.Value != "expired.junk.token" {
			rotated = true
		}
	}
	if !rotated {
		t.Fatal("expected rotated access cookie on silent refresh")
	}

	// same request without any refresh cookie must keep serving the SPA
	req = httptest.NewRequest("GET", "/hello", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "expired.junk.token"})
	rr = httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "app on path") {
		t.Fatalf("no refresh cookie should get SPA, got %d %s", rr.Code, rr.Body.String())
	}
}

func portOf(rawURL string) int {
	rawURL = strings.TrimPrefix(rawURL, "http://")
	host := rawURL
	if i := strings.Index(rawURL, ":"); i >= 0 {
		host = rawURL[i+1:]
	}
	var port int
	fmt.Sscanf(host, "%d", &port)
	return port
}
