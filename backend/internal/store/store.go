package store

import (
	"context"
	"time"

	"devcapsule/backend/internal/model"
)

type AccessLogsSummary struct {
	Count       int64
	Bytes       int64
	LatencySum  int64
	Last        *time.Time
	Online      int64
	Last24H     [24]int64
}

type Store interface {
	Close() error
	Migrate(ctx context.Context) error

	CreateUser(ctx context.Context, u *model.User) error
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	ListUsers(ctx context.Context) ([]*model.User, error)
	ListUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error)
	UpdateUser(ctx context.Context, u *model.User) error
	DeleteUser(ctx context.Context, id string) error
	CountUsers(ctx context.Context) (int64, error)

	CreateContainer(ctx context.Context, c *model.Container) error
	GetContainerByUserID(ctx context.Context, userID string) (*model.Container, error)
	GetContainerByID(ctx context.Context, id string) (*model.Container, error)
	ListContainers(ctx context.Context) ([]*model.Container, error)
	ListContainersByUserIDs(ctx context.Context, userIDs []string) ([]*model.Container, error)
	UpdateContainer(ctx context.Context, c *model.Container) error
	DeleteContainerByUserID(ctx context.Context, userID string) error

	CreateTemplate(ctx context.Context, t *model.Template) error
	GetTemplate(ctx context.Context, id string) (*model.Template, error)
	GetTemplateByName(ctx context.Context, name string) (*model.Template, error)
	ListTemplates(ctx context.Context) ([]*model.Template, error)
	UpdateTemplate(ctx context.Context, t *model.Template) error
	DeleteTemplate(ctx context.Context, id string) error

	LogAccess(ctx context.Context, l *model.AccessLog) error
	ListAccessLogs(ctx context.Context, limit int) ([]*model.AccessLog, error)
	StatsContainersByStatus(ctx context.Context) (map[model.ContainerStatus]int64, error)
	LastAccess(ctx context.Context, userID string) (*time.Time, error)
	AccessLogsSummary(ctx context.Context, since time.Time, onlineWindow time.Duration) (*AccessLogsSummary, error)
}
