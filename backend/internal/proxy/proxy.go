package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"devcapsule/backend/internal/auth"
	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/model"
	"devcapsule/backend/internal/store"
)

type ctxKey string

const ContextUser ctxKey = "claims"

type Proxy struct {
	tm   *auth.TokenManager
	st   store.Store
	orch *docker.Orchestrator
	rp   *httputil.ReverseProxy
	// hostFor overrides the upstream hostname for tests.
	hostFor func(rec *model.Container) string
}

func New(tm *auth.TokenManager, st store.Store, orch *docker.Orchestrator) *Proxy {
	p := &Proxy{tm: tm, st: st, orch: orch}
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	p.rp = &httputil.ReverseProxy{
		Transport:     transport,
		FlushInterval: -1,
		Director:      p.director,
		ErrorHandler:  p.errorHandler,
	}
	return p
}

func (p *Proxy) director(req *http.Request) {
	target, ok := req.Context().Value(ctxKey("target")).(*url.URL)
	if !ok {
		return
	}
	rec, ok := req.Context().Value(ctxKey("container")).(*model.Container)
	if !ok {
		return
	}
	if up, ok := req.Context().Value(ctxKey("upstreamPath")).(string); ok && up != "" {
		req.URL.Path = up
	}
	req.URL.Scheme = "http"
	req.URL.Host = target.Host
	// Keep the original Host header: tools like code-server verify that the
	// request Origin matches the Host, so rewriting it to the container name
	// would reject browser WebSocket connections (HTTP 403 / WS 1006).
	req.Header.Set("X-Forwarded-Host", req.Host)
	if rec.Secret != "" {
		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("opencode:"+rec.Secret))
		req.Header.Set("Authorization", authHeader)
	}
	if req.Header.Get("X-Forwarded-For") == "" {
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}
	req.Header.Set("X-Forwarded-Proto", "http")
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	http.Error(w, "upstream unavailable: "+err.Error(), http.StatusBadGateway)
}

// Handler serves any authenticated student request by forwarding it to that
// student's container on the original path. The platform owns the root path:
// the JWT middleware decides which container (or the SPA) handles the request.
func (p *Proxy) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFrom(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := p.st.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "user not found", http.StatusForbidden)
			return
		}
		if user.Status == model.StatusDisabled {
			http.Error(w, "account disabled", http.StatusForbidden)
			return
		}
		if user.Status == model.StatusExpired {
			http.Error(w, "account expired", http.StatusForbidden)
			return
		}

		rec, err := p.st.GetContainerByUserID(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "no container provisioned", http.StatusNotFound)
			return
		}
		// Always verify against the live docker state before proxying: the DB
		// status can lag (external stop, host reboot, OOM), and forwarding to a
		// stopped container would 502 with "connection refused".
		if _, err := p.orch.SyncStatus(r.Context(), rec); err == nil && rec.Status != model.ContainerRunning {
			if err := p.orch.EnsureRunning(r.Context(), rec, user, p.templateFor(r.Context(), rec)); err != nil {
				http.Error(w, "container unavailable: "+err.Error(), http.StatusServiceUnavailable)
				return
			}
			// give the freshly started container time to bind its port
			waitCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			_ = p.orch.WaitHealthy(waitCtx, rec, p.templateFor(r.Context(), rec))
			cancel()
		}

		tpl := p.templateFor(r.Context(), rec)
		port := rec.InternalPort
		// Original path is forwarded as-is (no /u/ prefix exists anymore).
		upstreamPath := r.URL.Path
		if upstreamPath == "" {
			upstreamPath = "/"
		}

		// Extra port routing: /port/{port}/... -> container:{port}, prefix stripped.
		if strings.HasPrefix(r.URL.Path, "/port/") {
			segs := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/port/"), "/", 2)
			n, err := strconv.Atoi(segs[0])
			if err != nil || !containsPort(tpl.AllPorts(), n) {
				http.Error(w, "unknown port", http.StatusNotFound)
				return
			}
			port = n
			upstreamPath = "/"
			if len(segs) > 1 {
				upstreamPath = "/" + segs[1]
			}
		}

		target := &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", p.upstreamHost(rec), port)}
		ctx := context.WithValue(r.Context(), ctxKey("target"), target)
		ctx = context.WithValue(ctx, ctxKey("container"), rec)
		ctx = context.WithValue(ctx, ctxKey("upstreamPath"), upstreamPath)
		w = &responseRecorder{ResponseWriter: w, status: http.StatusOK, start: time.Now()}
		p.rp.ServeHTTP(w, r.WithContext(ctx))
		if rw, ok := w.(*responseRecorder); ok {
			go p.logAccess(user, r, rw)
		}
	})
}

func containsPort(ports []int, want int) bool {
	for _, p := range ports {
		if p == want {
			return true
		}
	}
	return false
}

func (p *Proxy) upstreamHost(rec *model.Container) string {
	if p.hostFor != nil {
		return p.hostFor(rec)
	}
	return rec.ContainerName
}

// SetHostForForTesting overrides upstream hostname resolution in tests.
func (p *Proxy) SetHostForForTesting(f func(rec *model.Container) string) {
	p.hostFor = f
}

func (p *Proxy) templateFor(ctx context.Context, rec *model.Container) *model.Template {
	tpl, err := p.st.GetTemplate(ctx, rec.TemplateID)
	if err != nil {
		return &model.Template{ID: rec.TemplateID, Image: "", InternalPort: rec.InternalPort, WorkspaceDir: docker.DefaultWorkDir}
	}
	return tpl
}

func (p *Proxy) logAccess(user *model.User, r *http.Request, rw *responseRecorder) {
	l := &model.AccessLog{
		UserID:    user.ID,
		Path:      r.URL.Path,
		Status:    rw.status,
		Bytes:     rw.bytes,
		LatencyMS: time.Since(rw.start).Milliseconds(),
	}
	// Use a detached context: the request context is canceled once the
	// handler returns, which would abort the async DB write.
	writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p.st.LogAccess(writeCtx, l)
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
	start  time.Time
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack lets the reverse proxy upgrade to WebSocket. The recorder's status is
// snapshotted as 101 before handing the raw connection to the caller.
func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	r.status = http.StatusSwitchingProtocols
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
}

func ClaimsFrom(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(ContextUser).(*auth.Claims)
	return c, ok
}
