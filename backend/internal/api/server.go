package api

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"devcapsule/backend/internal/auth"
	"devcapsule/backend/internal/config"
	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/model"
	"devcapsule/backend/internal/proxy"
	"devcapsule/backend/internal/store"
)

//go:embed all:web/dist
var webFS embed.FS

type Server struct {
	cfg        config.Config
	st         store.Store
	tm         *auth.TokenManager
	docker     *docker.Client
	orch       *docker.Orchestrator
	px         *proxy.Proxy
	logger     *log.Logger
	loginLimit *loginLimiter
	initMu     sync.Mutex
}

func New(cfg config.Config, st store.Store, dc *docker.Client, logger *log.Logger) (*Server, error) {
	tm := auth.NewTokenManager(cfg.JWTSecret)
	orch := docker.NewOrchestrator(dc, st, cfg.NetworkName, cfg.BatchConcurrency)
	px := proxy.New(tm, st, orch)
	return &Server{
		cfg:        cfg,
		st:         st,
		tm:         tm,
		docker:     dc,
		orch:       orch,
		px:         px,
		logger:     logger,
		loginLimit: newLoginLimiter(),
	}, nil
}

// Router assembles all routes.
func (s *Server) Router() http.Handler {
	apiMux := http.NewServeMux()

	apiMux.HandleFunc("GET /auth/initialized", s.handleInitialized)
	apiMux.HandleFunc("POST /auth/initialize", s.handleInitialize)
	apiMux.HandleFunc("POST /auth/login", s.handleLogin)
	apiMux.HandleFunc("POST /auth/refresh", s.handleRefresh)
	apiMux.HandleFunc("GET /auth/logout", s.handleLogout)
	apiMux.HandleFunc("GET /auth/me", s.JWTAuth(s.handleMe))
	apiMux.HandleFunc("POST /auth/change-password", s.JWTAuth(s.handleChangePassword))

	// user management
	apiMux.HandleFunc("GET /admin/users", s.JWTAuth(s.AdminOnly(s.handleListUsers)))
	apiMux.HandleFunc("POST /admin/users/batch", s.JWTAuth(s.AdminOnly(s.handleBatchUsers)))
	apiMux.HandleFunc("POST /admin/users/batch/action", s.JWTAuth(s.AdminOnly(s.handleBatchUserAction)))
	apiMux.HandleFunc("PATCH /admin/users/{id}", s.JWTAuth(s.AdminOnly(s.handleUpdateUser)))
	apiMux.HandleFunc("DELETE /admin/users/{id}", s.JWTAuth(s.AdminOnly(s.handleDeleteUser)))
	apiMux.HandleFunc("GET /admin/users/export", s.JWTAuth(s.AdminOnly(s.handleExportUsers)))

	// containers
	apiMux.HandleFunc("GET /admin/containers", s.JWTAuth(s.AdminOnly(s.handleListContainers)))
	apiMux.HandleFunc("POST /admin/containers/batch", s.JWTAuth(s.AdminOnly(s.handleProvisionBatch)))
	apiMux.HandleFunc("POST /admin/containers/{id}/{action}", s.JWTAuth(s.AdminOnly(s.handleContainerAction)))
	apiMux.HandleFunc("GET /admin/containers/{id}/stats", s.JWTAuth(s.AdminOnly(s.handleContainerStats)))
	apiMux.HandleFunc("GET /admin/containers/stats/all", s.JWTAuth(s.AdminOnly(s.handleAllContainerStats)))

	// templates
	apiMux.HandleFunc("GET /admin/templates", s.JWTAuth(s.AdminOnly(s.handleListTemplates)))
	apiMux.HandleFunc("POST /admin/templates", s.JWTAuth(s.AdminOnly(s.handleCreateTemplate)))
	apiMux.HandleFunc("GET /admin/templates/{id}", s.JWTAuth(s.AdminOnly(s.handleGetTemplate)))
	apiMux.HandleFunc("PUT /admin/templates/{id}", s.JWTAuth(s.AdminOnly(s.handleUpdateTemplate)))
	apiMux.HandleFunc("DELETE /admin/templates/{id}", s.JWTAuth(s.AdminOnly(s.handleDeleteTemplate)))

	// images
	apiMux.HandleFunc("GET /admin/images", s.JWTAuth(s.AdminOnly(s.handleListImages)))
	apiMux.HandleFunc("POST /admin/images/import", s.JWTAuth(s.AdminOnly(s.handleImportImage)))
	apiMux.HandleFunc("GET /admin/images/{id}", s.JWTAuth(s.AdminOnly(s.handleGetImage)))
	apiMux.HandleFunc("DELETE /admin/images/{id}", s.JWTAuth(s.AdminOnly(s.handleDeleteImage)))
	apiMux.HandleFunc("POST /admin/images/pull", s.JWTAuth(s.AdminOnly(s.handlePullImage)))

	// platform config
	apiMux.HandleFunc("GET /admin/platform", s.JWTAuth(s.AdminOnly(s.handlePlatform)))

	// stats
	apiMux.HandleFunc("GET /admin/stats/dashboard", s.JWTAuth(s.AdminOnly(s.handleDashboard)))

	// dispatch:
	//   /platform/ -> platform REST API (unique prefix, never collides with
	//                 container root-relative APIs like /api/...)
	//   /api/health -> ops health check (no student context)
	//   /dc-static/ -> platform assets (vite base; unique so tool assets
	//                 served under /static or /assets are proxied untouched)
	//   /portal, /admin -> platform SPA pages (always platform UI)
	//   everything else -> logged-in student => proxy to their container,
	//                 anonymous / admin => platform SPA
	root := http.NewServeMux()
	root.Handle("/platform/", http.StripPrefix("/platform", apiMux))
	root.Handle("/dc-static/", http.StripPrefix("/dc-static", s.staticHandler()))
	root.Handle("/portal", s.staticHandler())
	root.Handle("/portal/", s.staticHandler())
	root.Handle("/initialize", s.staticHandler())
	root.Handle("/initialize/", s.staticHandler())
	root.Handle("/admin", s.staticHandler())
	root.Handle("/admin/", s.staticHandler())
	root.Handle("/api/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]any{"status": "ok"})
	}))
	root.Handle("/", s.rootHandler())

	return s.RequestLog(root)
}

// rootHandler routes the bare origin based on the caller's identity:
// a logged-in student is sent to their container (all root-relative paths),
// everyone else gets the SPA.
func (s *Server) rootHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		}
		if token == "" {
			if c, err := r.Cookie("access_token"); err == nil {
				token = c.Value
			}
		}
		var claims *auth.Claims
		var err error
		if token == "" {
			// No access token at all: still try the refresh cookie so a lost
			// access cookie (or a fresh page load after expiry) does not
			// bounce the student to the login screen mid-session.
			claims, _ = s.tryRefresh(w, r)
		} else {
			claims, err = s.tm.ParseAccess(token)
			if err != nil {
				// Access token expired but the 24h refresh cookie may still be
				// valid: silently rotate both tokens and keep the session
				// alive instead of serving the login page.
				if c, ok := s.tryRefresh(w, r); ok {
					claims = c
				}
			}
		}
		if claims == nil {
			s.staticHandler().ServeHTTP(w, r)
			return
		}
		if claims.Role != string(model.RoleUser) {
			// admin (or anything non-student) keeps the platform UI
			s.staticHandler().ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), proxy.ContextUser, claims)
		s.px.Handler(s.staticHandler()).ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SPA fallback: serve index.html for unknown non-file routes.
		// Handles both "/foo/bar" and stripped "foo/bar" paths.
		p := r.URL.Path
		rel := strings.TrimPrefix(p, "/")
		if rel == "" || rel == "index.html" {
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		if _, err := fs.Stat(sub, rel); err != nil {
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// StartBackground launches reconcile, expiry and idle-stop loops.
func (s *Server) StartBackground(ctx context.Context) {
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				users, err := s.st.ListUsers(ctx)
				if err != nil {
					continue
				}
				s.orch.ExpireAndStop(ctx)
				if s.cfg.IdleTimeoutMin > 0 {
					s.orch.IdleStop(ctx, time.Duration(s.cfg.IdleTimeoutMin)*time.Minute, users)
				}
				// sync DB container records with the real docker state
				s.orch.Reconcile(ctx)
			}
		}
	}()
}
