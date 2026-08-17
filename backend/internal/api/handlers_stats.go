package api

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"devcapsule/backend/internal/model"
)

const (
	onlineWindow = 5 * time.Minute
)

type dashboardResources struct {
	CPUCores float64 `json:"cpu_cores"`
	MemBytes float64 `json:"mem_bytes"`
	MemLimit int64   `json:"mem_limit"`
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	userStatus := map[string]int64{}
	active := int64(0)
	userByID := map[string]*model.User{}
	userByCourse := map[string]int64{}
	for _, u := range users {
		st := string(u.EffectiveStatus())
		userStatus[st]++
		if st == string(model.StatusActive) {
			active++
		}
		userByID[u.ID] = u
		userByCourse[u.Course]++
	}
	recs, err := s.st.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	contStatus := map[string]int64{}
	running := int64(0)
	contByCourse := map[string]int64{}
	for _, c := range recs {
		contStatus[string(c.Status)]++
		if c.Status == model.ContainerRunning {
			running++
			if u := userByID[c.UserID]; u != nil {
				contByCourse[u.Course]++
			}
		}
	}

	now := time.Now()
	sum, err := s.st.AccessLogsSummary(r.Context(), now.Add(-24*time.Hour), onlineWindow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	var avgLatency float64
	if sum.Count > 0 {
		avgLatency = float64(sum.LatencySum) / float64(sum.Count)
	}

	courses := []map[string]any{}
	names := make([]string, 0, len(userByCourse))
	for name := range userByCourse {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := userByCourse[name]
		if n == 0 {
			continue
		}
		courses = append(courses, map[string]any{
			"course":  name,
			"users":   n,
			"running": contByCourse[name],
		})
	}

	tpls, _ := s.st.ListTemplates(r.Context())

	writeData(w, map[string]any{
		"users":      map[string]any{"total": len(users), "status": userStatus, "active": active},
		"containers": map[string]any{"total": len(recs), "status": contStatus, "running": running},
		"requests": map[string]any{
			"count":          sum.Count,
			"bytes":          sum.Bytes,
			"last":           sum.Last,
			"online":         sum.Online,
			"avg_latency_ms": avgLatency,
			"last24h":        sum.Last24H[:],
		},
		"resources": s.collectResources(r.Context(), recs),
		"courses":   courses,
		"templates": map[string]any{"total": len(tpls)},
		"idle_timeout": map[string]any{
			"minutes": s.cfg.IdleTimeoutMin,
		},
	})
}

// collectResources aggregates live docker stats of running containers.
// Unreachable containers (e.g. no docker daemon) are skipped.
func (s *Server) collectResources(ctx context.Context, recs []*model.Container) *dashboardResources {
	res := &dashboardResources{}
	if !s.docker.Available() {
		return res
	}
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, c := range recs {
		if c.ContainerID == "" || c.Status != model.ContainerRunning {
			continue
		}
		wg.Add(1)
		go func(rec *model.Container) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			st, err := s.orch.Stats(ctx, rec)
			if err != nil {
				return
			}
			mu.Lock()
			res.CPUCores += st.CPUCores
			res.MemBytes += st.MemBytes
			res.MemLimit += st.MemLimit
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	return res
}
