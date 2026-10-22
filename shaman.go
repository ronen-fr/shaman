package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Build is one entry of /api/builds/<project>/<ref>/<sha1>/.
type Build struct {
	URL            string `json:"url"`
	LogURL         string `json:"log_url"`
	Ref            string `json:"ref"`
	SHA1           string `json:"sha1"`
	Flavor         string `json:"flavor"`
	Status         string `json:"status"`
	Modified       string `json:"modified"`
	Started        string `json:"started"`
	Completed      string `json:"completed"`
	Distro         string `json:"distro"`
	DistroVersion  string `json:"distro_version"`
	DistroCodename string `json:"distro_codename"`
	DistroArch     string `json:"distro_arch"`
	Extra          struct {
		JobName string `json:"job_name"`
	} `json:"extra"`
}

// Key identifies a build slot, e.g. "ubuntu/22.04/x86_64/default".
func (b Build) Key() string {
	return strings.Join([]string{b.Distro, b.DistroVersion, b.DistroArch, b.Flavor}, "/")
}

// Terminal reports whether the build will not change state any more.
func (b Build) Terminal() bool {
	return b.Status == "completed" || b.Status == "failed"
}

// MinGap is the minimum time between two requests to shaman.
const MinGap = 120 * time.Second

type Client struct {
	Base    string // e.g. https://shaman.ceph.com
	Project string
	HTTP    *http.Client
	Gap     time.Duration // minimum spacing between requests

	last time.Time // time of the previous request
}

func NewClient(base, project string) *Client {
	return &Client{
		Base:    strings.TrimRight(base, "/"),
		Project: project,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Gap:     MinGap,
	}
}

// throttle blocks until at least Gap has passed since the previous request.
func (c *Client) throttle(ctx context.Context) error {
	if !c.last.IsZero() {
		if wait := c.Gap - time.Since(c.last); wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	c.last = time.Now()
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

var errNotFound = fmt.Errorf("not found")

// Builds returns all builds shaman knows for ref@sha1. A sha1 with no builds
// yet yields an empty slice, not an error.
func (c *Client) Builds(ctx context.Context, ref, sha1 string) ([]Build, error) {
	var builds []Build
	path := fmt.Sprintf("/api/builds/%s/%s/%s/", url.PathEscape(c.Project), url.PathEscape(ref), url.PathEscape(sha1))
	err := c.getJSON(ctx, path, &builds)
	if err == errNotFound {
		return nil, nil
	}
	return builds, err
}
