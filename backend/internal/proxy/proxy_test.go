package proxy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"devcapsule/backend/internal/auth"
	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/model"
	"devcapsule/backend/internal/store"
)

func newTestProxy(t *testing.T) (*Proxy, *store.Memory, *auth.TokenManager) {
	t.Helper()
	st := store.NewMemory()
	tm := auth.NewTokenManager("test-secret")
	orch := docker.NewOrchestrator(&docker.Client{}, st, "testnet", 5)
	p := New(tm, st, orch)
	return p, st, tm
}

// authedRequest builds a request with the user's claims already injected,
// simulating what the JWT middleware does.
func authedRequest(t *testing.T, tm *auth.TokenManager, user *model.User, path string) *http.Request {
	t.Helper()
	token, _, err := tm.Issue(user.ID, user.Username, string(user.Role))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://localhost"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), ContextUser, claims)
	return req.WithContext(ctx)
}

func setupUserWithContainer(t *testing.T, st store.Store, username, secret string, port int) *model.User {
	t.Helper()
	hash, err := auth.HashPassword("pass12345")
	if err != nil {
		t.Fatal(err)
	}
	user := &model.User{
		ID:           model.NewID(),
		Username:     username,
		PasswordHash: hash,
		Role:         model.RoleUser,
	}
	if err := st.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	rec := &model.Container{
		ID:            model.NewID(),
		UserID:        user.ID,
		ContainerID:   "fake-" + username,
		ContainerName: "user-" + username,
		Status:        model.ContainerRunning,
		InternalPort:  port,
		Secret:        secret,
	}
	if err := st.CreateContainer(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestProxyForwardsWithBasePathAndAuth(t *testing.T) {
	var gotPath, gotAuth, gotHost string
	var gotHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		gotHeaders = r.Header
		fmt.Fprintln(w, "hello from container")
	}))
	defer upstream.Close()
	upPort := portOf(upstream.URL)

	p, st, tm := newTestProxy(t)
	p.hostFor = func(rec *model.Container) string { return "127.0.0.1" }
	user := setupUserWithContainer(t, st, "stu001", "topsecret", upPort)

	req := authedRequest(t, tm, user, "/api/session")
	req.Host = "lab.example.com"
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hello from container") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	// the original path is forwarded as-is (root-path routing, no prefix)
	if gotPath != "/api/session" {
		t.Fatalf("expected original path, got %q", gotPath)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("opencode:topsecret"))
	if gotAuth != want {
		t.Fatalf("expected basic auth %q, got %q", want, gotAuth)
	}
	// original Host must be preserved so tools can match Origin against it
	if gotHost != "lab.example.com" {
		t.Fatalf("expected original Host preserved, got %q", gotHost)
	}
	if gotHeaders.Get("X-Forwarded-Host") != "lab.example.com" {
		t.Fatalf("expected X-Forwarded-Host, got %q", gotHeaders.Get("X-Forwarded-Host"))
	}
	if gotHeaders.Get("X-Forwarded-For") == "" {
		t.Fatal("expected X-Forwarded-For header")
	}
}

func TestProxyStreamsSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fl := w.(http.Flusher)
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			fl.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer upstream.Close()
	upPort := portOf(upstream.URL)

	p, st, tm := newTestProxy(t)
	p.hostFor = func(rec *model.Container) string { return "127.0.0.1" }
	user := setupUserWithContainer(t, st, "stu001", "s3cr3t", upPort)
	req := authedRequest(t, tm, user, "/global/event")
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data: chunk-2") {
		t.Fatalf("expected all SSE chunks, got %q", rec.Body.String())
	}
}

func TestProxyRoutesToExtraPort(t *testing.T) {
	// main-port upstream
	mainUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "main on path %s", r.URL.Path)
	}))
	defer mainUp.Close()
	// extra-port upstream (port 3000)
	extraUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "extra on path %s", r.URL.Path)
	}))
	defer extraUp.Close()

	p, st, tm := newTestProxy(t)
	p.hostFor = func(rec *model.Container) string { return "127.0.0.1" }

	extraPort := portOf(extraUp.URL)

	hash, _ := auth.HashPassword("pass12345")
	user := &model.User{ID: model.NewID(), Username: "stu001", PasswordHash: hash, Role: model.RoleUser}
	st.CreateUser(context.Background(), user)

	tplID := "tpl1"
	st.CreateTemplate(context.Background(), &model.Template{
		ID:           tplID,
		Name:         "multi",
		Image:        "x",
		InternalPort: portOf(mainUp.URL),
		ExtraPorts:   []int{extraPort, 5173},
	})
	st.CreateContainer(context.Background(), &model.Container{
		ID:            model.NewID(),
		UserID:        user.ID,
		TemplateID:    tplID,
		ContainerID:   "fake-stu001",
		ContainerName: "user-stu001",
		Status:        model.ContainerRunning,
		InternalPort:  portOf(mainUp.URL),
		Secret:        "s3cret",
	})

	// main port forwards the original path
	req := authedRequest(t, tm, user, "/api/session")
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "main on path /api/session") {
		t.Fatalf("main port: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// extra port: prefix stripped, routed to extra upstream
	req = authedRequest(t, tm, user, "/port/"+strconv.Itoa(extraPort)+"/api/hello")
	rec = httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "extra on path /api/hello") {
		t.Fatalf("extra port: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// extra port root
	req = authedRequest(t, tm, user, "/port/"+strconv.Itoa(extraPort)+"/")
	rec = httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "extra on path /") {
		t.Fatalf("extra port root: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// unlisted port is rejected
	req = authedRequest(t, tm, user, "/port/9999/")
	rec = httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unlisted port, got %d", rec.Code)
	}

	// non-numeric port rejected
	req = authedRequest(t, tm, user, "/port/abc/")
	rec = httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-numeric port, got %d", rec.Code)
	}
}

func TestProxyRoutesToOwnContainerOnly(t *testing.T) {
	p, st, tm := newTestProxy(t)
	p.hostFor = func(rec *model.Container) string { return "127.0.0.1" }
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hidden")
	}))
	defer upstream.Close()

	user := setupUserWithContainer(t, st, "stu001", "s", portOf(upstream.URL))
	// the student's own request goes to their container
	req := authedRequest(t, tm, user, "/anything")
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for own container, got %d", rec.Code)
	}
}

func TestProxyNoContainer(t *testing.T) {
	p, st, tm := newTestProxy(t)
	user := addUserOnly(t, st, "stu001")
	req := authedRequest(t, tm, user, "/")
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for no container, got %d", rec.Code)
	}
}

func TestProxyDisabledUser(t *testing.T) {
	p, st, tm := newTestProxy(t)
	hash, _ := auth.HashPassword("pass12345")
	user := &model.User{ID: model.NewID(), Username: "stu001", PasswordHash: hash, Role: model.RoleUser, ManualDisabled: true}
	st.CreateUser(context.Background(), user)
	req := authedRequest(t, tm, user, "/")
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled user, got %d", rec.Code)
	}
}

func TestProxyDeletedUserRedirectsToLogout(t *testing.T) {
	p, st, tm := newTestProxy(t)
	hash, _ := auth.HashPassword("pass12345")
	user := &model.User{ID: model.NewID(), Username: "stu001", PasswordHash: hash, Role: model.RoleUser}
	st.CreateUser(context.Background(), user)
	req := authedRequest(t, tm, user, "/")
	// user deleted from the store while their JWT is still valid
	st.DeleteUser(context.Background(), user.ID)
	rec := httptest.NewRecorder()
	p.Handler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect to login, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/platform/auth/logout" {
		t.Fatalf("expected redirect to /platform/auth/logout, got %q", loc)
	}
}

func addUserOnly(t *testing.T, st store.Store, username string) *model.User {
	t.Helper()
	hash, _ := auth.HashPassword("pass12345")
	user := &model.User{ID: model.NewID(), Username: username, PasswordHash: hash, Role: model.RoleUser}
	if err := st.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	return user
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
