package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

const fixtureSHA1 = "1957f7493291472bc8a5890f9b3407d44fbaa544"

func fixtureServer(t *testing.T) *httptest.Server {
	data, err := os.ReadFile("testdata/builds.json")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/builds/ceph/main/"+fixtureSHA1+"/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestBuildsFixture(t *testing.T) {
	c := NewClient(fixtureServer(t).URL, "ceph")
	c.Gap = 0
	builds, err := c.Builds(context.Background(), "main", fixtureSHA1)
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 11 {
		t.Fatalf("got %d builds, want 11", len(builds))
	}
	v := evaluate(builds, 0, time.Time{}, time.Now(), time.Minute)
	if !v.Done || !v.Failed {
		t.Errorf("fixture verdict = %+v, want done and failed", v)
	}
	v = evaluate(builds, 11, time.Now(), time.Now(), time.Hour)
	if !v.Done {
		t.Errorf("count should be met: %+v", v)
	}
	v = evaluate(builds, 12, time.Now(), time.Now(), time.Hour)
	if v.Done || v.Missing != 1 {
		t.Errorf("count should not be met: %+v", v)
	}
}

func TestBuildsUnknownSHA1(t *testing.T) {
	c := NewClient(fixtureServer(t).URL, "ceph")
	c.Gap = 0
	builds, err := c.Builds(context.Background(), "main", "deadbeef")
	if err != nil || len(builds) != 0 {
		t.Errorf("got %v, %v; want no builds and no error", builds, err)
	}
}

func TestThrottle(t *testing.T) {
	c := NewClient(fixtureServer(t).URL, "ceph")
	c.Gap = 300 * time.Millisecond
	start := time.Now()
	for range 3 {
		if _, err := c.Builds(context.Background(), "main", fixtureSHA1); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 2*c.Gap {
		t.Errorf("3 requests took %s, want at least %s", elapsed, 2*c.Gap)
	}

	// A cancelled context must not wait out the gap.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Builds(ctx, "main", fixtureSHA1); err == nil {
		t.Error("expected error from cancelled context")
	}
}
