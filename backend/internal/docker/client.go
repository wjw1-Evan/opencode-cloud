package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Client struct {
	cli    *client.Client
	secret *SecretGen
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	if _, err := cli.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("docker unavailable: %w", err)
	}
	return &Client{cli: cli, secret: &SecretGen{}}, nil
}

func (c *Client) Close() error { return c.cli.Close() }

func (c *Client) Raw() *client.Client { return c.cli }

// Available reports whether the client is backed by a real docker daemon.
// A zero-value Client (tests) has no CLI and reports unavailable.
func (c *Client) Available() bool { return c != nil && c.cli != nil }

// EnsureNetwork creates the bridge network if it does not exist.
// Note: icc is intentionally left enabled - the proxy container must be able
// to reach student containers, and Docker cannot express per-container allow
// rules on a bridge network. Inter-user isolation relies on the per-container
// basic-auth secret injected by the proxy.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	_, err := c.cli.NetworkInspect(ctx, name, network.InspectOptions{})
	if err == nil {
		return nil
	}
	_, err = c.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
	})
	return err
}

// EnsureImage pulls the image if not present locally.
func (c *Client) EnsureImage(ctx context.Context, imageName string) error {
	_, err := c.cli.ImageInspect(ctx, imageName)
	if err == nil {
		return nil
	}
	rc, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}

type ContainerConfig struct {
	Name          string
	Image         string
	Network       string
	Env           map[string]string
	Cmd           []string
	Entrypoint    []string // empty slice clears the image entrypoint
	InternalPort  int
	CPULimit      float64
	MemLimitBytes int64
	PidsLimit     int64
	WorkDir       string
	Volumes       []string // "name:path" pairs
}

// CreateContainer builds and starts a container on the shared network.
// Returns the container ID and an internal secret used for upstream basic auth.
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (id string, err error) {
	if cfg.InternalPort <= 0 {
		cfg.InternalPort = 4096
	}
	exposed := nat.PortSet{
		nat.Port(fmt.Sprintf("%d/tcp", cfg.InternalPort)): struct{}{},
	}
	env := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}
	res := &container.Config{
		Image:        cfg.Image,
		Env:          env,
		ExposedPorts: exposed,
		WorkingDir:   cfg.WorkDir,
		Cmd:          cfg.Cmd,
		Entrypoint:   cfg.Entrypoint,
		Labels: map[string]string{
			"devcapsule": "managed",
		},
	}
	hostCfg := &container.HostConfig{
		NetworkMode: container.NetworkMode(cfg.Network),
		Resources: container.Resources{
			NanoCPUs:  int64(cfg.CPULimit * 1e9),
			Memory:    cfg.MemLimitBytes,
			PidsLimit: &cfg.PidsLimit,
		},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
		SecurityOpt: []string{"no-new-privileges"},
		CapDrop:     []string{"ALL"},
	}
	netCfg := &network.NetworkingConfig{}

	for _, v := range cfg.Volumes {
		hostCfg.Binds = append(hostCfg.Binds, v)
	}

	created, err := c.cli.ContainerCreate(ctx, res, hostCfg, netCfg, nil, cfg.Name)
	if err != nil {
		return "", err
	}
	if err := c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return created.ID, nil
}

// RemoveContainer stops and removes a container, keeping named volumes.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	_ = c.cli.ContainerStop(ctx, id, container.StopOptions{})
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: false})
}

func (c *Client) Start(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) Stop(ctx context.Context, id string) error {
	return c.cli.ContainerStop(ctx, id, container.StopOptions{})
}

func (c *Client) Restart(ctx context.Context, id string) error {
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{})
}

// InspectStatus returns the container state. empty string if not found.
func (c *Client) InspectStatus(ctx context.Context, id string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		if client.IsErrNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if info.State == nil {
		return "", nil
	}
	return string(info.State.Status), nil
}

// WaitHealthy polls the container until its HTTP endpoint responds or times out.
// Probing is done via docker exec inside the container, which works on every
// platform (host->container IP routing does not work on Docker Desktop/macOS).
func (c *Client) WaitHealthy(ctx context.Context, id string, port int, timeout time.Duration) error {
	probe := fmt.Sprintf(
		"wget -q -T 2 -O /dev/null http://127.0.0.1:%d/ >/dev/null 2>&1 || curl -fs -o /dev/null -m 2 http://127.0.0.1:%d/ >/dev/null 2>&1 || python3 -c \"import urllib.request;urllib.request.urlopen('http://127.0.0.1:%d/', timeout=2)\" >/dev/null 2>&1",
		port, port, port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		status, err := c.InspectStatus(ctx, id)
		if err != nil {
			return err
		}
		if status == "running" && port > 0 {
			if ok, _ := c.ProbeExec(ctx, id, []string{"/bin/sh", "-c", probe}); ok {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("container %s not healthy within %s", id, timeout)
}

// ProbeExec runs a command inside the container and reports its exit status.
func (c *Client) ProbeExec(ctx context.Context, id string, cmd []string) (bool, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return false, err
	}
	attach, err := c.cli.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return false, err
	}
	defer attach.Close()
	_, _ = io.Copy(io.Discard, attach.Reader)
	insp, err := c.cli.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return false, err
	}
	return insp.ExitCode == 0, nil
}

// ContainerIP returns the container's IP on the given network.
func (c *Client) ContainerIP(ctx context.Context, id, network string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	if info.NetworkSettings == nil || info.NetworkSettings.Networks == nil {
		return "", fmt.Errorf("no networks")
	}
	nw, ok := info.NetworkSettings.Networks[network]
	if !ok {
		return "", fmt.Errorf("not on network %s", network)
	}
	return nw.IPAddress, nil
}

// ProbeHTTP checks whether an HTTP endpoint responds on the given host:port.
// Works on Linux hosts; not usable on Docker Desktop (host->container IP).
func ProbeHTTP(ctx context.Context, host string, port int) (bool, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%d/", host, port), nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return true, nil
}

// ListManaged returns container IDs labelled as devcapsule managed.
func (c *Client) ListManaged(ctx context.Context) ([]types.Container, error) {
	return c.cli.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "devcapsule=managed"),
		),
	})
}

// Stats returns per-container CPU and memory usage.
func (c *Client) Stats(ctx context.Context, id string) (*ContainerStats, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	stats, err := c.cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()
	body, err := io.ReadAll(stats.Body)
	if err != nil {
		return nil, err
	}
	parsed := parseStatsJSON(body)
	parsed.MemLimit = info.HostConfig.Memory
	return parsed, nil
}

type SecretGen struct{}

// Next returns a random 24-char secret.
func (g *SecretGen) Next() (string, error) {
	const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 24)
	if _, err := cryptoRandRead(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}
