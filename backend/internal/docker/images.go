package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/image"
)

// ListImages returns all images on the Docker host.
func (c *Client) ListImages(ctx context.Context) ([]image.Summary, error) {
	return c.cli.ImageList(ctx, image.ListOptions{All: true})
}

// LoadImage imports a Docker image from a tar archive (docker save output).
// The caller must close the returned ReadCloser after draining it.
func (c *Client) LoadImage(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	resp, err := c.cli.ImageLoad(ctx, r)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// RemoveImage deletes a local Docker image by ID or reference.
func (c *Client) RemoveImage(ctx context.Context, id string) ([]image.DeleteResponse, error) {
	return c.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: true})
}

// InspectImage returns detailed information about an image.
func (c *Client) InspectImage(ctx context.Context, id string) (image.InspectResponse, error) {
	return c.cli.ImageInspect(ctx, id)
}

// PullImage pulls an image from a registry and blocks until complete.
func (c *Client) PullImage(ctx context.Context, name string) error {
	rc, err := c.cli.ImagePull(ctx, name, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}
