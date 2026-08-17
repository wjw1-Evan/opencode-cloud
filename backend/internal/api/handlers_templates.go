package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devcapsule/backend/internal/model"
)

// normalizePorts validates and de-duplicates the port list, keeping 1..65535.
func normalizePorts(ports []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// ensurePorts applies defaults and validation to a template's ports.
func ensurePorts(tpl *model.Template) {
	if tpl.InternalPort == 0 {
		tpl.InternalPort = 4096
	}
	tpl.ExtraPorts = normalizePorts(tpl.ExtraPorts)
}

// systemTemplates are seeded on first startup so admins can provision
// containers out of the box. They cannot be deleted from the UI.
var systemTemplates = []model.Template{
	{
		Name:         "opencode",
		Image:        "ghcr.io/anomalyco/opencode:latest",
		InternalPort: 4096,
		Envs:         map[string]string{},
		CPULimit:     0.5,
		MemLimit:     1 << 30,
		WorkspaceDir: "/workspace",
		Command:      []string{"opencode", "serve", "--mdns"},
		IsSystem:     true,
	},
	{
		Name:         "vscode",
		Image:        "codercom/code-server:latest",
		InternalPort: 8080,
		Envs:         map[string]string{},
		CPULimit:     0.5,
		MemLimit:     1 << 30,
		WorkspaceDir: "/home/coder",
		Command:      []string{"code-server", "--bind-addr", "0.0.0.0:8080", "--auth", "none"},
		IsSystem:     true,
	},
	{
		Name:         "jupyter",
		Image:        "jupyter/base-notebook:latest",
		InternalPort: 8888,
		Envs: map[string]string{
			"NOTEBOOK_ARGS": "--ServerApp.token= --ServerApp.password=",
		},
		CPULimit:     0.5,
		MemLimit:     1 << 30,
		WorkspaceDir: "/home/jovyan",
		IsSystem:     true,
	},
	{
		Name:         "dify",
		Image:        "jsonbored/dify-aio:latest",
		InternalPort: 8080,
		Envs:         map[string]string{},
		CPULimit:     1.0,
		MemLimit:     2 << 30,
		WorkspaceDir: "/appdata",
		RunUser:      "1000",
		IsSystem:     true,
	},
}

// EnsureSystemTemplates seeds the built-in templates if missing. Images are
// NOT pulled here: pulling can block startup for a long time on slow
// networks. Images get pulled lazily when a container is provisioned.
func (s *Server) EnsureSystemTemplates(ctx context.Context) error {
	for _, tpl := range systemTemplates {
		existing, err := s.st.GetTemplateByName(ctx, tpl.Name)
		if err == nil {
			// backfill is_system for templates seeded before the flag existed
			if !existing.IsSystem {
				existing.IsSystem = true
				s.st.UpdateTemplate(ctx, existing)
			}
			continue
		}
		tpl.ID = model.NewID()
		tpl.CreatedAt = time.Now().UTC()
		if err := s.st.CreateTemplate(ctx, &tpl); err != nil {
			return err
		}
		s.logger.Printf("seeded system template %q (%s)", tpl.Name, tpl.Image)
	}
	return nil
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	tpls, err := s.st.ListTemplates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, tpls)
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	tpl, err := s.st.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	writeData(w, tpl)
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var tpl model.Template
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if tpl.Name == "" || tpl.Image == "" {
		writeError(w, http.StatusBadRequest, "name and image required")
		return
	}
	ensurePorts(&tpl)
	if tpl.CPULimit <= 0 {
		tpl.CPULimit = 0.5
	}
	if tpl.MemLimit <= 0 {
		tpl.MemLimit = 1 << 30
	}
	if tpl.WorkspaceDir == "" {
		tpl.WorkspaceDir = "/workspace"
	}
	if tpl.Envs == nil {
		tpl.Envs = map[string]string{}
	}
	if _, err := s.st.GetTemplateByName(r.Context(), tpl.Name); err == nil {
		writeError(w, http.StatusConflict, "template name exists")
		return
	}
	if s.docker != nil && s.docker.Available() {
		if err := s.docker.EnsureImage(r.Context(), tpl.Image); err != nil {
			writeError(w, http.StatusBadRequest, "image pull failed: "+err.Error())
			return
		}
	}
	tpl.ID = model.NewID()
	tpl.CreatedAt = time.Now().UTC()
	tpl.IsSystem = false
	if err := s.st.CreateTemplate(r.Context(), &tpl); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, tpl)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	old, err := s.st.GetTemplate(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	var tpl model.Template
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if tpl.Image != "" {
		old.Image = tpl.Image
	}
	if tpl.InternalPort > 0 {
		old.InternalPort = tpl.InternalPort
	}
	if tpl.ExtraPorts != nil {
		old.ExtraPorts = normalizePorts(tpl.ExtraPorts)
	}
	if tpl.CPULimit > 0 {
		old.CPULimit = tpl.CPULimit
	}
	if tpl.MemLimit > 0 {
		old.MemLimit = tpl.MemLimit
	}
	if tpl.Healthcheck != "" {
		old.Healthcheck = tpl.Healthcheck
	}
	if tpl.WorkspaceDir != "" {
		old.WorkspaceDir = tpl.WorkspaceDir
	}
	if tpl.Envs != nil {
		old.Envs = tpl.Envs
	}
	if tpl.Command != nil {
		old.Command = tpl.Command
	}
	if tpl.RunUser != "" {
		old.RunUser = tpl.RunUser
	}
	if err := s.st.UpdateTemplate(r.Context(), old); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, old)
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	tpl, err := s.st.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if tpl.IsSystem {
		writeError(w, http.StatusForbidden, "system template cannot be deleted")
		return
	}
	if err := s.st.DeleteTemplate(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	writeData(w, map[string]any{"deleted": true})
}
