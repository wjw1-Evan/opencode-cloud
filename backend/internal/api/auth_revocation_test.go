package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"devcapsule/backend/internal/model"
)

func cookieValue(t *testing.T, rec *httptest.ResponseRecorder, name string) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func refreshWithCookie(t *testing.T, s *Server, refresh string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/platform/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

// TestRefreshTokenReplayRejected verifies a refresh token can only be used
// once: replaying it after a successful rotation is rejected.
func TestRefreshTokenReplayRejected(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")

	login := s.do(t, "POST", "/platform/auth/login", "", `{"username":"admin","password":"admin123"}`)
	old := cookieValue(t, login, "refresh_token")
	if old == "" {
		t.Fatal("no refresh cookie from login")
	}

	rr := refreshWithCookie(t, s, old)
	if rr.Code != http.StatusOK {
		t.Fatalf("first refresh must succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	rotated := cookieValue(t, rr, "refresh_token")
	if rotated == "" || rotated == old {
		t.Fatal("expected a rotated refresh cookie")
	}

	rr = refreshWithCookie(t, s, old)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh token must be rejected, got %d body=%s", rr.Code, rr.Body.String())
	}

	// the freshly rotated token still works (chain continues)
	rr = refreshWithCookie(t, s, rotated)
	if rr.Code != http.StatusOK {
		t.Fatalf("rotated token should refresh, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestLogoutRevokesRefreshTokens verifies logout invalidates outstanding
// refresh tokens server-side, not just clearing the browser cookies.
func TestLogoutRevokesRefreshTokens(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")

	login := s.do(t, "POST", "/platform/auth/login", "", `{"username":"admin","password":"admin123"}`)
	refresh := cookieValue(t, login, "refresh_token")
	if refresh == "" {
		t.Fatal("no refresh cookie from login")
	}

	logoutReq := httptest.NewRequest("GET", "/platform/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	logoutRec := httptest.NewRecorder()
	s.Router().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusFound {
		t.Fatalf("logout should redirect, got %d", logoutRec.Code)
	}

	if rr := refreshWithCookie(t, s, refresh); rr.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout must be rejected, got %d body=%s", rr.Code, rr.Body.String())
	}

	// a fresh login issues a brand-new token that is not revoked
	relogin := s.do(t, "POST", "/platform/auth/login", "", `{"username":"admin","password":"admin123"}`)
	again := cookieValue(t, relogin, "refresh_token")
	if again == "" {
		t.Fatal("no refresh cookie from re-login")
	}
	if rr := refreshWithCookie(t, s, again); rr.Code != http.StatusOK {
		t.Fatalf("fresh session after logout should refresh, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestSilentRefreshReplayRejected covers the cookie-only proxy path: after a
// silent refresh consumes the token, a full page load replaying the same
// refresh cookie must fall back to the SPA instead of proxying.
func TestSilentRefreshReplayRejected(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "pass12345", "user")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "app on path %s", r.URL.Path)
	}))
	defer upstream.Close()
	s.px.SetHostForForTesting(func(rec *model.Container) string { return "127.0.0.1" })

	st.CreateTemplate(t.Context(), &model.Template{ID: "tpl1", Name: "student", Image: "x", InternalPort: portOf(upstream.URL)})
	st.CreateContainer(t.Context(), &model.Container{
		ID:            model.NewID(),
		UserID:        user.ID,
		TemplateID:    "tpl1",
		ContainerID:   "fake",
		ContainerName: "user-stu001",
		Status:        model.ContainerRunning,
		InternalPort:  portOf(upstream.URL),
		Secret:        "s3cret",
	})

	login := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"pass12345"}`)
	refresh := cookieValue(t, login, "refresh_token")
	if refresh == "" {
		t.Fatal("no refresh cookie from login")
	}

	pageLoad := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/hello", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "expired.junk.token"})
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
		rr := httptest.NewRecorder()
		s.Router().ServeHTTP(rr, req)
		return rr
	}

	rr := pageLoad()
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "app on path") {
		t.Fatalf("first silent refresh should proxy, got %d %s", rr.Code, rr.Body.String())
	}
	if cookieValue(t, rr, "refresh_token") == refresh {
		t.Fatal("expected the refresh cookie to rotate on silent refresh")
	}

	rr = pageLoad()
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "app on path") {
		t.Fatalf("replayed silent refresh must fall back to SPA, got %d %s", rr.Code, rr.Body.String())
	}
}
