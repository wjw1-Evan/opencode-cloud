package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusDisabled UserStatus = "disabled"
	StatusExpired  UserStatus = "expired"
)

type ContainerStatus string

const (
	ContainerPending  ContainerStatus = "pending"
	ContainerCreating ContainerStatus = "creating"
	ContainerRunning  ContainerStatus = "running"
	ContainerStopped  ContainerStatus = "stopped"
	ContainerError    ContainerStatus = "error"
	ContainerRemoved  ContainerStatus = "removed"
)

type User struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	PasswordHash  string     `json:"-"`
	PasswordPlain string     `json:"password,omitempty"`
	Role          Role       `json:"role"`
	Status        UserStatus `json:"status"`
	Course        string     `json:"course"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CPULimit      float64    `json:"cpu_limit"`
	MemLimit      int64      `json:"mem_limit"`
	ContainerID   string     `json:"container_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Container struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	TemplateID    string          `json:"template_id"`
	ContainerID   string          `json:"container_id,omitempty"`
	ContainerName string          `json:"container_name"`
	Status        ContainerStatus `json:"status"`
	InternalPort  int             `json:"internal_port"`
	Secret        string          `json:"-"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Template struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	InternalPort int               `json:"internal_port"`
	ExtraPorts   []int             `json:"extra_ports"`
	Envs         map[string]string `json:"envs"`
	CPULimit     float64           `json:"cpu_limit"`
	MemLimit     int64             `json:"mem_limit"`
	Healthcheck  string            `json:"healthcheck_cmd,omitempty"`
	WorkspaceDir string            `json:"workspace_dir"`
	Command      []string          `json:"command,omitempty"`
	RunUser      string            `json:"run_user,omitempty"`
	CapAdd       []string          `json:"cap_add,omitempty"`
	IsSystem     bool              `json:"is_system"`
	CreatedAt    time.Time         `json:"created_at"`
}

// AllPorts returns the internal port plus all extra ports, de-duplicated.
func (t *Template) AllPorts() []int {
	seen := map[int]bool{t.InternalPort: true}
	out := []int{t.InternalPort}
	if t.InternalPort == 0 {
		out = nil
	}
	for _, p := range t.ExtraPorts {
		if p <= 0 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

type AccessLog struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Bytes     int64     `json:"bytes"`
	LatencyMS int64     `json:"latency_ms"`
	Timestamp time.Time `json:"timestamp"`
}

func NewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
