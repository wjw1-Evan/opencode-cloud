package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/model"
)

type batchProvisionRequest struct {
	TemplateID string   `json:"template_id"`
	UserIDs    []string `json:"user_ids"` // empty = all active users
	Force      bool     `json:"force"`
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	recs, err := s.st.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	// reconcile with live docker state so the UI always reflects reality
	startedAt := map[string]string{}
	for _, c := range recs {
		if c.ContainerID == "" {
			continue
		}
		if t, err := s.orch.SyncRuntime(r.Context(), c); err == nil && t != "" {
			startedAt[c.ContainerID] = t
		}
	}
	// real network per container, shown in the admin UI (single batched call)
	nets, _ := s.orch.ContainerNetworks(r.Context())
	userIDs := make([]string, 0, len(recs))
	for _, c := range recs {
		userIDs = append(userIDs, c.UserID)
	}
	users, _ := s.st.ListUsersByIDs(r.Context(), userIDs)
	byID := map[string]*UserView{}
	for _, u := range users {
		byID[u.ID] = &UserView{Username: u.Username, Status: string(u.EffectiveStatus())}
	}
	tpls, _ := s.st.ListTemplates(r.Context())
	tplByID := map[string]*model.Template{}
	for _, t := range tpls {
		tplByID[t.ID] = t
	}
	type view struct {
		*ContainerView
		Username string `json:"username"`
		User     string `json:"user_status"`
	}
	out := make([]view, 0, len(recs))
	for _, c := range recs {
		cv := toContainerView(c)
		cv.ExpectedNet = s.cfg.NetworkName
		if t, ok := tplByID[c.TemplateID]; ok {
			cv.ExtraPorts = t.ExtraPorts
		}
		if names := nets[c.ContainerID]; len(names) > 0 {
			sort.Strings(names)
			cv.Network = strings.Join(names, ", ")
		}
		cv.StartedAt = startedAt[c.ContainerID]
		uv := byID[c.UserID]
		out = append(out, view{
			ContainerView: cv,
			Username:      cv.Username,
			User:          cv.UserStatus,
		})
		if uv != nil {
			out[len(out)-1].Username = uv.Username
			out[len(out)-1].User = uv.Status
		}
	}
	writeData(w, out)
}

func (s *Server) handleProvisionBatch(w http.ResponseWriter, r *http.Request) {
	var req batchProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	var defaultTpl *model.Template
	if req.TemplateID != "" {
		t, err := s.st.GetTemplate(r.Context(), req.TemplateID)
		if err != nil {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		defaultTpl = t
	}
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	want := map[string]bool{}
	for _, id := range req.UserIDs {
		want[id] = true
	}
	selected := []*model.User{}
	for _, u := range users {
		if u.Role != model.RoleUser {
			continue
		}
		if len(req.UserIDs) > 0 {
			if want[u.ID] {
				selected = append(selected, u)
			}
		} else {
			if st := u.EffectiveStatus(); st == model.StatusActive || st == model.StatusExpired {
				selected = append(selected, u)
			}
		}
	}
	if len(selected) == 0 {
		writeData(w, map[string]any{"provisioned": 0, "results": []any{}})
		return
	}

	// Batch-fetch the containers of the selected users to know each one's
	// existing template, avoiding N per-user queries.
	userIDs := make([]string, 0, len(selected))
	for _, u := range selected {
		userIDs = append(userIDs, u.ID)
	}
	recs, _ := s.st.ListContainersByUserIDs(r.Context(), userIDs)
	tplByUser := map[string]string{}
	for _, c := range recs {
		tplByUser[c.UserID] = c.TemplateID
	}

	// Each user keeps the template they were created with: prefer the template
	// bound to their existing container record, fall back to the requested one.
	tplCache := map[string]*model.Template{}
	tplFor := func(templateID string) *model.Template {
		if t, ok := tplCache[templateID]; ok {
			return t
		}
		t, err := s.st.GetTemplate(r.Context(), templateID)
		if err != nil {
			return nil
		}
		tplCache[templateID] = t
		return t
	}

	grouped := map[string][]*model.User{}
	results := []docker.BatchResult{}
	skipped := []map[string]any{}
	for _, u := range selected {
		tid := tplByUser[u.ID]
		if tid == "" && defaultTpl != nil {
			tid = defaultTpl.ID
		}
		if tid == "" {
			skipped = append(skipped, map[string]any{"username": u.Username, "ok": false, "error": "no template (user has no container and no template given)"})
			continue
		}
		tpl := tplFor(tid)
		if tpl == nil {
			skipped = append(skipped, map[string]any{"username": u.Username, "ok": false, "error": "template not found"})
			continue
		}
		grouped[tid] = append(grouped[tid], u)
	}
	for tid, group := range grouped {
		tpl := tplFor(tid)
		if tpl == nil {
			continue
		}
		results = append(results, s.orch.ProvisionBatch(r.Context(), group, tpl, req.Force)...)
	}
	for _, s := range skipped {
		results = append(results, docker.BatchResult{
			Username: s["username"].(string),
			OK:       false,
			Error:    s["error"].(string),
		})
	}
	writeData(w, map[string]any{"provisioned": countOK(results), "results": results})
}

func (s *Server) handleContainerAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	action := r.PathValue("action")
	rec, err := s.st.GetContainerByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "container record not found")
		return
	}
	switch action {
	case "start":
		err = s.orch.Start(r.Context(), rec)
	case "stop":
		err = s.orch.Stop(r.Context(), rec)
	case "restart":
		err = s.orch.Restart(r.Context(), rec)
	case "remove":
		err = s.orch.Remove(r.Context(), rec)
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, rec)
}

func (s *Server) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := s.st.GetContainerByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "container record not found")
		return
	}
	stats, err := s.orch.Stats(r.Context(), rec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, stats)
}

func (s *Server) handleAllContainerStats(w http.ResponseWriter, r *http.Request) {
	recs, err := s.st.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	type out struct {
		ContainerID string     `json:"id"`
		Name        string     `json:"container_name"`
		Status      string     `json:"status"`
		Stats       *StatsView `json:"stats,omitempty"`
	}
	results := []out{}
	for _, rec := range recs {
		o := out{ContainerID: rec.ID, Name: rec.ContainerName, Status: string(rec.Status)}
		if rec.ContainerID != "" && rec.Status == StatusRunning {
			if st, err := s.orch.Stats(r.Context(), rec); err == nil {
				o.Stats = &StatsView{CPUCores: st.CPUCores, MemBytes: st.MemBytes, MemLimit: st.MemLimit}
			}
		}
		results = append(results, o)
	}
	writeData(w, results)
}
