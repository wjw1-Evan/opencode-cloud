package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
)

// maxImageSize is the largest docker save tarball accepted for import (2GB).
const maxImageSize = 2 << 30

// multipartOverhead covers boundary delimiters and headers so a file of
// exactly maxImageSize still fits inside the request-body cap.
const multipartOverhead = 1 << 20

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
	usage := s.usageForImages(r.Context(), images)
	type imgView struct {
		ID       string   `json:"id"`
		ParentID string   `json:"parent_id"`
		RepoTags []string `json:"repo_tags"`
		Size     int64    `json:"size"`
		Created  int64    `json:"created"`
		InUse    bool     `json:"in_use"`
		UsedBy   []string `json:"used_by"`
	}
	out := make([]imgView, 0, len(images))
	for _, im := range images {
		out = append(out, imgView{
			ID:       im.ID,
			ParentID: im.ParentID,
			RepoTags: im.RepoTags,
			Size:     im.Size,
			Created:  im.Created,
			InUse:    usage[im.ID].inUse,
			UsedBy:   usage[im.ID].usedBy,
		})
	}
	writeData(w, out)
}

// imageUsage reports whether an image is referenced by containers or
// templates, and by whom.
type imageUsage struct {
	inUse  bool
	usedBy []string
}

// usageForImages computes, for each image, whether it is still referenced by
// any container on the host or by any configured template. Failures to list
// containers/templates degrade gracefully to "not in use" so the list still
// renders; the delete path re-checks and Docker itself refuses images that
// are genuinely in use.
func (s *Server) usageForImages(ctx context.Context, images []image.Summary) map[string]imageUsage {
	usage := make(map[string]imageUsage, len(images))
	if len(images) == 0 {
		return usage
	}
	containers, cerr := s.docker.ListAllContainers(ctx)
	tpls, terr := s.st.ListTemplates(ctx)

	containerRefs := make(map[string][]string, len(containers))
	if cerr == nil {
		for _, c := range containers {
			name := strings.TrimPrefix(strings.Join(c.Names, ", "), "/")
			containerRefs[c.ImageID] = append(containerRefs[c.ImageID], name)
		}
	}
	var templateRefs []string
	if terr == nil {
		for _, tpl := range tpls {
			templateRefs = append(templateRefs, tpl.Image)
		}
	}

	for _, im := range images {
		var u imageUsage
		for _, name := range containerRefs[im.ID] {
			u.inUse = true
			u.usedBy = append(u.usedBy, name)
		}
		for _, ref := range templateRefs {
			if imageRefMatches(im.RepoTags, ref) {
				u.inUse = true
				u.usedBy = append(u.usedBy, "template: "+ref)
			}
		}
		usage[im.ID] = u
	}
	return usage
}

// imageRefMatches reports whether a template's image reference (e.g.
// "nginx:latest", "ghcr.io/x/y") equals one of an image's repo tags. A bare
// name like "nginx" is treated as "nginx:latest".
func imageRefMatches(repoTags []string, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	if !strings.Contains(ref, ":") {
		ref += ":latest"
	}
	for _, tag := range repoTags {
		if tag == ref {
			return true
		}
	}
	return false
}

func (s *Server) handleImportImage(w http.ResponseWriter, r *http.Request) {
	if !dockerOK(s) {
		writeError(w, http.StatusServiceUnavailable, "docker not available")
		return
	}
	// Buffer only small parts in memory (large files spill to temp files) and
	// cap the whole request body slightly above the file limit to account for
	// multipart framing, so oversized uploads fail fast instead of exhausting
	// memory or being silently truncated.
	r.Body = http.MaxBytesReader(w, r.Body, maxImageSize+multipartOverhead)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "image file exceeds 2GB limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()
	// header.Size is not always set by the client; since ParseMultipartForm has
	// fully buffered the part, seek to the end to get the true size.
	size := header.Size
	if size <= 0 {
		if end, err := file.Seek(0, io.SeekEnd); err == nil {
			size = end
			file.Seek(0, io.SeekStart)
		}
	}
	if size <= 0 {
		writeError(w, http.StatusBadRequest, "empty file")
		return
	}
	if size > maxImageSize {
		writeError(w, http.StatusRequestEntityTooLarge, "image file exceeds 2GB limit")
		return
	}
	start := time.Now()
	body, err := s.docker.LoadImage(r.Context(), file)
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
		ID           string            `json:"id"`
		RepoTags     []string          `json:"repo_tags"`
		RepoDigests  []string          `json:"repo_digests"`
		Architecture string            `json:"architecture"`
		Variant      string            `json:"variant"`
		OS           string            `json:"os"`
		Size         int64             `json:"size"`
		VirtualSize  int64             `json:"virtual_size"`
		Created      string            `json:"created"`
		Author       string            `json:"author"`
		User         string            `json:"user"`
		ExposedPorts []string          `json:"exposed_ports"`
		Volumes      []string          `json:"volumes"`
		StopSignal   string            `json:"stop_signal"`
		Healthcheck  string            `json:"healthcheck"`
		Env          []string          `json:"env"`
		Cmd          []string          `json:"cmd"`
		Entrypoint   []string          `json:"entrypoint"`
		WorkingDir   string            `json:"working_dir"`
		Labels       map[string]string `json:"labels"`
		Layers       []string          `json:"layers"`
		RootFS       image.RootFS      `json:"root_fs"`
		InUse        bool              `json:"in_use"`
		UsedBy       []string          `json:"used_by"`
	}
	var exposedPorts, volumes []string
	if info.Config != nil {
		for p := range info.Config.ExposedPorts {
			exposedPorts = append(exposedPorts, string(p))
		}
		for v := range info.Config.Volumes {
			volumes = append(volumes, v)
		}
	}
	sort.Strings(exposedPorts)
	sort.Strings(volumes)
	v := inspectView{
		ID:           info.ID,
		RepoTags:     info.RepoTags,
		RepoDigests:  info.RepoDigests,
		Architecture: info.Architecture,
		Variant:      info.Variant,
		OS:           info.Os,
		Size:         info.Size,
		VirtualSize:  info.Size,
		Created:      info.Created,
		Author:       info.Author,
		ExposedPorts: exposedPorts,
		Volumes:      volumes,
		Layers:       info.RootFS.Layers,
		RootFS:       info.RootFS,
	}
	if info.Config != nil {
		v.User = info.Config.User
		v.StopSignal = info.Config.StopSignal
		v.Healthcheck = formatHealthcheck(info.Config.Healthcheck)
		v.Env = info.Config.Env
		v.Cmd = info.Config.Cmd
		v.Entrypoint = info.Config.Entrypoint
		v.WorkingDir = info.Config.WorkingDir
		v.Labels = info.Config.Labels
	}
	usage := s.usageForImages(r.Context(), []image.Summary{{ID: info.ID, RepoTags: info.RepoTags}})
	v.InUse = usage[info.ID].inUse
	v.UsedBy = usage[info.ID].usedBy
	writeData(w, v)
}

// formatHealthcheck renders a Docker healthcheck config as a single line,
// e.g. `CMD-SHELL curl -f http://localhost/ (interval=5s timeout=3s retries=3)`.
func formatHealthcheck(hc *dockerspec.HealthcheckConfig) string {
	if hc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.Join(hc.Test, " "))
	var opts []string
	if hc.Interval > 0 {
		opts = append(opts, "interval="+hc.Interval.String())
	}
	if hc.Timeout > 0 {
		opts = append(opts, "timeout="+hc.Timeout.String())
	}
	if hc.StartPeriod > 0 {
		opts = append(opts, "start_period="+hc.StartPeriod.String())
	}
	if hc.Retries > 0 {
		opts = append(opts, fmt.Sprintf("retries=%d", hc.Retries))
	}
	if len(opts) > 0 {
		b.WriteString(" (" + strings.Join(opts, " ") + ")")
	}
	return b.String()
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
	info, err := s.docker.InspectImage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	usage := s.usageForImages(r.Context(), []image.Summary{{
		ID:       info.ID,
		RepoTags: info.RepoTags,
	}})
	if usage[info.ID].inUse {
		writeError(w, http.StatusConflict, "image is in use by "+strings.Join(usage[info.ID].usedBy, ", "))
		return
	}
	_, err = s.docker.RemoveImage(r.Context(), id)
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
