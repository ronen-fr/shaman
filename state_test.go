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
	rocky := b("rocky", "10", "x86_64", "default", "completed")
	rockyRunning := b("rocky", "10", "x86_64", "default", "started")

	tests := []struct {
		name       string
		builds     []Build
		count      int
		quiet      time.Duration
		wantDone   bool
		wantFailed bool
	}{
		{"no builds", nil, 0, time.Hour, false, false},
		{"still running", []Build{jammy, centosRunning}, 0, time.Hour, false, false},
		{"finished but not settled", []Build{jammy, centos}, 0, 5 * time.Minute, false, false},
		{"finished and settled", []Build{jammy, centos}, 0, settle, true, false},
		{"failure reported", []Build{jammy, centosFailed}, 0, settle, true, true},
		{"count met skips settle", []Build{jammy, centos}, 2, 0, true, false},
		{"count exceeded skips settle", []Build{jammy, centos}, 1, 0, true, false},
		{"count not reached", []Build{jammy}, 2, time.Hour, false, false},
		{"count reached but running", []Build{jammy, centosRunning}, 2, time.Hour, false, false},
		{"count overrides 3 completed", []Build{jammy, centos, rocky}, 4, time.Hour, false, false},
		{"3 completed skips settle", []Build{jammy, centos, rocky}, 0, 0, true, false},
		{"3 completed waits for running", []Build{jammy, centos, rocky, centosRunning}, 0, 0, false, false},
		{"3 completed plus failure", []Build{jammy, centos, rocky, centosFailed}, 0, 0, true, true},
		{"failures don't count as completed", []Build{jammy, centos, centosFailed}, 0, 5 * time.Minute, false, false},
		{"2 completed still settles", []Build{jammy, centos, rockyRunning}, 0, time.Hour, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := evaluate(tt.builds, tt.count, now.Add(-tt.quiet), now, settle)
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
