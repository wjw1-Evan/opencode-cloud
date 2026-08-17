package docker

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"devcapsule/backend/internal/model"
	"devcapsule/backend/internal/store"
)

const (
	OcodeConfigDir = "/root/.config/opencode"
	OcodeDataDir   = "/root/.local/share/opencode"
	DefaultWorkDir = "/workspace"
)

type Orchestrator struct {
	dc          *Client
	st          store.Store
	network     string
	concurrency int
}

func NewOrchestrator(dc *Client, st store.Store, network string, concurrency int) *Orchestrator {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &Orchestrator{dc: dc, st: st, network: network, concurrency: concurrency}
}

func (o *Orchestrator) ContainerName(username string) string {
	return "user-" + username
}

// Provision creates the user's container (idempotent: reuses a running one).
// When force is true the existing container is removed and recreated.
func (o *Orchestrator) Provision(ctx context.Context, user *model.User, tpl *model.Template, force bool) (*model.Container, error) {
	if !o.dc.Available() {
		return nil, fmt.Errorf("docker unavailable")
	}
	if err := o.dc.EnsureNetwork(ctx, o.network); err != nil {
		return nil, fmt.Errorf("ensure network: %w", err)
	}
	if err := o.dc.EnsureImage(ctx, tpl.Image); err != nil {
		return nil, fmt.Errorf("ensure image: %w", err)
	}

	name := o.ContainerName(user.Username)
	existing, err := o.st.GetContainerByUserID(ctx, user.ID)
	if err == nil {
		if existing.ContainerID != "" {
			status, err := o.dc.InspectStatus(ctx, existing.ContainerID)
			if err == nil && !force {
				if status == "running" {
					if existing.Status != model.ContainerRunning {
						existing.Status = model.ContainerRunning
						o.st.UpdateContainer(ctx, existing)
					}
					return existing, nil
				}
				if status == "paused" || status == "restarting" {
					return existing, fmt.Errorf("container %s in state %s", existing.ContainerID, status)
				}
			}
			// remove stale docker container so we can recreate with same name
			o.dc.RemoveContainer(ctx, existing.ContainerID)
		}
	} else if err != store.ErrNotFound {
		return nil, err
	}

	secret, err := o.dc.secret.Next()
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for k, v := range tpl.Envs {
		env[k] = v
	}
	env["OPENCODE_SERVER_USERNAME"] = "opencode"
	env["OPENCODE_SERVER_PASSWORD"] = secret
	env["OPENCODE_WORKDIR"] = tpl.WorkspaceDir

	workDir := tpl.WorkspaceDir
	if workDir == "" {
		workDir = DefaultWorkDir
	}
	volumes := []string{
		fmt.Sprintf("code-%s:%s", user.Username, workDir),
		fmt.Sprintf("ocdata-%s:%s", user.Username, OcodeDataDir),
	}

	entrypoint := []string{}
	if len(tpl.Command) == 0 {
		entrypoint = nil // keep image entrypoint when no command is set
	}
	id, err := o.dc.CreateContainer(ctx, ContainerConfig{
		Name:          name,
		Image:         tpl.Image,
		Network:       o.network,
		Env:           env,
		Cmd:           tpl.Command,
		Entrypoint:    entrypoint,
		InternalPort:  tpl.InternalPort,
		CPULimit:      tpl.CPULimit,
		MemLimitBytes: tpl.MemLimit,
		PidsLimit:     128,
		WorkDir:       workDir,
		Volumes:       volumes,
		RunUser:       tpl.RunUser,
	})
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	rec := &model.Container{
		ID:            model.NewID(),
		UserID:        user.ID,
		TemplateID:    tpl.ID,
		ContainerID:   id,
		ContainerName: name,
		Status:        model.ContainerRunning,
		InternalPort:  tpl.InternalPort,
		Secret:        secret,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if existing != nil {
		rec.ID = existing.ID
		rec.CreatedAt = existing.CreatedAt
		o.st.UpdateContainer(ctx, rec)
	} else {
		o.st.CreateContainer(ctx, rec)
	}
	user.ContainerID = id
	o.st.UpdateUser(ctx, user)
	// best-effort health check: surface slow/broken startups via status
	healthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := o.dc.WaitHealthy(healthCtx, id, tpl.InternalPort, 30*time.Second); err != nil {
		rec.Status = model.ContainerRunning
		o.st.UpdateContainer(ctx, rec)
	}
	return rec, nil
}

type BatchResult struct {
	Username string `json:"username"`
	UserID   string `json:"user_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// ProvisionBatch creates containers for many users concurrently.
// If force is false, users that already have a running container are skipped.
func (o *Orchestrator) ProvisionBatch(ctx context.Context, users []*model.User, tpl *model.Template, force bool) []BatchResult {
	results := make([]BatchResult, len(users))
	var wg sync.WaitGroup
	sem := make(chan struct{}, o.concurrency)
	for i, user := range users {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, user *model.User) {
			defer wg.Done()
			defer func() { <-sem }()
			res := BatchResult{Username: user.Username, UserID: user.ID}
			if !force && o.dc.Available() {
				if c, err := o.st.GetContainerByUserID(ctx, user.ID); err == nil && c.ContainerID != "" {
					if status, err := o.dc.InspectStatus(ctx, c.ContainerID); err == nil && status == "running" {
						res.OK = true
						results[i] = res
						return
					}
				}
			}
			_, err := o.Provision(ctx, user, tpl, force)
			res.OK = err == nil
			if err != nil {
				res.Error = err.Error()
			}
			results[i] = res
		}(i, user)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Username < results[j].Username })
	return results
}

// EnsureRunning wakes the container if it is stopped or missing.
func (o *Orchestrator) EnsureRunning(ctx context.Context, rec *model.Container, user *model.User, tpl *model.Template) error {
	if rec.ContainerID == "" {
		_, err := o.Provision(ctx, user, tpl, false)
		return err
	}
	status, err := o.dc.InspectStatus(ctx, rec.ContainerID)
	if err != nil {
		return err
	}
	if status == "running" {
		return nil
	}
	if status == "" {
		return fmt.Errorf("container %s missing", rec.ContainerID)
	}
	if err := o.dc.Start(ctx, rec.ContainerID); err != nil {
		return err
	}
	rec.Status = model.ContainerRunning
	rec.UpdatedAt = time.Now().UTC()
	return o.st.UpdateContainer(ctx, rec)
}

func (o *Orchestrator) Start(ctx context.Context, rec *model.Container) error {
	if err := o.dc.Start(ctx, rec.ContainerID); err != nil {
		return err
	}
	rec.Status = model.ContainerRunning
	rec.UpdatedAt = time.Now().UTC()
	return o.st.UpdateContainer(ctx, rec)
}

func (o *Orchestrator) Stop(ctx context.Context, rec *model.Container) error {
	if err := o.dc.Stop(ctx, rec.ContainerID); err != nil {
		return err
	}
	rec.Status = model.ContainerStopped
	rec.UpdatedAt = time.Now().UTC()
	return o.st.UpdateContainer(ctx, rec)
}

func (o *Orchestrator) Restart(ctx context.Context, rec *model.Container) error {
	if err := o.dc.Restart(ctx, rec.ContainerID); err != nil {
		return err
	}
	rec.Status = model.ContainerRunning
	rec.UpdatedAt = time.Now().UTC()
	return o.st.UpdateContainer(ctx, rec)
}

func (o *Orchestrator) Remove(ctx context.Context, rec *model.Container) error {
	if rec.ContainerID != "" {
		if err := o.dc.RemoveContainer(ctx, rec.ContainerID); err != nil {
			return err
		}
	}
	rec.Status = model.ContainerRemoved
	rec.ContainerID = ""
	rec.UpdatedAt = time.Now().UTC()
	if err := o.st.UpdateContainer(ctx, rec); err != nil {
		return err
	}
	return nil
}

// IdleStop stops running containers whose last access is older than idle.
func (o *Orchestrator) IdleStop(ctx context.Context, idle time.Duration, users []*model.User) ([]string, error) {
	stopped := []string{}
	for _, user := range users {
		rec, err := o.st.GetContainerByUserID(ctx, user.ID)
		if err != nil || rec.ContainerID == "" || rec.Status != model.ContainerRunning {
			continue
		}
		last, err := o.st.LastAccess(ctx, user.ID)
		if err != nil {
			continue
		}
		if last != nil && time.Since(*last) < idle {
			continue
		}
		if err := o.Stop(ctx, rec); err != nil {
			continue
		}
		stopped = append(stopped, rec.ContainerName)
	}
	return stopped, nil
}

// ExpireAndStop marks expired users and stops their containers.
func (o *Orchestrator) ExpireAndStop(ctx context.Context) (int64, error) {
	n, err := o.st.ExpireUsers(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	users, err := o.st.ListUsers(ctx)
	if err != nil {
		return n, err
	}
	for _, u := range users {
		if u.Status != model.StatusExpired {
			continue
		}
		rec, err := o.st.GetContainerByUserID(ctx, u.ID)
		if err != nil || rec.ContainerID == "" || rec.Status == model.ContainerStopped {
			continue
		}
		o.Stop(ctx, rec)
	}
	return n, nil
}

// Reconcile syncs DB container records with the real docker state.
func (o *Orchestrator) Reconcile(ctx context.Context) error {
	if !o.dc.Available() {
		return nil
	}
	recs, err := o.st.ListContainers(ctx)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		if rec.ContainerID == "" {
			if rec.Status != model.ContainerRemoved {
				rec.Status = model.ContainerError
				o.st.UpdateContainer(ctx, rec)
			}
			continue
		}
		status, err := o.dc.InspectStatus(ctx, rec.ContainerID)
		if err != nil {
			continue
		}
		var want model.ContainerStatus
		switch status {
		case "running":
			want = model.ContainerRunning
		case "exited", "created", "dead":
			want = model.ContainerStopped
		case "":
			want = model.ContainerRemoved
			rec.ContainerID = ""
		default:
			want = rec.Status
		}
		if want != rec.Status {
			rec.Status = want
			rec.UpdatedAt = time.Now().UTC()
			o.st.UpdateContainer(ctx, rec)
		}
	}
	return nil
}

// SyncStatus checks the live docker state of one container record and updates
// the stored status if they differ. Returns the (possibly updated) record.
func (o *Orchestrator) SyncStatus(ctx context.Context, rec *model.Container) (*model.Container, error) {
	if !o.dc.Available() {
		return rec, nil
	}
	if rec.ContainerID == "" {
		if rec.Status != model.ContainerRemoved {
			rec.Status = model.ContainerError
			rec.UpdatedAt = time.Now().UTC()
			o.st.UpdateContainer(ctx, rec)
		}
		return rec, nil
	}
	status, err := o.dc.InspectStatus(ctx, rec.ContainerID)
	if err != nil {
		return rec, err
	}
	switch status {
	case "running":
		rec.Status = model.ContainerRunning
	case "exited", "created", "dead":
		rec.Status = model.ContainerStopped
	case "":
		rec.Status = model.ContainerRemoved
		rec.ContainerID = ""
	default:
		// paused / restarting: leave as-is
		return rec, nil
	}
	rec.UpdatedAt = time.Now().UTC()
	o.st.UpdateContainer(ctx, rec)
	return rec, nil
}

// Stats returns the live CPU/mem usage for a container.
func (o *Orchestrator) Stats(ctx context.Context, rec *model.Container) (*ContainerStats, error) {
	if rec.ContainerID == "" {
		return nil, fmt.Errorf("no container")
	}
	return o.dc.Stats(ctx, rec.ContainerID)
}

// WaitHealthy polls the container until its HTTP endpoint responds or times out.
func (o *Orchestrator) WaitHealthy(ctx context.Context, rec *model.Container, tpl *model.Template) error {
	if rec.ContainerID == "" {
		return fmt.Errorf("no container")
	}
	return o.dc.WaitHealthy(ctx, rec.ContainerID, tpl.InternalPort, 15*time.Second)
}
