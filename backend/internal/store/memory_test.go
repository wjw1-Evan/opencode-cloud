package store

import (
	"context"
	"testing"
	"time"

	"devcapsule/backend/internal/model"
)

func TestNewMemory(t *testing.T) {
	m := NewMemory()
	if m == nil {
		t.Fatal("NewMemory returned nil")
	}
}

func TestMemoryCloseAndMigrate(t *testing.T) {
	m := NewMemory()
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryUserCRUD(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	u := &model.User{
		ID: "u1", Username: "alice", PasswordHash: "h", PasswordPlain: "p",
		Role: model.RoleUser,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := m.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	// duplicate
	if err := m.CreateUser(ctx, u); err != ErrDuplicate {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}

	got, err := m.GetUserByID(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "alice" {
		t.Fatalf("username = %q", got.Username)
	}

	got2, err := m.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != "u1" {
		t.Fatalf("ID = %q", got2.ID)
	}

	// not found
	if _, err := m.GetUserByID(ctx, "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := m.GetUserByUsername(ctx, "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	users, err := m.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	count, err := m.CountUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	u.Username = "bob"
	if err := m.UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got3, _ := m.GetUserByUsername(ctx, "bob")
	if got3 == nil {
		t.Fatal("username not updated")
	}

	// update not found
	u2 := &model.User{ID: "nope", Username: "x"}
	if err := m.UpdateUser(ctx, u2); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := m.DeleteUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetUserByID(ctx, "u1"); err != ErrNotFound {
		t.Fatal("expected deleted user to be gone")
	}
	if err := m.DeleteUser(ctx, "u1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on re-delete, got %v", err)
	}
}

func TestMemoryEnsureUser(t *testing.T) {
	m := NewMemory()
	u := &model.User{ID: "u1", Username: "alice"}
	m.EnsureUser(u)
	got, err := m.GetUserByID(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "alice" {
		t.Fatal("EnsureUser did not store user")
	}
}

func TestMemoryContainerCRUD(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	c := &model.Container{
		ID: "c1", UserID: "u1", TemplateID: "t1",
		ContainerName: "test", Status: model.ContainerRunning, InternalPort: 4096,
	}

	if err := m.CreateContainer(ctx, c); err != nil {
		t.Fatal(err)
	}

	got, err := m.GetContainerByID(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainerName != "test" {
		t.Fatalf("name = %q", got.ContainerName)
	}

	got2, err := m.GetContainerByUserID(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != "c1" {
		t.Fatalf("ID = %q", got2.ID)
	}

	if _, err := m.GetContainerByID(ctx, "nope"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
	if _, err := m.GetContainerByUserID(ctx, "nope"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}

	list, err := m.ListContainers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	c.ContainerName = "updated"
	if err := m.UpdateContainer(ctx, c); err != nil {
		t.Fatal(err)
	}
	got3, _ := m.GetContainerByID(ctx, "c1")
	if got3.ContainerName != "updated" {
		t.Fatal("container not updated")
	}

	// update not found
	if err := m.UpdateContainer(ctx, &model.Container{ID: "nope"}); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}

	if err := m.DeleteContainerByUserID(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetContainerByID(ctx, "c1"); err != ErrNotFound {
		t.Fatal("expected deleted")
	}
	if err := m.DeleteContainerByUserID(ctx, "u1"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
}

func TestMemoryTemplateCRUD(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	tpl := &model.Template{
		ID: "t1", Name: "student", Image: "ghcr.io/test", InternalPort: 4096,
	}

	if err := m.CreateTemplate(ctx, tpl); err != nil {
		t.Fatal(err)
	}
	// duplicate
	if err := m.CreateTemplate(ctx, tpl); err != ErrDuplicate {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}

	got, err := m.GetTemplate(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "student" {
		t.Fatalf("name = %q", got.Name)
	}

	got2, err := m.GetTemplateByName(ctx, "student")
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != "t1" {
		t.Fatalf("ID = %q", got2.ID)
	}

	if _, err := m.GetTemplate(ctx, "nope"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
	if _, err := m.GetTemplateByName(ctx, "nope"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}

	list, err := m.ListTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	tpl.Name = "updated"
	if err := m.UpdateTemplate(ctx, tpl); err != nil {
		t.Fatal(err)
	}
	got3, _ := m.GetTemplate(ctx, "t1")
	if got3.Name != "updated" {
		t.Fatal("template not updated")
	}

	// update not found
	if err := m.UpdateTemplate(ctx, &model.Template{ID: "nope"}); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}

	if err := m.DeleteTemplate(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetTemplate(ctx, "t1"); err != ErrNotFound {
		t.Fatal("expected deleted")
	}
	if err := m.DeleteTemplate(ctx, "t1"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
}

func TestMemoryAccessLogs(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	now := time.Now()
	l1 := &model.AccessLog{UserID: "u1", Path: "/", Status: 200, Timestamp: now}
	l2 := &model.AccessLog{UserID: "u1", Path: "/api", Status: 200, Timestamp: now.Add(time.Minute)}
	l3 := &model.AccessLog{UserID: "u2", Path: "/", Status: 200, Timestamp: now.Add(2 * time.Minute)}

	m.LogAccess(ctx, l1)
	m.LogAccess(ctx, l2)
	m.LogAccess(ctx, l3)

	logs, err := m.ListAccessLogs(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	last, err := m.LastAccess(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if last == nil || !last.Equal(l2.Timestamp) {
		t.Fatalf("expected last access at %v, got %v", l2.Timestamp, last)
	}

	last2, _ := m.LastAccess(ctx, "u2")
	if last2 == nil || !last2.Equal(l3.Timestamp) {
		t.Fatal("expected last access for u2")
	}

	if _, err := m.LastAccess(ctx, "u3"); err != nil {
		t.Fatal(err) // nil pointer is fine, no logs
	}
}

func TestMemoryStatsContainersByStatus(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	m.CreateContainer(ctx, &model.Container{ID: "c1", UserID: "u1", Status: model.ContainerRunning})
	m.CreateContainer(ctx, &model.Container{ID: "c2", UserID: "u2", Status: model.ContainerRunning})
	m.CreateContainer(ctx, &model.Container{ID: "c3", UserID: "u3", Status: model.ContainerStopped})

	stats, err := m.StatsContainersByStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats[model.ContainerRunning] != 2 {
		t.Fatalf("expected 2 running, got %d", stats[model.ContainerRunning])
	}
	if stats[model.ContainerStopped] != 1 {
		t.Fatalf("expected 1 stopped, got %d", stats[model.ContainerStopped])
	}
}

func TestEffectiveStatusDerivation(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	cases := []struct {
		name string
		u    model.User
		want model.UserStatus
	}{
		{"no facts", model.User{}, model.StatusActive},
		{"future expiry", model.User{ExpiresAt: &future}, model.StatusActive},
		{"past expiry", model.User{ExpiresAt: &past}, model.StatusExpired},
		{"manual ban", model.User{ManualDisabled: true}, model.StatusDisabled},
		{"manual ban with future expiry", model.User{ManualDisabled: true, ExpiresAt: &future}, model.StatusDisabled},
		// expiry wins over the manual ban
		{"manual ban with past expiry", model.User{ManualDisabled: true, ExpiresAt: &past}, model.StatusExpired},
	}
	for _, c := range cases {
		if got := c.u.EffectiveStatus(); got != c.want {
			t.Errorf("%s: EffectiveStatus = %s, want %s", c.name, got, c.want)
		}
	}
}
