package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"devcapsule/backend/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicate = errors.New("duplicate")

type Postgres struct {
	db *sql.DB
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Postgres{db: db}, nil
}

func (p *Postgres) Close() error { return p.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id              TEXT PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    password_plain  TEXT NOT NULL DEFAULT '',
    role            TEXT NOT NULL DEFAULT 'user',
    manual_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    course          TEXT NOT NULL DEFAULT '',
    expires_at      TIMESTAMPTZ,
    cpu_limit       DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    mem_limit       BIGINT NOT NULL DEFAULT 1073741824,
    container_id    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS user_containers (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    template_id    TEXT,
    container_id   TEXT,
    container_name TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending',
    internal_port  INTEGER NOT NULL DEFAULT 4096,
    secret         TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS image_templates (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    image          TEXT NOT NULL,
    internal_port  INTEGER NOT NULL DEFAULT 4096,
    extra_ports    JSONB NOT NULL DEFAULT '[]',
    envs           JSONB NOT NULL DEFAULT '{}',
    cpu_limit      DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    mem_limit      BIGINT NOT NULL DEFAULT 1073741824,
    healthcheck_cmd TEXT,
    workspace_dir  TEXT NOT NULL DEFAULT '/workspace',
    command        JSONB NOT NULL DEFAULT '[]',
    run_user       TEXT NOT NULL DEFAULT '',
    cap_add        JSONB NOT NULL DEFAULT '[]',
    is_system      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE image_templates ADD COLUMN IF NOT EXISTS command JSONB NOT NULL DEFAULT '[]';
ALTER TABLE image_templates ADD COLUMN IF NOT EXISTS extra_ports JSONB NOT NULL DEFAULT '[]';
ALTER TABLE image_templates ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE image_templates ADD COLUMN IF NOT EXISTS run_user TEXT NOT NULL DEFAULT '';
ALTER TABLE image_templates ADD COLUMN IF NOT EXISTS cap_add JSONB NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS course TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_plain TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS manual_disabled BOOLEAN NOT NULL DEFAULT FALSE;
-- migrate from the legacy status model: disabled -> manual_disabled
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='status') THEN
    UPDATE users SET manual_disabled = TRUE WHERE status = 'disabled';
  END IF;
END $$;
ALTER TABLE users DROP COLUMN IF EXISTS status;
ALTER TABLE users DROP COLUMN IF EXISTS auto_disabled;
CREATE TABLE IF NOT EXISTS access_logs (
    id         BIGSERIAL PRIMARY KEY,
    user_id    TEXT,
    path       TEXT,
    status     INTEGER,
    bytes      BIGINT,
    latency_ms BIGINT,
    ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS refresh_tokens (
    jti         TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_containers_user ON user_containers(user_id);
CREATE INDEX IF NOT EXISTS idx_access_logs_user_ts ON access_logs(user_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
`

func (p *Postgres) Migrate(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, schema)
	return err
}

// CreateRefreshToken records a refresh token so it can be consumed once and
// revoked on logout. Expired rows are cleaned up opportunistically on issue.
func (p *Postgres) CreateRefreshToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	if _, err := p.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expires_at < now()`); err != nil {
		return err
	}
	_, err := p.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (jti, user_id, expires_at) VALUES ($1,$2,$3)`,
		jti, userID, expiresAt)
	return err
}

// ConsumeRefreshToken atomically marks a refresh token as used. It reports
// false when the token is unknown, expired, already consumed, or revoked, so
// replaying the same token is rejected.
func (p *Postgres) ConsumeRefreshToken(ctx context.Context, jti, userID string) (bool, error) {
	res, err := p.db.ExecContext(ctx, `
UPDATE refresh_tokens SET consumed_at = now()
WHERE jti=$1 AND user_id=$2 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at > now()`,
		jti, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RevokeRefreshTokens invalidates every outstanding refresh token for a user
// (e.g. on logout), without touching tokens that were already consumed.
func (p *Postgres) RevokeRefreshTokens(ctx context.Context, userID string) error {
	_, err := p.db.ExecContext(ctx, `
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id=$1 AND revoked_at IS NULL AND consumed_at IS NULL`, userID)
	return err
}

func (p *Postgres) CreateUser(ctx context.Context, u *model.User) error {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO users (id, username, password_hash, password_plain, role, manual_disabled, course, expires_at, cpu_limit, mem_limit, container_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		u.ID, u.Username, u.PasswordHash, u.PasswordPlain, string(u.Role), u.ManualDisabled, u.Course,
		u.ExpiresAt, u.CPULimit, u.MemLimit, nullString(u.ContainerID), u.CreatedAt, u.UpdatedAt)
	return err
}

func (p *Postgres) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	return p.scanUser(p.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, password_plain, role, manual_disabled, course, expires_at, cpu_limit, mem_limit, container_id, created_at, updated_at
FROM users WHERE id=$1`, id))
}

func (p *Postgres) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return p.scanUser(p.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, password_plain, role, manual_disabled, course, expires_at, cpu_limit, mem_limit, container_id, created_at, updated_at
FROM users WHERE username=$1`, username))
}

func (p *Postgres) scanUser(row interface{ Scan(...any) error }) (*model.User, error) {
	var u model.User
	var containerID sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PasswordPlain, (*string)(&u.Role),
		&u.ManualDisabled, &u.Course, &u.ExpiresAt, &u.CPULimit, &u.MemLimit, &containerID, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.ContainerID = containerID.String
	return &u, nil
}

func (p *Postgres) ListUsers(ctx context.Context) ([]*model.User, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT id, username, password_hash, password_plain, role, manual_disabled, course, expires_at, cpu_limit, mem_limit, container_id, created_at, updated_at
FROM users ORDER BY course, username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		u, err := p.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) ListUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := p.db.QueryContext(ctx, `
SELECT id, username, password_hash, password_plain, role, manual_disabled, course, expires_at, cpu_limit, mem_limit, container_id, created_at, updated_at
FROM users WHERE id = ANY($1) ORDER BY course, username`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		u, err := p.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateUser(ctx context.Context, u *model.User) error {
	res, err := p.db.ExecContext(ctx, `
UPDATE users SET password_hash=$2, password_plain=$3, role=$4, manual_disabled=$5, course=$6, expires_at=$7, cpu_limit=$8, mem_limit=$9, container_id=$10, updated_at=now()
WHERE id=$1`,
		u.ID, u.PasswordHash, u.PasswordPlain, string(u.Role), u.ManualDisabled, u.Course, u.ExpiresAt, u.CPULimit, u.MemLimit, nullString(u.ContainerID))
	if err != nil {
		return err
	}
	return checkRows(res)
}

func (p *Postgres) DeleteUser(ctx context.Context, id string) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	return checkRows(res)
}

func (p *Postgres) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	err := p.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (p *Postgres) CreateContainer(ctx context.Context, c *model.Container) error {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO user_containers (id, user_id, template_id, container_id, container_name, status, internal_port, secret, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID, c.UserID, c.TemplateID, nullString(c.ContainerID), c.ContainerName, string(c.Status), c.InternalPort, c.Secret, c.CreatedAt, c.UpdatedAt)
	return err
}

func (p *Postgres) scanContainer(row interface{ Scan(...any) error }) (*model.Container, error) {
	var c model.Container
	var containerID sql.NullString
	var templateID sql.NullString
	err := row.Scan(&c.ID, &c.UserID, &templateID, &containerID, &c.ContainerName, (*string)(&c.Status), &c.InternalPort, &c.Secret, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.ContainerID = containerID.String
	c.TemplateID = templateID.String
	return &c, nil
}

func (p *Postgres) GetContainerByUserID(ctx context.Context, userID string) (*model.Container, error) {
	return p.scanContainer(p.db.QueryRowContext(ctx, `
SELECT id, user_id, template_id, container_id, container_name, status, internal_port, secret, created_at, updated_at
FROM user_containers WHERE user_id=$1`, userID))
}

func (p *Postgres) GetContainerByID(ctx context.Context, id string) (*model.Container, error) {
	return p.scanContainer(p.db.QueryRowContext(ctx, `
SELECT id, user_id, template_id, container_id, container_name, status, internal_port, secret, created_at, updated_at
FROM user_containers WHERE id=$1`, id))
}

func (p *Postgres) ListContainers(ctx context.Context) ([]*model.Container, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT id, user_id, template_id, container_id, container_name, status, internal_port, secret, created_at, updated_at
FROM user_containers ORDER BY container_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Container
	for rows.Next() {
		c, err := p.scanContainer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Postgres) ListContainersByUserIDs(ctx context.Context, userIDs []string) ([]*model.Container, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := p.db.QueryContext(ctx, `
SELECT id, user_id, template_id, container_id, container_name, status, internal_port, secret, created_at, updated_at
FROM user_containers WHERE user_id = ANY($1) ORDER BY container_name`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Container
	for rows.Next() {
		c, err := p.scanContainer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateContainer(ctx context.Context, c *model.Container) error {
	res, err := p.db.ExecContext(ctx, `
UPDATE user_containers SET template_id=$3, container_id=$4, container_name=$5, status=$6, internal_port=$7, secret=$8, updated_at=now()
WHERE user_id=$1 AND id=$2`,
		c.UserID, c.ID, nullString(c.TemplateID), nullString(c.ContainerID), c.ContainerName, string(c.Status), c.InternalPort, c.Secret)
	if err != nil {
		return err
	}
	return checkRows(res)
}

func (p *Postgres) DeleteContainerByUserID(ctx context.Context, userID string) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM user_containers WHERE user_id=$1`, userID)
	if err != nil {
		return err
	}
	return checkRows(res)
}

func (p *Postgres) CreateTemplate(ctx context.Context, t *model.Template) error {
	envs, err := json.Marshal(t.Envs)
	if err != nil {
		return err
	}
	extra, err := json.Marshal(t.ExtraPorts)
	if err != nil {
		return err
	}
	caps, err := json.Marshal(t.CapAdd)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `
INSERT INTO image_templates (id, name, image, internal_port, extra_ports, envs, cpu_limit, mem_limit, healthcheck_cmd, workspace_dir, command, run_user, cap_add, is_system, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		t.ID, t.Name, t.Image, t.InternalPort, extra, envs, t.CPULimit, t.MemLimit, nullString(t.Healthcheck), t.WorkspaceDir, mustJSON(t.Command), t.RunUser, caps, t.IsSystem, t.CreatedAt)
	return err
}

func (p *Postgres) scanTemplate(row interface{ Scan(...any) error }) (*model.Template, error) {
	var t model.Template
	var health sql.NullString
	var envs, cmd, extra, caps []byte
	err := row.Scan(&t.ID, &t.Name, &t.Image, &t.InternalPort, &extra, &envs, &t.CPULimit, &t.MemLimit, &health, &t.WorkspaceDir, &cmd, &t.RunUser, &caps, &t.IsSystem, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Healthcheck = health.String
	if len(extra) > 0 {
		json.Unmarshal(extra, &t.ExtraPorts)
	}
	if t.ExtraPorts == nil {
		t.ExtraPorts = []int{}
	}
	if len(envs) > 0 {
		json.Unmarshal(envs, &t.Envs)
	}
	if t.Envs == nil {
		t.Envs = map[string]string{}
	}
	if len(cmd) > 0 {
		json.Unmarshal(cmd, &t.Command)
	}
	if len(caps) > 0 {
		json.Unmarshal(caps, &t.CapAdd)
	}
	return &t, nil
}

func (p *Postgres) GetTemplate(ctx context.Context, id string) (*model.Template, error) {
	return p.scanTemplate(p.db.QueryRowContext(ctx, `
SELECT id, name, image, internal_port, extra_ports, envs, cpu_limit, mem_limit, healthcheck_cmd, workspace_dir, command, run_user, cap_add, is_system, created_at
FROM image_templates WHERE id=$1`, id))
}

func (p *Postgres) GetTemplateByName(ctx context.Context, name string) (*model.Template, error) {
	return p.scanTemplate(p.db.QueryRowContext(ctx, `
SELECT id, name, image, internal_port, extra_ports, envs, cpu_limit, mem_limit, healthcheck_cmd, workspace_dir, command, run_user, cap_add, is_system, created_at
FROM image_templates WHERE name=$1`, name))
}

func (p *Postgres) ListTemplates(ctx context.Context) ([]*model.Template, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT id, name, image, internal_port, extra_ports, envs, cpu_limit, mem_limit, healthcheck_cmd, workspace_dir, command, run_user, cap_add, is_system, created_at
FROM image_templates ORDER BY is_system DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Template
	for rows.Next() {
		t, err := p.scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateTemplate(ctx context.Context, t *model.Template) error {
	envs, err := json.Marshal(t.Envs)
	if err != nil {
		return err
	}
	extra, err := json.Marshal(t.ExtraPorts)
	if err != nil {
		return err
	}
	caps, err := json.Marshal(t.CapAdd)
	if err != nil {
		return err
	}
	res, err := p.db.ExecContext(ctx, `
UPDATE image_templates SET image=$3, internal_port=$4, extra_ports=$5, envs=$6, cpu_limit=$7, mem_limit=$8, healthcheck_cmd=$9, workspace_dir=$10, command=$11, run_user=$12, cap_add=$13
WHERE id=$1 AND name=$2`,
		t.ID, t.Name, t.Image, t.InternalPort, extra, envs, t.CPULimit, t.MemLimit, nullString(t.Healthcheck), t.WorkspaceDir, mustJSON(t.Command), t.RunUser, caps)
	if err != nil {
		return err
	}
	return checkRows(res)
}

func (p *Postgres) DeleteTemplate(ctx context.Context, id string) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM image_templates WHERE id=$1`, id)
	if err != nil {
		return err
	}
	return checkRows(res)
}

func (p *Postgres) LogAccess(ctx context.Context, l *model.AccessLog) error {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO access_logs (user_id, path, status, bytes, latency_ms) VALUES ($1,$2,$3,$4,$5)`,
		l.UserID, l.Path, l.Status, l.Bytes, l.LatencyMS)
	return err
}

func (p *Postgres) ListAccessLogs(ctx context.Context, limit int) ([]*model.AccessLog, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT id, user_id, path, status, bytes, latency_ms, ts FROM access_logs ORDER BY ts DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.AccessLog
	for rows.Next() {
		var l model.AccessLog
		var uid sql.NullString
		if err := rows.Scan(&l.ID, &uid, &l.Path, &l.Status, &l.Bytes, &l.LatencyMS, &l.Timestamp); err != nil {
			return nil, err
		}
		l.UserID = uid.String
		out = append(out, &l)
	}
	return out, rows.Err()
}

func (p *Postgres) StatsContainersByStatus(ctx context.Context) (map[model.ContainerStatus]int64, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT status, count(*) FROM user_containers GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[model.ContainerStatus]int64{}
	for rows.Next() {
		var s string
		var n int64
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[model.ContainerStatus(s)] = n
	}
	return out, rows.Err()
}

func (p *Postgres) LastAccess(ctx context.Context, userID string) (*time.Time, error) {
	var t sql.NullTime
	err := p.db.QueryRowContext(ctx, `SELECT MAX(ts) FROM access_logs WHERE user_id=$1`, userID).Scan(&t)
	if err != nil {
		return nil, err
	}
	// MAX(ts) is NULL when the user has no access logs: report (nil, nil) so
	// callers like IdleStop can treat "never accessed" as "idle".
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}

func (p *Postgres) AccessLogsSummary(ctx context.Context, since time.Time, onlineWindow time.Duration) (*AccessLogsSummary, error) {
	var s AccessLogsSummary
	err := p.db.QueryRowContext(ctx, `
SELECT
  count(*),
  COALESCE(sum(bytes), 0),
  COALESCE(sum(latency_ms), 0),
  max(ts),
  count(DISTINCT user_id) FILTER (WHERE ts >= $2 AND user_id IS NOT NULL AND user_id <> '')
FROM access_logs WHERE ts >= $1`, since, time.Now().Add(-onlineWindow)).Scan(
		&s.Count, &s.Bytes, &s.LatencySum, &s.Last, &s.Online)
	if err != nil {
		return nil, err
	}
	rows, err := p.db.QueryContext(ctx, `
SELECT extract(hour FROM (now() - ts))::int AS hours_ago, count(*)
FROM access_logs
WHERE ts >= $1
GROUP BY 1`, time.Now().Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hoursAgo int
		var n int64
		if err := rows.Scan(&hoursAgo, &n); err != nil {
			return nil, err
		}
		idx := 23 - hoursAgo
		if idx >= 0 && idx < 24 {
			s.Last24H[idx] = n
		}
	}
	return &s, rows.Err()
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func checkRows(res interface{ RowsAffected() (int64, error) }) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
