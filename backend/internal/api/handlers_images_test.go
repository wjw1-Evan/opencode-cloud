package api

import (
	"testing"
	"time"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
)

func TestImageRefMatches(t *testing.T) {
	tags := []string{"ghcr.io/anomalyco/opencode:latest", "nginx:1.27", "python:3.12-alpine"}
	cases := []struct {
		ref  string
		want bool
	}{
		{"ghcr.io/anomalyco/opencode:latest", true},
		{"ghcr.io/anomalyco/opencode", true}, // bare name -> :latest
		{"nginx:1.27", true},
		{"nginx", false}, // bare name maps to nginx:latest, which is not present
		{"nginx:latest", false},
		{"python:3.12-alpine", true},
		{"", false},
		{"  ghcr.io/anomalyco/opencode:latest  ", true},
	}
	for _, c := range cases {
		if got := imageRefMatches(tags, c.ref); got != c.want {
			t.Errorf("imageRefMatches(%q) = %v, want %v", c.ref, got, c.want)
		}
	}
}

func TestFormatHealthcheck(t *testing.T) {
	if got := formatHealthcheck(nil); got != "" {
		t.Fatalf("nil healthcheck should format to empty, got %q", got)
	}
	hc := &dockerspec.HealthcheckConfig{
		Test:     []string{"CMD-SHELL", "curl -f http://localhost/"},
		Interval: 5 * time.Second,
		Timeout:  3 * time.Second,
		Retries:  3,
	}
	want := "CMD-SHELL curl -f http://localhost/ (interval=5s timeout=3s retries=3)"
	if got := formatHealthcheck(hc); got != want {
		t.Fatalf("formatHealthcheck = %q, want %q", got, want)
	}
}
