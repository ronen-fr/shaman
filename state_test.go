package main

import (
	"testing"
	"time"
)

func b(distro, version, arch, flavor, status string) Build {
	return Build{Distro: distro, DistroVersion: version, DistroArch: arch, Flavor: flavor, Status: status}
}

func TestEvaluate(t *testing.T) {
	now := time.Now()
	settle := 20 * time.Minute
	jammy := b("ubuntu", "22.04", "x86_64", "default", "completed")
	jammy.DistroCodename = "jammy"
	centos := b("centos", "9", "x86_64", "default", "completed")
	centosFailed := b("centos", "9", "x86_64", "default", "failed")
	centosRunning := b("centos", "9", "x86_64", "default", "started")

	tests := []struct {
		name       string
		builds     []Build
		expect     string
		quiet      time.Duration
		wantDone   bool
		wantFailed bool
	}{
		{"no builds", nil, "", time.Hour, false, false},
		{"still running", []Build{jammy, centosRunning}, "", time.Hour, false, false},
		{"finished but not settled", []Build{jammy, centos}, "", 5 * time.Minute, false, false},
		{"finished and settled", []Build{jammy, centos}, "", settle, true, false},
		{"failure reported", []Build{jammy, centosFailed}, "", settle, true, true},
		{"expect met skips settle", []Build{jammy, centos}, "ubuntu/jammy,centos/9/x86_64/default", 0, true, false},
		{"expect missing", []Build{jammy}, "ubuntu,centos", time.Hour, false, false},
		{"expect present but running", []Build{jammy, centosRunning}, "ubuntu,centos", time.Hour, false, false},
		{"expect wrong flavor", []Build{jammy}, "ubuntu/22.04/x86_64/debug", time.Hour, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := evaluate(tt.builds, parseExpect(tt.expect), now.Add(-tt.quiet), now, settle)
			if v.Done != tt.wantDone || (v.Done && v.Failed != tt.wantFailed) {
				t.Errorf("got %+v, want done=%v failed=%v", v, tt.wantDone, tt.wantFailed)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	prev := []Build{b("centos", "9", "x86_64", "default", "started")}
	cur := []Build{
		b("centos", "9", "x86_64", "default", "completed"),
		b("ubuntu", "22.04", "x86_64", "default", "queued"),
	}
	got := diff(prev, cur)
	want := []string{
		"centos/9/x86_64/default: started → completed",
		"ubuntu/22.04/x86_64/default: queued (new)",
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("diff = %q, want %q", got, want)
	}
	if d := diff(cur, cur); len(d) != 0 {
		t.Errorf("diff of identical = %q", d)
	}
}
