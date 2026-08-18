package api

import (
	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/model"
)

// View types keep docker/internal fields out of API responses.
type UserView struct {
	Username string `json:"username"`
	Status   string `json:"status"`
}

type ContainerView struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	TemplateID    string `json:"template_id"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Status        string `json:"status"`
	StartedAt     string `json:"started_at"`
	Network       string `json:"network"`
	ExpectedNet   string `json:"expected_network"`
	InternalPort  int    `json:"internal_port"`
	ExtraPorts    []int  `json:"extra_ports"`
	Username      string `json:"username"`
	UserStatus    string `json:"user_status"`
	CreatedAt     string `json:"created_at"`
}

type StatsView struct {
	CPUCores float64 `json:"cpu_cores"`
	MemBytes float64 `json:"mem_bytes"`
	MemLimit int64   `json:"mem_limit"`
}

const (
	StatusRunning = model.ContainerRunning
)

func toContainerView(c *model.Container) *ContainerView {
	return &ContainerView{
		ID:            c.ID,
		UserID:        c.UserID,
		TemplateID:    c.TemplateID,
		ContainerID:   c.ContainerID,
		ContainerName: c.ContainerName,
		Status:        string(c.Status),
		InternalPort:  c.InternalPort,
		CreatedAt:     c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func countOK(results []docker.BatchResult) int {
	n := 0
	for _, r := range results {
		if r.OK {
			n++
		}
	}
	return n
}
