package store

import (
	"context"
	"os"
	"testing"
	"time"

	"devcapsule/backend/internal/model"
)

func TestPostgresCRUD(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping postgres integration test")
	}
	ctx := context.Background()
	st, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	u := &model.User{
		ID:            model.NewID(),
		Username:      "pgtest" + model.NewID()[:6],
		PasswordHash:  "hash",
		PasswordPlain: "plainpass",
		Role:          model.RoleUser,
		Status:        model.StatusActive,
		Course:        "Python 基础",
		CPULimit:      0.5,
		MemLimit:      1 << 30,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, err := st.GetUserByUsername(ctx, u.Username)
	if err != nil || got.ID != u.ID {
		t.Fatalf("get by username: %v %+v", err, got)
	}
	if got.Course != "Python 基础" {
		t.Fatalf("course not preserved: %q", got.Course)
	}
	if got.PasswordPlain != "plainpass" {
		t.Fatalf("plaintext password not preserved: %q", got.PasswordPlain)
	}

	// update course + plaintext password
	got.Course = "数据结构"
	got.PasswordPlain = "newpass"
	if err := st.UpdateUser(ctx, got); err != nil {
		t.Fatalf("update user: %v", err)
	}
	got2, _ := st.GetUserByUsername(ctx, u.Username)
	if got2.Course != "数据结构" || got2.PasswordPlain != "newpass" {
		t.Fatalf("course/password not updated: course=%q pwd=%q", got2.Course, got2.PasswordPlain)
	}

	// template with envs + command + extra ports round-trip
	tpl := &model.Template{
		ID:           model.NewID(),
		Name:         "pg-tpl" + model.NewID()[:6],
		Image:        "devcapsule/student:1",
		InternalPort: 4096,
		ExtraPorts:   []int{3000, 5173},
		Envs:         map[string]string{"K": "V"},
		CPULimit:     1.0,
		MemLimit:     2 << 30,
		WorkspaceDir: "/workspace",
		Command:      []string{"opencode", "web"},
		CreatedAt:    now,
	}
	if err := st.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	tplGot, err := st.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if tplGot.Envs["K"] != "V" || len(tplGot.Command) != 2 || tplGot.Command[0] != "opencode" {
		t.Fatalf("template fields not preserved: %+v", tplGot)
	}
	if len(tplGot.ExtraPorts) != 2 || tplGot.ExtraPorts[0] != 3000 || tplGot.ExtraPorts[1] != 5173 {
		t.Fatalf("extra ports not preserved: %+v", tplGot.ExtraPorts)
	}

	// update template with changed extra ports
	tplGot.ExtraPorts = []int{8000}
	if err := st.UpdateTemplate(ctx, tplGot); err != nil {
		t.Fatalf("update template: %v", err)
	}
	tplGot2, _ := st.GetTemplate(ctx, tpl.ID)
	if len(tplGot2.ExtraPorts) != 1 || tplGot2.ExtraPorts[0] != 8000 {
		t.Fatalf("extra ports not updated: %+v", tplGot2.ExtraPorts)
	}

	// container record with secret
	c := &model.Container{
		ID:            model.NewID(),
		UserID:        u.ID,
		TemplateID:    tpl.ID,
		ContainerID:   "abc123",
		ContainerName: "user-" + u.Username,
		Status:        model.ContainerRunning,
		InternalPort:  4096,
		Secret:        "s3cr3t",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := st.CreateContainer(ctx, c); err != nil {
		t.Fatalf("create container: %v", err)
	}
	crec, err := st.GetContainerByUserID(ctx, u.ID)
	if err != nil || crec.Secret != "s3cr3t" {
		t.Fatalf("container round-trip: %v %+v", err, crec)
	}
	c.Status = model.ContainerStopped
	if err := st.UpdateContainer(ctx, c); err != nil {
		t.Fatalf("update container: %v", err)
	}

	// access log + last access
	lg := &model.AccessLog{UserID: u.ID, Path: "/u/x", Status: 200, Bytes: 10, LatencyMS: 5, Timestamp: now}
	if err := st.LogAccess(ctx, lg); err != nil {
		t.Fatalf("log access: %v", err)
	}
	last, err := st.LastAccess(ctx, u.ID)
	if err != nil || last == nil {
		t.Fatalf("last access: %v %v", err, last)
	}
	logs, _ := st.ListAccessLogs(ctx, 10)
	if len(logs) == 0 {
		t.Fatal("expected access logs")
	}

	// expiry
	exp := now.Add(-time.Hour)
	eu := &model.User{ID: model.NewID(), Username: "pg-exp" + model.NewID()[:6], PasswordHash: "h", Role: model.RoleUser, Status: model.StatusActive, ExpiresAt: &exp, CreatedAt: now, UpdatedAt: now}
	if err := st.CreateUser(ctx, eu); err != nil {
		t.Fatalf("create expire user: %v", err)
	}
	n, err := st.ExpireUsers(ctx, now)
	if err != nil || n < 1 {
		t.Fatalf("expire users: %v n=%d", err, n)
	}
	euGot, _ := st.GetUserByID(ctx, eu.ID)
	if euGot.Status != model.StatusExpired {
		t.Fatalf("expected expired, got %s", euGot.Status)
	}

	// stats
	stats, err := st.StatsContainersByStatus(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats[model.ContainerStopped] < 1 {
		t.Fatalf("expected stopped containers, got %v", stats)
	}

	// cleanup
	if err := st.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if err := st.DeleteTemplate(ctx, tpl.ID); err != nil {
		t.Fatalf("delete template: %v", err)
	}
}
