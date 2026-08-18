package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"devcapsule/backend/internal/model"
)

// Memory is an in-memory Store implementation for tests and dev mode.
type Memory struct {
	mu         sync.RWMutex
	users      map[string]*model.User
	byName     map[string]string // username -> id
	containers map[string]*model.Container
	byUser     map[string]string // user_id -> container id
	templates  map[string]*model.Template
	byTplName  map[string]string
	logs       []*model.AccessLog
	refresh    map[string]*memRefresh
}

type memRefresh struct {
	userID    string
	expiresAt time.Time
	consumed  bool
	revoked   bool
}

func NewMemory() *Memory {
	return &Memory{
		users:      map[string]*model.User{},
		byName:     map[string]string{},
		containers: map[string]*model.Container{},
		byUser:     map[string]string{},
		templates:  map[string]*model.Template{},
		byTplName:  map[string]string{},
		logs:       []*model.AccessLog{},
		refresh:    map[string]*memRefresh{},
	}
}

func (m *Memory) Close() error                      { return nil }
func (m *Memory) Migrate(ctx context.Context) error { return nil }

func (m *Memory) CreateUser(ctx context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byName[u.Username]; ok {
		return ErrDuplicate
	}
	copy := *u
	m.users[u.ID] = &copy
	m.byName[u.Username] = u.ID
	return nil
}

func (m *Memory) CreateRefreshToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.refresh[jti]; ok {
		return ErrDuplicate
	}
	m.refresh[jti] = &memRefresh{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *Memory) ConsumeRefreshToken(ctx context.Context, jti, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.refresh[jti]
	if !ok || rec.userID != userID || rec.consumed || rec.revoked || !time.Now().Before(rec.expiresAt) {
		return false, nil
	}
	rec.consumed = true
	return true, nil
}

func (m *Memory) RevokeRefreshTokens(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range m.refresh {
		if rec.userID == userID && !rec.consumed && !rec.revoked {
			rec.revoked = true
		}
	}
	return nil
}

func (m *Memory) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	c := *u
	return &c, nil
}

func (m *Memory) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byName[username]
	if !ok {
		return nil, ErrNotFound
	}
	c := *m.users[id]
	return &c, nil
}

func (m *Memory) ListUsers(ctx context.Context) ([]*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*model.User
	for _, u := range m.users {
		c := *u
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

func (m *Memory) ListUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []*model.User
	for id, u := range m.users {
		if want[id] {
			c := *u
			out = append(out, &c)
		}
	}
	return out, nil
}

func (m *Memory) UpdateUser(ctx context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.users[u.ID]
	if !ok {
		return ErrNotFound
	}
	if old.Username != u.Username {
		delete(m.byName, old.Username)
	}
	c := *u
	m.users[u.ID] = &c
	m.byName[c.Username] = c.ID
	return nil
}

func (m *Memory) DeleteUser(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.users, id)
	delete(m.byName, u.Username)
	if cid, ok := m.byUser[id]; ok {
		delete(m.containers, cid)
		delete(m.byUser, id)
	}
	return nil
}

func (m *Memory) CountUsers(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.users)), nil
}

func (m *Memory) CreateContainer(ctx context.Context, c *model.Container) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *c
	m.containers[c.ID] = &copy
	m.byUser[c.UserID] = c.ID
	return nil
}

func (m *Memory) GetContainerByUserID(ctx context.Context, userID string) (*model.Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cid, ok := m.byUser[userID]
	if !ok {
		return nil, ErrNotFound
	}
	c := *m.containers[cid]
	return &c, nil
}

func (m *Memory) GetContainerByID(ctx context.Context, id string) (*model.Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.containers[id]
	if !ok {
		return nil, ErrNotFound
	}
	cc := *c
	return &cc, nil
}

func (m *Memory) ListContainers(ctx context.Context) ([]*model.Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*model.Container
	for _, c := range m.containers {
		cc := *c
		out = append(out, &cc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ContainerName < out[j].ContainerName })
	return out, nil
}

func (m *Memory) ListContainersByUserIDs(ctx context.Context, userIDs []string) ([]*model.Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	want := map[string]bool{}
	for _, id := range userIDs {
		want[id] = true
	}
	var out []*model.Container
	for uid, cid := range m.byUser {
		if want[uid] {
			c := *m.containers[cid]
			out = append(out, &c)
		}
	}
	return out, nil
}

func (m *Memory) UpdateContainer(ctx context.Context, c *model.Container) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.containers[c.ID]; !ok {
		return ErrNotFound
	}
	cc := *c
	m.containers[c.ID] = &cc
	m.byUser[c.UserID] = c.ID
	return nil
}

func (m *Memory) DeleteContainerByUserID(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cid, ok := m.byUser[userID]
	if !ok {
		return ErrNotFound
	}
	delete(m.containers, cid)
	delete(m.byUser, userID)
	return nil
}

func (m *Memory) CreateTemplate(ctx context.Context, t *model.Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byTplName[t.Name]; ok {
		return ErrDuplicate
	}
	c := *t
	m.templates[t.ID] = &c
	m.byTplName[t.Name] = t.ID
	return nil
}

func (m *Memory) GetTemplate(ctx context.Context, id string) (*model.Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[id]
	if !ok {
		return nil, ErrNotFound
	}
	c := *t
	return &c, nil
}

func (m *Memory) GetTemplateByName(ctx context.Context, name string) (*model.Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byTplName[name]
	if !ok {
		return nil, ErrNotFound
	}
	c := *m.templates[id]
	return &c, nil
}

func (m *Memory) ListTemplates(ctx context.Context) ([]*model.Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*model.Template
	for _, t := range m.templates {
		c := *t
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) UpdateTemplate(ctx context.Context, t *model.Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.templates[t.ID]; !ok {
		return ErrNotFound
	}
	c := *t
	m.templates[t.ID] = &c
	return nil
}

func (m *Memory) DeleteTemplate(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.templates[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.templates, id)
	delete(m.byTplName, t.Name)
	return nil
}

func (m *Memory) LogAccess(ctx context.Context, l *model.AccessLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// The postgres store fills ts via DEFAULT now(); mirror that here so
	// LastAccess / AccessLogsSummary behave the same in memory mode.
	if l.Timestamp.IsZero() {
		l.Timestamp = time.Now()
	}
	m.logs = append(m.logs, l)
	return nil
}

func (m *Memory) ListAccessLogs(ctx context.Context, limit int) ([]*model.AccessLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*model.AccessLog, 0, len(m.logs))
	for _, l := range m.logs {
		cc := *l
		out = append(out, &cc)
	}
	return out, nil
}

func (m *Memory) StatsContainersByStatus(ctx context.Context) (map[model.ContainerStatus]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[model.ContainerStatus]int64{}
	for _, c := range m.containers {
		out[c.Status]++
	}
	return out, nil
}

func (m *Memory) LastAccess(ctx context.Context, userID string) (*time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var last *time.Time
	for _, l := range m.logs {
		if l.UserID != userID {
			continue
		}
		if last == nil || l.Timestamp.After(*last) {
			t := l.Timestamp
			last = &t
		}
	}
	return last, nil
}

func (m *Memory) AccessLogsSummary(ctx context.Context, since time.Time, onlineWindow time.Duration) (*AccessLogsSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var s AccessLogsSummary
	now := time.Now()
	onlineSince := now.Add(-onlineWindow)
	seenOnline := map[string]bool{}
	for _, l := range m.logs {
		if l.Timestamp.Before(since) {
			continue
		}
		s.Count++
		s.Bytes += l.Bytes
		s.LatencySum += l.LatencyMS
		if s.Last == nil || l.Timestamp.After(*s.Last) {
			t := l.Timestamp
			s.Last = &t
		}
		if l.Timestamp.After(onlineSince) && l.UserID != "" && !seenOnline[l.UserID] {
			seenOnline[l.UserID] = true
			s.Online++
		}
		idx := 23 - int(now.Sub(l.Timestamp).Hours())
		if idx >= 0 && idx < 24 {
			s.Last24H[idx]++
		}
	}
	return &s, nil
}

// EnsureUser exists for tests.
func (m *Memory) EnsureUser(u *model.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	m.byName[u.Username] = u.ID
}
