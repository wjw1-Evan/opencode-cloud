package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/image"
)

func dockerOK(s *Server) bool {
	return s.docker != nil && s.docker.Available()
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	if !dockerOK(s) {
		writeError(w, http.StatusServiceUnavailable, "docker not available")
		return
	}
	images, err := s.docker.ListImages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list images: "+err.Error())
		return
	}
	type imgView struct {
		ID       string   `json:"id"`
		ParentID string   `json:"parent_id"`
		RepoTags []string `json:"repo_tags"`
		Size     int64    `json:"size"`
		Created  int64    `json:"created"`
	}
	out := make([]imgView, 0, len(images))
	for _, im := range images {
		out = append(out, imgView{
			ID:       im.ID,
			ParentID: im.ParentID,
			RepoTags: im.RepoTags,
			Size:     im.Size,
			Created:  im.Created,
		})
	}
	writeData(w, out)
}

func (s *Server) handleImportImage(w http.ResponseWriter, r *http.Request) {
	if !dockerOK(s) {
		writeError(w, http.StatusServiceUnavailable, "docker not available")
		return
	}
	if err := r.ParseMultipartForm(2 << 30); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()
	if header.Size <= 0 {
		writeError(w, http.StatusBadRequest, "empty file")
		return
	}
	reader := io.LimitReader(file, 2<<30)
	start := time.Now()
	body, err := s.docker.LoadImage(r.Context(), reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, "image load failed: "+err.Error())
		return
	}
	defer body.Close()
	io.Copy(io.Discard, body)
	writeData(w, map[string]any{
		"message": "image imported successfully",
		"elapsed": time.Since(start).String(),
	})
}

func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	if !dockerOK(s) {
		writeError(w, http.StatusServiceUnavailable, "docker not available")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "image id required")
		return
	}
	info, err := s.docker.InspectImage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	type inspectView struct {
		ID           string                 `json:"id"`
		RepoTags     []string               `json:"repo_tags"`
		RepoDigests  []string               `json:"repo_digests"`
		Architecture string                 `json:"architecture"`
		OS           string                 `json:"os"`
		Size         int64                  `json:"size"`
		VirtualSize  int64                  `json:"virtual_size"`
		Created      string                 `json:"created"`
		Author       string                 `json:"author"`
		Env          []string               `json:"env"`
		Cmd          []string               `json:"cmd"`
		Entrypoint   []string               `json:"entrypoint"`
		WorkingDir   string                 `json:"working_dir"`
		Labels       map[string]string      `json:"labels"`
		Layers       []string               `json:"layers"`
		RootFS       image.RootFS           `json:"root_fs"`
	}
	v := inspectView{
		ID:           info.ID,
		RepoTags:     info.RepoTags,
		RepoDigests:  info.RepoDigests,
		Architecture: info.Architecture,
		OS:           info.Os,
		Size:         info.Size,
		VirtualSize:  info.Size,
		Created:      info.Created,
		Author:       info.Author,
		Env:          info.Config.Env,
		Cmd:          info.Config.Cmd,
		Entrypoint:   info.Config.Entrypoint,
		WorkingDir:   info.Config.WorkingDir,
		Labels:       info.Config.Labels,
		Layers:       info.RootFS.Layers,
		RootFS:       info.RootFS,
	}
	writeData(w, v)
}

func (s *Server) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	if !dockerOK(s) {
		writeError(w, http.StatusServiceUnavailable, "docker not available")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "image id required")
		return
	}
	_, err := s.docker.RemoveImage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "delete failed: "+err.Error())
		return
	}
	writeData(w, map[string]any{"deleted": true})
}

type pullRequest struct {
	Image string `json:"image"`
}

func (s *Server) handlePullImage(w http.ResponseWriter, r *http.Request) {
	if !dockerOK(s) {
		writeError(w, http.StatusServiceUnavailable, "docker not available")
		return
	}
	var req pullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.Image == "" {
		writeError(w, http.StatusBadRequest, "image name required")
		return
	}
	start := time.Now()
	if err := s.docker.PullImage(r.Context(), req.Image); err != nil {
		writeError(w, http.StatusBadRequest, "pull failed: "+err.Error())
		return
	}
	writeData(w, map[string]any{
		"message": "image pulled successfully",
		"elapsed": time.Since(start).String(),
	})
}
