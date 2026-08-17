package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"devcapsule/backend/internal/auth"
	"devcapsule/backend/internal/batch"
	"devcapsule/backend/internal/model"
)

type batchUsersRequest struct {
	Count          int      `json:"count"`
	Prefix         string   `json:"prefix"`
	PasswordLength int      `json:"password_length"`
	Usernames      []string `json:"usernames"` // explicit usernames (password auto-generated)
	Course         string   `json:"course"`
	ExpiresInDays  int      `json:"expires_in_days"`
	CPULimit       float64  `json:"cpu_limit"`
	MemLimit       int64    `json:"mem_limit"`
}

// handleBatchUsers creates users: either n generated or explicit usernames.
func (s *Server) handleBatchUsers(w http.ResponseWriter, r *http.Request) {
	var req batchUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.Prefix == "" {
		req.Prefix = batch.SlugPrefix(req.Course)
	}
	if req.PasswordLength <= 0 {
		req.PasswordLength = 12
	}
	if req.CPULimit <= 0 {
		req.CPULimit = 0.5
	}
	if req.MemLimit <= 0 {
		req.MemLimit = 1 << 30
	}

	existing, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	taken := map[string]bool{}
	for _, u := range existing {
		taken[u.Username] = true
	}

	var accounts []batch.Account
	if len(req.Usernames) > 0 {
		// Dedupe within the request: duplicate names would otherwise be
		// created once and then fail the UNIQUE constraint mid-batch,
		// leaving a partial creation behind and erroring the whole request.
		seen := map[string]bool{}
		for _, name := range req.Usernames {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			pw, err := batch.GeneratePassword(req.PasswordLength)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "generate password")
				return
			}
			accounts = append(accounts, batch.Account{Username: name, Password: pw})
		}
	} else {
		if req.Count <= 0 || req.Count > 1000 {
			writeError(w, http.StatusBadRequest, "count must be 1..1000")
			return
		}
		accounts, err = batch.GenerateAccounts(req.Prefix, req.Count, taken, req.PasswordLength)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "generate accounts")
			return
		}
	}

	var created []*model.User
	var skipped []string
	now := time.Now().UTC()

	// Hash passwords concurrently (argon2 is CPU-bound) with a bounded pool.
	hashes := make([]string, len(accounts))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var hashErr error
	var hashErrMu sync.Mutex
	for i, acc := range accounts {
		wg.Add(1)
		go func(i int, pw string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			h, err := auth.HashPassword(pw)
			if err != nil {
				hashErrMu.Lock()
				hashErr = err
				hashErrMu.Unlock()
				return
			}
			hashes[i] = h
		}(i, acc.Password)
	}
	wg.Wait()
	if hashErr != nil {
		writeError(w, http.StatusInternalServerError, "hash password")
		return
	}

	for i, acc := range accounts {
		if taken[acc.Username] {
			skipped = append(skipped, acc.Username)
			continue
		}
		var expires *time.Time
		if req.ExpiresInDays > 0 {
			t := now.AddDate(0, 0, req.ExpiresInDays)
			expires = &t
		}
		u := &model.User{
			ID:            model.NewID(),
			Username:      acc.Username,
			PasswordHash:  hashes[i],
			PasswordPlain: acc.Password,
			Role:          model.RoleUser,
			Status:        model.StatusActive,
			Course:        req.Course,
			ExpiresAt:     expires,
			CPULimit:      req.CPULimit,
			MemLimit:      req.MemLimit,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := s.st.CreateUser(r.Context(), u); err != nil {
			writeError(w, http.StatusInternalServerError, "create user: "+err.Error())
			return
		}
		created = append(created, u)
		taken[u.Username] = true
	}

	writeData(w, map[string]any{
		"created":  len(created),
		"skipped":  skipped,
		"accounts": accounts,
		"users":    created,
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, users)
}

type updateUserRequest struct {
	Password      *string  `json:"password"`
	Status        *string  `json:"status"`
	Course        *string  `json:"course"`
	ExpiresInDays *int     `json:"expires_in_days"`
	CPULimit      *float64 `json:"cpu_limit"`
	MemLimit      *int64   `json:"mem_limit"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := s.st.GetUserByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.Password != nil {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash password")
			return
		}
		user.PasswordHash = hash
		user.PasswordPlain = *req.Password
	}
	if req.Status != nil {
		switch model.UserStatus(*req.Status) {
		case model.StatusActive, model.StatusDisabled, model.StatusExpired:
			user.Status = model.UserStatus(*req.Status)
		default:
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
	}
	if req.Course != nil {
		user.Course = *req.Course
	}
	if req.ExpiresInDays != nil {
		if *req.ExpiresInDays > 0 {
			t := time.Now().UTC().AddDate(0, 0, *req.ExpiresInDays)
			user.ExpiresAt = &t
		} else {
			user.ExpiresAt = nil
		}
	}
	if req.CPULimit != nil {
		user.CPULimit = *req.CPULimit
	}
	if req.MemLimit != nil {
		user.MemLimit = *req.MemLimit
	}
	if err := s.st.UpdateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := s.st.GetUserByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	// remove docker container first if any
	if rec, err := s.st.GetContainerByUserID(r.Context(), user.ID); err == nil && rec.ContainerID != "" {
		s.orch.Remove(r.Context(), rec)
	}
	if err := s.st.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeData(w, map[string]any{"deleted": true})
}

// batchUserActionRequest drives the multi-select bulk actions from the admin UI.
type batchUserActionRequest struct {
	UserIDs []string `json:"user_ids"`
	Action  string   `json:"action"` // delete | restart | stop
}

// batchUserActionResult is one row of the bulk-operation result.
type batchUserActionResult struct {
	Username string `json:"username"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// handleBatchUserAction applies an action to many users at once.
func (s *Server) handleBatchUserAction(w http.ResponseWriter, r *http.Request) {
	var req batchUserActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if len(req.UserIDs) == 0 {
		writeError(w, http.StatusBadRequest, "user_ids required")
		return
	}
	switch req.Action {
	case "delete", "restart", "stop", "start":
	default:
		writeError(w, http.StatusBadRequest, "unknown action: "+req.Action)
		return
	}

	results := make([]batchUserActionResult, 0, len(req.UserIDs))
	// Batch-fetch users and containers up front to avoid per-user queries.
	users, _ := s.st.ListUsersByIDs(r.Context(), req.UserIDs)
	byID := map[string]*model.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	recs, _ := s.st.ListContainersByUserIDs(r.Context(), req.UserIDs)
	recByUser := map[string]*model.Container{}
	for _, c := range recs {
		recByUser[c.UserID] = c
	}

	for _, id := range req.UserIDs {
		res := batchUserActionResult{OK: true}
		user := byID[id]
		if user == nil {
			res.OK = false
			res.Error = "user not found"
			results = append(results, res)
			continue
		}
		res.Username = user.Username
		if user.Role == model.RoleAdmin {
			res.OK = false
			res.Error = "admin cannot be bulk-managed"
			results = append(results, res)
			continue
		}

		switch req.Action {
		case "delete":
			if rec := recByUser[id]; rec != nil && rec.ContainerID != "" {
				if err := s.orch.Remove(r.Context(), rec); err != nil {
					res.OK = false
					res.Error = err.Error()
				}
			}
			if res.OK {
				if err := s.st.DeleteUser(r.Context(), user.ID); err != nil {
					res.OK = false
					res.Error = err.Error()
				}
			}
		case "start", "restart", "stop":
			rec := recByUser[id]
			if rec == nil {
				res.OK = false
				res.Error = "no container"
				break
			}
			var err error
			switch req.Action {
			case "start":
				err = s.orch.Start(r.Context(), rec)
			case "restart":
				err = s.orch.Restart(r.Context(), rec)
			case "stop":
				err = s.orch.Stop(r.Context(), rec)
			}
			if err != nil {
				res.OK = false
				res.Error = err.Error()
			}
		}
		results = append(results, res)
	}
	writeData(w, results)
}

func (s *Server) handleExportUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	type row struct {
		Username string `json:"username"`
		Status   string `json:"status"`
		Expires  string `json:"expires_at"`
		Role     string `json:"role"`
	}
	out := make([]row, 0, len(users))
	for _, u := range users {
		exp := ""
		if u.ExpiresAt != nil {
			exp = u.ExpiresAt.Format(time.RFC3339)
		}
		out = append(out, row{Username: u.Username, Status: string(u.Status), Expires: exp, Role: string(u.Role)})
	}
	writeData(w, out)
}
