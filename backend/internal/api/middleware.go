package api

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"devcapsule/backend/internal/model"
)

type ctxKey string

const ctxUser ctxKey = "user"

func userFrom(ctx context.Context) (*model.User, bool) {
	u, ok := ctx.Value(ctxUser).(*model.User)
	return u, ok
}

// JWTAuth validates the bearer token / cookie and loads the user.
func (s *Server) JWTAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		}
		if token == "" {
			if c, err := r.Cookie("access_token"); err == nil {
				token = c.Value
			}
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing token")
			return
		}
		claims, err := s.tm.ParseAccess(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		user, err := s.st.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		if st := user.EffectiveStatus(); st == model.StatusDisabled || st == model.StatusExpired {
			writeError(w, http.StatusForbidden, "account "+string(st))
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFrom(r.Context())
		if !ok || u.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "admin required")
			return
		}
		next(w, r)
	}
}

func (s *Server) RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer so streaming responses (SSE,
// long-polling) are not buffered by the request-log wrapper.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack supports WebSocket upgrades through the request-log wrapper.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	r.status = http.StatusSwitchingProtocols
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
}

type loginLimiter struct {
	mu     sync.Mutex
	hits   map[string]*window
	window time.Duration
	max    int
	lastGC time.Time
}

type window struct {
	count int
	reset time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{hits: map[string]*window{}, window: time.Minute, max: 10}
}

// deny records a failed login attempt for the key and reports true while the
// key is still within its failure budget. Once the budget is exceeded it
// reports false and the caller should respond 429. Successful logins never
// call deny, so a whole class logging in at once from one NAT IP is not
// blocked; only repeated failed attempts (brute force) are throttled.
func (l *loginLimiter) deny(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	// Opportunistic GC: evict expired windows to bound memory growth.
	if now.Sub(l.lastGC) >= l.window {
		for k, w := range l.hits {
			if now.After(w.reset) {
				delete(l.hits, k)
			}
		}
		l.lastGC = now
	}
	w, ok := l.hits[key]
	if !ok || now.After(w.reset) {
		l.hits[key] = &window{count: 1, reset: now.Add(l.window)}
		return true
	}
	w.count++
	return w.count <= l.max
}
