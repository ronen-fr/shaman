package main

import (
	"strings"
	"time"
)

// Expectation is one --expect entry: distro/version/arch/flavor, where version
// may also be a codename and trailing fields may be omitted (matching anything).
type Expectation []string

func parseExpect(s string) []Expectation {
	var out []Expectation
	for _, e := range strings.Split(s, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, strings.Split(e, "/"))
		}
	}
	return out
}

func (e Expectation) Matches(b Build) bool {
	fields := [][]string{
		{b.Distro},
		{b.DistroVersion, b.DistroCodename},
		{b.DistroArch},
		{b.Flavor},
	}
	if len(e) > len(fields) {
		return false
	}
	for i, want := range e {
		ok := false
		for _, have := range fields[i] {
			if have != "" && strings.EqualFold(have, want) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (e Expectation) String() string { return strings.Join(e, "/") }

// Verdict is the outcome of evaluating one snapshot of builds.
type Verdict struct {
	Done    bool
	Failed  bool          // at least one build failed (only meaningful if Done)
	Missing []Expectation // expected builds not yet registered
	Pending int           // builds not yet in a terminal state
	Wait    time.Duration // remaining settle time, if that is all that's left
}

// evaluate decides whether the sha1 is done. lastChange is when the build set
// last changed; settle is how long it must stay unchanged once all builds are
// terminal. If expectations are given and all are met, settle is skipped.
func evaluate(builds []Build, expect []Expectation, lastChange, now time.Time, settle time.Duration) Verdict {
	var v Verdict
	for _, b := range builds {
		if !b.Terminal() {
			v.Pending++
		}
		if b.Status == "failed" {
			v.Failed = true
		}
	}
	for _, e := range expect {
		found := false
		for _, b := range builds {
			if e.Matches(b) {
				found = true
				break
			}
		}
		if !found {
			v.Missing = append(v.Missing, e)
		}
	}
	if len(builds) == 0 || v.Pending > 0 || len(v.Missing) > 0 {
		return v
	}
	if len(expect) > 0 {
		v.Done = true
		return v
	}
	if quiet := now.Sub(lastChange); quiet < settle {
		v.Wait = settle - quiet
		return v
	}
	v.Done = true
	return v
}

// diff returns human-readable status transitions from prev to cur.
func diff(prev, cur []Build) []string {
	old := map[string]string{}
	for _, b := range prev {
		old[b.Key()] = b.Status
	}
	var lines []string
	for _, b := range cur {
		was, ok := old[b.Key()]
		switch {
		case !ok:
			lines = append(lines, b.Key()+": "+b.Status+" (new)")
		case was != b.Status:
			lines = append(lines, b.Key()+": "+was+" → "+b.Status)
		}
	}
	return lines
}

// fingerprint captures everything whose change should reset the settle timer.
func fingerprint(builds []Build) string {
	var sb strings.Builder
	for _, b := range builds {
		sb.WriteString(b.Key() + "=" + b.Status + "@" + b.Modified + ";")
	}
	return sb.String()
}
