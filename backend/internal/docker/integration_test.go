package docker

import (
	"context"
	"os"
	"testing"
	"time"

	"devcapsule/backend/internal/model"
	"devcapsule/backend/internal/store"
)

const (
	testNetwork = "devcapsule-itest-net"
	testImage   = "python:3.12-alpine"
	testPort    = 8080
)

func httpServerCmd() []string {
	return []string{"python3", "-m", "http.server", "8080", "--bind", "0.0.0.0"}
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("TEST_DOCKER") == "0" {
		t.Skip("TEST_DOCKER=0, skipping docker integration test")
	}
	c, err := NewClient()
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		// clean up any test containers
		containers, _ := c.ListManaged(ctx)
		for _, ct := range containers {
			if len(ct.Names) > 0 && containsPrefix(ct.Names[0], "/user-ocitest") {
				c.RemoveContainer(ctx, ct.ID)
			}
		}
		_ = c.cli.NetworkRemove(ctx, testNetwork)
		c.Close()
	})
	return c
}

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestClientNetworkAndImage(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	if err := c.EnsureNetwork(ctx, testNetwork); err != nil {
		t.Fatalf("ensure network: %v", err)
	}
	if err := c.EnsureImage(ctx, testImage); err != nil {
		t.Fatalf("ensure image: %v", err)
	}
}

func TestCreateProbeStatsAndRemove(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	if err := c.EnsureNetwork(ctx, testNetwork); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureImage(ctx, testImage); err != nil {
		t.Fatal(err)
	}
	id, err := c.CreateContainer(ctx, ContainerConfig{
		Name:          "user-ocitest-1",
		Image:         testImage,
		Network:       testNetwork,
		Env:           map[string]string{"OPENCODE_SERVER_PASSWORD": "testsecret"},
		Cmd:           httpServerCmd(),
		InternalPort:  testPort,
		CPULimit:      0.5,
		MemLimitBytes: 1 << 30,
		PidsLimit:     128,
		WorkDir:       "/",
	})
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	defer c.RemoveContainer(ctx, id)

	status, err := c.InspectStatus(ctx, id)
	if err != nil || status != "running" {
		t.Fatalf("status=%s err=%v", status, err)
	}

	if err := c.WaitHealthy(ctx, id, testPort, 60*time.Second); err != nil {
		t.Fatalf("wait healthy: %v", err)
	}

	// exec-based probe inside the container
	if ok, _ := c.ProbeExec(ctx, id, []string{"python3", "-c",
		"import urllib.request;urllib.request.urlopen('http://127.0.0.1:8080/', timeout=3)"}); !ok {
		t.Fatal("exec probe failed")
	}

	stats, err := c.Stats(ctx, id)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.MemLimit != 1<<30 {
		t.Fatalf("mem limit=%d want %d", stats.MemLimit, int64(1)<<30)
	}
	if stats.MemBytes < 0 {
		t.Fatal("negative mem")
	}

	if err := c.Stop(ctx, id); err != nil {
		t.Fatalf("stop: %v", err)
	}
	status, _ = c.InspectStatus(ctx, id)
	if status != "exited" {
		t.Fatalf("expected exited, got %s", status)
	}
	if err := c.Start(ctx, id); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := c.Restart(ctx, id); err != nil {
		t.Fatalf("restart: %v", err)
	}
}

func TestOrchestratorProvisionLifecycle(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	st := store.NewMemory()

	orch := NewOrchestrator(c, st, testNetwork, 3)

	user := &model.User{ID: model.NewID(), Username: "ocitest1", CPULimit: 0.5, MemLimit: 1 << 30}
	tpl := &model.Template{
		ID:           model.NewID(),
		Name:         "itest",
		Image:        testImage,
		InternalPort: testPort,
		CPULimit:     0.5,
		MemLimit:     1 << 30,
		WorkspaceDir: "/tmp",
		Command:      httpServerCmd(),
	}

	rec, err := orch.Provision(ctx, user, tpl, false)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if rec.Status != model.ContainerRunning || rec.ContainerID == "" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.Secret == "" {
		t.Fatal("missing internal secret")
	}

	// idempotent re-provision returns same container
	rec2, err := orch.Provision(ctx, user, tpl, false)
	if err != nil {
		t.Fatal(err)
	}
	if rec2.ContainerID != rec.ContainerID {
		t.Fatalf("expected same container id, got %s vs %s", rec2.ContainerID, rec.ContainerID)
	}

	// force re-provision removes and recreates with a different container id
	rec3, err := orch.Provision(ctx, user, tpl, true)
	if err != nil {
		t.Fatalf("force provision: %v", err)
	}
	if rec3.ContainerID == rec.ContainerID {
		t.Fatalf("expected a new container after force, got same %s", rec3.ContainerID)
	}
	status, err := c.InspectStatus(ctx, rec3.ContainerID)
	if err != nil || status != "running" {
		t.Fatalf("force-recreated container status=%s err=%v", status, err)
	}
	// the orchestrator reuses the persisted record, so use rec3 for the rest
	rec = rec3

	if err := orch.Stop(ctx, rec); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if rec.Status != model.ContainerStopped {
		t.Fatalf("expected stopped, got %s", rec.Status)
	}
	// EnsureRunning should wake it back up
	if err := orch.EnsureRunning(ctx, rec, user, tpl); err != nil {
		t.Fatalf("ensure running: %v", err)
	}
	if rec.Status != model.ContainerRunning {
		t.Fatalf("expected running after wake, got %s", rec.Status)
	}

	if err := orch.Remove(ctx, rec); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if rec.Status != model.ContainerRemoved {
		t.Fatalf("expected removed, got %s", rec.Status)
	}
}

func TestOrchestratorIdleStopAndExpire(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	st := store.NewMemory()
	orch := NewOrchestrator(c, st, testNetwork, 3)

	user := &model.User{ID: model.NewID(), Username: "ocitest2", CPULimit: 0.5, MemLimit: 1 << 30}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	tpl := &model.Template{
		ID:           model.NewID(),
		Name:         "itest2",
		Image:        testImage,
		InternalPort: testPort,
		CPULimit:     0.5,
		MemLimit:     1 << 30,
		WorkspaceDir: "/tmp",
		Command:      httpServerCmd(),
	}
	rec, err := orch.Provision(ctx, user, tpl, false)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if err := orch.WaitHealthy(ctx, rec, tpl); err != nil {
		t.Fatalf("wait healthy: %v", err)
	}

	// no access logs -> should be stopped by IdleStop immediately
	stopped, err := orch.IdleStop(ctx, time.Minute, []*model.User{user})
	if err != nil {
		t.Fatalf("idle stop: %v", err)
	}
	if len(stopped) != 1 {
		t.Fatalf("expected 1 stopped, got %v", stopped)
	}
	rec, _ = st.GetContainerByUserID(ctx, user.ID)
	if rec.Status != model.ContainerStopped {
		t.Fatalf("expected stopped, got %s", rec.Status)
	}

	// recent access should keep it running
	if err := orch.Start(ctx, rec); err != nil {
		t.Fatalf("start: %v", err)
	}
	st.LogAccess(ctx, &model.AccessLog{UserID: user.ID, Timestamp: time.Now()})
	stopped, err = orch.IdleStop(ctx, time.Hour, []*model.User{user})
	if err != nil {
		t.Fatalf("idle stop: %v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("expected nothing stopped, got %v", stopped)
	}

	// expiry: past expires_at -> user expired and container stopped
	past := time.Now().Add(-time.Hour)
	user.ExpiresAt = &past
	st.UpdateUser(ctx, user)
	n, err := orch.ExpireAndStop(ctx)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected >=1 expired users, got %d", n)
	}
	if err := orch.Remove(ctx, rec); err != nil {
		t.Fatalf("remove: %v", err)
	}
}

func TestSyncStatusDetectsExternalStopAndWake(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	st := store.NewMemory()
	orch := NewOrchestrator(c, st, testNetwork, 3)

	user := &model.User{ID: model.NewID(), Username: "ocitest3", CPULimit: 0.5, MemLimit: 1 << 30}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	tpl := &model.Template{
		ID:           model.NewID(),
		Name:         "itest3",
		Image:        testImage,
		InternalPort: testPort,
		CPULimit:     0.5,
		MemLimit:     1 << 30,
		WorkspaceDir: "/tmp",
		Command:      httpServerCmd(),
	}
	rec, err := orch.Provision(ctx, user, tpl, false)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	defer orch.Remove(ctx, rec)

	// simulate an external stop: docker stop bypassing the API
	if err := c.Stop(ctx, rec.ContainerID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// DB record still says running (stale)
	rec.Status = model.ContainerRunning
	st.UpdateContainer(ctx, rec)

	// SyncStatus must correct the DB to stopped
	synced, err := orch.SyncStatus(ctx, rec)
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if synced.Status != model.ContainerStopped {
		t.Fatalf("expected stopped after sync, got %s", synced.Status)
	}
	stored, _ := st.GetContainerByUserID(ctx, user.ID)
	if stored.Status != model.ContainerStopped {
		t.Fatalf("expected DB status stopped, got %s", stored.Status)
	}

	// EnsureRunning wakes the container back up and WaitHealthy confirms it
	if err := orch.EnsureRunning(ctx, synced, user, tpl); err != nil {
		t.Fatalf("ensure running: %v", err)
	}
	if err := orch.WaitHealthy(ctx, synced, tpl); err != nil {
		t.Fatalf("wait healthy after wake: %v", err)
	}
	if synced.Status != model.ContainerRunning {
		t.Fatalf("expected running after wake, got %s", synced.Status)
	}
}
