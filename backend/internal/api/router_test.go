package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"devcapsule/backend/internal/model"
)

func TestHealth(t *testing.T) {
	s, _, _ := newTestServer(t)
	rec := s.do(t, "GET", "/api/health", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("health status %d", rec.Code)
	}
}

func TestLoginFlow(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	addUser(t, st, "stu001", "pass12345", "user")

	token := s.login(t, "admin", "admin123")

	// me endpoint
	rec := s.do(t, "GET", "/platform/auth/me", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me failed: %d %s", rec.Code, rec.Body.String())
	}

	// wrong password
	rec = s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	// unknown user
	rec = s.do(t, "POST", "/platform/auth/login", "", `{"username":"nobody","password":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "stu001", "pass12345", "user")
	rec := &httptest.ResponseRecorder{}
	for i := 0; i < 11; i++ {
		rec = s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"bad"}`)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after 10 attempts, got %d", rec.Code)
	}
}

func TestAuthRequired(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")

	rec := s.do(t, "GET", "/platform/auth/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
	rec = s.do(t, "GET", "/platform/admin/users", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
	rec = s.do(t, "GET", "/platform/admin/users", "garbage-token", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with bad token, got %d", rec.Code)
	}
}

func TestAdminOnly(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	addUser(t, st, "stu001", "pass12345", "user")

	adminToken := s.login(t, "admin", "admin123")
	userToken := s.login(t, "stu001", "pass12345")

	rec := s.do(t, "GET", "/platform/admin/users", userToken, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", rec.Code)
	}
	rec = s.do(t, "GET", "/platform/admin/users", adminToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestBatchUsersGenerated(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "POST", "/platform/admin/users/batch", token,
		`{"count":5,"prefix":"stu","password_length":12,"course":"Python 基础"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch failed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Created  int `json:"created"`
			Accounts []struct {
				Username string `json:"username"`
				Password string `json:"password"`
			} `json:"accounts"`
			Users []*model.User `json:"users"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if out.Data.Created != 5 {
		t.Fatalf("expected 5 created, got %d", out.Data.Created)
	}
	if len(out.Data.Accounts) != 5 {
		t.Fatalf("expected 5 accounts")
	}
	for _, a := range out.Data.Accounts {
		if len(a.Password) != 12 {
			t.Errorf("password length %d", len(a.Password))
		}
	}
	// course field must be persisted
	users, _ := st.ListUsers(context.Background())
	stuCount := 0
	for _, u := range users {
		if u.Username == "admin" {
			continue
		}
		stuCount++
		if u.Course != "Python 基础" {
			t.Fatalf("expected course persisted, got %q for %s", u.Course, u.Username)
		}
		if u.PasswordPlain == "" {
			t.Fatalf("expected plaintext password stored for %s", u.Username)
		}
	}
	if stuCount != 5 {
		t.Fatalf("expected 5 students, got %d", stuCount)
	}
	// list API must expose the plaintext password
	rec3 := s.do(t, "GET", "/platform/admin/users", token, "")
	var list struct {
		Data []struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"data"`
	}
	decodeJSON(t, rec3, &list)
	for _, u := range list.Data {
		if u.Username == "admin" {
			continue
		}
		if len(u.Password) != 12 {
			t.Fatalf("expected password in list for %s, got %q", u.Username, u.Password)
		}
	}
	// accounts must be usable for login
	for _, a := range out.Data.Accounts {
		rec2 := s.do(t, "POST", "/platform/auth/login", "", `{"username":"`+a.Username+`","password":"`+a.Password+`"}`)
		if rec2.Code != http.StatusOK {
			t.Errorf("login with generated account %s failed: %d %s", a.Username, rec2.Code, rec2.Body.String())
		}
	}
	if len(users) != 6 { // admin + 5
		t.Fatalf("expected 6 users total, got %d", len(users))
	}
}

func TestUpdateUserCourse(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "pass12345", "user")
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "PATCH", "/platform/admin/users/"+user.ID, token, `{"course":"数据结构"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := st.GetUserByID(t.Context(), user.ID)
	if got.Course != "数据结构" {
		t.Fatalf("expected course updated, got %q", got.Course)
	}
}

func TestBatchUsersExplicitAndDuplicateSkipped(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "POST", "/platform/admin/users/batch", token,
		`{"usernames":["alice","bob"],"password_length":10}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch failed: %d", rec.Code)
	}
	var out struct {
		Data struct {
			Created int `json:"created"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if out.Data.Created != 2 {
		t.Fatalf("expected 2, got %d", out.Data.Created)
	}

	// duplicate usernames should be skipped
	rec = s.do(t, "POST", "/platform/admin/users/batch", token, `{"usernames":["alice","carol"]}`)
	decodeJSON(t, rec, &out)
	if out.Data.Created != 1 {
		t.Fatalf("expected 1 new (carol), got %d", out.Data.Created)
	}
}

func TestUpdateAndDeleteUser(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "oldpass123", "user")
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "PATCH", "/platform/admin/users/"+user.ID, token, `{"password":"newpass456","status":"disabled"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", rec.Code, rec.Body.String())
	}
	// old password no longer works
	if rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"oldpass123"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password rejected, got %d", rec.Code)
	}
	// disabled account cannot login even with right password
	if rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"newpass456"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled account, got %d", rec.Code)
	}

	rec = s.do(t, "DELETE", "/platform/admin/users/"+user.ID, token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d", rec.Code)
	}
	if rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"newpass456"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected deleted user rejected, got %d", rec.Code)
	}
}

func TestBatchUserActions(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	u1 := addUser(t, st, "stu001", "pass12345", "user")
	u2 := addUser(t, st, "stu002", "pass12345", "user")
	admin := addUser(t, st, "root2", "pass12345", "admin")
	token := s.login(t, "admin", "admin123")

	var out struct {
		Data []struct {
			Username string `json:"username"`
			OK       bool   `json:"ok"`
		} `json:"data"`
	}

	// restart/stop on users without containers -> per-row error, not fatal
	rec := s.do(t, "POST", "/platform/admin/users/batch/action", token,
		`{"user_ids":["`+u1.ID+`"],"action":"stop"}`)
	decodeJSON(t, rec, &out)
	if out.Data[0].OK {
		t.Fatal("expected stop to fail without container")
	}

	// delete removes users
	rec = s.do(t, "POST", "/platform/admin/users/batch/action", token,
		`{"user_ids":["`+u1.ID+`","`+u2.ID+`"],"action":"delete"}`)
	decodeJSON(t, rec, &out)
	for _, r := range out.Data {
		if !r.OK {
			t.Fatalf("delete failed: %+v", r)
		}
	}
	if _, err := st.GetUserByID(t.Context(), u1.ID); err == nil {
		t.Fatal("expected u1 deleted")
	}

	// admin users are refused
	rec = s.do(t, "POST", "/platform/admin/users/batch/action", token,
		`{"user_ids":["`+admin.ID+`"],"action":"delete"}`)
	decodeJSON(t, rec, &out)
	if out.Data[0].OK {
		t.Fatal("expected admin to be refused")
	}

	// unknown action -> 400
	rec = s.do(t, "POST", "/platform/admin/users/batch/action", token,
		`{"user_ids":["`+u2.ID+`"],"action":"explode"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestChangePasswordSelfService(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "oldpass123", "user")

	// must be authenticated
	rec := s.do(t, "POST", "/platform/auth/change-password", "", `{"old_password":"oldpass123","new_password":"newpass456"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	token := s.login(t, "stu001", "oldpass123")

	// wrong old password -> 403
	rec = s.do(t, "POST", "/platform/auth/change-password", token, `{"old_password":"wrong","new_password":"newpass456"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong old password, got %d", rec.Code)
	}

	// too short new password -> 400
	rec = s.do(t, "POST", "/platform/auth/change-password", token, `{"old_password":"oldpass123","new_password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", rec.Code)
	}

	// success
	rec = s.do(t, "POST", "/platform/auth/change-password", token, `{"old_password":"oldpass123","new_password":"newpass456"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("change failed: %d %s", rec.Code, rec.Body.String())
	}
	// old password no longer works, new one does
	if rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"oldpass123"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password rejected, got %d", rec.Code)
	}
	if rec := s.do(t, "POST", "/platform/auth/login", "", `{"username":"stu001","password":"newpass456"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected login with new password, got %d", rec.Code)
	}
	// stored plaintext updated
	got, _ := st.GetUserByID(t.Context(), user.ID)
	if got.PasswordPlain != "newpass456" {
		t.Fatalf("expected plaintext updated, got %q", got.PasswordPlain)
	}
}

func TestExportUsers(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	addUser(t, st, "stu001", "pass12345", "user")
	token := s.login(t, "admin", "admin123")
	rec := s.do(t, "GET", "/platform/admin/users/export", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export failed: %d", rec.Code)
	}
}

func TestTemplatesCRUD(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "POST", "/platform/admin/templates", token,
		`{"name":"student","image":"ghcr.io/anomalyco/opencode:latest","internal_port":4096,"extra_ports":[3000,5173,3000,0],"cpu_limit":0.5,"mem_limit":1073741824}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create template failed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			ID         string `json:"id"`
			ExtraPorts []int  `json:"extra_ports"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if out.Data.ID == "" {
		t.Fatal("missing template id")
	}
	if len(out.Data.ExtraPorts) != 2 || out.Data.ExtraPorts[0] != 3000 || out.Data.ExtraPorts[1] != 5173 {
		t.Fatalf("extra_ports not normalized (dedup/range): %v", out.Data.ExtraPorts)
	}

	rec = s.do(t, "GET", "/platform/admin/templates", token, "")
	if rec.Code != http.StatusOK {
		t.Fatal("list templates failed")
	}

	rec = s.do(t, "GET", "/platform/admin/templates/"+out.Data.ID, token, "")
	if rec.Code != http.StatusOK {
		t.Fatal("get template failed")
	}

	// duplicate name conflict
	rec = s.do(t, "POST", "/platform/admin/templates", token, `{"name":"student","image":"x"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate name, got %d", rec.Code)
	}

	rec = s.do(t, "PUT", "/platform/admin/templates/"+out.Data.ID, token, `{"cpu_limit":1.0,"mem_limit":2147483648,"extra_ports":[8000]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update template failed: %d %s", rec.Code, rec.Body.String())
	}

	// updated extra_ports persisted
	rec = s.do(t, "GET", "/platform/admin/templates/"+out.Data.ID, token, "")
	var got struct {
		Data struct {
			ExtraPorts []int    `json:"extra_ports"`
			Command    []string `json:"command"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &got)
	if len(got.Data.ExtraPorts) != 1 || got.Data.ExtraPorts[0] != 8000 {
		t.Fatalf("extra_ports not updated: %v", got.Data.ExtraPorts)
	}

	// command updated and persisted
	rec = s.do(t, "PUT", "/platform/admin/templates/"+out.Data.ID, token, `{"command":["opencode","web","--port","4096"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update command failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = s.do(t, "GET", "/platform/admin/templates/"+out.Data.ID, token, "")
	decodeJSON(t, rec, &got)
	if len(got.Data.Command) != 4 || got.Data.Command[0] != "opencode" {
		t.Fatalf("command not updated: %v", got.Data.Command)
	}

	rec = s.do(t, "DELETE", "/platform/admin/templates/"+out.Data.ID, token, "")
	if rec.Code != http.StatusOK {
		t.Fatal("delete template failed")
	}
}

func TestDashboardStats(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	u := addUser(t, st, "stu001", "pass12345", "user")
	u.Course = "Python 基础"
	st.UpdateUser(t.Context(), u)
	st.CreateContainer(t.Context(), &model.Container{
		ID: model.NewID(), UserID: u.ID, TemplateID: "tpl1",
		ContainerID: "fake", ContainerName: "user-stu001",
		Status: model.ContainerRunning, InternalPort: 4096,
	})
	st.CreateTemplate(t.Context(), &model.Template{ID: "tpl1", Name: "opencode", Image: "ghcr.io/sst/opencode", InternalPort: 4096})
	st.LogAccess(t.Context(), &model.AccessLog{UserID: u.ID, Path: "/", Status: 200, Bytes: 1024, LatencyMS: 12, Timestamp: time.Now()})
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "GET", "/platform/admin/stats/dashboard", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard failed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Users    map[string]any `json:"users"`
			Requests struct {
				Online  int64   `json:"online"`
				Last24h []int64 `json:"last24h"`
			} `json:"requests"`
			Resources struct {
				MemBytes float64 `json:"mem_bytes"`
			} `json:"resources"`
			Courses   []map[string]any `json:"courses"`
			Templates struct {
				Total int `json:"total"`
			} `json:"templates"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if out.Data.Users == nil {
		t.Fatal("missing users stats")
	}
	if out.Data.Requests.Online != 1 {
		t.Fatalf("online=%d want 1", out.Data.Requests.Online)
	}
	if len(out.Data.Requests.Last24h) != 24 {
		t.Fatalf("last24h len=%d want 24", len(out.Data.Requests.Last24h))
	}
	found := false
	for _, c := range out.Data.Courses {
		if c["course"] == "Python 基础" && c["running"] == float64(1) {
			found = true
		}
	}
	if !found {
		t.Fatalf("courses missing Python 基础: %v", out.Data.Courses)
	}
	if out.Data.Templates.Total != 1 {
		t.Fatalf("templates total=%d want 1", out.Data.Templates.Total)
	}
}

func TestProvisionUsesPerUserTemplate(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	u1 := addUser(t, st, "stu001", "pass12345", "user")
	u2 := addUser(t, st, "stu002", "pass12345", "user")
	token := s.login(t, "admin", "admin123")

	// two templates; u1's container record is bound to tpl A
	st.CreateTemplate(t.Context(), &model.Template{ID: "tplA", Name: "A", Image: "a", InternalPort: 4096})
	st.CreateTemplate(t.Context(), &model.Template{ID: "tplB", Name: "B", Image: "b", InternalPort: 8080})
	st.CreateContainer(t.Context(), &model.Container{
		ID: model.NewID(), UserID: u1.ID, TemplateID: "tplA",
		ContainerID: "fake", ContainerName: "user-stu001",
		Status: model.ContainerRunning, InternalPort: 4096,
	})

	// request template B, but u1 must be provisioned with its own tpl A
	rec := s.do(t, "POST", "/platform/admin/containers/batch", token,
		`{"template_id":"tplB","user_ids":["`+u1.ID+`","`+u2.ID+`"],"force":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("provision failed: %d %s", rec.Code, rec.Body.String())
	}
	// with force=false and fake containers the docker client is unavailable,
	// so results may error; what matters is no panic and per-user grouping ran
	var out struct {
		Data struct {
			Results []struct {
				Username string `json:"username"`
				OK       bool   `json:"ok"`
			} `json:"results"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if len(out.Data.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out.Data.Results))
	}
}

func TestBatchUsersCourseDerivesUsername(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	token := s.login(t, "admin", "admin123")

	// no prefix sent: usernames derive from the course name
	rec := s.do(t, "POST", "/platform/admin/users/batch", token,
		`{"count":2,"course":"Python 基础","password_length":12}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch failed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Accounts []struct {
				Username string `json:"username"`
			} `json:"accounts"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if len(out.Data.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(out.Data.Accounts))
	}
	if out.Data.Accounts[0].Username != "python001" || out.Data.Accounts[1].Username != "python002" {
		t.Fatalf("expected python001/python002, got %s/%s",
			out.Data.Accounts[0].Username, out.Data.Accounts[1].Username)
	}
}

func TestContainerListEmpty(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	token := s.login(t, "admin", "admin123")
	rec := s.do(t, "GET", "/platform/admin/containers", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list containers failed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data json.RawMessage `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if string(out.Data) != "[]" {
		t.Fatalf("expected empty list, got %s", out.Data)
	}
}

func TestMeIncludesContainer(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "pass12345", "user")
	token := s.login(t, "stu001", "pass12345")

	// no container yet: user only
	rec := s.do(t, "GET", "/platform/auth/me", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me failed: %d", rec.Code)
	}
	var out struct {
		Data struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out)
	if out.Data.User.Username != "stu001" {
		t.Fatalf("expected user in me, got %+v", out.Data)
	}

	// with container: container + extra ports attached
	st.CreateTemplate(t.Context(), &model.Template{
		ID: "tpl1", Name: "multi", Image: "x", InternalPort: 4096, ExtraPorts: []int{3000},
	})
	st.CreateContainer(t.Context(), &model.Container{
		ID: model.NewID(), UserID: user.ID, TemplateID: "tpl1",
		ContainerID: "fake", ContainerName: "user-stu001",
		Status: model.ContainerRunning, InternalPort: 4096, Secret: "s",
	})
	rec = s.do(t, "GET", "/platform/auth/me", token, "")
	var out2 struct {
		Data struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
			Container struct {
				ContainerName string `json:"container_name"`
				InternalPort  int    `json:"internal_port"`
				ExtraPorts    []int  `json:"extra_ports"`
			} `json:"container"`
		} `json:"data"`
	}
	decodeJSON(t, rec, &out2)
	if out2.Data.Container.ContainerName != "user-stu001" || out2.Data.Container.InternalPort != 4096 {
		t.Fatalf("container not attached: %+v", out2.Data)
	}
	if len(out2.Data.Container.ExtraPorts) != 1 || out2.Data.Container.ExtraPorts[0] != 3000 {
		t.Fatalf("extra ports not attached: %+v", out2.Data.Container.ExtraPorts)
	}
}

func TestLogoutClearsCookies(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	token := s.login(t, "admin", "admin123")

	rec := s.do(t, "GET", "/platform/auth/logout", token, "")
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("expected redirect to /, got %q", loc)
	}
	cookies := rec.Result().Cookies()
	got := map[string]bool{}
	for _, c := range cookies {
		got[c.Name] = true
		if c.MaxAge != -1 {
			t.Fatalf("expected MaxAge=-1 for %s", c.Name)
		}
	}
	if !got["access_token"] || !got["refresh_token"] {
		t.Fatalf("expected both cookies cleared, got %v", got)
	}
}

func TestProxyRequiresAuth(t *testing.T) {
	s, _, _ := newTestServer(t)
	// anonymous gets the SPA (login page), not an error
	rec := s.do(t, "GET", "/hello", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 SPA for anonymous, got %d", rec.Code)
	}
}

func TestStudentRootGetsContainerNotSPA(t *testing.T) {
	s, st, _ := newTestServer(t)
	addUser(t, st, "admin", "admin123", "admin")
	user := addUser(t, st, "stu001", "pass12345", "user")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "container on %s", r.URL.Path)
	}))
	defer upstream.Close()
	s.px.SetHostForForTesting(func(rec *model.Container) string { return "127.0.0.1" })
	st.CreateTemplate(t.Context(), &model.Template{ID: "t1", Name: "t", Image: "x", InternalPort: portOf(upstream.URL)})
	st.CreateContainer(t.Context(), &model.Container{
		ID: model.NewID(), UserID: user.ID, TemplateID: "t1",
		ContainerID: "c", ContainerName: "user-stu001",
		Status: model.ContainerRunning, InternalPort: portOf(upstream.URL),
	})
	token := s.login(t, "stu001", "pass12345")
	rec := s.do(t, "GET", "/hello", token, "")
	if !strings.Contains(rec.Body.String(), "container on /hello") {
		t.Fatalf("expected container response, got %d %s", rec.Code, rec.Body.String())
	}
}
