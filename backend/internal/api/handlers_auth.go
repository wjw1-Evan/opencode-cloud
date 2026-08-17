package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"devcapsule/backend/internal/auth"
	"devcapsule/backend/internal/model"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimit.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many attempts, try later")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	user, err := s.st.GetUserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if user.Status == model.StatusDisabled {
		writeError(w, http.StatusForbidden, "account disabled")
		return
	}
	if user.Status == model.StatusExpired {
		writeError(w, http.StatusForbidden, "account expired")
		return
	}
	access, refresh, err := s.tm.Issue(user.ID, user.Username, string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "access_token", Value: access,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 1800,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: refresh,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400,
	})
	writeData(w, map[string]any{
		"user":     user,
		"access":   access,
		"refresh":  refresh,
		"redirect": "/portal",
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Refresh string `json:"refresh"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if body.Refresh == "" {
		if c, err := r.Cookie("refresh_token"); err == nil {
			body.Refresh = c.Value
		}
	}
	claims, err := s.tm.ParseRefresh(body.Refresh)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	user, err := s.st.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}
	if user.Status == model.StatusDisabled || user.Status == model.StatusExpired {
		writeError(w, http.StatusForbidden, "account "+string(user.Status))
		return
	}
	access, refresh, err := s.tm.Issue(user.ID, user.Username, string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "access_token", Value: access,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 1800,
	})
	// Rotate the refresh token in the cookie too: the browser keeps using the
	// cookie on the next silent refresh, so without this it would hold a stale
	// token until the 24h window ends and force a re-login.
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: refresh,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400,
	})
	writeData(w, map[string]any{"access": access, "refresh": refresh})
}

// handleLogout clears the auth cookies and redirects to the login page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: "access_token", Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0),
	})
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// changePasswordRequest lets a logged-in user set a new password themselves.
type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleChangePassword verifies the current password and updates it.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}
	ok, err := auth.VerifyPassword(req.OldPassword, u.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "old password incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password")
		return
	}
	u.PasswordHash = hash
	u.PasswordPlain = req.NewPassword
	if err := s.st.UpdateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, map[string]any{"updated": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	// attach the user's container (with ports) so the portal can show status
	rec, err := s.st.GetContainerByUserID(r.Context(), u.ID)
	if err == nil {
		s.orch.SyncStatus(r.Context(), rec)
		cv := toContainerView(rec)
		if t, terr := s.st.GetTemplate(r.Context(), rec.TemplateID); terr == nil {
			cv.ExtraPorts = t.ExtraPorts
		}
		writeData(w, map[string]any{
			"user":      u,
			"container": cv,
		})
		return
	}
	writeData(w, map[string]any{"user": u})
}

// handleInitialized returns whether the system has been set up (admin account exists).
func (s *Server) handleInitialized(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	initialized := false
	for _, u := range users {
		if u.Role == model.RoleAdmin {
			initialized = true
			break
		}
	}
	writeData(w, map[string]any{"initialized": initialized})
}

type initializeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleInitialize creates the initial admin account. Only works once.
func (s *Server) handleInitialize(w http.ResponseWriter, r *http.Request) {
	// Serialize the check-then-create so two concurrent initialize requests
	// cannot both pass the "no admin yet" check and create two admins.
	s.initMu.Lock()
	defer s.initMu.Unlock()
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	for _, u := range users {
		if u.Role == model.RoleAdmin {
			writeError(w, http.StatusConflict, "system already initialized")
			return
		}
	}
	var req initializeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password")
		return
	}
	now := time.Now().UTC()
	if err := s.st.CreateUser(r.Context(), &model.User{
		ID:            model.NewID(),
		Username:      req.Username,
		PasswordHash:  hash,
		PasswordPlain: req.Password,
		Role:          model.RoleAdmin,
		Status:        model.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "create admin failed")
		return
	}
	writeData(w, map[string]any{"initialized": true})
}

func clientIP(r *http.Request) string {
	// X-Forwarded-For is only trusted when the direct peer is a proxy we
	// control (loopback / private network, e.g. the nginx sidecar). Otherwise
	// an attacker hitting the API directly could spoof the header and bypass
	// the login rate limit.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" && isTrustedProxy(r.RemoteAddr) {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isTrustedProxy reports whether the direct peer is loopback or on a private
// network range (the deployment's nginx sidecar connects from the bridge).
func isTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}
