package api

import (
	"encoding/json"
	"io"
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
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	user, err := s.st.GetUserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if err != nil {
		if !s.loginLimit.deny(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many failed attempts, try later")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		if !s.loginLimit.deny(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many failed attempts, try later")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	switch user.EffectiveStatus() {
	case model.StatusDisabled:
		writeError(w, http.StatusForbidden, "account disabled")
		return
	case model.StatusExpired:
		writeError(w, http.StatusForbidden, "account expired")
		return
	}
	access, refresh, err := s.issueTokens(w, r, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	writeData(w, map[string]any{
		"user":     user,
		"access":   access,
		"refresh":  refresh,
		"redirect": "/portal",
	})
}

// issueTokens signs a fresh token pair, records the refresh token server-side
// (so it can be consumed exactly once and revoked on logout), and writes both
// cookies.
func (s *Server) issueTokens(w http.ResponseWriter, r *http.Request, user *model.User) (access, refresh string, err error) {
	access, refresh, err = s.tm.Issue(user.ID, user.Username, string(user.Role))
	if err != nil {
		return "", "", err
	}
	claims, err := s.tm.ParseRefresh(refresh)
	if err != nil {
		return "", "", err
	}
	if err := s.st.CreateRefreshToken(r.Context(), claims.ID, user.ID, claims.ExpiresAt.Time); err != nil {
		return "", "", err
	}
	s.setTokenCookies(w, access, refresh)
	return access, refresh, nil
}

// setTokenCookies writes fresh access/refresh cookies (also used to rotate
// both tokens after a silent refresh).
func (s *Server) setTokenCookies(w http.ResponseWriter, access, refresh string) {
	http.SetCookie(w, s.tokenCookie("access_token", access, 1800))
	http.SetCookie(w, s.tokenCookie("refresh_token", refresh, 86400))
}

// tokenCookie builds an auth cookie. Secure is applied only when the platform
// is served over HTTPS (COOKIE_SECURE=1); plain-HTTP dev keeps working with
// Secure disabled.
func (s *Server) tokenCookie(name, value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name: name, Value: value,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: maxAge, Secure: s.cfg.SecureCookies,
	}
	if maxAge < 0 {
		c.Expires = time.Unix(0, 0)
	}
	return c
}

// tryRefresh silently re-authenticates via the refresh cookie and rotates
// both token cookies. Returns the claims on success.
func (s *Server) tryRefresh(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	c, err := r.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		return nil, false
	}
	claims, err := s.tm.ParseRefresh(c.Value)
	if err != nil {
		return nil, false
	}
	user, err := s.st.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		return nil, false
	}
	if st := user.EffectiveStatus(); st == model.StatusDisabled || st == model.StatusExpired {
		return nil, false
	}
	// Consume first: replaying a stolen refresh token is rejected, and a
	// failure after this point only forces a re-login rather than leaving
	// the presented token usable.
	consumed, err := s.st.ConsumeRefreshToken(r.Context(), claims.ID, claims.UserID)
	if err != nil || !consumed {
		return nil, false
	}
	if _, _, err := s.issueTokens(w, r, user); err != nil {
		return nil, false
	}
	return claims, true
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Refresh string `json:"refresh"`
	}
	// The frontend calls this with an empty body (the refresh token lives in
	// the HttpOnly cookie that JS cannot read), so an empty body is fine.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
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
	if st := user.EffectiveStatus(); st == model.StatusDisabled || st == model.StatusExpired {
		writeError(w, http.StatusForbidden, "account "+string(st))
		return
	}
	consumed, err := s.st.ConsumeRefreshToken(r.Context(), claims.ID, claims.UserID)
	if err != nil || !consumed {
		writeError(w, http.StatusUnauthorized, "refresh token already used or revoked")
		return
	}
	access, refresh, err := s.issueTokens(w, r, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	writeData(w, map[string]any{"access": access, "refresh": refresh})
}

// handleLogout clears the auth cookies and redirects to the login page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke outstanding refresh tokens for the session owner so a stolen
	// refresh cookie cannot outlive an explicit logout.
	if c, err := r.Cookie("refresh_token"); err == nil && c.Value != "" {
		if claims, err := s.tm.ParseRefresh(c.Value); err == nil {
			s.st.RevokeRefreshTokens(r.Context(), claims.UserID)
		}
	}
	http.SetCookie(w, s.tokenCookie("access_token", "", -1))
	http.SetCookie(w, s.tokenCookie("refresh_token", "", -1))
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
