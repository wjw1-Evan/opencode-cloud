package api

import "net/http"

// handlePlatform returns platform-level configuration for the admin UI.
func (s *Server) handlePlatform(w http.ResponseWriter, r *http.Request) {
	writeData(w, map[string]any{
		"network": s.cfg.NetworkName,
	})
}
